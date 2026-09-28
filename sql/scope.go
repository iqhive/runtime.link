package sql

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"runtime.link/sql/std/sodium"
	"runtime.link/xyz"
)

const (
	// ErrUnauthorized is returned when a [Map] with scoped columns is used with a
	// context that holds no access to one of its scopes. See [Scope].
	ErrUnauthorized = errorString("no access to scope")
	// ErrAccessDenied is returned when a write would store a value that lies
	// outside of the scopes held by the context, or would overwrite one.
	ErrAccessDenied = errorString("value is out of scope")
)

// Scope restricts the context to rows whose column tagged `sql:"name,scoped"`
// holds one of the given values. Every [Map] operation made with the context
// is filtered this way: searches, counts, updates and deletes only see rows
// inside the scope, and inserts outside of it fail with [ErrAccessDenied].
//
// A [Map] with a scoped column cannot be used at all with a context that has
// no access to that scope, it fails with [ErrUnauthorized]. Access comes from
// Scope, from [Widen] or from [Admin].
//
// Scope only ever narrows: scoping a context that is already scoped keeps
// the values common to both. Scoping with no values matches nothing. T must
// be the Go type of the scoped column.
func Scope[T comparable](ctx context.Context, scope string, values ...T) context.Context {
	prev := scopingOf(ctx)
	next := prev.clone()
	g := grant{rtype: reflect.TypeFor[T]()}
	for _, value := range values {
		g.values = append(g.values, value)
	}
	if old, ok := prev.scopes[scope]; ok && !old.all {
		g.values = slices.DeleteFunc(g.values, func(value any) bool {
			return !slices.Contains(old.values, value)
		})
	}
	next.scopes[scope] = g
	return context.WithValue(ctx, scopeKey{}, next)
}

// Widen gives the context access to every value of the given scope, replacing
// any narrower [Scope] held for it. Unlike [Scope], Widen increases access, so
// it belongs where access is decided, not where it is used. As with [Scope], T
// must be the Go type of the scoped column.
func Widen[T comparable](ctx context.Context, scope string) context.Context {
	next := scopingOf(ctx).clone()
	next.scopes[scope] = grant{all: true, rtype: reflect.TypeFor[T]()}
	return context.WithValue(ctx, scopeKey{}, next)
}

// Admin gives the context access to every value of every scope, for work that
// is not done on behalf of anyone in particular, such as a scheduled job. The
// reason is required and says why the work needs unscoped access. A [Scope]
// applied afterwards still narrows the context.
func Admin(ctx context.Context, reason string) context.Context {
	if reason == "" {
		panic("sql.Admin requires a reason")
	}
	next := scopingOf(ctx).clone()
	next.admin = reason
	return context.WithValue(ctx, scopeKey{}, next)
}

type scopeKey struct{}

// scoping is the access held by a context, it is never modified once stored.
type scoping struct {
	admin  string
	scopes map[string]grant
}

func scopingOf(ctx context.Context) scoping {
	s, _ := ctx.Value(scopeKey{}).(scoping)
	return s
}

func (s scoping) clone() scoping {
	scopes := make(map[string]grant, len(s.scopes)+1)
	for name, g := range s.scopes {
		scopes[name] = g
	}
	s.scopes = scopes
	return s
}

// grant is the access held to a single scope.
type grant struct {
	all    bool
	rtype  reflect.Type
	values []any
}

// scopeOf returns the scope of a field tagged with the 'scoped' option, as in
// `sql:"name,scoped"`. The scope is named after the column, without the prefix
// of any struct it is nested in, so that tables share a scope by sharing the
// name of the column.
func scopeOf(field reflect.StructField) (string, bool) {
	name, options, _ := strings.Cut(field.Tag.Get("sql"), ",")
	if !slices.Contains(strings.Split(options, ","), "scoped") {
		return "", false
	}
	if name == "" {
		name = strings.ToLower(field.Name)
		if tag := field.Tag.Get("txt"); tag != "" {
			name = tag
		}
	}
	return name, true
}

// scopedField is a field of a [Map] value tagged `sql:"name,scoped"`.
type scopedField struct {
	scope string
	index []int
	rtype reflect.Type
}

type scopedFields struct {
	fields []scopedField
	err    error
}

var scopedFieldsCache sync.Map // reflect.Type -> scopedFields

func scopedFieldsOf(rtype reflect.Type) scopedFields {
	if cached, ok := scopedFieldsCache.Load(rtype); ok {
		return cached.(scopedFields)
	}
	var result scopedFields
	var walk func(rtype reflect.Type, index []int)
	walk = func(rtype reflect.Type, index []int) {
		for i := range rtype.NumField() {
			field := rtype.Field(i)
			if !field.IsExported() {
				continue
			}
			path := append(slices.Clip(index), i)
			if scope, ok := scopeOf(field); ok {
				switch field.Type.Kind() {
				case reflect.String, reflect.Bool,
					reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				default:
					result.err = fmt.Errorf("scoped field %v must be a string, bool or integer, not %v", field.Name, field.Type)
					return
				}
				result.fields = append(result.fields, scopedField{scope: scope, index: path, rtype: field.Type})
				continue
			}
			if field.Type.Kind() == reflect.Struct {
				walk(field.Type, path)
			}
		}
	}
	if rtype.Kind() == reflect.Struct {
		walk(rtype, nil)
	}
	scopedFieldsCache.Store(rtype, result)
	return result
}

// scoper applies the access held by a context to a [Map] value type.
type scoper[V any] struct {
	fields []scopedField
	grants []grant
	empty  bool // at least one scope matches nothing.
}

// scopeFor returns the scoper to apply for V, or nil if V is unscoped or the
// context has access to every value of its scopes.
func scopeFor[V any](ctx context.Context) (*scoper[V], error) {
	scoped := scopedFieldsOf(reflect.TypeFor[V]())
	if scoped.err != nil {
		return nil, scoped.err
	}
	if len(scoped.fields) == 0 {
		return nil, nil
	}
	access := scopingOf(ctx)
	var s scoper[V]
	for _, field := range scoped.fields {
		g, ok := access.scopes[field.scope]
		if !ok {
			if access.admin != "" {
				continue
			}
			return nil, fmt.Errorf("%w %q", ErrUnauthorized, field.scope)
		}
		if g.rtype != field.rtype {
			return nil, fmt.Errorf("scope %q is held as %v, but the column is %v", field.scope, g.rtype, field.rtype)
		}
		if g.all {
			continue
		}
		if len(g.values) == 0 {
			s.empty = true
		}
		s.fields = append(s.fields, field)
		s.grants = append(s.grants, g)
	}
	if len(s.fields) == 0 {
		return nil, nil
	}
	return &s, nil
}

// expressions that match the values inside the scope. v is either a sentinal,
// in which case these are column expressions for a [Database], or a real value,
// in which case they are evaluated immediately.
func (s *scoper[V]) expressions(v *V) []sodium.Expression {
	var exprs []sodium.Expression
	for i, field := range s.fields {
		values := s.grants[i].values
		rvalue := reflect.ValueOf(v).Elem().FieldByIndex(field.index)
		columns, ok := columnOf(rvalue.Addr().Interface())
		if !ok {
			exprs = append(exprs, sodium.Expressions.Value.As(slices.Contains(values, rvalue.Interface())))
			continue
		}
		var cases []sodium.Expression
		for _, value := range values {
			cases = append(cases, sodium.Expressions.Index.As(
				xyz.NewPair(columns[0], normalise(reflect.ValueOf(value))),
			))
		}
		exprs = append(exprs, sodium.Expressions.Cases.As(cases))
	}
	return exprs
}

// admits reports whether the value lies inside the scope.
func (s *scoper[V]) admits(v *V) bool {
	for i, field := range s.fields {
		rvalue := reflect.ValueOf(v).Elem().FieldByIndex(field.index)
		if !slices.Contains(s.grants[i].values, rvalue.Interface()) {
			return false
		}
	}
	return true
}

// admitsPatch reports whether every scoped column set by the patch is set to
// a value inside the scope. v must be the sentinal the patch was built from.
func (s *scoper[V]) admitsPatch(v *V, patch Patch) bool {
	var check func(mods []sodium.Modification) bool
	check = func(mods []sodium.Modification) bool {
		for _, mod := range mods {
			switch xyz.ValueOf(mod) {
			case sodium.Modifications.Arr:
				if !check(sodium.Modifications.Arr.Get(mod)) {
					return false
				}
			case sodium.Modifications.Set:
				column, value := sodium.Modifications.Set.Get(mod).Split()
				for i, field := range s.fields {
					columns, ok := columnOf(reflect.ValueOf(v).Elem().FieldByIndex(field.index).Addr().Interface())
					if !ok || columns[0].Name != column.Name {
						continue
					}
					decoded := reflect.New(field.rtype)
					if _, err := decode(decoded, []sodium.Value{value}); err != nil {
						return false
					}
					if !slices.Contains(s.grants[i].values, decoded.Elem().Interface()) {
						return false
					}
				}
			}
		}
		return true
	}
	return check(patch)
}

func scopedQuery[K comparable, V any](s *scoper[V], query QueryFunc[K, V]) QueryFunc[K, V] {
	return func(k *K, v *V) Query {
		var base Query
		if query != nil {
			base = query(k, v)
		}
		return append(slices.Clip(base), s.expressions(v)...)
	}
}

func scopedCheck[V any](s *scoper[V], check CheckFunc[V]) CheckFunc[V] {
	return func(v *V) Check {
		var base Check
		if check != nil {
			base = check(v)
		}
		return append(slices.Clip(base), s.expressions(v)...)
	}
}

// scopedPatch guards a patch applied directly to values (without a [Database]),
// reverting any value the patch moves out of scope and recording that it did.
func scopedPatch[V any](s *scoper[V], patch PatchFunc[V], violated *bool) PatchFunc[V] {
	return func(v *V) Patch {
		old := *v
		mods := patch(v)
		if !s.admits(v) {
			*v = old
			*violated = true
		}
		return mods
	}
}

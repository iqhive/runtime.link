package sql

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
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
// inside the scope, and inserts outside of it fail with [ErrAccessDenied]. The
// column can be part of the value of a [Map] or of a composite key, and a row
// whose key lies outside of the scope is as out of reach as any other.
//
// A [Map] with a scoped column cannot be used at all with a context that has
// no access to that scope, it fails with [ErrUnauthorized]. Access comes from
// Scope, from [Widen] or from [Admin]. Nor can a [Map] be used at all if a sql
// tag of its key or value has an option that is 'scoped' written otherwise, as
// with a space or a capital: every operation fails, with any context, rather
// than leave open a column that was meant to be scoped. The same goes for a
// field tagged 'scoped' that is not a column: one that is not exported, or that
// is inside of a field that is not, such as an embedded struct whose type is not
// exported, or one that is behind a pointer, a slice or a map. The values a
// field of an interface type holds are not looked at.
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
// applied afterwards still narrows the context, and so does a [Scope] that the
// context already holds: Admin only gives access to the scopes it has none of.
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
// name of the column. Other options are left alone, except for one that only
// differs from 'scoped' by a space or a capital, which is an error, because
// dropping it would drop the scope it was written to ask for.
func scopeOf(field reflect.StructField) (string, bool, error) {
	name, options, _ := strings.Cut(field.Tag.Get("sql"), ",")
	scoped := false
	for _, option := range strings.Split(options, ",") {
		switch {
		case option == "scoped":
			scoped = true
		case strings.EqualFold(strings.TrimSpace(option), "scoped"):
			return "", false, fmt.Errorf("sql tag of field %v has the option %q, which is not %q", field.Name, option, "scoped")
		}
	}
	if !scoped {
		return "", false, nil
	}
	if name == "" {
		name = strings.ToLower(field.Name)
		if tag := field.Tag.Get("txt"); tag != "" {
			name = tag
		}
	}
	return name, true, nil
}

// scopedField is a field of a [Map] key or value tagged `sql:"name,scoped"`.
type scopedField struct {
	scope string
	index []int // struct field or array element at each step
	rtype reflect.Type
	inKey bool // the index starts at the key of the Map, not at its value.
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
		if rtype.Kind() == reflect.Array {
			for i := 0; i < rtype.Len(); i++ {
				walk(rtype.Elem(), append(slices.Clip(index), i))
			}
			return
		}
		// A [Map] without a [Database] stores what is behind these, and no row
		// has one value there to hold a scope to.
		switch rtype.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map:
			if name, found := scopedTagIn(rtype, make(map[reflect.Type]bool)); found {
				result.err = fmt.Errorf("scoped field %v must be a column, not behind the %v %v", name, rtype.Kind(), rtype)
			}
			return
		}
		if rtype.Kind() != reflect.Struct {
			return
		}
		for i := range rtype.NumField() {
			field := rtype.Field(i)
			if !field.IsExported() {
				// Not a column, though a [Map] without a [Database] stores it,
				// so a scope asked for in here would be asked for in vain.
				if asksForScope(field) {
					result.err = fmt.Errorf("scoped field %v must be a column, which a field that is not exported is not", field.Name)
					return
				}
				if name, found := scopedTagIn(field.Type, make(map[reflect.Type]bool)); found {
					result.err = fmt.Errorf("scoped field %v must be a column, not inside of the field %v, which is not exported", name, field.Name)
					return
				}
				continue
			}
			path := append(slices.Clip(index), i)
			scope, ok, err := scopeOf(field)
			if err != nil {
				result.err = err
				return
			}
			if ok {
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
			walk(field.Type, path)
		}
	}
	walk(rtype, nil)
	scopedFieldsCache.Store(rtype, result)
	return result
}

// asksForScope reports whether the sql tag of a field asks for it to be scoped,
// which it also does if the option is one that [scopeOf] refuses.
func asksForScope(field reflect.StructField) bool {
	_, scoped, err := scopeOf(field)
	return scoped || err != nil
}

// scopedTagIn returns the name of a field that asks to be scoped anywhere inside
// of a type, exported or not and behind whatever leads to it. The types seen are
// kept, as a type can lead back to itself through a pointer.
func scopedTagIn(rtype reflect.Type, seen map[reflect.Type]bool) (string, bool) {
	if seen[rtype] {
		return "", false
	}
	seen[rtype] = true
	switch rtype.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return scopedTagIn(rtype.Elem(), seen)
	case reflect.Map:
		if name, found := scopedTagIn(rtype.Key(), seen); found {
			return name, true
		}
		return scopedTagIn(rtype.Elem(), seen)
	case reflect.Struct:
		for i := range rtype.NumField() {
			field := rtype.Field(i)
			if asksForScope(field) {
				return field.Name, true
			}
			if name, found := scopedTagIn(field.Type, seen); found {
				return name, true
			}
		}
	}
	return "", false
}

// value follows both struct fields and array elements in a scoped path.
func (field scopedField) value(value reflect.Value) reflect.Value {
	for _, i := range field.index {
		if value.Kind() == reflect.Array {
			value = value.Index(i)
		} else {
			value = value.Field(i)
		}
	}
	return value
}

// scoper applies the access held by a context to the key and value types of a
// [Map].
type scoper[K comparable, V any] struct {
	fields []scopedField
	grants []grant
	empty  bool // at least one scope matches nothing.
}

// scopeFor returns the scoper to apply for a [Map] of K to V, or nil if neither
// is scoped or the context has access to every value of their scopes.
func scopeFor[K comparable, V any](ctx context.Context) (*scoper[K, V], error) {
	index := scopedFieldsOf(reflect.TypeFor[K]())
	value := scopedFieldsOf(reflect.TypeFor[V]())
	if index.err != nil {
		return nil, index.err
	}
	if value.err != nil {
		return nil, value.err
	}
	if len(index.fields)+len(value.fields) == 0 {
		return nil, nil
	}
	access := scopingOf(ctx)
	var s scoper[K, V]
	for i, field := range slices.Concat(index.fields, value.fields) {
		field.inKey = i < len(index.fields)
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

// of returns the scoped field of the key or of the value, whichever it is in.
func (field scopedField) of(k, v reflect.Value) reflect.Value {
	if field.inKey {
		return field.value(k.Elem())
	}
	return field.value(v.Elem())
}

// expressions that match the rows inside the scope. A k or v that is a
// sentinal gives column expressions for a [Database], and one that is a real
// key or value is evaluated immediately.
func (s *scoper[K, V]) expressions(k *K, v *V) []sodium.Expression {
	var exprs []sodium.Expression
	for i, field := range s.fields {
		values := s.grants[i].values
		rvalue := field.of(reflect.ValueOf(k), reflect.ValueOf(v))
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

// admits reports whether a row with this key and value lies inside the scope.
func (s *scoper[K, V]) admits(k *K, v *V) bool {
	return s.admitsKey(k) && s.admitsValue(v)
}

// admitsKey reports whether the scope has room for any row at the key.
func (s *scoper[K, V]) admitsKey(k *K) bool {
	return s.holds(reflect.ValueOf(k).Elem(), true)
}

// admitsValue reports whether the value lies inside the scope, whatever its
// key, which is what a patch can move out of it.
func (s *scoper[K, V]) admitsValue(v *V) bool {
	return s.holds(reflect.ValueOf(v).Elem(), false)
}

func (s *scoper[K, V]) holds(root reflect.Value, isKey bool) bool {
	for i, field := range s.fields {
		if field.inKey != isKey {
			continue
		}
		if !slices.Contains(s.grants[i].values, field.value(root).Interface()) {
			return false
		}
	}
	return true
}

func scopedQuery[K comparable, V any](s *scoper[K, V], query QueryFunc[K, V]) QueryFunc[K, V] {
	return func(k *K, v *V) Query {
		var base Query
		if query != nil {
			base = query(k, v)
		}
		return append(slices.Clip(base), s.expressions(k, v)...)
	}
}

// scopedCheck adds the scope to the check of the row at a key that is known,
// so a key that lies outside of the scope fails the check whatever the value.
func scopedCheck[K comparable, V any](s *scoper[K, V], key K, check CheckFunc[V]) CheckFunc[V] {
	return func(v *V) Check {
		if !s.admits(&key, v) {
			return Check{sodium.Expressions.Value.As(false)}
		}
		if check == nil {
			return nil
		}
		checked := cloneRAMValue(*v)
		return check(&checked)
	}
}

// checkedPatch snapshots the modifications the driver will receive. The patch
// callback may reuse its slices on the next row. Resolve names against this
// table so the driver and the scope check always refer to the same column.
func (s *scoper[K, V]) checkedPatch(k *K, v *V, table sodium.Table, patch Patch) (Patch, bool) {
	var checked Patch
	for _, mod := range patch {
		switch xyz.ValueOf(mod) {
		case sodium.Modifications.Arr:
			nested, ok := s.checkedPatch(k, v, table, sodium.Modifications.Arr.Get(mod))
			if !ok {
				return nil, false
			}
			checked = append(checked, sodium.Modifications.Arr.As(nested))
		case sodium.Modifications.Set:
			column, value := sodium.Modifications.Set.Get(mod).Split()
			named := func(c sodium.Column) bool { return c.Name == column.Name }
			if i := slices.IndexFunc(table.Value, named); i >= 0 {
				column = table.Value[i]
			} else if slices.ContainsFunc(table.Index, named) {
				// A modification of a column of the index rewrites the key of
				// the row, which moves it: onto a key that may be another
				// row's, and one that no scope grants. It is refused.
				return nil, false
			} else {
				return nil, false
			}
			for i, field := range s.fields {
				address := field.of(reflect.ValueOf(k), reflect.ValueOf(v)).Addr().Interface()
				columns, ok := columnOf(address)
				if !ok || len(columns) != 1 {
					return nil, false
				}
				if columns[0].Name == column.Name && !slices.ContainsFunc(s.grants[i].values, func(grant any) bool {
					return reflect.DeepEqual(normalise(reflect.ValueOf(grant)), value)
				}) {
					return nil, false
				}
			}
			if xyz.ValueOf(value) == sodium.Values.Bytes {
				value = sodium.Values.Bytes.As(slices.Clone(sodium.Values.Bytes.Get(value)))
			}
			checked = append(checked, sodium.Modifications.Set.As(xyz.NewPair(column, value)))
		}
	}
	return checked, true
}

// scopedPatch guards a patch applied directly to values (without a [Database]),
// reverting any value the patch moves out of scope and recording that it did.
func scopedPatch[K comparable, V any](s *scoper[K, V], patch PatchFunc[V], violated *bool) PatchFunc[V] {
	return func(v *V) Patch {
		// Patches can change slices, maps and pointers in place. Preserve
		// their contents too, so a rejected patch leaves the row unchanged.
		old := cloneRAMValue(*v)
		mods := patch(v)
		if !s.admitsValue(v) {
			*v = old
			*violated = true
		}
		return mods
	}
}

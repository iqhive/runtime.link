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
	// outside of the scopes held by the context, or would change or delete a
	// row that the context can read but not write.
	ErrAccessDenied = errorString("value is out of scope")
	// ErrUpdateDenied is returned when a write is made to a [Map] with a
	// context that may read the rows of one of its scopes, but holds no
	// [Write] access to that scope at all, as a read-only context does.
	ErrUpdateDenied = errorString("no write access to scope")
)

<<<<<<< HEAD
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
// than leave open a column that was meant to be scoped.
=======
// access that a [Scope] or [Widen] grants to the rows of a scope.
type access bool

const (
	Reads access = false // means the rows can be searched, looked up and counted.
	Write access = true  // means the rows can be added, changed and deleted.
)

func (a access) String() string {
	if a == Write {
		return "write"
	}
	return "read"
}

// Scope restricts the given access of the context to rows whose scoped column
// of type T holds one of the given values. A column is scoped by tagging it
// with the 'scoped' option, as in `sql:"team,scoped"`, and its scope is its
// type, which must be a named string, bool or integer type, such as
// `type Team string`: every table with a scoped column of that type shares the
// scope, whatever the column is called.
//
// Every [Map] operation made with the context is filtered this way: searches
// and counts only see the rows inside the [Reads] scope, updates and deletes
// only change the rows inside both the [Reads] and the [Write] scope, and
// inserts outside of either fail with [ErrAccessDenied]. A row that can be
// read but not written is refused with [ErrAccessDenied] rather than skipped,
// as it can be seen.
//
// A [Map] with a scoped column cannot be used with a context that has no read
// access to that scope; it fails with [ErrUnauthorized]. Nor can it be written
// to without write access as well; it fails with [ErrUpdateDenied]. Access
// comes from Scope, from [Widen] or from [Admin], so a context granted only
// [Reads] is a read-only one.
>>>>>>> upstream-translated
//
// Scope only ever narrows: scoping a context that is already scoped keeps
// the values common to both. Scoping with no values matches nothing. Scope
// panics if T is not a named string, bool or integer type.
func Scope[T comparable](ctx context.Context, to access, values ...T) context.Context {
	key := scopeName{scopeType[T](), to}
	prev := scopingOf(ctx)
	next := prev.clone()
	var g grant
	for _, value := range values {
		g.values = append(g.values, value)
	}
	if old, ok := prev.scopes[key]; ok && !old.all {
		g.values = slices.DeleteFunc(g.values, func(value any) bool {
			return !slices.Contains(old.values, value)
		})
	}
	next.scopes[key] = g
	return context.WithValue(ctx, scopeKey{}, next)
}

// Widen gives the context the given access to every value of the scope of T,
// replacing any narrower [Scope] held for it. Unlike [Scope], Widen increases
// access, so it belongs where access is decided, not where it is used. As with
// [Scope], it panics if T is not a named string, bool or integer type.
func Widen[T comparable](ctx context.Context, to access) context.Context {
	next := scopingOf(ctx).clone()
	next.scopes[scopeName{scopeType[T](), to}] = grant{all: true}
	return context.WithValue(ctx, scopeKey{}, next)
}

// Admin gives the context access to every value of every scope, for work that
// is not done on behalf of anyone in particular, such as a scheduled job. The
<<<<<<< HEAD
// reason is required and says why the work needs unscoped access. A [Scope]
// applied afterwards still narrows the context, and so does a [Scope] that the
// context already holds: Admin only gives access to the scopes it has none of.
=======
// reason is required and says why the work needs unscoped access. Whatever
// access the context held before is set aside; a [Scope] applied afterwards
// still narrows it.
>>>>>>> upstream-translated
func Admin(ctx context.Context, reason string) context.Context {
	if reason == "" {
		panic("sql.Admin requires a reason")
	}
	return context.WithValue(ctx, scopeKey{}, scoping{admin: reason, scopes: make(map[scopeName]grant)})
}

type scopeKey struct{}

// scoping is the access held by a context, it is never modified once stored.
type scoping struct {
	admin  string
	scopes map[scopeName]grant
}

// scopeName is a scope, identified by the type of its columns, and the access
// granted to it.
type scopeName struct {
	rtype  reflect.Type
	access access
}

func scopingOf(ctx context.Context) scoping {
	s, _ := ctx.Value(scopeKey{}).(scoping)
	return s
}

func (s scoping) clone() scoping {
	scopes := make(map[scopeName]grant, len(s.scopes)+1)
	for name, g := range s.scopes {
		scopes[name] = g
	}
	s.scopes = scopes
	return s
}

// grant is the access held to a single scope.
type grant struct {
	all    bool
	values []any
}

<<<<<<< HEAD
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
=======
// scopeType returns the scope of T, panicking if it cannot be one.
func scopeType[T comparable]() reflect.Type {
	rtype := reflect.TypeFor[T]()
	if err := checkScopeType(rtype); err != nil {
		panic("sql: " + err.Error())
	}
	return rtype
}

// checkScopeType reports why rtype cannot be the type of a scope, if it
// cannot: only a named string, bool or integer type can, so that a scope is
// declared once, as a type, and a column can only be in it on purpose.
func checkScopeType(rtype reflect.Type) error {
	switch rtype.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return fmt.Errorf("a scope must be a string, bool or integer type, not %v", rtype)
	}
	if rtype.Name() == "" || rtype.PkgPath() == "" {
		return fmt.Errorf("a scope must be a named type, such as type Team %v, not %v", rtype.Kind(), rtype)
	}
	return nil
}

// isScoped reports whether the field is tagged with the 'scoped' option, as
// in `sql:"name,scoped"`.
func isScoped(field reflect.StructField) bool {
	_, options, _ := strings.Cut(field.Tag.Get("sql"), ",")
	return slices.Contains(strings.Split(options, ","), "scoped")
}

// scopedField is a field of a [Map] value tagged `sql:"name,scoped"`, whose
// type is its scope.
type scopedField struct {
	index []int
>>>>>>> upstream-translated
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
		if rtype.Kind() != reflect.Struct {
			return
		}
		for i := range rtype.NumField() {
			field := rtype.Field(i)
			if !field.IsExported() {
				continue
			}
			path := append(slices.Clip(index), i)
<<<<<<< HEAD
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
=======
			if isScoped(field) {
				if err := checkScopeType(field.Type); err != nil {
					result.err = fmt.Errorf("scoped field %v: %w", field.Name, err)
>>>>>>> upstream-translated
					return
				}
				result.fields = append(result.fields, scopedField{index: path, rtype: field.Type})
				continue
			}
			walk(field.Type, path)
		}
	}
	walk(rtype, nil)
	scopedFieldsCache.Store(rtype, result)
	return result
}

<<<<<<< HEAD
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
=======
// scoper applies the access held by a context to a [Map] value type. A field
// appears once for each grant that bounds it: a write is bounded by both the
// read and the write grant of each scope.
type scoper[V any] struct {
>>>>>>> upstream-translated
	fields []scopedField
	grants []grant
	access []access // the access each grant was held for.
	empty  bool     // at least one scope matches nothing.

	// narrower reports that a write is bounded by more than a read is, so a
	// row may be readable without being writable.
	narrower bool
}

<<<<<<< HEAD
// scopeFor returns the scoper to apply for a [Map] of K to V, or nil if neither
// is scoped or the context has access to every value of their scopes.
func scopeFor[K comparable, V any](ctx context.Context) (*scoper[K, V], error) {
	index := scopedFieldsOf(reflect.TypeFor[K]())
	value := scopedFieldsOf(reflect.TypeFor[V]())
	if index.err != nil {
		return nil, index.err
=======
// scopeFor returns the scoper to apply for V with the given access, or nil if
// V is unscoped or the context has that access to every value of its scopes.
func scopeFor[V any](ctx context.Context, want access) (*scoper[V], error) {
	scoped := scopedFieldsOf(reflect.TypeFor[V]())
	if scoped.err != nil {
		return nil, scoped.err
>>>>>>> upstream-translated
	}
	if value.err != nil {
		return nil, value.err
	}
	if len(index.fields)+len(value.fields) == 0 {
		return nil, nil
	}
<<<<<<< HEAD
	access := scopingOf(ctx)
	var s scoper[K, V]
	for i, field := range slices.Concat(index.fields, value.fields) {
		field.inKey = i < len(index.fields)
		g, ok := access.scopes[field.scope]
		if !ok {
			if access.admin != "" {
=======
	held := scopingOf(ctx)
	needs := []access{Reads}
	if want == Write {
		needs = append(needs, Write)
	}
	var s scoper[V]
	// Every read grant is checked before any write grant, so that a context
	// without read access to a scope is unauthorized rather than read-only.
	for _, need := range needs {
		for _, field := range scoped.fields {
			g, ok := held.scopes[scopeName{field.rtype, need}]
			if !ok {
				if held.admin != "" {
					continue
				}
				if need == Write {
					return nil, fmt.Errorf("%w %v", ErrUpdateDenied, field.rtype)
				}
				return nil, fmt.Errorf("%w %v", ErrUnauthorized, field.rtype)
			}
			if g.all {
>>>>>>> upstream-translated
				continue
			}
			if len(g.values) == 0 {
				s.empty = true
			}
			if need == Write {
				s.narrower = true
			}
			s.fields = append(s.fields, field)
			s.grants = append(s.grants, g)
			s.access = append(s.access, need)
		}
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
<<<<<<< HEAD
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
=======
	for i := range s.fields {
		exprs = append(exprs, s.expression(i, v))
>>>>>>> upstream-translated
	}
	return exprs
}

<<<<<<< HEAD
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
=======
// expression that matches the values inside the i'th grant, see [scoper.expressions].
func (s *scoper[V]) expression(i int, v *V) sodium.Expression {
	field, values := s.fields[i], s.grants[i].values
	rvalue := reflect.ValueOf(v).Elem().FieldByIndex(field.index)
	columns, ok := columnOf(rvalue.Addr().Interface())
	if !ok {
		return sodium.Expressions.Value.As(slices.Contains(values, rvalue.Interface()))
	}
	var cases []sodium.Expression
	for _, value := range values {
		cases = append(cases, sodium.Expressions.Index.As(
			xyz.NewPair(columns[0], normalise(reflect.ValueOf(value))),
		))
	}
	return sodium.Expressions.Cases.As(cases)
}

// unwritable returns an expression that matches the rows lying outside of the
// [Write] grants, see [scoper.expressions]. It reports false when there is no
// need for one, because a [Write] grant holds no values and so every row lies
// outside of it.
func (s *scoper[V]) unwritable(v *V) (sodium.Expression, bool) {
	var writes []sodium.Expression
	for i := range s.fields {
		if s.access[i] != Write {
			continue
		}
		if len(s.grants[i].values) == 0 {
			return sodium.Expression{}, false
		}
		writes = append(writes, s.expression(i, v))
	}
	return sodium.Expressions.Avoid.As(sodium.Expressions.Group.As(writes)), true
}

// admits reports whether the value lies inside the scope.
func (s *scoper[V]) admits(v *V) bool {
>>>>>>> upstream-translated
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
		var base Check
		if check != nil {
			base = check(v)
		}
		return append(slices.Clip(base), s.expressions(&key, v)...)
	}
}

// admitsPatch reports whether every scoped column set by the patch is set to
// a value inside the scope. v must be the sentinal the patch was built from.
func (s *scoper[K, V]) admitsPatch(v *V, patch Patch) bool {
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
					if field.inKey {
						continue
					}
					columns, ok := columnOf(field.value(reflect.ValueOf(v).Elem()).Addr().Interface())
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

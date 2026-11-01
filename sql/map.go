package sql

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// Database represents a connection to a SQL database.
type Database = sodium.Database

// Table name, may contain a slash to indicate the default
// primary key column. If no slash is present, the primary
// key column is 'id'.
type Table string

// Map represents a distinct mapping of data stored in a [Database].
type Map[K comparable, V any] struct {
	to sodium.Table
	db sodium.Database
	kv ram[K, V]

	index *K // sentinal passed to a [QueryFunc] that is run for a [Database].
	value *V // sentinal passed to a [QueryFunc] that is run for a [Database].
}

// fieldsOf describes the key and value of a [Map] as struct fields.
func fieldsOf[K comparable, V any](index string) (key, val reflect.StructField) {
	key = reflect.StructField{
		Name: index,
		Type: reflect.TypeOf([0]K{}).Elem(),
	}
	if key.Type.Kind() == reflect.Struct {
		key.Anonymous = true
	}
	val = reflect.StructField{
		Name: "value",
		Type: reflect.TypeOf([0]V{}).Elem(),
	}
	if val.Type.Kind() == reflect.Struct {
		val.Anonymous = true
	}
	return key, val
}

func (m *Map[K, V]) open(db Database, table Table) {
	name, index, ok := strings.Cut(string(table), "/")
	if !ok {
		index = "id"
	}
	key, val := fieldsOf[K, V](index)
	to := sodium.Table{
		Name:  name,
		Index: columnsOf(key),
		Value: columnsOf(val),
	}
	*m = Map[K, V]{
		to:    to,
		db:    db,
		index: sentinals.index.assert(name, key, new(K), to.Index).(*K),
		value: sentinals.value.assert(name, val, new(V), to.Value).(*V),
	}
}

// Open a database structure, by linking each [Map] inside the struct
// via the given [Database]. Each field should have a sql tag that
// will be interpreted as a [Table] in a call to [OpenTable].
//
// For each [Map] a 'sql' or else, a 'txt' tag controls
// the name of the column. If no tag is specified, the ToLower(name)
// of the field is used. If the key is not a struct, the column name
// is the [Table] default, otherwise it is treated as a composite key
// across each struct field. If the value is not a struct, the column
// name is 'value'.
//
// Nested structures are named with an underscore used to seperate
// the field path unless the structure is embedded, in which case
// the nested fields are promoted. Arrays elements are suffixed by
// their index.
func Open[T any](db Database) *T {
	if db == &stub {
		return new(T)
	}
	type opener interface {
		open(db Database, table Table)
	}
	var zero T
	rtype := reflect.TypeOf(zero)
	value := reflect.ValueOf(&zero).Elem()
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if table, ok := field.Tag.Lookup("sql"); ok {
			value.Field(i).Addr().Interface().(opener).open(db, Table(table))
		}
	}
	return &zero
}

// OpenTable a new [Map] from the given [Database] and specified
// table. Index and Value columns line up by position with the
// fields of K and V. If either is left empty, they are named as
// documented in [Open], with 'id' as the [Table] default.
func OpenTable[K comparable, V any](db Database, table sodium.Table) Map[K, V] {
	if db == &stub {
		db = nil
	}
	if db == nil {
		return Map[K, V]{to: table}
	}
	key, val := fieldsOf[K, V]("id")
	if len(table.Index) == 0 {
		table.Index = columnsOf(key)
	}
	if len(table.Value) == 0 {
		table.Value = columnsOf(val)
	}
	return Map[K, V]{
		to:    table,
		db:    db,
		index: sentinals.index.assert(table.Name, key, new(K), table.Index).(*K),
		value: sentinals.value.assert(table.Name, val, new(V), table.Value).(*V),
	}
}

// Add the specified value to the [Map], if possible, a key will be
// automatically selected. If a key is required in order to add to
// the [Map], an [ErrInsertOnly] error will be returned and nothing
// is added, though inside of a transaction it is up to the caller
// to roll back. If the K type implements Randomize, this method will
// be called on an zero value of K to generate a random key.
func (m *Map[K, V]) Add(ctx context.Context, value V) (K, error) {
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return [1]K{}[0], err
	}
	if scope != nil && !scope.admits(&value) {
		return [1]K{}[0], ErrAccessDenied
	}
	if m.db == nil {
		return m.kv.Add(ctx, value)
	}
	var key K
	if r, ok := any(&key).(interface {
		Randomize()
	}); ok {
		r.Randomize()
	}
	// Only the insert can tell whether a key is required, so it has to be
	// possible to take the insert back. Inside the caller's transaction, that
	// is left to the caller, who has the error.
	err = m.Manage(ctx, 0, func(ctx context.Context) error {
		key, err = m.add(ctx, key, value)
		return err
	})
	return key, err
}

func (m *Map[K, V]) add(ctx context.Context, key K, value V) (K, error) {
	tx, err := m.manage(ctx)
	if err != nil {
		return key, xray.New(err)
	}
	var values = ValuesOf(key)
	insert := m.db.Insert(m.to, values, bool(Create), nil, ValuesOf(value))
	select {
	case tx <- insert:
	case <-ctx.Done():
		return key, xray.New(ctx.Err())
	}
	n, err := insert.Wait(ctx)
	if err != nil {
		if err == ErrDuplicate {
			return key, err
		}
		return key, xray.New(err)
	}
	if n == -1 {
		return key, ErrDuplicate
	}
	if _, err := decode(reflect.ValueOf(&key), values); err != nil {
		return key, xray.New(err)
	}
	var zero K
	if key == zero {
		return key, ErrInsertOnly
	}
	return key, nil
}

// Get returns the value at the given key from the [Map]. If the key
// does not exist, a zero value is returned.
func (m *Map[K, V]) Get(ctx context.Context, key K) (V, error) {
	val, _, err := m.Lookup(ctx, key)
	return val, err
}

// Set the value at the given key in the [Map]. If the key does not
// exist, a new value is inserted. If the key does exist, the value
// is updated.
func (m *Map[K, V]) Set(ctx context.Context, key K, value V) error {
	return m.Insert(ctx, key, Upsert, value)
}

// Insert a new value into the [Map] at the given key. The given [Flag] determines
// how the value is intended to be inserted. If the [Flag] is [Upsert], the value
// will overwrite any existing value at the given key. If the [Flag] is [Create],
// the value will only be inserted if there is no existing value at the given key,
// otherwise an error will be returned if the existing value differs from the
// given value.
func (m *Map[K, V]) Insert(ctx context.Context, key K, flag Flag, value V) error {
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return err
	}
	if scope != nil {
		if !scope.admits(&value) {
			return ErrAccessDenied
		}
		if flag == Upsert {
			return m.upsertScoped(ctx, key, scope, value)
		}
	}
	var zero K
	if key == zero {
		return ErrInvalidKey
	}
	if m.db == nil {
		return m.kv.Insert(ctx, key, flag, value)
	}
	tx, err := m.manage(ctx)
	if err != nil {
		return xray.New(err)
	}
	insert := m.db.Insert(m.to, ValuesOf(key), bool(flag), nil, ValuesOf(value))
	select {
	case tx <- insert:
	case <-ctx.Done():
		return xray.New(ctx.Err())
	}
	closeStandaloneTransaction(ctx, tx)
	n, err := insert.Wait(ctx)
	if err != nil {
		if err == ErrDuplicate {
			return m.createdAlready(ctx, key, value)
		}
		return xray.New(err)
	}
	if n == -1 {
		return m.createdAlready(ctx, key, value)
	}
	return nil
}

// createdAlready is the outcome of a [Create] that found a value at the key,
// which is only a failure if that value differs from the given one. It is the
// columns that are compared, as they are all that a [Database] stores.
func (m *Map[K, V]) createdAlready(ctx context.Context, key K, value V) error {
	existing, ok, err := m.Lookup(ctx, key)
	if err != nil || !ok || !reflect.DeepEqual(ValuesOf(existing), ValuesOf(value)) {
		return ErrDuplicate
	}
	return nil
}

// impossible reports whether the query holds an expression already known to be
// false, in which case it matches nothing and no [Database] needs to see it.
func (m *Map[K, V]) impossible(query Query) bool {
	return !m.kv.match(query)
}

func (m *Map[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return err
	}
	if scope != nil {
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		return m.kv.Output(ctx, query, stats)
	}
	if stats == nil {
		return nil // no stats, nothing to do.
	}
	key, val := m.index, m.value
	var sql Query
	if query != nil {
		sql = query(key, val)
	}
	ptr := stats(key, val)
	if m.impossible(sql) || (scope != nil && scope.empty) {
		for _, stat := range ptr {
			stat.reset()
		}
		return nil
	}
	get := make(chan []sodium.Value, 1)

	var out sodium.Stats
	for _, stat := range ptr {
		out = append(out, stat.calculation())
	}
	do := m.db.Output(m.to, sodium.Query(sql), sodium.Stats(out), get)
	tx, err := m.manage(ctx)
	if err != nil {
		return xray.New(err)
	}
	select {
	case tx <- do:
		closeStandaloneTransaction(ctx, tx)
	case <-ctx.Done():
		closeStandaloneTransaction(ctx, tx)
		return xray.New(ctx.Err())
	}
	if _, err := do.Wait(ctx); err != nil {
		return xray.New(err)
	}
	select {
	case output := <-get:
		for i, stat := range ptr {
			stat.update(output[i])
		}
		return nil
	case <-ctx.Done():
		return xray.New(ctx.Err())
	}
}

func (m *Map[K, V]) Search(ctx context.Context, query QueryFunc[K, V], issue *error) iter.Seq2[K, V] {
	scope, err := scopeFor[V](ctx)
	if err != nil {
		*issue = err
		return func(yield func(K, V) bool) {}
	}
	if scope != nil {
		if scope.empty && m.db != nil {
			*issue = nil
			return func(yield func(K, V) bool) {}
		}
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		return m.kv.Search(ctx, query, issue)
	}
	*issue = nil
	var sql Query
	if query != nil {
		sql = query(m.index, m.value)
	}
	if m.impossible(sql) {
		return func(yield func(K, V) bool) {}
	}
	return func(yield func(K, V) bool) {
		ch := make(chan []sodium.Value, 64)
		do := m.db.Search(m.to, sodium.Query(sql), ch)
		tx, err := m.manage(ctx)
		if err != nil {
			*issue = err
			return
		}
		select {
		case tx <- do:
		case <-ctx.Done():
			*issue = xray.New(ctx.Err())
			return
		}
		closeStandaloneTransaction(ctx, tx)
		waited := false
		for {
			var values []sodium.Value
			var open bool
			select {
			case values, open = <-ch:
			case <-ctx.Done():
				*issue = xray.New(ctx.Err())
				return
			}
			if values == nil {
				// The job says how it went, either when the channel closes or
				// when a nil row announces a failure. It is only asked once.
				if !waited {
					waited = true
					if _, err := do.Wait(ctx); err != nil {
						*issue = xray.New(err)
						return
					}
				}
				if !open {
					return
				}
				continue
			}
			var key K
			var val V
			_, keyErr := decode(reflect.ValueOf(&key), values[:len(m.to.Index)])
			_, valErr := decode(reflect.ValueOf(&val), values[len(m.to.Index):])
			if keyErr != nil || valErr != nil {
				*issue = errors.Join(keyErr, valErr)
				return
			}
			if !yield(key, val) {
				return
			}
		}
	}
}

// First returns the first entry in the map that matches the given query.
func (m *Map[K, V]) First(ctx context.Context, query QueryFunc[K, V]) (K, V, bool, error) {
	var err error
	for k, v := range m.Search(ctx, query, &err) {
		return k, v, true, nil
	}
	return [1]K{}[0], [1]V{}[0], false, err
}

// Count returns the number of entries in the map that match the given query.
func (m *Map[K, V]) Count(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	var count atomic.Int64
	err := m.Output(ctx, query, func(k *K, v *V) Stats { return Stats{Count(&count)} })
	return int(count.Load()), err
}

// Length returns the total number of entries in the map.
func (m *Map[K, V]) Length(ctx context.Context) (int, error) {
	return m.Count(ctx, nil)
}

// Lookup the specified key in the map and return the value associated with it, if
// the value is not present in the map, the resulting boolean will be false.
func (m *Map[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	var zero K
	var value V
	if key == zero {
		return value, false, ErrInvalidKey
	}
	if m.db == nil {
		scope, err := scopeFor[V](ctx)
		if err != nil {
			return [1]V{}[0], false, err
		}
		val, ok, err := m.kv.Lookup(ctx, key)
		if ok && scope != nil && !scope.admits(&val) {
			return [1]V{}[0], false, nil
		}
		return val, ok, err
	}
	var err error
	result := m.Search(ctx, func(primary *K, _ *V) Query {
		return Query{
			Index(primary).Equals(key),
			Slice(0, 1),
		}
	}, &err)
	next, stop := iter.Pull2(result)
	defer stop()
	_, val, ok := next()
	if ok {
		return val, true, nil
	}
	return value, false, xray.New(err)
}

// Delete the value at the specified key in the map if the specified check passes.
// Boolean returned is true if a value was deleted this way.
func (m *Map[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
	var zero K
	if key == zero {
		return false, ErrInvalidKey
	}
	if m.db == nil {
		scope, err := scopeFor[V](ctx)
		if err != nil {
			return false, err
		}
		if scope != nil {
			check = scopedCheck(scope, check)
		}
		return m.kv.Delete(ctx, key, check)
	}
	query := func(k *K, v *V) Query {
		index := Query{
			Index(k).Equals(key),
			Slice(0, 1),
		}
		if check == nil {
			return index
		}
		return append(index, check(v)...)
	}
	count, err := m.UnsafeDelete(ctx, query)
	if err != nil {
		return false, xray.New(err)
	}
	return count > 0, nil
}

// UnsafeDelete each value in the map that matches the given query. The number of
// values that were deleted is returned, along with any error that occurred. The
// query must include a non-empty slice operation that limits the number of values
// that can be deleted, otherwise the operation will fail. Unsafe because a large
// amount of data can be permanently deleted this way.
func (m *Map[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	if !query.finite() {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return 0, err
	}
	if scope != nil {
		if scope.empty && m.db != nil {
			return 0, nil
		}
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		return m.kv.UnsafeDelete(ctx, query)
	}
	var sql = query(m.index, m.value)
	if m.impossible(sql) {
		return 0, nil
	}
	do := m.db.Delete(m.to, sodium.Query(sql))
	tx, err := m.manage(ctx)
	if err != nil {
		return 0, xray.New(err)
	}
	select {
	case tx <- do:
	case <-ctx.Done():
		return 0, xray.New(ctx.Err())
	}
	closeStandaloneTransaction(ctx, tx)
	result, err := do.Wait(ctx)
	return result, xray.New(err)
}

// Update each value in the map that matches the given query with the given patch. The number of
// values that were updated is returned, along with any error that occurred. The query must
// include a non-empty slice operation that limits the number of values that can be updated,
// otherwise the operation will fail. A [Database] applies one patch to every value, so the patch must
// not depend on the value it is given, use [Map.Mutate] for that. Without a [Database], the
// patch can run more than once for a value that changes while it is being patched.
func (m *Map[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	if !query.finite() {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return 0, err
	}
	if scope != nil {
		if scope.empty && m.db != nil {
			return 0, nil
		}
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		if scope == nil {
			return m.kv.Update(ctx, query, patch)
		}
		var violated bool
		count, err := m.kv.Update(ctx, query, scopedPatch(scope, patch, &violated))
		if err == nil && violated {
			err = ErrAccessDenied
		}
		return count, err
	}
	sql := query(m.index, m.value)
	mod := patch(m.value)
	if scope != nil && !scope.admitsPatch(m.value, mod) {
		return 0, ErrAccessDenied
	}
	if m.impossible(sql) {
		return 0, nil
	}
	return m.update(ctx, sql, mod)
}

func (m *Map[K, V]) update(ctx context.Context, sql Query, mod Patch) (int, error) {
	do := m.db.Update(m.to, sodium.Query(sql), sodium.Patch(mod))
	tx, err := m.manage(ctx)
	if err != nil {
		return 0, xray.New(err)
	}
	select {
	case tx <- do:
	case <-ctx.Done():
		return 0, xray.New(ctx.Err())
	}
	closeStandaloneTransaction(ctx, tx)
	result, err := do.Wait(ctx)
	return result, xray.New(err)
}

// Mutate the value at the specified key in the map. The [CheckFunc] is called with
// the current value at the specified key, if the [CheckFunc] returns true, then the
// [PatchFunc] is called with the current value at the specified key. The [PatchFunc]
// should return the modifications to be made to the value at the specified key.
// With a [Database], the value is read and then written inside of a transaction,
// joining the one held by the context if there is one, and only the columns that
// the patch changed are written.
func (m *Map[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
	var zero K
	if key == zero {
		return false, ErrInvalidKey
	}
	if m.db == nil {
		scope, err := scopeFor[V](ctx)
		if err != nil {
			return false, err
		}
		if scope == nil {
			return m.kv.Mutate(ctx, key, check, patch)
		}
		var violated bool
		ok, err := m.kv.Mutate(ctx, key, scopedCheck(scope, check), scopedPatch(scope, patch, &violated))
		if err == nil && violated {
			err = ErrAccessDenied
		}
		return ok, err
	}
	query := func(k *K, v *V) Query {
		index := Query{
			Index(k).Equals(key),
			Slice(0, 1),
		}
		if check == nil {
			return index
		}
		return append(index, check(v)...)
	}
	var mutated bool
	err := m.Manage(ctx, ReadWrites|LockWrites, func(ctx context.Context) error {
		var err error
		mutated, err = m.mutate(ctx, query, patch)
		return err
	})
	if err != nil {
		return false, xray.New(err)
	}
	return mutated, nil
}

// mutate runs the patch here, on the current value, because a [Database] cannot:
// given to a sentinal, a patch that reads its value would read zero.
func (m *Map[K, V]) mutate(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (bool, error) {
	scope, err := scopeFor[V](ctx)
	if err != nil {
		return false, err
	}
	_, current, found, err := m.First(ctx, query)
	if err != nil || !found {
		return false, err
	}
	updated := current
	patch(&updated)
	if scope != nil {
		if !scope.admits(&updated) {
			return false, ErrAccessDenied
		}
		query = scopedQuery(scope, query)
	}
	mods := changedColumns(m.to.Value, ValuesOf(current), ValuesOf(updated))
	if len(mods) == 0 {
		return true, nil
	}
	count, err := m.update(ctx, query(m.index, m.value), Patch{sodium.Modifications.Arr.As(mods)})
	return count > 0, err
}

// changedColumns returns the modifications that turn one set of values into another.
func changedColumns(columns []sodium.Column, before, after []sodium.Value) []sodium.Modification {
	var mods []sodium.Modification
	for i, column := range columns {
		if !reflect.DeepEqual(before[i], after[i]) {
			mods = append(mods, sodium.Modifications.Set.As(xyz.NewPair(column, after[i])))
		}
	}
	return mods
}

// upsertScoped overwrites the value at key only if the existing value is inside
// the scope held by the context, so that an upsert cannot take over a record
// belonging to another scope. The [Database] checks the existing value in the
// same operation that overwrites it.
func (m *Map[K, V]) upsertScoped(ctx context.Context, key K, scope *scoper[V], value V) error {
	var zero K
	if key == zero {
		return ErrInvalidKey
	}
	if m.db == nil {
		if !m.kv.Upsert(ctx, key, scopedCheck(scope, nil), value) {
			return ErrAccessDenied
		}
		return nil
	}
	upsert := m.db.Insert(m.to, ValuesOf(key), bool(Upsert), sodium.Query(scope.expressions(m.value)), ValuesOf(value))
	tx, err := m.manage(ctx)
	if err != nil {
		return xray.New(err)
	}
	select {
	case tx <- upsert:
	case <-ctx.Done():
		return xray.New(ctx.Err())
	}
	closeStandaloneTransaction(ctx, tx)
	n, err := upsert.Wait(ctx)
	if err != nil {
		return xray.New(err)
	}
	if n == 0 {
		return ErrAccessDenied
	}
	return nil
}

// transactionLock is held for the length of a transaction on a RAM [Map]. It is
// a channel so that waiting for it can end with the context or a timeout.
var transactionLock = make(chan struct{}, 1)

type transactionKey struct{}

func (m *Map[K, V]) manage(ctx context.Context) (chan<- sodium.Job, error) {
	if tx, ok := ctx.Value(transactionKey{}).(chan<- sodium.Job); ok {
		return tx, nil
	}
	return m.db.Manage(ctx, 0)
}

// closeStandaloneTransaction leaves a shared transaction to its Manage caller.
func closeStandaloneTransaction(ctx context.Context, tx chan<- sodium.Job) {
	if _, shared := ctx.Value(transactionKey{}).(chan<- sodium.Job); !shared {
		close(tx)
	}
}

// Manage runs fn as a transaction, writes made with the context it is given are
// committed if fn returns nil and rolled back if it fails or panics. A nested call
// joins the transaction held by its context, so its failure only rolls back if it
// fails the outer fn too. Without a [Database], transactions run one at a time, but
// other readers see their writes, and a rollback leaves alone any row that has been
// written from outside of the transaction. One that waits a minute for its turn fails.
func (m *Map[K, V]) Manage(ctx context.Context, level TransactionLevel, fn func(context.Context) error) error {
	if m.db == nil {
		return manageRAM(ctx, ramTransactionWait, fn)
	}
	if _, shared := ctx.Value(transactionKey{}).(chan<- sodium.Job); shared {
		return fn(ctx)
	}
	tx, err := m.db.Manage(ctx, level)
	if err != nil {
		return xray.New(err)
	}
	commit := false
	defer func() {
		if !commit {
			select {
			case tx <- nil: // Roll back on callback failure or panic.
			case <-ctx.Done():
			}
		}
		close(tx)
	}()
	err = fn(context.WithValue(ctx, transactionKey{}, tx))
	commit = err == nil
	return err
}

var smutex sync.RWMutex
var mirror = make(map[any][]sodium.Column)

var sentinals struct {
	index sentinal
	value sentinal
}

type sentinal map[sentinalKey]any

type sentinalKey struct {
	rtype   reflect.Type
	table   string
	columns string // a table can be opened with different names for its columns.
}

// assert returns the sentinal for the field's type in a table with the given
// columns, which is arg if there isn't one yet.
func (s *sentinal) assert(table string, field reflect.StructField, arg any, columns []sodium.Column) any {
	smutex.Lock()
	defer smutex.Unlock()
	if *s == nil {
		*s = make(map[sentinalKey]any)
	}
	var names []string
	for _, column := range columns {
		names = append(names, column.Name)
	}
	key := sentinalKey{
		table:   table,
		rtype:   field.Type,
		columns: strings.Join(names, ","),
	}
	if existing, ok := (*s)[key]; ok {
		return existing
	}
	(*s)[key] = arg
	s.walk(renamedColumns(field, columns), field, reflect.ValueOf(arg).Elem())
	return arg
}

// renamedColumns maps the columns that [columnsOf] derives for a field to the
// columns the table has at the same positions.
func renamedColumns(field reflect.StructField, columns []sodium.Column) map[string]sodium.Column {
	derived := columnsOf(field)
	if len(derived) != len(columns) {
		return nil
	}
	renamed := make(map[string]sodium.Column, len(columns))
	for i, column := range derived {
		renamed[column.Name] = columns[i]
	}
	return renamed
}

func (s *sentinal) walk(renamed map[string]sodium.Column, field reflect.StructField, arg reflect.Value, path ...string) {
	name := columnName(field)
	columns := columnsOf(field, path...)
	for i, column := range columns {
		if chosen, ok := renamed[column.Name]; ok {
			columns[i] = chosen
		}
	}
	mirror[arg.Addr().Interface()] = columns
	if raw := arg.Addr().MethodByName("RawPointer"); raw.IsValid() && raw.Type().NumOut() == 1 {
		mirror[raw.Call(nil)[0].Interface()] = columns
	}
	switch field.Type.Kind() {
	case reflect.Struct:
		for i := range field.Type.NumField() {
			if !field.Type.Field(i).IsExported() {
				continue
			}
			promote := append(path, name)
			if field.Anonymous {
				promote = path
			}
			s.walk(renamed, field.Type.Field(i), arg.Field(i), promote...)
		}
	case reflect.Array:
		for i := range field.Type.Len() {
			vfield := reflect.StructField{
				Name: fmt.Sprintf("%s%d", name, i+1),
				Type: field.Type.Elem(),
			}
			s.walk(renamed, vfield, arg.Index(i), path...)
		}
	}
}

package sql

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
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
	to   sodium.Table
	db   sodium.Database
	from *opening // the call that gave the Map its Database.
	kv   ram[K, V]

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

// opening is one call to [Open] or [OpenTable]. The maps it made were given one
// [Database], which is how they are known to share it when that is a value
// that cannot be compared with another.
type opening struct {
	db sodium.Database
}

func (m *Map[K, V]) open(from *opening, table Table) {
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
		db:    from.db,
		from:  from,
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
// name is 'value'. A 'sql' tag may also hold the option 'scoped', see
// [Scope].
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
		open(from *opening, table Table)
	}
	var zero T
	from := &opening{db: db}
	rtype := reflect.TypeOf(zero)
	value := reflect.ValueOf(&zero).Elem()
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if table, ok := field.Tag.Lookup("sql"); ok {
			value.Field(i).Addr().Interface().(opener).open(from, Table(table))
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
		from:  &opening{db: db},
		index: sentinals.index.assert(table.Name, key, new(K), table.Index).(*K),
		value: sentinals.value.assert(table.Name, val, new(V), table.Value).(*V),
	}
}

// Add the specified value to the [Map], if possible, a key will be
// automatically selected. If a key is required in order to add to
// the [Map], an [ErrInsertOnly] error will be returned and nothing
// is added, though inside of a transaction it is up to the caller
// to roll back. If the K type implements Randomize, this method will
// be called on an zero value of K to generate a random key. A key
// is also required, with a [Database] as without one, when the key
// has a scoped column, see [Scope], and the zero key is outside of
// the scopes held by the context: such a column is not for a
// database to pick. A selected key that is outside of those scopes
// is an [ErrAccessDenied].
//
// Without a [Database], an integer key is instead the next number in
// a sequence that starts at 1, whether or not K implements Randomize,
// and it wraps around once it no longer fits in K. If the selected
// key is already in use, [ErrDuplicate] is returned with that key and
// nothing is added.
func (m *Map[K, V]) Add(ctx context.Context, value V) (K, error) {
<<<<<<< HEAD
	scope, err := scopeFor[K, V](ctx)
=======
	scope, err := scopeFor[V](ctx, Write)
>>>>>>> upstream-translated
	if err != nil {
		return [1]K{}[0], err
	}
	if scope != nil && !scope.admitsValue(&value) {
		return [1]K{}[0], ErrAccessDenied
	}
	if m.db == nil {
		if scope == nil {
			return m.kv.Add(ctx, value, nil)
		}
		return m.kv.Add(ctx, value, scope.admitsKey)
	}
	var key K
	if r, ok := any(&key).(interface {
		Randomize()
	}); ok {
		r.Randomize()
	}
	// The scoped columns of a key are not for a [Database] to pick, so a key
	// that is out of scope before the insert is refused without one. If it is
	// the key that nobody picked, what it lacks is a key from the caller.
	if scope != nil && !scope.admitsKey(&key) {
		if m.zeroKey(key) {
			return [1]K{}[0], ErrInsertOnly
		}
		return [1]K{}[0], ErrAccessDenied
	}
	// Only the insert can tell whether a key is required, so it has to be
	// possible to take the insert back. Inside the caller's transaction, that
	// is left to the caller, who has the error.
	err = m.Manage(ctx, 0, func(ctx context.Context) error {
		key, err = m.add(ctx, key, value)
		if err == nil && scope != nil && !scope.admitsKey(&key) {
			return ErrAccessDenied
		}
		return err
	})
	return key, err
}

func (m *Map[K, V]) add(ctx context.Context, key K, value V) (K, error) {
	var values = ValuesOf(key)
	n, err := m.run(ctx, m.db.Insert(m.to, values, bool(Create), nil, ValuesOf(value)))
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
	if m.zeroKey(key) {
		return key, ErrInsertOnly
	}
	return key, nil
}

// zeroKey reports whether the key is the zero key, which is not the key of
// any row in a [Database].
func (m *Map[K, V]) zeroKey(key K) bool {
	var zero K
	return key == zero
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
// otherwise [ErrDuplicate] is returned, even if the existing value is the same as
// the given value. With a [Database], the zero key is an [ErrInvalidKey]. With a
// [Database] that is a [sodium.Committer], nil is only returned once the database
// has said that the write is committed, or, inside of a transaction, that it is
// made: see [Map.Manage].
func (m *Map[K, V]) Insert(ctx context.Context, key K, flag Flag, value V) error {
<<<<<<< HEAD
	scope, err := scopeFor[K, V](ctx)
=======
	scope, err := scopeFor[V](ctx, Write)
>>>>>>> upstream-translated
	if err != nil {
		return err
	}
	if scope != nil {
		if !scope.admits(&key, &value) {
			return ErrAccessDenied
		}
		if flag == Upsert {
			return m.upsertScoped(ctx, key, scope, value)
		}
	}
	if m.db == nil {
		return m.kv.Insert(ctx, key, flag, value)
	}
	if m.zeroKey(key) {
		return ErrInvalidKey
	}
	n, err := m.run(ctx, m.db.Insert(m.to, ValuesOf(key), bool(flag), nil, ValuesOf(value)))
	if err != nil {
		if err == ErrDuplicate {
			return err
		}
		return xray.New(err)
	}
	if n == -1 {
		return ErrDuplicate
	}
	return nil
}

// impossible reports whether the query holds an expression already known to be
// false, in which case it matches nothing and no [Database] needs to see it.
func (m *Map[K, V]) impossible(query Query) bool {
	return !m.kv.match(query)
}

// build the query a [Database] is sent.
func (m *Map[K, V]) build(query QueryFunc[K, V]) Query {
	if query == nil {
		return nil
	}
	return query(m.index, m.value)
}

// orderColumn returns the column a [Database] is to order by. There is none
// for an [Order] made from a pointer that is no part of a sentinal, which
// orders values in memory.
func orderColumn(order sodium.OrderExpression) (sodium.Column, bool) {
	switch xyz.ValueOf(order) {
	case sodium.OrderExpressions.Increasing:
		return sodium.OrderExpressions.Increasing.Get(order), true
	case sodium.OrderExpressions.Decreasing:
		return sodium.OrderExpressions.Decreasing.Get(order), true
	}
	return sodium.Column{}, false
}

// evaluated returns an error for an expression that was not worked out from a
// value, which is all that a [Map] without a [Database] can go by. Such an
// expression is on a column: it was built from the field of another [Map],
// one that has a [Database], and no value here can be asked about it.
func evaluated(exprs []sodium.Expression) error {
	for _, expr := range exprs {
		switch xyz.ValueOf(expr) {
		case sodium.Expressions.Value, sodium.Expressions.Range:
			continue
		case sodium.Expressions.Order:
			if _, column := orderColumn(sodium.Expressions.Order.Get(expr)); !column {
				continue
			}
		}
		return xray.New(errors.New("sql: an expression for a Map without a Database is on a column of a Database: an expression must be built from the arguments of the function that returns it, and this one is from another Map"))
	}
	return nil
}

// evaluable returns an error for a query that a [Map] without a [Database]
// cannot go by, see [evaluated]. Such an expression does not depend on the
// values it is asked for, so it shows for the zero ones.
func (fn QueryFunc[K, V]) evaluable() error {
	if fn == nil {
		return nil
	}
	var key K
	var val V
	return evaluated(fn(&key, &val))
}

// evaluable is [QueryFunc.evaluable] for a check.
func (fn CheckFunc[V]) evaluable() error {
	if fn == nil {
		return nil
	}
	var val V
	return evaluated(fn(&val))
}

func (m *Map[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
<<<<<<< HEAD
	scope, err := scopeFor[K, V](ctx)
=======
	scope, err := scopeFor[V](ctx, Reads)
>>>>>>> upstream-translated
	if err != nil {
		return err
	}
	if scope != nil {
		if scope.empty && m.db != nil {
			return nil
		}
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		return m.kv.Output(ctx, query, stats)
	}
	if stats == nil {
		return nil // no stats, nothing to do.
	}
	key, val := m.index, m.value
	sql := m.build(query)
	ptr := stats(key, val)
	if m.impossible(sql) {
		for _, stat := range ptr {
			stat.reset()
		}
		return nil
	}
	var out sodium.Stats
	for _, stat := range ptr {
		out = append(out, stat.calculation())
	}
	get := make(chan []sodium.Value, 1)
	if _, err := m.run(ctx, m.db.Output(m.to, sodium.Query(sql), out, get)); err != nil {
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

// Search returns the entries in the map that match the given query, and leaves
// in issue the error that ended the search early, if one did. With a [Database],
// rows are read as the caller takes them. A search that the caller stops early
// is cancelled, unless it is part of a transaction, which cannot move on to
// its next operation until the rest of the rows have been read and dropped:
// a query with a [Slice] keeps that short. Any other operation of the
// transaction can be made while the rows of a search are being taken, and the
// rows not taken yet are then held in memory until they are.
func (m *Map[K, V]) Search(ctx context.Context, query QueryFunc[K, V], issue *error) iter.Seq2[K, V] {
<<<<<<< HEAD
	none := func(yield func(K, V) bool) {}
	scope, err := scopeFor[K, V](ctx)
=======
	scope, err := scopeFor[V](ctx, Reads)
>>>>>>> upstream-translated
	if err != nil {
		*issue = err
		return none
	}
	if scope != nil {
		if scope.empty && m.db != nil {
			*issue = nil
			return none
		}
		query = scopedQuery(scope, query)
	}
	if m.db == nil {
		return m.kv.Search(ctx, query, issue)
	}
	*issue = nil
	sql := m.build(query)
	if m.impossible(sql) {
		return none
	}
	return func(yield func(K, V) bool) {
		if err := m.search(ctx, sodium.Query(sql), yield); err != nil {
			*issue = err
		}
	}
}

// search a [Database], in the transaction held by the context if there is one.
// Otherwise the search is a transaction of its own, which lets it be cancelled
// when the caller has seen enough.
func (m *Map[K, V]) search(ctx context.Context, query sodium.Query, yield func(K, V) bool) error {
	rows := make(chan []sodium.Value, 64)
	job := m.db.Search(m.to, query, rows)
	if tx := m.joined(ctx); tx != nil {
		return m.searchJoined(ctx, tx, job, rows, yield)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // tells the database to send no more rows.
	tx, err := m.begin(ctx, 0)
	if err != nil {
		return xray.New(err)
	}
	defer tx.end(ctx, false)
	over, err := m.receive(ctx, tx, job, &reading{rows: rows}, yield)
	if err != nil || !over {
		return err
	}
	return xray.New(tx.commit(ctx))
}

// searchJoined is a search in a transaction that has more to do after it. A
// transaction runs its jobs in turn, and a search is not over until its last
// row has somewhere to go, so however the search is left, by a break, a
// failure or a panic of the caller's, what is left of its rows is dropped.
func (m *Map[K, V]) searchJoined(ctx context.Context, tx *transaction, job sodium.Job, rows <-chan []sodium.Value, yield func(K, V) bool) (err error) {
	search := &reading{rows: rows}
	over := false
	defer func() {
		if over {
			tx.forget(search)
			return
		}
		if left := tx.drop(ctx, search); err == nil {
			err = xray.New(left)
		}
	}()
	over, err = m.receive(ctx, tx, job, search, yield)
	return err
}

// receive starts a search and yields its rows. It reports whether the search
// is over, which it is not if it was left because yield asked for no more, or
// because a row could not be read: the database may still be sending rows.
func (m *Map[K, V]) receive(ctx context.Context, tx *transaction, job sodium.Job, search *reading, yield func(K, V) bool) (bool, error) {
	if err := tx.send(ctx, job, search); err != nil {
		return false, xray.New(err)
	}
	waited := false
	for {
		values, open, err := tx.next(ctx, search)
		if err != nil {
			return false, xray.New(err)
		}
		if values == nil {
			// The job says how it went, either when the channel closes or
			// when a nil row announces a failure. It is only asked once.
			if !waited {
				waited = true
				if _, err := job.Wait(ctx); err != nil {
					return true, xray.New(err)
				}
			}
			if !open {
				return true, nil
			}
			continue
		}
		var key K
		var val V
		_, keyErr := decode(reflect.ValueOf(&key), values[:len(m.to.Index)])
		_, valErr := decode(reflect.ValueOf(&val), values[len(m.to.Index):])
		if keyErr != nil || valErr != nil {
			return false, errors.Join(keyErr, valErr)
		}
		if !yield(key, val) {
			return false, nil
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
// the value is not present in the map, the resulting boolean will be false. With a
// [Database], the zero key is an [ErrInvalidKey].
func (m *Map[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	if m.db == nil {
<<<<<<< HEAD
		scope, err := scopeFor[K, V](ctx)
=======
		scope, err := scopeFor[V](ctx, Reads)
>>>>>>> upstream-translated
		if err != nil {
			return [1]V{}[0], false, err
		}
		val, ok, err := m.kv.Lookup(ctx, key)
		if ok && scope != nil && !scope.admits(&key, &val) {
			return [1]V{}[0], false, nil
		}
		return val, ok, err
	}
	var value V
	if m.zeroKey(key) {
		return value, false, ErrInvalidKey
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
// Boolean returned is true if a value was deleted this way. With a [Database], the
// zero key is an [ErrInvalidKey]. Without one, a check built from what the check
// of a [Map] with a [Database] was given fails, see [QueryFunc].
func (m *Map[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
	if m.db == nil {
<<<<<<< HEAD
		if err := check.evaluable(); err != nil {
			return false, err
		}
		scope, err := scopeFor[K, V](ctx)
=======
		scope, err := scopeFor[V](ctx, Write)
>>>>>>> upstream-translated
		if err != nil {
			return false, err
		}
		if err := m.refuseUnwritable(ctx, scope, keyed(key, check)); err != nil {
			return false, err
		}
		if scope != nil {
			check = scopedCheck(scope, key, check)
		}
		return m.kv.Delete(ctx, key, check)
	}
	if m.zeroKey(key) {
		return false, ErrInvalidKey
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
// query must include a slice operation that limits the number of values that can
// be deleted. A nil query fails. Without a [Database], a query with no slice, or an
// empty one, deletes nothing and returns no error, and a query built from what the
// QueryFunc of a [Map] with a [Database] was given fails, see [QueryFunc]. A
// [Database] is given such a query, which the SODIUM interface requires it to fail.
// Unsafe because a large amount of data can be permanently deleted this way.
func (m *Map[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	if query == nil {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
<<<<<<< HEAD
	scope, err := scopeFor[K, V](ctx)
	if err != nil {
		return 0, err
	}
	if m.db == nil {
		if err := query.evaluable(); err != nil {
			return 0, err
		}
=======
	scope, err := scopeFor[V](ctx, Write)
	if err != nil {
		return 0, err
	}
	if err := m.refuseUnwritable(ctx, scope, query); err != nil {
		return 0, err
>>>>>>> upstream-translated
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
	sql := m.build(query)
	if m.impossible(sql) {
		return 0, nil
	}
	result, err := m.run(ctx, m.db.Delete(m.to, sodium.Query(sql)))
	return result, xray.New(err)
}

// Update each value in the map that matches the given query with the given patch. The number of
// values that were updated is returned, along with any error that occurred. A nil query fails.
// Without a [Database], a query with no slice operation, or an empty one, updates nothing and
// returns no error, and a query built from what the QueryFunc of a [Map] with a [Database] was
// given fails, see [QueryFunc]. A [Database] is given a query with no slice, or an empty one,
// which the SODIUM interface requires it to fail. The patch is run on each value, whether it uses [Set] or assigns, and whether or not it goes
// by the value it is given. With a [Database] the values are read, patched here and written
// back, a row at a time and only the columns that the patch changed, inside of a transaction
// that locks what it writes, joining the one held by the context if there is one: that is as
// many operations as there are values and one more, which the slice of the query bounds.
// Without a [Database], the patch can run more than once for a value that changes while it is
// being patched. A patch that moves a value out of the scopes held by the context is refused
// with [ErrAccessDenied]: without a [Database] that value is left as it was and the others are
// still updated, and with a [Database] nothing is updated at all. Any other failure with a [Database] leaves nothing updated either, unless the update
// joined a transaction, which it is then up to the caller to roll back.
func (m *Map[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	if query == nil {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
<<<<<<< HEAD
	scope, err := scopeFor[K, V](ctx)
	if err != nil {
		return 0, err
	}
	if m.db == nil {
		if err := query.evaluable(); err != nil {
			return 0, err
		}
=======
	scope, err := scopeFor[V](ctx, Write)
	if err != nil {
		return 0, err
	}
	if err := m.refuseUnwritable(ctx, scope, query); err != nil {
		return 0, err
>>>>>>> upstream-translated
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
	sql := m.build(query)
	if !query.finite() {
		// The [Database] is given a query without a range as it always has
		// been, which the SODIUM interface requires it to fail, so there are
		// no rows to read and patch here.
		mod := patch(m.value)
		if scope != nil && !scope.admitsPatch(m.value, mod) {
			return 0, ErrAccessDenied
		}
		if m.impossible(sql) {
			return 0, nil
		}
		return m.update(ctx, sql, mod)
	}
	if m.impossible(sql) {
		return 0, nil
	}
	var count int
	err = m.Manage(ctx, ReadWrites|LockWrites, func(ctx context.Context) error {
		var err error
		count, err = m.patchEach(ctx, sodium.Query(sql), patch, scope)
		return err
	})
	if err != nil {
		// Returned as it is, which for a refusal is the same error as without
		// a [Database]. A failure was traced where it happened.
		return 0, err
	}
	return count, nil
}

// patchEach runs the patch here, on each value the query finds, because a
// [Database] cannot: it applies one set of modifications to every row, and
// what a patch does to a value is only known by running it on that value.
// Every row is read before any is patched, and patched before any is written,
// so that a patch that is refused leaves nothing written.
func (m *Map[K, V]) patchEach(ctx context.Context, query sodium.Query, patch PatchFunc[V], scope *scoper[K, V]) (int, error) {
	type row struct {
		key K
		val V
	}
	type change struct {
		key  K
		mods []sodium.Modification
	}
	var found []row
	err := m.search(ctx, query, func(key K, val V) bool {
		found = append(found, row{key, val})
		return true
	})
	if err != nil {
		return 0, err
	}
	var changes []change
	for _, current := range found {
		updated := cloneRAMValue(current.val)
		made := patch(&updated)
		if scope != nil && !scope.admitsValue(&updated) {
			return 0, ErrAccessDenied
		}
		mods := changedColumns(m.to.Value, ValuesOf(current.val), ValuesOf(updated))
		// Given a value that is not the sentinal, Set assigns and returns
		// nothing, so what is left is a modification the patch made for itself.
		for _, mod := range made {
			if !reflect.ValueOf(mod).IsZero() {
				mods = append(mods, mod)
			}
		}
		changes = append(changes, change{current.key, mods})
	}
	count := 0
	for _, change := range changes {
		// A row that the patch leaves as it is was updated to what it is.
		if len(change.mods) == 0 {
			count++
			continue
		}
		wrote, err := m.update(ctx, m.rowAt(change.key, scope), Patch{sodium.Modifications.Arr.As(change.mods)})
		if err != nil {
			return 0, err
		}
		count += wrote
	}
	return count, nil
}

// rowAt is the query for the row at a key, if it is inside of the scope.
func (m *Map[K, V]) rowAt(key K, scope *scoper[K, V]) Query {
	query := QueryFunc[K, V](func(k *K, _ *V) Query {
		return Query{
			Index(k).Equals(key),
			Slice(0, 1),
		}
	})
	if scope != nil {
		query = scopedQuery(scope, query)
	}
	return query(m.index, m.value)
}

func (m *Map[K, V]) update(ctx context.Context, sql Query, mod Patch) (int, error) {
	result, err := m.run(ctx, m.db.Update(m.to, sodium.Query(sql), sodium.Patch(mod)))
	return result, xray.New(err)
}

// Mutate the value at the specified key in the map. The [CheckFunc] is called with
// the current value at the specified key, if the [CheckFunc] returns true, then the
// [PatchFunc] is called with the current value at the specified key. The [PatchFunc]
// should return the modifications to be made to the value at the specified key.
// With a [Database], the value is read and then written inside of a transaction,
// joining the one held by the context if there is one, and only the columns that
// the patch changed are written. With a [Database], the zero key is an [ErrInvalidKey].
// Without one, a check built from what the check of a [Map] with a [Database] was given
// fails, see [QueryFunc].
func (m *Map[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
	if m.db == nil {
<<<<<<< HEAD
		if err := check.evaluable(); err != nil {
			return false, err
		}
		scope, err := scopeFor[K, V](ctx)
=======
		scope, err := scopeFor[V](ctx, Write)
>>>>>>> upstream-translated
		if err != nil {
			return false, err
		}
		if scope == nil {
			return m.kv.Mutate(ctx, key, check, patch)
		}
		if err := m.refuseUnwritable(ctx, scope, keyed(key, check)); err != nil {
			return false, err
		}
		var violated bool
		ok, err := m.kv.Mutate(ctx, key, scopedCheck(scope, key, check), scopedPatch(scope, patch, &violated))
		if err == nil && violated {
			err = ErrAccessDenied
		}
		return ok, err
	}
	if m.zeroKey(key) {
		return false, ErrInvalidKey
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
	scope, err := scopeFor[K, V](ctx)
	if err != nil {
		return false, err
	}
	_, current, found, err := m.First(ctx, query)
	if err != nil || !found {
		return false, err
	}
	updated := cloneRAMValue(current)
	patch(&updated)
	if scope != nil {
		if !scope.admitsValue(&updated) {
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

// refuseUnwritable fails with [ErrAccessDenied] when the query matches a row
// that the context can read but not write, so that a write never quietly
// skips a row the caller can see.
func (m *Map[K, V]) refuseUnwritable(ctx context.Context, scope *scoper[V], query QueryFunc[K, V]) error {
	if scope == nil || !scope.narrower {
		return nil
	}
	_, _, found, err := m.First(ctx, func(k *K, v *V) Query {
		var base Query
		if query != nil {
			base = query(k, v)
		}
		unwritable, needed := scope.unwritable(v)
		if !needed {
			return base
		}
		return append(slices.Clip(base), unwritable)
	})
	if err != nil {
		return err
	}
	if found {
		return ErrAccessDenied
	}
	return nil
}

// upsertScoped overwrites the value at key only if the existing value is inside
// the scope held by the context, so that an upsert cannot take over a record
// belonging to another scope. The [Database] checks the existing value in the
// same operation that overwrites it.
func (m *Map[K, V]) upsertScoped(ctx context.Context, key K, scope *scoper[K, V], value V) error {
	if m.db == nil {
		if !m.kv.Upsert(ctx, key, scopedCheck(scope, key, nil), value) {
			return ErrAccessDenied
		}
		return nil
	}
	if m.zeroKey(key) {
		return ErrInvalidKey
	}
	n, err := m.run(ctx, m.db.Insert(m.to, ValuesOf(key), bool(Upsert), sodium.Query(scope.expressions(m.index, m.value)), ValuesOf(value)))
	if err != nil {
		return xray.New(err)
	}
	if n == 0 {
		return ErrAccessDenied
	}
	return nil
}

// transactionLock is held for the length of a transaction on a RAM [Map]. It is
// a channel so that waiting for it can end with the context.
var transactionLock = make(chan struct{}, 1)

type transactionKey struct{}

// transaction is one on a [Database]: the one a context holds, or the one
// that a single operation is when its context holds none.
type transaction struct {
	db    sodium.Database
	from  *opening     // of the Map the transaction was started with.
	outer *transaction // on another Database, held by the same context.

	// mutex is held to send a job and to take a row of the search being read,
	// which keeps the two in turn: see send.
	mutex      sync.Mutex
	jobs       chan<- sodium.Job
	reading    *reading // the search that the database may still be sending rows of.
	committing bool     // a commit was sent, which is the last thing sent.
	ended      bool     // the channel is closed.

	// answers is set for a [sodium.Committer], which is sent a [sodium.Commit]
	// to commit. Any other [Database] commits when the channel is closed.
	answers bool
}

// reading is a search whose rows are being taken from a transaction.
type reading struct {
	rows   <-chan []sodium.Value
	kept   [][]sodium.Value // taken off rows to let the next job run, and still to be read.
	sent   bool             // the database was sent the search, and will close rows.
	closed bool             // rows has been closed, what there is left is in kept.
}

// begin a transaction of the [Map]'s [Database].
func (m *Map[K, V]) begin(ctx context.Context, level TransactionLevel) (*transaction, error) {
	jobs, err := m.db.Manage(ctx, level)
	if err != nil {
		return nil, err
	}
	return &transaction{db: m.db, from: m.from, jobs: jobs, answers: sodium.AnswersCommits(m.db)}, nil
}

// sameDatabase reports whether two values are known to be one [Database],
// which they are not if they cannot be compared: two of one type may well be
// connections to different databases, and a job is only ever sent to the
// [Database] of the [Map] that made it.
func sameDatabase(a, b sodium.Database) bool {
	if reflect.TypeOf(a) != reflect.TypeOf(b) || !reflect.ValueOf(a).Comparable() {
		return false
	}
	return a == b
}

// joined returns the transaction on the [Database] of the [Map] that the
// context holds, if it holds one. A transaction on another [Database] is not
// one to join: its jobs are not for this one to run. The maps of one [Open]
// were given the same [Database], whatever it is, and other maps are in it if
// theirs is equal to it.
func (m *Map[K, V]) joined(ctx context.Context) *transaction {
	held, _ := ctx.Value(transactionKey{}).(*transaction)
	for tx := held; tx != nil; tx = tx.outer {
		if (m.from != nil && tx.from == m.from) || sameDatabase(tx.db, m.db) {
			return tx
		}
	}
	return nil
}

// run a job and wait for it, in the transaction held by the context if there
// is one. Otherwise the job is a transaction of its own, committed only once
// the job has worked: closed without that, the transaction of a
// [sodium.Committer] is rolled back, and that of any other [Database] is left
// to it, as it always has been.
func (m *Map[K, V]) run(ctx context.Context, job sodium.Job) (int, error) {
	if tx := m.joined(ctx); tx != nil {
		return tx.run(ctx, job)
	}
	tx, err := m.begin(ctx, 0)
	if err != nil {
		return 0, err
	}
	defer tx.end(ctx, false)
	n, err := tx.run(ctx, job)
	if err != nil {
		return n, err
	}
	return n, tx.commit(ctx)
}

// run a job in the transaction and wait for it.
func (tx *transaction) run(ctx context.Context, job sodium.Job) (int, error) {
	if err := tx.send(ctx, job, nil); err != nil {
		return 0, err
	}
	return job.Wait(ctx)
}

// send a job to the transaction, which is a search if it has rows to be read.
// A transaction runs its jobs in turn, and a search is not over until its last
// row has somewhere to go. So the rows of a search that is still being read
// are taken into memory first, where its reader finds them: the job may well
// be sent by that reader, for the row it was just given.
func (tx *transaction) send(ctx context.Context, job sodium.Job, search *reading) error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	if tx.ended {
		return ErrTransactionEnded
	}
	if tx.reading != nil {
		if err := tx.reading.keep(ctx); err != nil {
			return err
		}
		tx.reading = nil
	}
	select {
	case tx.jobs <- job:
	case <-ctx.Done():
		return ctx.Err()
	}
	if search != nil {
		search.sent = true
	}
	tx.reading = search
	return nil
}

// keep takes the rest of the rows into memory, in the order they are sent.
func (search *reading) keep(ctx context.Context) error {
	for !search.closed {
		select {
		case values, open := <-search.rows:
			if !open {
				search.closed = true
				break
			}
			search.kept = append(search.kept, values)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// next returns the next row of a search, and false once there are no more. A
// nil row is not one of the rows, see [Map.receive].
func (tx *transaction) next(ctx context.Context, search *reading) ([]sodium.Value, bool, error) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	if len(search.kept) > 0 {
		values := search.kept[0]
		search.kept = search.kept[1:]
		return values, true, nil
	}
	if search.closed {
		return nil, false, nil
	}
	select {
	case values, open := <-search.rows:
		if !open {
			search.closed = true
			tx.forgetLocked(search)
		}
		return values, open, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

// forget a search that is over, the database has no more rows of it to send.
func (tx *transaction) forget(search *reading) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	tx.forgetLocked(search)
}

func (tx *transaction) forgetLocked(search *reading) {
	search.kept = nil
	if tx.reading == search {
		tx.reading = nil
	}
}

// drop reads and drops the rows a search still has to send, for a reader that
// has left it.
func (tx *transaction) drop(ctx context.Context, search *reading) error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	return tx.dropLocked(ctx, search)
}

func (tx *transaction) dropLocked(ctx context.Context, search *reading) error {
	search.kept = nil
	// A search that the transaction could not be sent has no rows coming.
	for search.sent && !search.closed {
		select {
		case _, open := <-search.rows:
			search.closed = !open
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	tx.forgetLocked(search)
	return nil
}

// commit asks for the transaction to be committed and waits for the database
// to say that it has been, which is the only thing that makes its writes
// safe to report: a commit can fail, and until it is made the end of the
// context still rolls back. A commit is not asked of a transaction whose
// context has ended, as that has rolled it back. A [Database] that is not a
// [sodium.Committer] commits when the channel is closed, and cannot say how
// that went.
func (tx *transaction) commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !tx.answers {
		tx.mutex.Lock()
		defer tx.mutex.Unlock()
		if tx.ended {
			return ErrTransactionEnded
		}
		tx.committing = true
		tx.ended = true
		close(tx.jobs)
		return nil
	}
	commit := sodium.NewCommit()
	if err := tx.send(ctx, commit, nil); err != nil {
		return err
	}
	tx.mutex.Lock()
	tx.committing = true
	tx.mutex.Unlock()
	_, err := commit.Wait(ctx)
	return err
}

// end the transaction by closing its channel, if a commit has not closed it
// already. For a [sodium.Committer] that rolls it back unless a commit was
// sent, and for any other [Database] a nil [sodium.Job] is announced first to
// roll back, as closing the channel is how it is told to commit. An operation
// made with its context after that fails.
func (tx *transaction) end(ctx context.Context, announce bool) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	if tx.ended {
		return
	}
	if announce && !tx.committing {
		// A search left unread by whoever started it would keep the database
		// from taking anything else.
		if tx.reading != nil {
			tx.dropLocked(ctx, tx.reading)
		}
		select {
		case tx.jobs <- nil:
		case <-ctx.Done():
		}
	}
	tx.ended = true
	close(tx.jobs)
}

// Manage runs fn as a transaction, writes made with the context it is given are
// committed if fn returns nil and rolled back if it fails or panics. With a [Database],
// they are also rolled back if the context ends before fn returns, in which case Manage
// returns the error of the context even if fn returned nil. With a [Database] that is a
// [sodium.Committer], nil is only returned once the database has said that the commit
// is made, after which nothing rolls the writes back, the end of the context included.
// A commit that the database could not make is returned as its error, and nothing is
// kept. Only when it is the context that ends while the database is committing does
// the error not say whether the writes were kept. Any other [Database] is told to
// commit by closing the channel of the transaction, and cannot say how that went.
//
// A nested call joins the transaction held by its context, so its failure only rolls
// back if it fails the outer fn too. A transaction is on one [Database], or on the
// maps that have none: a write to any other [Map] made with its context is not part
// of it, and is neither committed nor rolled back with it. Maps are on one [Database]
// if one call to [Open] made them, or if their databases are equal. A [Database] that
// cannot be compared, such as a struct that holds a slice, is never taken for another,
// so maps of it from different calls to [Open] do not share a transaction.
//
// Without a [Database], transactions run one at a time, but other readers see their
// writes, and a rollback leaves alone any row that has been written from outside of the
// transaction. A transaction waits for its turn until its context ends, so a nested call
// must be given the context passed to fn: given the outer context, it waits for its own
// transaction.
//
// The context passed to fn is for fn. With a [Database], once Manage has returned, an
// operation on a [Map] of the transaction that is made with it fails with
// [ErrTransactionEnded].
func (m *Map[K, V]) Manage(ctx context.Context, level TransactionLevel, fn func(context.Context) error) error {
	if m.db == nil {
		return manageRAM(ctx, fn)
	}
	if tx := m.joined(ctx); tx != nil {
		if tx.over() {
			return ErrTransactionEnded
		}
		return fn(ctx)
	}
	tx, err := m.begin(ctx, level)
	if err != nil {
		return xray.New(err)
	}
	defer tx.end(ctx, true)
	tx.outer, _ = ctx.Value(transactionKey{}).(*transaction)
	if err := fn(context.WithValue(ctx, transactionKey{}, tx)); err != nil {
		return err
	}
	return xray.New(tx.commit(ctx))
}

// over reports whether the transaction has ended.
func (tx *transaction) over() bool {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()
	return tx.ended
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
	name := strings.ToLower(field.Name)
	if tag := field.Tag.Get("txt"); tag != "" {
		name = tag
	}
	if tag := field.Tag.Get("sql"); tag != "" {
		name, _, _ = strings.Cut(tag, ",")
	}
	if len(path) > 0 {
		name = strings.Join(path, "_") + "_" + name
	}
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

// keyed is a query for the value at key, if it passes the check.
func keyed[K comparable, V any](key K, check CheckFunc[V]) QueryFunc[K, V] {
	return func(k *K, v *V) Query {
		query := Query{Index(k).Equals(key)}
		if check != nil {
			query = append(query, check(v)...)
		}
		return query
	}
}

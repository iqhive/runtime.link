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

	"runtime.link/api/xray"
	"runtime.link/sql/std/sodium"
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
}

func (m *Map[K, V]) open(db Database, table Table) {
	name, index, ok := strings.Cut(string(table), "/")
	if !ok {
		index = "id"
	}
	key := reflect.StructField{
		Name: index,
		Type: reflect.TypeOf([0]K{}).Elem(),
	}
	if key.Type.Kind() == reflect.Struct {
		key.Anonymous = true
	}
	val := reflect.StructField{
		Name: "value",
		Type: reflect.TypeOf([0]V{}).Elem(),
	}
	if val.Type.Kind() == reflect.Struct {
		val.Anonymous = true
	}
	sentinals.index.assert(name, key, new(K))
	sentinals.value.assert(name, val, new(V))
	*m = Map[K, V]{
		to: sodium.Table{
			Name:  name,
			Index: columnsOf(key),
			Value: columnsOf(val),
		},
		db: db,
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
// table.
func OpenTable[K comparable, V any](db Database, table sodium.Table) Map[K, V] {
	if db == &stub {
		db = nil
	}
	return Map[K, V]{
		to: table,
		db: db,
	}
}

// Add the specified value to the [Map], if possible, a key will be
// automatically selected. If a key is required in order to add to
// the [Map], an [ErrInsertOnly] error will be returned. If the K
// type implements Randomize, this method will be called on an zero
// value of K to generate a random key.
func (m *Map[K, V]) Add(ctx context.Context, value V) (K, error) {
	if m.db == nil {
		return m.kv.Add(ctx, value)
	}
	var key K
	if r, ok := any(key).(interface {
		Randomize()
	}); ok {
		r.Randomize()
	}
	tx, err := m.manage(ctx)
	if err != nil {
		return key, xray.New(err)
	}
	var values = ValuesOf(key)
	insert := m.db.Insert(m.to, values, bool(Create), ValuesOf(value))
	select {
	case tx <- insert:
	case <-ctx.Done():
		return key, xray.New(ctx.Err())
	}
	close(tx)
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
	if m.db == nil {
		return m.kv.Insert(ctx, key, flag, value)
	}
	var zero K
	if key == zero {
		return ErrInvalidKey
	}
	tx, err := m.manage(ctx)
	if err != nil {
		return xray.New(err)
	}
	insert := m.db.Insert(m.to, ValuesOf(key), bool(flag), ValuesOf(value))
	select {
	case tx <- insert:
	case <-ctx.Done():
		return xray.New(ctx.Err())
	}
	close(tx)
	n, err := insert.Wait(ctx)
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

func (m *Map[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
	if m.db == nil {
		return m.kv.Output(ctx, query, stats)
	}
	key := sentinals.index[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]K{}).Elem()}].(*K)
	val := sentinals.value[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]V{}).Elem()}].(*V)
	var sql Query
	if query != nil {
		sql = query(key, val)
	}
	var ptr Stats
	if stats != nil {
		ptr = stats(key, val)
	} else {
		return nil // no stats, nothing to do.
	}
	get := make(chan []sodium.Value, 1)

	var out sodium.Stats
	for _, stat := range ptr {
		switch stat := stat.(type) {
		case counter[atomic.Int32]:
			out = append(out, stat.calc)
		case counter[atomic.Int64]:
			out = append(out, stat.calc)
		case counter[atomic.Uint32]:
			out = append(out, stat.calc)
		case counter[atomic.Uint64]:
			out = append(out, stat.calc)
		default:
			return xray.New(errors.New("unsupported stat type"))
		}
	}
	do := m.db.Output(m.to, sodium.Query(sql), sodium.Stats(out), get)
	tx, err := m.manage(ctx)
	if err != nil {
		return xray.New(err)
	}
	select {
	case tx <- do:
		close(tx)
	case <-ctx.Done():
		close(tx)
		return xray.New(ctx.Err())
	}
	if _, err := do.Wait(ctx); err != nil {
		return xray.New(err)
	}
	select {
	case output := <-get:
		for i, stat := range ptr {
			switch stat := stat.(type) {
			case counter[atomic.Int32]:
				stat.ptr.Store(int32(sodium.Values.Uint64.Get(output[i])))
			case counter[atomic.Int64]:
				stat.ptr.Store(int64(sodium.Values.Uint64.Get(output[i])))
			case counter[atomic.Uint32]:
				stat.ptr.Store(uint32(sodium.Values.Uint64.Get(output[i])))
			case counter[atomic.Uint64]:
				stat.ptr.Store(sodium.Values.Uint64.Get(output[i]))
			}
		}
		return nil
	case <-ctx.Done():
		return xray.New(ctx.Err())
	}
}

func (m *Map[K, V]) Search(ctx context.Context, query QueryFunc[K, V], issue *error) iter.Seq2[K, V] {
	if m.db == nil {
		return m.kv.Search(ctx, query, issue)
	}
	key := sentinals.index[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]K{}).Elem()}].(*K)
	val := sentinals.value[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]V{}).Elem()}].(*V)
	var sql Query
	if query != nil {
		sql = query(key, val)
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
		close(tx)
		for values := range ch {
			var key K
			var val V
			if values == nil {
				_, err := do.Wait(ctx)
				if err != nil {
					*issue = xray.New(err)
					return
				}
				if !yield(key, val) {
					return
				}
				continue
			}
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
	err := m.Output(ctx, nil, func(k *K, v *V) Stats { return Stats{Count(&count)} })
	return int(count.Load()), err
}

// Length returns the total number of entries in the map.
func (m *Map[K, V]) Length(ctx context.Context) (int, error) {
	return m.Count(ctx, nil)
}

// Lookup the specified key in the map and return the value associated with it, if
// the value is not present in the map, the resulting boolean will be false.
func (m *Map[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	if m.db == nil {
		return m.kv.Lookup(ctx, key)
	}
	var zero K
	var value V
	if key == zero {
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
// Boolean returned is true if a value was deleted this way.
func (m *Map[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
	if m.db == nil {
		return m.kv.Delete(ctx, key, check)
	}
	var zero K
	if key == zero {
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
// be deleted, otherwise the operation will fail. Unsafe because a large amount of
// data can be permanently deleted this way.
func (m *Map[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	if m.db == nil {
		return m.kv.UnsafeDelete(ctx, query)
	}
	if query == nil {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
	key := sentinals.index[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]K{}).Elem()}].(*K)
	val := sentinals.value[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]V{}).Elem()}].(*V)
	var sql = query(key, val)
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
	close(tx)
	result, err := do.Wait(ctx)
	return result, xray.New(err)
}

// Update each value in the map that matches the given query with the given patch. The number of
// values that were updated is returned, along with any error that occurred.
func (m *Map[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	if m.db == nil {
		return m.kv.Update(ctx, query, patch)
	}
	if query == nil {
		return 0, xray.New(errors.New("please provide a query with a finite range"))
	}
	key := sentinals.index[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]K{}).Elem()}].(*K)
	val := sentinals.value[sentinalKey{table: m.to.Name, rtype: reflect.TypeOf([0]V{}).Elem()}].(*V)
	sql := query(key, val)
	mod := patch(val)
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
	close(tx)
	result, err := do.Wait(ctx)
	return result, xray.New(err)
}

// Mutate the value at the specified key in the map. The [CheckFunc] is called with
// the current value at the specified key, if the [CheckFunc] returns true, then the
// [PatchFunc] is called with the current value at the specified key. The [PatchFunc]
// should return the modifications to be made to the value at the specified key.
func (m *Map[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
	if m.db == nil {
		return m.kv.Mutate(ctx, key, check, patch)
	}
	var zero K
	if key == zero {
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
	count, err := m.Update(ctx, query, patch)
	if err != nil {
		return false, xray.New(err)
	}
	return count > 0, nil
}

var transactionLock sync.Mutex

type transactionKey struct{}

func (m *Map[K, V]) manage(ctx context.Context) (chan<- sodium.Job, error) {
	if tx, ok := ctx.Value(transactionKey{}).(chan<- sodium.Job); ok {
		return tx, nil
	}
	return m.db.Manage(ctx, 0)
}

func (m *Map[K, V]) Manage(ctx context.Context, level TransactionLevel, fn func(context.Context) error) error {
	if m.db == nil {
		transactionLock.Lock()
		defer transactionLock.Unlock()
		return fn(ctx)
	}
	if _, ok := ctx.Value(transactionKey{}).(chan<- sodium.Job); !ok {
		var err error
		tx, err := m.db.Manage(ctx, level)
		if err != nil {
			return xray.New(err)
		}
		ctx = context.WithValue(ctx, transactionKey{}, tx)
	}
	return fn(ctx)
}

var smutex sync.RWMutex
var mirror = make(map[any][]sodium.Column)

var sentinals struct {
	index sentinal
	value sentinal
}

type sentinal map[sentinalKey]any

type sentinalKey struct {
	rtype reflect.Type
	table string
}

func (s *sentinal) assert(table string, field reflect.StructField, arg any) {
	smutex.Lock()
	defer smutex.Unlock()
	if *s == nil {
		*s = make(map[sentinalKey]any)
	}
	_, ok := (*s)[sentinalKey{
		table: table,
		rtype: field.Type,
	}]
	if ok {
		return
	}
	(*s)[sentinalKey{
		table: table,
		rtype: field.Type,
	}] = arg
	s.walk(table, field, reflect.ValueOf(arg).Elem())
}

func (s *sentinal) walk(table string, field reflect.StructField, arg reflect.Value, path ...string) {
	name := strings.ToLower(field.Name)
	if tag := field.Tag.Get("txt"); tag != "" {
		name = tag
	}
	if tag := field.Tag.Get("sql"); tag != "" {
		name = tag
	}
	if len(path) > 0 {
		name = strings.Join(path, "_") + "_" + name
	}
	mirror[arg.Addr().Interface()] = columnsOf(field, path...)
	switch field.Type.Kind() {
	case reflect.Struct:
		for i := 0; i < field.Type.NumField(); i++ {
			if !field.Type.Field(i).IsExported() {
				continue
			}
			promote := append(path, name)
			if field.Anonymous {
				promote = path
			}
			s.walk(table, field.Type.Field(i), arg.Field(i), promote...)
		}
	case reflect.Array:
		for i := 0; i < field.Type.Len(); i++ {
			vfield := reflect.StructField{
				Name: fmt.Sprintf("%s%d", name, i+1),
				Type: field.Type.Elem(),
			}
			s.walk(table, vfield, arg.Index(i), path...)
		}
	}
}

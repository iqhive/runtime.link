package sql

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"runtime.link/api/xray"
	"runtime.link/sql/std/sodium"
	"runtime.link/xyz"
)

// Database represents a connection to a SQL database.
type Database = sodium.Database

// Table name.
type Table string

// Map represents a distinct mapping of data stored in a [Database].
type Map[K comparable, V any] struct {
	to sodium.Table
	db sodium.Database
}

// Open a new [Map] from the given [Database]. The table schema is
// derived from the key and value types 'K' and 'V', following the
// same rules as [ValuesOf]. A 'sql' or else, a 'txt' tag controls
// the name of the column. If no tag is specified, the ToLower(name)
// of the field is used. If the key is not a struct, the column name
// is 'id', otherwise it is treated as a composite key across each
// field. If the value is not a struct, the column name is 'value'.
// Nested structures are named with an underscore used to seperate
// the field path unless the structure is embedded, in which case
// the nested fields are promoted. Arrays elements are suffixed by
// their index.
func Open[K comparable, V any](db Database, table Table) Map[K, V] {
	key := reflect.StructField{
		Name: "id",
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
	sentinals.index.assert(key, new(K))
	sentinals.value.assert(val, new(V))
	return Map[K, V]{
		to: sodium.Table{
			Name:  string(table),
			Index: columnsOf(key),
			Value: columnsOf(val),
		},
		db: db,
	}
}

// OpenTable a new [Map] from the given [Database] and specified
// table.
func OpenTable[K comparable, V any](db sodium.Database, table sodium.Table) Map[K, V] {
	return Map[K, V]{
		to: table,
		db: db,
	}
}

// Insert a new value into the [Map] at the given key. The given [Flag] determines
// how the value is inserted. If the [Flag] is [Upsert], the value will overwrite
// any existing value at the given key. If the [Flag] is [Create], the value will
// only be inserted if there is no existing value at the given key, otherwise an
// error will be returned.
func (m Map[K, V]) Insert(ctx context.Context, key K, flag Flag, value V) error {
	tx, err := m.db.Manage(ctx, 0)
	if err != nil {
		return xray.Error(err)
	}
	insert := m.db.Insert(m.to, ValuesOf(key), bool(flag), ValuesOf(value))
	select {
	case tx <- insert:
	case <-ctx.Done():
		return xray.Error(ctx.Err())
	}
	close(tx)
	n, err := insert.Wait(ctx)
	if err != nil {
		if err == ErrDuplicate {
			return err
		}
		return xray.Error(err)
	}
	if n == -1 {
		return ErrDuplicate
	}
	return nil
}

func (m Map[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
	return errors.New("not implemented")
}

func (m Map[K, V]) Search(ctx context.Context, query QueryFunc[K, V]) Chan[K, V] {
	key := sentinals.index[reflect.TypeOf([0]K{}).Elem()].(*K)
	val := sentinals.value[reflect.TypeOf([0]V{}).Elem()].(*V)
	sql := query(key, val)
	out := make(Chan[K, V])
	go func() {
		defer close(out)
		ch := make(chan []sodium.Value, 64)
		do := m.db.Search(m.to, sodium.Query(sql), ch)
		tx, err := m.db.Manage(ctx, 0)
		if err != nil {
			select {
			case out <- xyz.Trio[K, V, error]{Z: err}:
				return
			case <-ctx.Done():
				return
			}
		}
		select {
		case tx <- do:
		case <-ctx.Done():
			close(tx)
			return
		}
		close(tx)
		for values := range ch {
			var key K
			var val V
			if values == nil {
				_, err := do.Wait(ctx)
				select {
				case out <- xyz.NewTrio(key, val, error(err)):
					continue
				case <-ctx.Done():
					return
				}
			}
			decode(reflect.ValueOf(&key), values[:len(m.to.Index)])
			decode(reflect.ValueOf(&val), values[len(m.to.Index):])
			select {
			case out <- xyz.NewTrio(key, val, error(nil)):
				continue
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Lookup the specified key in the map and return the value associated with it, if
// the value is not present in the map, the resulting boolean will be false.
func (m Map[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	result := m.Search(ctx, func(primary *K, _ *V) Query {
		return Query{
			Index(primary).Equals(key),
			Slice(0, 1),
		}
	})
	var zero V
	select {
	case result, ok := <-result:
		_, val, err := result.Get()
		if err != nil {
			return val, ok, xray.Error(err)
		}
		if !ok {
			return zero, false, nil
		}
		return val, true, nil
	case <-ctx.Done():
		return zero, false, xray.Error(ctx.Err())
	}
}

// Delete the value at the specified key in the map if the specified check passes.
// Boolean returned is true if a value was deleted this way.
func (m Map[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
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
		return false, xray.Error(err)
	}
	return count > 0, nil
}

// UnsafeDelete each value in the map that matches the given query. The number of
// values that were deleted is returned, along with any error that occurred. The
// query must include a slice operation that limits the number of values that can
// be deleted, otherwise the operation will fail. Unsafe because a large amount of
// data can be permanently deleted this way.
func (m Map[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	key := sentinals.index[reflect.TypeOf([0]K{}).Elem()].(*K)
	val := sentinals.value[reflect.TypeOf([0]V{}).Elem()].(*V)
	sql := query(key, val)
	do := m.db.Delete(m.to, sodium.Query(sql))
	tx, err := m.db.Manage(ctx, 0)
	if err != nil {
		return 0, xray.Error(err)
	}
	select {
	case tx <- do:
	case <-ctx.Done():
		return 0, xray.Error(ctx.Err())
	}
	close(tx)
	result, err := do.Wait(ctx)
	return result, xray.Error(err)
}

// Update each value in the map that matches the given query with the given patch. The number of
// values that were updated is returned, along with any error that occurred.
func (m Map[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	if query == nil {
		return 0, xray.Error(errors.New("please provide a query with a finite range"))
	}
	key := sentinals.index[reflect.TypeOf([0]K{}).Elem()].(*K)
	val := sentinals.value[reflect.TypeOf([0]V{}).Elem()].(*V)
	sql := query(key, val)
	mod := patch(val)
	do := m.db.Update(m.to, sodium.Query(sql), sodium.Patch(mod))
	tx, err := m.db.Manage(ctx, 0)
	if err != nil {
		return 0, xray.Error(err)
	}
	select {
	case tx <- do:
	case <-ctx.Done():
		return 0, xray.Error(ctx.Err())
	}
	close(tx)
	result, err := do.Wait(ctx)
	return result, xray.Error(err)
}

// Mutate the value at the specified key in the map. The [CheckFunc] is called with
// the current value at the specified key, if the [CheckFunc] returns true, then the
// [PatchFunc] is called with the current value at the specified key. The [PatchFunc]
// should return the modifications to be made to the value at the specified key.
func (m Map[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
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
		return false, xray.Error(err)
	}
	return count > 0, nil
}

var smutex sync.RWMutex
var mirror = make(map[any][]sodium.Column)

var sentinals struct {
	index sentinal
	value sentinal
}

type sentinal map[reflect.Type]any

func (s *sentinal) assert(field reflect.StructField, arg any) {
	smutex.Lock()
	defer smutex.Unlock()
	if *s == nil {
		*s = make(map[reflect.Type]any)
	}
	_, ok := (*s)[field.Type]
	if ok {
		return
	}
	(*s)[field.Type] = arg
	s.walk(field, reflect.ValueOf(arg).Elem())
}

func (s *sentinal) walk(field reflect.StructField, arg reflect.Value, path ...string) {
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
			promote := append(path, name)
			if field.Type.Field(i).Anonymous {
				promote = path
			}
			s.walk(field.Type.Field(i), arg.Field(i), promote...)
		}
	case reflect.Array:
		for i := 0; i < field.Type.Len(); i++ {
			s.walk(field.Type.Field(i), arg.Index(i), append(path, name+strconv.Itoa(i))...)
		}
	}
}

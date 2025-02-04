package kvs

import (
	"context"
	"iter"
	"reflect"
	"sync"
)

type Map[K comparable, V any] struct {
	Lookup func(context.Context, K) (V, bool, error)
	Commit func(ctx context.Context, insert map[K]V, delete ...K) error
	Values func(context.Context, *error, Filter[K]) iter.Seq2[K, V]
}

func (m *Map[K, V]) open(db Database, format Format) {
	*m = Map[K, V]{
		Lookup: func(ctx context.Context, key K) (V, bool, error) {
			var val V
			ok, err := db.Lookup(ctx, format, key, &val)
			return val, ok, err
		},
		Commit: func(ctx context.Context, insert map[K]V, delete ...K) error {
			for key, val := range insert {
				if err := db.Insert(ctx, format, key, val); err != nil {
					return err
				}
			}
			for _, key := range delete {
				if err := db.Delete(ctx, format, key); err != nil {
					return err
				}
			}
			return nil
		},
		Values: func(ctx context.Context, err *error, filter Filter[K]) iter.Seq2[K, V] {
			return func(yield func(K, V) bool) {
				for loader := range db.Values(ctx, err, format, Filter[any]{
					Prefix: filter.Prefix,
					Cursor: filter.Cursor,
					Offset: filter.Offset,
				}) {
					var key K
					var val V
					loader(&key, &val)
					if !yield(key, val) {
						break
					}
				}
			}
		},
	}
}

func (m Map[K, V]) All(ctx context.Context, err *error) iter.Seq2[K, V] {
	return m.Values(ctx, err, Filter[K]{})
}

func (m Map[K, V]) Get(ctx context.Context, key K) (V, error) {
	val, _, err := m.Lookup(ctx, key)
	return val, err
}

func (m Map[K, V]) Set(ctx context.Context, key K, val V) error {
	return m.Commit(ctx, map[K]V{key: val})
}

func (m Map[K, V]) Del(ctx context.Context, key K) error {
	return m.Commit(ctx, nil, key)
}

type Filter[K comparable] struct {
	Prefix K
	Cursor K
	Offset int
}

func New[K comparable, V any]() Map[K, V] {
	var DB sync.Map
	return Map[K, V]{
		Lookup: func(ctx context.Context, key K) (V, bool, error) {
			val, ok := DB.Load(key)
			return val.(V), ok, nil
		},
		Commit: func(ctx context.Context, insert map[K]V, delete ...K) error {
			for key, val := range insert {
				DB.Store(key, val)
			}
			for _, key := range delete {
				DB.Delete(key)
			}
			return nil
		},
		Values: func(ctx context.Context, err *error, filter Filter[K]) iter.Seq2[K, V] {
			return func(yield func(K, V) bool) {
				DB.Range(func(key, value any) bool {
					return yield(key.(K), value.(V))
				})
			}
		},
	}
}

type Format string

type Database interface {
	Lookup(ctx context.Context, fmt Format, key, val any) (bool, error)
	Insert(ctx context.Context, fmt Format, key, val any) error
	Delete(ctx context.Context, fmt Format, key any) error
	Values(context.Context, *error, Format, Filter[any]) iter.Seq[func(any, any)]
}

func Open[Schema any](db Database) *Schema {
	var schema = new(Schema)
	var rvalue = reflect.ValueOf(schema).Elem()
	var rtype = rvalue.Type()
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if !field.IsExported() {
			continue
		}
		var format = Format(field.Tag.Get("kvs"))
		opener, ok := rvalue.Field(i).Addr().Interface().(interface{ open(Database, Format) })
		if ok {
			opener.open(db, format)
		}
	}
	return schema
}

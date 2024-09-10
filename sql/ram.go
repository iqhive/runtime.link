package sql

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"

	"runtime.link/sql/std/sodium"
	"runtime.link/xyz"
)

var stub struct {
	Database
}

func New() Database { return &stub }

type ram[K comparable, V any] struct {
	sync.Mutex

	Map sync.Map

	id atomic.Uint64
}

func (m *ram[K, V]) match(expr []sodium.Expression) bool {
	for _, e := range expr {
		switch xyz.ValueOf(e) {
		case sodium.Expressions.Value:
			if !sodium.Expressions.Value.Get(e) {
				return false
			}
		}
	}
	return true
}

func (r *ram[K, V]) Add(ctx context.Context, value V) (K, error) {
	rtype := reflect.TypeOf([0]K{}).Elem()
	var u64 uint64
	var key K
	switch rtype.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		u64 = r.id.Add(1)
		reflect.ValueOf(&key).Elem().SetInt(int64(u64))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u64 = r.id.Add(1)
		reflect.ValueOf(&key).Elem().SetUint(u64)
	default:
		return key, ErrInsertOnly
	}
	_, exists := r.Map.LoadOrStore(key, value)
	if exists {
		return key, ErrDuplicate
	}
	return key, nil
}

func (r *ram[K, V]) Insert(ctx context.Context, key K, flag Flag, value V) error {
	if flag {
		r.Map.Store(key, value)
		return nil
	}
	_, exists := r.Map.LoadOrStore(key, value)
	if exists {
		return ErrDuplicate
	}
	return nil
}

func (m *ram[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
	m.Map.Range(func(ikey, ival any) bool {
		key := ikey.(K)
		val := ival.(V)
		if query != nil && !m.match(query(&key, &val)) {
			return true
		}
		stats(&key, &val)
		return true
	})
	return nil
}

func (m *ram[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	var count int
	var limit = query.limit()
	m.Map.Range(func(ikey, ival any) bool {
		key := ikey.(K)
		val := ival.(V)
		if !m.match(query(&key, &val)) {
			return true
		}
		if count >= limit.From {
			m.Map.Delete(key)
		}
		count++
		return count < limit.Upto
	})
	return count, nil
}

func (m *ram[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	var count int
	var limit = query.limit()
	m.Map.Range(func(ikey, ival any) bool {
		key := ikey.(K)
		val := ival.(V)
		if !m.match(query(&key, &val)) {
			return true
		}
		if count >= limit.From {
			patch(&val)
			m.Map.Store(key, val)
		}
		count++
		return count < limit.Upto
	})
	return count, nil
}

func (m *ram[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
	m.Lock()
	defer m.Unlock()
	ival, exists := m.Map.Load(key)
	if !exists {
		return false, nil
	}
	val := ival.(V)
	if check != nil && !m.match(check(&val)) {
		return false, nil
	}
	patch(&val)
	m.Map.Store(key, val)
	return true, nil
}

func (m *ram[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
	m.Lock()
	defer m.Unlock()
	ival, exists := m.Map.Load(key)
	if !exists {
		return false, nil
	}
	val := ival.(V)
	if check != nil && !m.match(check(&val)) {
		return false, nil
	}
	m.Map.Delete(key)
	return true, nil
}

func (m *ram[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	ival, exists := m.Map.Load(key)
	if !exists {
		var zero V
		return zero, false, nil
	}
	return ival.(V), true, nil
}

func (m *ram[K, V]) Search(ctx context.Context, query QueryFunc[K, V]) Chan[K, V] {
	ch := make(Chan[K, V])
	go func() {
		var ordering = query.ordered()
		var matching []K
		var snapshot []V
		m.Map.Range(func(ikey, ival any) bool {
			key := ikey.(K)
			val := ival.(V)
			if query != nil && !m.match(query(&key, &val)) {
				return true
			}
			if ordering {
				matching = append(matching, key)
				snapshot = append(snapshot, val)
			} else {
				ch <- xyz.NewTrio[K, V, error](key, val, nil)
			}
			return true
		})
		if ordering && len(matching) > 0 {
			var sortingKey K
			var sortingVal V
			var orders []exOrdering
			for _, expr := range query(&sortingKey, &sortingVal) {
				if xyz.ValueOf(expr) == sodium.Expressions.Order {
					orders = append(orders, exOrderExpressions.Functional.Get(sodium.Expressions.Order.Get(expr)))
				}
			}
			sort.Slice(matching, func(i, j int) bool {
				for _, order := range orders {
					sortingKey = matching[i]
					sortingVal = snapshot[i]
					order.Load()
					sortingKey = matching[j]
					sortingVal = snapshot[j]
					if !order.Less() {
						return false
					}
				}
				return true
			})
			for _, key := range matching {
				val, ok := m.Map.Load(key)
				if ok {
					ch <- xyz.NewTrio[K, V, error](key, val.(V), nil)
				}
			}
		}
		close(ch)
	}()
	return ch
}

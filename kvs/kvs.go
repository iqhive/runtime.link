// Package kvs provides an interface for key-value stores.
package kvs

import (
	"context"
	"sync"

	"runtime.link/xyz"
)

// Map interface to a key-value store.
type Map[K comparable, V any] interface {
	// Del deletes a key, the operation is idempotent. It has HTTP
	// DELETE semantics.
	Del(context.Context, K) error
	// All returns a result iterator over all key-value pairs.
	All(context.Context) chan xyz.Trio[K, V, error]
	// Has returns true if the key exists.
	Get(context.Context, K) (V, bool, error)
	// Set a key overwriting any existing value at the given key. This
	// operation is idempotent. It has HTTP PUT semantics.
	Set(context.Context, K, V) error
	// Len returns the number of key-value pairs in the map.
	Len(context.Context) (int, error)
}

// New returns a new map-backed [Map].
func New[K comparable, V any]() Map[K, V] {
	return &ram[K, V]{
		values: make(map[K]V),
	}
}

type ram[K comparable, V any] struct {
	mutex  sync.RWMutex
	values map[K]V
}

func (ram *ram[K, V]) Del(ctx context.Context, key K) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	delete(ram.values, key)
	return nil
}

func (ram *ram[K, V]) All(ctx context.Context) chan xyz.Trio[K, V, error] {
	var ch = make(chan xyz.Trio[K, V, error])
	go func() {
		ram.mutex.RLock()
		defer ram.mutex.RUnlock()
		defer close(ch)
		for k, v := range ram.values {
			ch <- xyz.NewTrio(k, v, error(nil))
		}
	}()
	return ch
}

func (ram *ram[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()
	val, ok := ram.values[key]
	return val, ok, nil
}

func (ram *ram[K, V]) Set(ctx context.Context, key K, value V) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	ram.values[key] = value
	return nil
}

func (ram *ram[K, V]) Len(ctx context.Context) (int, error) {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()
	return len(ram.values), nil
}

package ram

import (
	"iter"

	"runtime.link/via"
)

type Map[K comparable, V any] via.Any[MapProxy[K, V]]

func (m Map[K, V]) Index(key K) V {
	val, _ := via.Methods(m).Index(via.Internal(m), key)
	return val
}

func (m Map[K, V]) SetIndex(key K, val V) {
	via.Methods(m).SetIndex(via.Internal(m), key, val)
}

type MapProxy[K comparable, V any] interface {
	via.API

	Index(via.CachedState, K) (V, bool)
	SetIndex(via.CachedState, K, V)
	Delete(via.CachedState, K)
	Clear(via.CachedState)
	Iter(via.CachedState) iter.Seq2[K, V]
}

func MapInMemory[K comparable, V any](m map[K]V) Map[K, V] {
	return via.New[Map[K, V], MapProxy[K, V]](goMemoryMap[K, V](m), via.CachedState{})
}

type goMemoryMap[K comparable, V any] map[K]V

func (m goMemoryMap[K, V]) Index(cache via.CachedState, key K) (V, bool) {
	val, ok := m[key]
	return val, ok
}

func (m goMemoryMap[K, V]) SetIndex(cache via.CachedState, key K, val V) {
	m[key] = val
}

func (m goMemoryMap[K, V]) Delete(cache via.CachedState, key K) {
	delete(m, key)
}

func (m goMemoryMap[K, V]) Clear(cache via.CachedState) {
	clear(m)
}

func (m goMemoryMap[K, V]) Iter(cache via.CachedState) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, val := range m {
			if !yield(key, val) {
				break
			}
		}
	}
}

func (m goMemoryMap[K, V]) Alive(cache via.CachedState) bool { return m != nil }

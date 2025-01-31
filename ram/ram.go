package ram

import (
	"runtime.link/via"
)

type Address string

type Any struct {
	cache via.CachedState
	proxy any
}

func NewAny(val any) Any {
	return Any{proxy: val}
}

/*

type Uint via.Any[UintProxy]

type UintProxy interface {
	via.API

	Uint64(via.Cache) uint64
}

type goMemoryUint struct{}

func (goMemoryUint) Uint64(cache via.Cache) uint64 { return via.Cached[uint64](cache) }
func (goMemoryUint) Alive(cache via.Cache) bool    { return via.Cached[uint64](cache) != 0 }

func NewUint[T ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr](val T) Uint {
	return via.New[Uint, UintProxy](goMemoryUint{}, via.NewCache(val))
}

type Float via.Any[FloatProxy]

type FloatProxy interface {
	via.API

	Float64(via.Cache) float64
}

type goMemoryFloat struct{}

func (goMemoryFloat) Float64(cache via.Cache) float64 { return via.Cached[float64](cache) }
func (goMemoryFloat) Alive(cache via.Cache) bool      { return via.Cached[float64](cache) != 0 }

func NewFloat[T ~float32 | ~float64](val T) Float {
	return via.New[Float, FloatProxy](goMemoryFloat{}, via.NewCache(val))
}

type Complex via.Any[ComplexProxy]

type ComplexProxy interface {
	via.API

	Complex128(via.Cache) complex128
}

type goMemoryComplex struct{}

func (goMemoryComplex) Complex128(cache via.Cache) complex128 { return via.Cached[complex128](cache) }
func (goMemoryComplex) Alive(cache via.Cache) bool            { return via.Cached[complex128](cache) != 0 }

func NewComplex[T ~complex64 | ~complex128](val T) Complex {
	return via.New[Complex, ComplexProxy](goMemoryComplex{}, via.NewCache(val))
}


type Slice[V any] via.Any[SliceProxy[V]]

func (slice Slice[V]) Len() int {
	return slice.proxy.Len(slice.cache)
}

type SliceProxy[V any] interface {
	via.API

	Len(via.Cache) int
	Cap(via.Cache) int
	Index(via.Cache, int) V
	SetIndex(via.Cache, int, V)
	Append(via.Cache, V) Slice[V]
	AppendSlice(via.Cache, Slice[V]) Slice[V]
	Iter(via.Cache) iter.Seq[V]
	Slice(via.Cache, int, int, int) Slice[V]
}

type goMemorySlice[V any] struct{ ptr *V }

func (s goMemorySlice[V]) Len(cache via.Cache) int { return int(via.Cached[[2]uint](cache)[0]) }
func (s goMemorySlice[V]) Cap(cache via.Cache) int { return int(via.Cached[[2]uint](cache)[1]) }

func (s goMemorySlice[V]) Index(cache via.Cache, index int) V {
	len := s.Len(cache)
	if index < 0 || index >= int(len) {
		panic("index out of bounds")
	}
	return *(*V)(unsafe.Add(unsafe.Pointer(s.ptr), index))
}

func (s goMemorySlice[V]) SetIndex(cache via.Cache, index int, val V) {
	len := s.Len(cache)
	if index < 0 || index >= int(len) {
		panic("index out of bounds")
	}
	*(*V)(unsafe.Add(unsafe.Pointer(s.ptr), index)) = val
}

func (s goMemorySlice[V]) Append(cache via.Cache, val V) Slice[V] {
	length, capacity := s.Len(cache), s.Cap(cache)
	slice := unsafe.Slice(s.ptr, int(length))[:int(length):int(capacity)]
	slice = append(slice, val)
	return via.New[Slice[V], SliceProxy[V]](goMemorySlice[V]{ptr: &slice[0]}, via.NewCache([2]uint{uint(len(slice)), uint(cap(slice))}))
}

func (s goMemorySlice[V]) AppendSlice(cache via.Cache, other Slice[V]) Slice[V] {
	if other.Len() == 0 {
		if s.ptr == nil {
			return Slice[V]{}
		}
		return via.New[Slice[V], SliceProxy[V]](goMemorySlice[V]{ptr: s.ptr}, cache)
	}
	lencap := via.Cached[[2]uint](cache)
	slice := unsafe.Slice(s.ptr, int(lencap[1]))[:int(lencap[0]):int(lencap[1])]
	for _, val := range other.Iter() {
		slice = append(slice, val)
	}
	return via.New[Slice[V], SliceProxy[V]](goMemorySlice[V]{ptr: &slice[0]}, via.NewCache([2]uint{uint(len(slice)), uint(cap(slice))}))
}

func (s goMemorySlice[V]) Iter(cache via.Cache) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, val := range unsafe.Slice(s.ptr, s.Len(cache)) {
			if !yield(val) {
				break
			}
		}
	}
}

func (s goMemorySlice[V]) Slice(cache via.Cache, start, end, capacity int) Slice[V] {
	slice := unsafe.Slice(s.ptr, s.Cap(cache))[start:end:capacity]
	return via.New[Slice[V], SliceProxy[V]](goMemorySlice[V]{ptr: &slice[0]}, via.NewCache([2]uint{uint(len(slice)), uint(cap(slice))}))
}

func (s goMemorySlice[V]) Alive(cache via.Cache) bool { return s.ptr != nil }

type Chan[V any] via.Any[ChanProxy[V]]

type ChanProxy[V any] interface {
	via.API

	Send(via.Cache, V)
	Recv(via.Cache) (V, bool)
}

type String via.Any[StringProxy]

type StringProxy interface {
	via.API

	String(via.Cache) string
	Slice(via.Cache, int, int) String
}

type Pointer[V any] via.Any[PointerProxy[V]]

type PointerProxy[V any] interface {
	via.API

	Get(via.Cache) CachedProxy[V]
	Set(via.Cache, V)
}

type Cached[V any] via.Any[CachedProxy[V]]

type CachedProxy[V any] interface {
	via.API

	Latest(via.Cache) V
	Cached(via.Cache) V
}

type Mutex via.Any[MutexProxy]

type MutexProxy interface {
	via.API

	Lock(via.Cache, time.Duration)
	Unlock(via.Cache, time.Duration)
	}*/

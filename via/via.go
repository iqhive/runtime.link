package via

import (
	"reflect"
	"unsafe"
)

type CachedState = struct {
	rtype reflect.Type
	value complex128
}

type Cacheable interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~int8 | ~int16 | ~int32 | ~int64 | ~int | ~float32 | ~float64 | ~complex64 | ~complex128 | ~bool |
		~[1]uint8 | ~[2]uint8 | ~[3]uint8 | ~[4]uint8 | ~[5]uint8 | ~[6]uint8 | ~[7]uint8 | ~[8]uint8 | ~[9]uint8 | ~[10]uint8 | ~[11]uint8 | ~[12]uint8 | ~[13]uint8 | ~[14]uint8 | ~[15]uint8 | ~[16]uint8 |
		~[1]uint16 | ~[2]uint16 | ~[3]uint16 | ~[4]uint16 | ~[5]uint16 | ~[6]uint16 | ~[7]uint16 | ~[8]uint16 |
		~[1]uint32 | ~[2]uint32 | ~[3]uint32 | ~[4]uint32 |
		~[1]uint64 | ~[2]uint64 | ~[1]uintptr | ~[2]uintptr |
		~[1]int8 | ~[2]int8 | ~[3]int8 | ~[4]int8 | ~[5]int8 | ~[6]int8 | ~[7]int8 | ~[8]int8 | ~[9]int8 | ~[10]int8 | ~[11]int8 | ~[12]int8 | ~[13]int8 | ~[14]int8 | ~[15]int8 | ~[16]int8 |
		~[1]int16 | ~[2]int16 | ~[3]int16 | ~[4]int16 | ~[5]int16 | ~[6]int16 | ~[7]int16 | ~[8]int16 |
		~[1]int32 | ~[2]int32 | ~[3]int32 | ~[4]int32 |
		~[1]int64 | ~[2]int64 | ~[1]int | ~[2]int | ~[1]uint | ~[2]uint |
		~[1]float32 | ~[2]float32 | ~[3]float32 | ~[4]float32 |
		~[1]float64 | ~[2]float64 | ~[1]complex64 | ~[2]complex64 |
		~[1]complex128
}

func NewCache[P any, T Cacheable](state T) CachedState {
	var value complex128
	*(*T)(unsafe.Pointer(&value)) = state
	return CachedState{
		rtype: reflect.TypeFor[P](),
		value: value,
	}
}
func Cached[P any, T Cacheable](state CachedState) T {
	if state.rtype != reflect.TypeFor[P]() {
		panic("invalid state: " + state.rtype.String() + " != " + reflect.TypeFor[P]().String())
	}
	return *(*T)(unsafe.Pointer(&state.value))
}

func TryCache(val any) (CachedState, bool) {
	if val == nil {
		return CachedState{}, false
	}
	if !canCache(reflect.ValueOf(val)) {
		return CachedState{}, false
	}
	return *(*CachedState)(unsafe.Pointer(&val)), true
}

func canCache(rvalue reflect.Value) bool {
	rtype := rvalue.Type()
	if rtype.Size() > 16 {
		return false
	}
	switch rtype.Kind() {
	case reflect.String, reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
		return false
	case reflect.Struct:
		for i := 0; i < rvalue.NumField(); i++ {
			field := rvalue.Field(i)
			if !canCache(field) {
				return false
			}
		}
	}
	return true
}

type Any[T API] struct {
	state complex128
	proxy T
}

func Methods[T isAny[I], I API](a T) I {
	return Any[I](a).proxy
}
func Internal[T isAny[I], I API](a T) CachedState {
	return CachedState{value: Any[I](a).state, rtype: reflect.TypeOf(Any[I](a).proxy)}
}

type isAny[T API] interface {
	~struct {
		state complex128
		proxy T
	}
}

type API interface {
	Alive(CachedState) bool
}

type goAPI struct{}

func (goAPI) Alive(CachedState) bool { return false }

func New[T isAny[I], I API](proxy I, cache CachedState) T {
	return T(Any[I]{
		state: cache.value,
		proxy: proxy,
	})
}

func Proxy[T isAny[I], I API, V API](val T, alloc func(T) (V, CachedState)) (V, CachedState) {
	value := Any[I](val)
	already, ok := any(value.proxy).(V)
	if ok && value.proxy.Alive(Internal(value)) {
		return already, Internal(value)
	}
	proxy, state := alloc(val)
	if trans, ok := any(value.proxy).(interface {
		Proxy(T)
	}); ok {
		trans.Proxy(New[T](any(proxy).(I), state))
	}
	return proxy, state
}

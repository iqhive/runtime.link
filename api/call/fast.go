package call

import (
	"iter"
	"reflect"
	"unsafe"
)

//go:nosplit
func fast_call() uint64

func unsafe_pointer(ptr unsafe.Pointer) unsafe.Pointer { return ptr }

//go:linkname noescape runtime.link/api/call.unsafe_pointer
//go:noescape
func noescape(p unsafe.Pointer) unsafe.Pointer

var fast_call_ptr = reflect.ValueOf(fast_call).Pointer()
var fast_call_func = unsafe.Pointer(&fast_call_ptr)

var testing_func_type = reflect.FuncOf(
	[]reflect.Type{reflect.TypeFor[unsafe.Pointer](), reflect.TypeFor[int]()},
	nil,
	false)

type Value struct {
	value complex128
	point unsafe.Pointer
	rtype reflect.Type
}

func (v Value) Int64() int64 {
	if v.rtype.Kind() != reflect.Int64 {
		panic("not an int64")
	}
	return *(*int64)(unsafe.Pointer(&v.value))
}

// Fast is experiemental and unsafe.
func Fast(fn any, arguments iter.Seq[reflect.Value]) iter.Seq[Value] {
	return func(yield func(Value) bool) {
		var hijack struct {
			r0 uint64
			fn uintptr
			sp uintptr
		}
		var hijack_ptr = noescape(unsafe.Pointer(&hijack))
		var value = reflect.ValueOf(fn)
		var rtype = value.Type()
		var args [8]reflect.Value
		var heap []reflect.Value
		var fits int
		for arg := range arguments {
			if fits < 8 {
				args[fits] = arg
				fits++
			} else {
				if heap == nil {
					heap = make([]reflect.Value, 8, 16)
					copy(heap, args[:fits])
				}
				heap = append(heap, arg)
			}
		}
		if rtype.NumOut() == 1 {
			switch rtype.Out(0).Kind() {
			case reflect.Int64:
				hijack.fn = value.Pointer()
				reflect.NewAt(rtype.In(0), hijack_ptr).Elem().Set(args[0])
				value = reflect.NewAt(testing_func_type, unsafe.Pointer(&fast_call_func)).Elem()
				args[0] = reflect.ValueOf(hijack_ptr)
				value.Call(args[:fits])
				var value Value
				*(*int64)(unsafe.Pointer(&value.value)) = *(*int64)(unsafe.Pointer(&hijack.r0))
				value.rtype = rtype.Out(0)
				yield(value)
				return
			}
		}
		var results []reflect.Value
		if heap == nil {
			results = value.Call(args[:fits])
		} else {
			results = value.Call(heap)
		}
		if yield == nil {
			return
		}
		for range results {
			if !yield(Value{}) {
				return
			}
		}
	}
}

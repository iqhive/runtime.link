package wasm

import (
	"errors"
	"reflect"
	"unsafe"

	"runtime.link/api"
)

//go:wasmimport runtime.link dlopen
//go:noescape
func dlopen(library string) uint64

//go:wasmimport runtime.link dlsym
//go:noescape
func dlsym(library uint64, symbol string) uint64

//go:wasmimport runtime.link string_len
func string_len(str_head, str_body uint64) uint32

//go:wasmimport runtime.link string_iter
func string_iter(yield uint64)

//go:wasmimport runtime.link string_copy
//go:noescape
func string_copy(dst unsafe.Pointer, str_head, str_body uint64, off, max uint32) uint32

//go:wasmimport runtime.link string_data
func string_data(str_head, str_body uint64) unsafe.Pointer

//go:wasmimport runtime.link string_free
func string_free(str_head, str_body uint64)

//go:wasmimport runtime.link func_jump
//go:noescape
func func_jump(fn uint64, stack *byte, stack_len uint32)

//go:wasmimport runtime.link func_jump0
func func_jump0(fn uint64) uint64

//go:wasmimport runtime.link func_jump1
func func_jump1(fn, arg1 uint64) uint64

//go:wasmimport runtime.link func_jump2
func func_jump2(fn, arg1, arg2 uint64) uint64

//go:wasmimport runtime.link func_jump4
func func_jump4(fn, arg1, arg2, arg3, arg4 uint64) uint64

//go:wasmimport runtime.link func_jump8
func func_jump8(fn, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8 uint64) uint64

func Import[API api.WithSpecification]() API {
	var zero API
	spec := api.StructureOf(&zero)
	lib := dlopen(spec.Name)
	if lib == 0 {
		for fn := range spec.Iter() {
			fn.MakeError(errors.New("dlopen failed"))
		}
	}
	for fn := range spec.Iter() {
		pc := dlsym(lib, fn.Name)
		switch jumpType(fn.Type) {
		case 0:
			fn.Make(func(args []reflect.Value) []reflect.Value {
				func_jump0(pc)
				return nil
			})
		default:
			var rtype = fn.Type
			var args_frame_size int
			for i := 0; i < rtype.NumIn(); i++ {
				args_frame_size += int(rtype.In(i).Size())
			}
			var rets_frame_size int
			for i := 0; i < rtype.NumOut(); i++ {
				rets_frame_size += int(rtype.Out(i).Size())
			}
			var stack_size = max(args_frame_size, rets_frame_size)
			fn.Make(func(args []reflect.Value) []reflect.Value {
				var stack = make([]byte, stack_size)
				var args_offset int
				for i, arg := range args {
					in := rtype.In(i)
					reflect.NewAt(in, unsafe.Pointer(&stack[args_offset])).Elem().Set(arg)
					args_offset += int(in.Size())
				}
				func_jump(pc, unsafe.SliceData(stack), uint32(len(stack)))
				var rets = make([]reflect.Value, rtype.NumOut())
				var rets_offset int
				for i := 0; i < rtype.NumOut(); i++ {
					out := rtype.Out(i)
					switch out.Kind() {
					case reflect.Int:
						rets[i] = reflect.NewAt(out, unsafe.Pointer(&stack[rets_offset])).Elem()
					case reflect.String:
						var shared_string = *(*[2]uint64)(unsafe.Pointer(&stack[rets_offset]))
						var buf = make([]byte, string_len(shared_string[0], shared_string[1]))
						string_copy(unsafe.Pointer(&buf[0]), shared_string[0], shared_string[1], 0, uint32(len(buf)))
						rets[i] = reflect.ValueOf(unsafe.String(&buf[0], len(buf)))
					default:
						panic("not implemented")
					}
					rets_offset += int(out.Size())
				}
				return rets
			})
		}

	}
	return zero
}

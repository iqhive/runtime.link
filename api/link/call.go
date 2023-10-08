package link

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"runtime.link/api/link/internal/cgo/dyncall"
	"runtime.link/jit"
)

type platform struct{}

// CallingConvention TODO
func (platform) CallingConvention(reflect.Type) (args, rets []jit.Location, err error) {
	return nil, nil, errors.New("not implemented, use Call directly")
}

// Call with a CGO dyncall.
func (platform) Call(ptr unsafe.Pointer, args []reflect.Value, rets ...reflect.Type) ([]reflect.Value, []func(), error) {
	var free = make([]func(), 0, len(rets))
	var vm = dyncall.NewVM(4096)
	defer vm.Free()
	for _, arg := range args {
		switch arg.Kind() {
		case reflect.Int8:
			vm.PushChar(int8(arg.Int()))
		case reflect.Int16:
			vm.PushShort(int16(arg.Int()))
		case reflect.Int32:
			vm.PushSignedInt(int32(arg.Int()))
		case reflect.Int:
			vm.PushSignedLong(int(arg.Int()))
		case reflect.Int64:
			vm.PushSignedLongLong(arg.Int())
		case reflect.Uint8:
			u8 := uint8(arg.Uint())
			i8 := *(*int8)(unsafe.Pointer(&u8))
			vm.PushChar(i8)
		case reflect.Uint16:
			u16 := uint16(arg.Uint())
			i16 := *(*int16)(unsafe.Pointer(&u16))
			vm.PushShort(i16)
		case reflect.Uint32:
			u32 := uint32(arg.Uint())
			i32 := *(*int32)(unsafe.Pointer(&u32))
			vm.PushSignedInt(i32)
		case reflect.Uint:
			u := uint(arg.Uint())
			i := *(*int)(unsafe.Pointer(&u))
			vm.PushSignedLong(i)
		case reflect.Uint64:
			u64 := arg.Uint()
			i64 := *(*int64)(unsafe.Pointer(&u64))
			vm.PushSignedLongLong(i64)
		case reflect.Uintptr:
			u := uintptr(arg.Uint())
			i := *(*int64)(unsafe.Pointer(&u))
			vm.PushSignedLongLong(i)
		case reflect.Float32:
			vm.PushFloat(float32(arg.Float()))
		case reflect.Float64:
			vm.PushDouble(arg.Float())
		case reflect.Pointer, reflect.Func, reflect.Chan, reflect.Map, reflect.UnsafePointer:
			vm.PushPointer(arg.UnsafePointer())
		default:
			return nil, free, fmt.Errorf("unsupported call argument type %s", arg.Type())
		}
	}
	if len(rets) == 0 || rets[0] == nil {
		vm.Call(ptr)
		return nil, free, nil
	}
	var value reflect.Value
	var out = rets[0]
	switch out.Kind() {
	case reflect.Bool:
		i8 := vm.CallChar(ptr)
		if i8 != 0 {
			value = reflect.ValueOf(true).Convert(out)
		} else {
			value = reflect.ValueOf(false).Convert(out)
		}
	case reflect.Int8:
		value = reflect.ValueOf(vm.CallChar(ptr)).Convert(out)
	case reflect.Int16:
		value = reflect.ValueOf(vm.CallShort(ptr)).Convert(out)
	case reflect.Int32:
		value = reflect.ValueOf(vm.CallInt(ptr)).Convert(out)
	case reflect.Int:
		value = reflect.ValueOf(vm.CallLong(ptr)).Convert(out)
	case reflect.Int64:
		value = reflect.ValueOf(vm.CallLongLong(ptr)).Convert(out)
	case reflect.Uint8:
		i8 := vm.CallChar(ptr)
		value = reflect.ValueOf(*(*uint8)(unsafe.Pointer(&i8))).Convert(out)
	case reflect.Uint16:
		i16 := vm.CallShort(ptr)
		value = reflect.ValueOf(*(*uint16)(unsafe.Pointer(&i16))).Convert(out)
	case reflect.Uint32:
		i32 := vm.CallInt(ptr)
		value = reflect.ValueOf(*(*uint32)(unsafe.Pointer(&i32))).Convert(out)
	case reflect.Uint:
		i := vm.CallLong(ptr)
		value = reflect.ValueOf(*(*uint)(unsafe.Pointer(&i))).Convert(out)
	case reflect.Uint64:
		i64 := vm.CallLongLong(ptr)
		value = reflect.ValueOf(*(*uint64)(unsafe.Pointer(&i64))).Convert(out)
	case reflect.Uintptr:
		i64 := vm.CallLongLong(ptr)
		value = reflect.ValueOf(*(*uintptr)(unsafe.Pointer(&i64))).Convert(out)
	case reflect.Float32:
		value = reflect.ValueOf(vm.CallFloat(ptr)).Convert(out)
	case reflect.Float64:
		value = reflect.ValueOf(vm.CallDouble(ptr)).Convert(out)
	case reflect.UnsafePointer:
		value = reflect.ValueOf(vm.CallPointer(ptr)).Convert(out)
	case reflect.Pointer, reflect.Chan, reflect.Map, reflect.Func:
		value = reflect.NewAt(rets[0].Elem(), vm.CallPointer(ptr))
	default:
		return nil, free, fmt.Errorf("unsupported call result type %s", rets[0])
	}
	return []reflect.Value{value}, free, nil
}

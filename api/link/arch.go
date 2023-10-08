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

func (platform) CallingConvention(reflect.Type) (args, rets []jit.Location, err error) {
	return nil, nil, errors.New("not implemented, use Call directly")
}

func (platform) Call(ptr unsafe.Pointer, args []reflect.Value, rets ...reflect.Type) ([]reflect.Value, error) {
	var vm = dyncall.NewVM(4096)
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
		case reflect.Float32:
			vm.PushFloat(float32(arg.Float()))
		case reflect.Float64:
			vm.PushDouble(arg.Float())
		case reflect.Pointer:
			vm.PushPointer(arg.UnsafePointer())
		default:
			return nil, fmt.Errorf("unsupported type %s", arg.Type())
		}
	}
	if len(rets) == 0 || rets[0] == nil {
		vm.Call(ptr)
		return nil, nil
	}
	var value any
	switch rets[0].Kind() {
	case reflect.Int8:
		value = vm.CallChar(ptr)
	case reflect.Int16:
		value = vm.CallShort(ptr)
	case reflect.Int32:
		value = vm.CallInt(ptr)
	case reflect.Int:
		value = vm.CallLong(ptr)
	case reflect.Int64:
		value = vm.CallLongLong(ptr)
	case reflect.Uint8:
		i8 := vm.CallChar(ptr)
		value = *(*uint8)(unsafe.Pointer(&i8))
	case reflect.Uint16:
		i16 := vm.CallShort(ptr)
		value = *(*uint16)(unsafe.Pointer(&i16))
	case reflect.Uint32:
		i32 := vm.CallInt(ptr)
		value = *(*uint32)(unsafe.Pointer(&i32))
	case reflect.Uint:
		i := vm.CallLong(ptr)
		value = *(*uint)(unsafe.Pointer(&i))
	case reflect.Uint64:
		i64 := vm.CallLongLong(ptr)
		value = *(*uint64)(unsafe.Pointer(&i64))
	case reflect.Float32:
		value = vm.CallFloat(ptr)
	case reflect.Float64:
		value = vm.CallDouble(ptr)
	case reflect.Pointer:
		value = vm.CallPointer(ptr)
	default:
		return nil, fmt.Errorf("unsupported type %s", rets[0])
	}
	return []reflect.Value{reflect.ValueOf(value)}, nil
}

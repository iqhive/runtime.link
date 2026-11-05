package call

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/iqhive/runtime.link/api/call/internal/cgo/dyncall"
	"github.com/iqhive/runtime.link/api/call/internal/jit"
	"github.com/iqhive/runtime.link/api/xray"
)

type platform struct{}

// CallingConvention TODO
func (platform) CallingConvention(reflect.Type) (args, rets []jit.Location, err error) {
	return nil, nil, xray.New(errors.New("not implemented, use Call directly"))
}

// vmSize returns a size for a VM that has room for every argument on its
// stack, dyncall drops the ones that don't fit without saying so.
func vmSize(args int) int {
	const (
		minimum   = 4096
		registers = 256 // the VM takes its saved registers out of the size it's given.
		stackSlot = 8
	)
	return max(minimum, registers+args*stackSlot)
}

// Call with a CGO dyncall.
func (platform) Call(ptr unsafe.Pointer, args []reflect.Value, rets ...reflect.Type) ([]reflect.Value, []func(), error) {
	// the VM is given pointers as bytes, so this is what keeps them reachable during the call.
	defer runtime.KeepAlive(args)
	var free = make([]func(), 0, len(rets))
	var vm = dyncall.NewVM(vmSize(len(args)))
	defer vm.Free()
	for _, arg := range args {
		switch arg.Kind() {
		case reflect.Bool:
			vm.PushBool(arg.Bool())
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
			vm.PushUnsignedChar(uint8(arg.Uint()))
		case reflect.Uint16:
			vm.PushUnsignedShort(uint16(arg.Uint()))
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
		value = reflect.ValueOf(uintptr(vm.CallPointer(ptr))).Convert(out)
	case reflect.Float32:
		value = reflect.ValueOf(vm.CallFloat(ptr)).Convert(out)
	case reflect.Float64:
		value = reflect.ValueOf(vm.CallDouble(ptr)).Convert(out)
	case reflect.UnsafePointer:
		value = reflect.ValueOf(vm.CallPointer(ptr)).Convert(out)
	case reflect.Pointer:
		value = reflect.NewAt(rets[0].Elem(), vm.CallPointer(ptr))
	case reflect.Func:
		value = reflect.ValueOf(unsafe.Pointer(vm.CallPointer(ptr)))
	default:
		return nil, free, fmt.Errorf("unsupported call result type %s", rets[0])
	}
	return []reflect.Value{value}, free, nil
}

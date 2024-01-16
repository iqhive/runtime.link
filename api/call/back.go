package call

import (
	"reflect"
	"unsafe"

	"runtime.link/api/call/internal/cgo"
	"runtime.link/api/call/internal/cgo/dyncall"
)

// #include <stdbool.h>
import "C"

// Back requires a Go function type parameter that specifies the type of this
// callback function. This callback can be passed over FFI boundaries to enable
// Go functions to be passed as callbacks.
type Back[T any] struct {
	dyn *dyncall.Callback
}

func (back Back[T]) isCallback() {}

func (back *Back[T]) Set(fn T) {
	rtype := reflect.TypeOf(fn)
	if rtype.Kind() != reflect.Func {
		panic("call.Back.Set: T must be a function type")
	}
	sig := newSignature(rtype)
	dyn := dyncall.NewCallback(sig, newCallback(sig, reflect.ValueOf(fn)))
	back.dyn = dyn
}

func (back Back[T]) Free() {
	back.dyn.Free()
}

func sigRune(t reflect.Type) rune {
	if t.Implements(reflect.TypeOf((*interface{ isCallback() })(nil)).Elem()) {
		return dyncall.Pointer
	}
	switch t.Kind() {
	case cgo.Types.LookupKind("bool"):
		return dyncall.Bool
	case cgo.Types.LookupKind("char"):
		return dyncall.Char
	case cgo.Types.LookupKind("unsigned_char"):
		return dyncall.UnsignedChar
	case cgo.Types.LookupKind("short"):
		return dyncall.Short
	case cgo.Types.LookupKind("unsigned_short"):
		return dyncall.UnsignedShort
	case cgo.Types.LookupKind("int"):
		return dyncall.Int
	case cgo.Types.LookupKind("unsigned_int"):
		return dyncall.Uint
	case cgo.Types.LookupKind("long"):
		return dyncall.Long
	case cgo.Types.LookupKind("unsigned_long"):
		return dyncall.UnsignedLong
	case cgo.Types.LookupKind("long_long"):
		return dyncall.LongLong
	case cgo.Types.LookupKind("unsigned_long_long"):
		return dyncall.UnsignedLongLong
	case cgo.Types.LookupKind("float"):
		return dyncall.Float
	case cgo.Types.LookupKind("double"):
		return dyncall.Double
	case reflect.String:
		return dyncall.String
	case reflect.Pointer, reflect.Uintptr, reflect.UnsafePointer, reflect.Func:
		return dyncall.Pointer
	default:
		if t.Kind() == reflect.Struct && t.NumField() == 1 {
			return sigRune(t.Field(0).Type)
		}
		panic("unsupported type " + t.String())
	}
}

func newSignature(ftype reflect.Type) dyncall.Signature {
	var sig dyncall.Signature
	for i := 0; i < ftype.NumIn(); i++ {
		sig.Args = append(sig.Args, sigRune(ftype.In(i)))
	}
	if ftype.NumOut() >= 1 {
		sig.Returns = sigRune(ftype.Out(0))
	} else {
		sig.Returns = dyncall.Void
	}
	return sig
}

func newCallback(signature dyncall.Signature, function reflect.Value) dyncall.CallbackHandler {
	return func(cb *dyncall.Callback, args *dyncall.Args, result unsafe.Pointer) rune {
		var values = make([]reflect.Value, len(signature.Args))
		for i := range values {
			rtype := function.Type().In(i)
			values[i] = reflect.New(rtype).Elem()
		}
		for i := 0; i < len(signature.Args); i++ {
			switch signature.Args[i] {
			case dyncall.Bool:
				switch args.Bool() {
				case 0:
					values[i].SetBool(false)
				case 1:
					values[i].SetBool(true)
				}
			case dyncall.Char:
				values[i].SetInt(int64(args.Char()))
			case dyncall.UnsignedChar:
				values[i].SetUint(uint64(args.UnsignedChar()))
			case dyncall.Short:
				values[i].SetInt(int64(args.Short()))
			case dyncall.UnsignedShort:
				values[i].SetUint(uint64(args.UnsignedShort()))
			case dyncall.Int:
				values[i].SetInt(int64(args.Int()))
			case dyncall.Uint:
				values[i].SetUint(uint64(args.UnsignedInt()))
			case dyncall.Long:
				values[i].SetInt(int64(args.Long()))
			case dyncall.UnsignedLong:
				values[i].SetUint(uint64(args.UnsignedLong()))
			case dyncall.LongLong:
				values[i].SetInt(int64(args.LongLong()))
			case dyncall.UnsignedLongLong:
				values[i].SetUint(uint64(args.UnsignedLongLong()))
			case dyncall.Float:
				values[i].SetFloat(float64(args.Float()))
			case dyncall.Double:
				values[i].SetFloat(float64(args.Double()))
			case dyncall.Pointer:
				switch values[i].Kind() {
				case reflect.UnsafePointer:
					values[i].SetPointer(unsafe.Pointer(args.Pointer()))
				case reflect.Uintptr:
					values[i].SetUint(uint64(uintptr(args.Pointer())))
				case reflect.Pointer:
					values[i] = reflect.NewAt(values[i].Type().Elem(), unsafe.Pointer(args.Pointer()))
				default:
					settable, ok := values[i].Addr().Interface().(interface {
						SetPointer(unsafe.Pointer)
					})
					if !ok {
						panic("unsupported type " + values[i].Type().String())
					}
					settable.SetPointer(unsafe.Pointer(args.Pointer()))
				}
			default:
				panic("unsupported type " + string(signature.Args[i]))
			}
		}
		results := function.Call(values)
		switch signature.Returns {
		case dyncall.Void:
		case dyncall.Bool:
			var b C.bool
			if results[0].Bool() {
				b = true
			}
			*(*C.bool)(result) = b
		case dyncall.Char:
			*(*C.char)(result) = C.char(results[0].Int())
		case dyncall.UnsignedChar:
			*(*C.uchar)(result) = C.uchar(results[0].Uint())
		case dyncall.Short:
			*(*C.short)(result) = C.short(results[0].Int())
		case dyncall.UnsignedShort:
			*(*C.ushort)(result) = C.ushort(results[0].Uint())
		case dyncall.Int:
			*(*C.int)(result) = C.int(results[0].Int())
		case dyncall.Uint:
			*(*C.uint)(result) = C.uint(results[0].Uint())
		case dyncall.Long:
			*(*C.long)(result) = C.long(results[0].Int())
		case dyncall.UnsignedLong:
			*(*C.ulong)(result) = C.ulong(results[0].Uint())
		case dyncall.LongLong:
			*(*C.longlong)(result) = C.longlong(results[0].Int())
		case dyncall.UnsignedLongLong:
			*(*C.ulonglong)(result) = C.ulonglong(results[0].Uint())
		case dyncall.Float:
			*(*C.float)(result) = C.float(results[0].Float())
		case dyncall.Double:
			*(*C.double)(result) = C.double(results[0].Float())
		case dyncall.Pointer:
			if results[0].Kind() == reflect.Struct && results[0].NumField() == 1 {
				results[0] = results[0].Field(0)
			}
			switch results[0].Kind() {
			case reflect.Uintptr:
				*(*uintptr)(result) = uintptr(results[0].Uint())
			default:
				*(*unsafe.Pointer)(result) = unsafe.Pointer(results[0].Pointer())
			}
		default:
			panic("unsupported type " + results[0].Type().String())
		}
		return signature.Returns
	}
}

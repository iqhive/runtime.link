package ffi

import (
	"errors"
	"reflect"
	"unsafe"

	"runtime.link/api/link/internal/cgo/dyncall"
	"runtime.link/api/link/internal/cpu"
	"runtime.link/api/xray"
)

import "C"

func sigRune(t reflect.Type) rune {
	switch t.Kind() {
	case reflect.TypeOf(c_bool(0)).Kind():
		return dyncall.Bool
	case reflect.TypeOf(c_char(0)).Kind():
		return dyncall.Char
	case reflect.TypeOf(c_unsigned_char(0)).Kind():
		return dyncall.UnsignedChar
	case reflect.TypeOf(c_short(0)).Kind():
		return dyncall.Short
	case reflect.TypeOf(c_short(0)).Kind():
		return dyncall.UnsignedShort
	case reflect.TypeOf(c_int(0)).Kind():
		return dyncall.Int
	case reflect.TypeOf(c_unsigned_int(0)).Kind():
		return dyncall.Uint
	case reflect.TypeOf(c_long(0)).Kind():
		return dyncall.Long
	case reflect.TypeOf(c_unsigned_long(0)).Kind():
		return dyncall.UnsignedLong
	case reflect.TypeOf(c_longlong(0)).Kind():
		return dyncall.LongLong
	case reflect.TypeOf(c_unsigned_longlong(0)).Kind():
		return dyncall.UnsignedLongLong
	case reflect.TypeOf(c_float(0)).Kind():
		return dyncall.Float
	case reflect.TypeOf(c_double(0)).Kind():
		return dyncall.Double
	case reflect.String:
		return dyncall.String
	case reflect.Pointer:
		return dyncall.Pointer
	default:
		panic("unsupported callback argument type " + t.String())
	}
}

func newSignature(ftype reflect.Type) dyncall.Signature {
	var sig dyncall.Signature
	for i := 0; i < ftype.NumIn(); i++ {
		sig.Args = append(sig.Args, sigRune(ftype.In(i)))
	}
	if ftype.NumOut() > 1 {
		sig.Returns = sigRune(ftype.Out(0))
	} else {
		sig.Returns = dyncall.Void
	}
	return sig
}

// callback is a lazy way to convert a Go function into an ABI-compatible one. It is slow, unsafe and leaks memory.
// A more performant callback could be written that uses unsafe cpu.Registers to pass arguments and fetch
// the return value on supported Go runtimes. Safety can be improved by appropriately handling the [Type] mapping.
func callback(from reflect.Type, into Type) (func(cpu.Register) cpu.Register, error) {
	if into.Name != "func" {
		return nil, xray.Error(errors.New("Go function can only be passed as a function type, not a " + into.Name))
	}
	return func(reg cpu.Register) cpu.Register {
		ptr := reg.UnsafePointer()
		function := reflect.NewAt(from, unsafe.Pointer(&ptr)).Elem()
		signature := newSignature(from)
		compatible := dyncall.NewCallback(signature, func(cb *dyncall.Callback, args *dyncall.Args, result unsafe.Pointer) rune {
			var values = make([]reflect.Value, len(signature.Args))
			for i := range values {
				values[i] = reflect.New(function.Type().In(i)).Elem()
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
				case dyncall.String:
					ptr := args.Pointer()
					switch values[i].Kind() {
					case reflect.String:
						values[i].SetString(C.GoString((*C.char)(ptr)))
					default:
						panic("unsupported type " + values[i].Type().String())
					}
				case dyncall.Pointer:
					switch values[i].Kind() {
					case reflect.UnsafePointer:
						values[i].SetPointer(unsafe.Pointer(args.Pointer()))
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
				var b c_bool
				if results[0].Bool() {
					b = 1
				}
				*(*c_bool)(result) = b
			case dyncall.Char:
				*(*c_char)(result) = c_char(results[0].Int())
			case dyncall.UnsignedChar:
				*(*c_unsigned_char)(result) = c_unsigned_char(results[0].Uint())
			case dyncall.Short:
				*(*c_short)(result) = c_short(results[0].Int())
			case dyncall.UnsignedShort:
				*(*c_unsigned_short)(result) = c_unsigned_short(results[0].Uint())
			case dyncall.Int:
				*(*c_int)(result) = c_int(results[0].Int())
			case dyncall.Uint:
				*(*c_unsigned_int)(result) = c_unsigned_int(results[0].Uint())
			case dyncall.Long:
				*(*c_long)(result) = c_long(results[0].Int())
			case dyncall.UnsignedLong:
				*(*c_unsigned_long)(result) = c_unsigned_long(results[0].Uint())
			case dyncall.LongLong:
				*(*c_longlong)(result) = c_longlong(results[0].Int())
			case dyncall.UnsignedLongLong:
				*(*c_unsigned_longlong)(result) = c_unsigned_longlong(results[0].Uint())
			case dyncall.Float:
				*(*c_float)(result) = c_float(results[0].Float())
			case dyncall.Double:
				*(*c_double)(result) = c_double(results[0].Float())
			case dyncall.Pointer:
				*(*unsafe.Pointer)(result) = results[0].UnsafePointer()
			default:
				panic("unsupported type " + results[0].Type().String())
			}
			return signature.Returns
		})
		reg.SetUnsafePointer(unsafe.Pointer(compatible))
		return reg
	}, nil
}

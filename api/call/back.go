package call

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/iqhive/runtime.link/api/call/internal/cgo/dyncall"
	"github.com/iqhive/runtime.link/api/call/internal/ffi"
)

// Back requires a Go function type parameter that specifies the type of this
// callback function. This callback can be passed over FFI boundaries to enable
// Go functions to be passed as callbacks. Foreign code may keep hold of it until
// [Back.Free] is called, where a plain Go func is only valid during the call
// it was passed to.
type Back[T any] struct {
	dyn *dyncall.Callback
}

// callback is implemented by every [Back], whatever its type parameter.
type callback interface {
	isCallback()
	pointer() unsafe.Pointer
	funcType() reflect.Type
}

func (back Back[T]) isCallback() {}

// pointer returns the address that foreign code can call.
func (back Back[T]) pointer() unsafe.Pointer { return unsafe.Pointer(back.dyn) }

func (back *Back[T]) setPointer(pointer unsafe.Pointer) {
	back.dyn = (*dyncall.Callback)(pointer)
}

func (back Back[T]) funcType() reflect.Type { return reflect.TypeFor[T]() }

// Set the Go function that this callback runs, a nil function results in a
// NULL pointer being passed to foreign code. Set panics if T is not a function
// type or if foreign code has no way to call a function of that type.
func (back *Back[T]) Set(fn T) {
	rtype := reflect.TypeFor[T]()
	if rtype.Kind() != reflect.Func {
		panic("call.Back.Set: T must be a function type")
	}
	sig, err := newSignature(rtype)
	if err != nil {
		panic("call.Back.Set: " + err.Error())
	}
	function := reflect.ValueOf(fn)
	if function.IsNil() {
		back.dyn = nil
		return
	}
	dyn := dyncall.NewCallback(sig, newCallback(sig, function))
	if dyn == nil {
		panic("call.Back.Set: unable to allocate executable memory")
	}
	back.dyn = dyn
}

// Free releases the callback, foreign code must not call it afterwards.
func (back *Back[T]) Free() {
	if back.dyn == nil {
		return
	}
	back.dyn.Free()
	back.dyn = nil
}

func isCallback(t reflect.Type) bool {
	return t.Implements(reflect.TypeFor[callback]())
}

// sigRune returns the dyncall type that a Go parameter or result is passed as.
func sigRune(t reflect.Type) (rune, error) {
	if isCallback(t) {
		return dyncall.Pointer, nil
	}
	switch t.Kind() {
	case reflect.Bool:
		return dyncall.Bool, nil
	case reflect.Int8:
		return dyncall.Char, nil
	case reflect.Uint8:
		return dyncall.UnsignedChar, nil
	case reflect.Int16:
		return dyncall.Short, nil
	case reflect.Uint16:
		return dyncall.UnsignedShort, nil
	case reflect.Int32:
		return dyncall.Int, nil
	case reflect.Uint32:
		return dyncall.Uint, nil
	case reflect.Int, reflect.Int64:
		return dyncall.LongLong, nil
	case reflect.Uint, reflect.Uint64:
		return dyncall.UnsignedLongLong, nil
	case reflect.Float32:
		return dyncall.Float, nil
	case reflect.Float64:
		return dyncall.Double, nil
	case reflect.String:
		return dyncall.String, nil
	case reflect.Pointer, reflect.Uintptr, reflect.UnsafePointer, reflect.Func:
		return dyncall.Pointer, nil
	case reflect.Struct:
		if t.NumField() == 1 {
			return sigRune(t.Field(0).Type)
		}
	}
	return 0, errors.New("unsupported type " + t.String())
}

// unwrapped returns the type that a single-field struct is passed as.
func unwrapped(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Struct && t.NumField() == 1 && !isCallback(t) {
		t = t.Field(0).Type
	}
	return t
}

// newSignature returns the signature for foreign code to call a function of the
// given type, or an error if it cannot be called.
func newSignature(ftype reflect.Type) (dyncall.Signature, error) {
	var sig dyncall.Signature
	if ftype.IsVariadic() {
		return sig, fmt.Errorf("unsupported variadic callback %s", ftype)
	}
	if ftype.NumOut() > 1 {
		return sig, fmt.Errorf("unsupported callback %s with more than one result", ftype)
	}
	for i := 0; i < ftype.NumIn(); i++ {
		char, err := sigRune(ftype.In(i))
		if err != nil {
			return sig, err
		}
		if in := unwrapped(ftype.In(i)); in.Kind() == reflect.Func {
			if _, err := foreignFunc(ftype.String(), platform{}, in, nil); err != nil {
				return sig, err
			}
		}
		sig.Args = append(sig.Args, char)
	}
	sig.Returns = dyncall.Void
	if ftype.NumOut() == 0 {
		return sig, nil
	}
	char, err := sigRune(ftype.Out(0))
	if err != nil {
		return sig, err
	}
	// neither of these has a representation that C can use once the callback has returned.
	switch out := unwrapped(ftype.Out(0)); out.Kind() {
	case reflect.Func, reflect.String:
		return sig, fmt.Errorf("unsupported callback result %s", out)
	}
	sig.Returns = char
	return sig, nil
}

func newCallback(signature dyncall.Signature, function reflect.Value) dyncall.CallbackHandler {
	ftype := function.Type()
	return func(cb *dyncall.Callback, args *dyncall.Args, result unsafe.Pointer) rune {
		var values = make([]reflect.Value, len(signature.Args))
		for i := range values {
			values[i] = reflect.New(ftype.In(i)).Elem()
			setArgument(values[i], signature.Args[i], args)
		}
		results := function.Call(values)
		if signature.Returns != dyncall.Void {
			reflect.NewAt(ftype.Out(0), result).Elem().Set(results[0])
		}
		return signature.Returns
	}
}

// setArgument takes the next argument from args, so it must
// be called once for each parameter, in order.
func setArgument(value reflect.Value, sig rune, args *dyncall.Args) {
	if value.Kind() == reflect.Struct && !isCallback(value.Type()) {
		field := value.Field(0)
		setArgument(reflect.NewAt(field.Type(), field.Addr().UnsafePointer()).Elem(), sig, args)
		return
	}
	switch sig {
	case dyncall.Bool:
		value.SetBool(args.Bool() != 0)
	case dyncall.Char:
		value.SetInt(int64(args.Char()))
	case dyncall.UnsignedChar:
		value.SetUint(uint64(args.UnsignedChar()))
	case dyncall.Short:
		value.SetInt(int64(args.Short()))
	case dyncall.UnsignedShort:
		value.SetUint(uint64(args.UnsignedShort()))
	case dyncall.Int:
		value.SetInt(int64(args.Int()))
	case dyncall.Uint:
		value.SetUint(uint64(args.UnsignedInt()))
	case dyncall.LongLong:
		value.SetInt(int64(args.LongLong()))
	case dyncall.UnsignedLongLong:
		value.SetUint(uint64(args.UnsignedLongLong()))
	case dyncall.Float:
		value.SetFloat(float64(args.Float()))
	case dyncall.Double:
		value.SetFloat(float64(args.Double()))
	case dyncall.String:
		value.SetString(dyncall.GoString(unsafe.Pointer(args.Pointer())))
	case dyncall.Pointer:
		setPointer(value, unsafe.Pointer(args.Pointer()))
	}
}

func setPointer(value reflect.Value, pointer unsafe.Pointer) {
	if isCallback(value.Type()) {
		value.Addr().Interface().(interface{ setPointer(unsafe.Pointer) }).setPointer(pointer)
		return
	}
	switch value.Kind() {
	case reflect.Uintptr:
		value.SetUint(uint64(uintptr(pointer)))
	case reflect.Pointer:
		value.Set(reflect.NewAt(value.Type().Elem(), pointer).Convert(value.Type()))
	case reflect.Func:
		// [newSignature] has already proven that this type works.
		if convert, err := foreignFunc(value.Type().String(), platform{}, value.Type(), nil); err == nil {
			value.Set(convert(pointer))
		}
	default:
		value.SetPointer(pointer)
	}
}

// foreignType returns the C type that foreign code is taken to use for a Go type,
// since a Go function type that is called from C has no tag of its own.
func foreignType(t reflect.Type) (ffi.Type, error) {
	switch t.Kind() {
	case reflect.Bool:
		return ffi.Type{Name: "bool"}, nil
	case reflect.Int8:
		return ffi.Type{Name: "signed_char"}, nil
	case reflect.Uint8:
		return ffi.Type{Name: "unsigned_char"}, nil
	case reflect.Int16:
		return ffi.Type{Name: "short"}, nil
	case reflect.Uint16:
		return ffi.Type{Name: "unsigned_short"}, nil
	case reflect.Int32:
		return ffi.Type{Name: "int"}, nil
	case reflect.Uint32:
		return ffi.Type{Name: "unsigned_int"}, nil
	case reflect.Int, reflect.Int64:
		return ffi.Type{Name: "long_long"}, nil
	case reflect.Uint, reflect.Uint64:
		return ffi.Type{Name: "unsigned_long_long"}, nil
	case reflect.Float32:
		return ffi.Type{Name: "float"}, nil
	case reflect.Float64:
		return ffi.Type{Name: "double"}, nil
	case reflect.Uintptr:
		return ffi.Type{Name: "uintptr_t"}, nil
	case reflect.String:
		return ffi.Type{Name: "char", Free: '&'}, nil
	case reflect.Pointer, reflect.UnsafePointer:
		return ffi.Type{Name: "void", Free: '&'}, nil
	case reflect.Func:
		return foreignFuncType(t)
	}
	return ffi.Type{}, errors.New("unsupported type " + t.String())
}

func foreignFuncType(fn reflect.Type) (ffi.Type, error) {
	tag := ffi.Type{Name: "func"}
	for i := 0; i < fn.NumIn(); i++ {
		arg, err := foreignType(fn.In(i))
		if err != nil {
			return tag, err
		}
		arg.Maps = i + 1
		tag.Args = append(tag.Args, arg)
	}
	if fn.NumOut() > 0 {
		ret, err := foreignType(fn.Out(0))
		if err != nil {
			return tag, err
		}
		tag.Func = &ret
	}
	return tag, nil
}

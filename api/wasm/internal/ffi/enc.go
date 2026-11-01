package ffi

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unsafe"
)

// Encode sends dst a copy of value as scalars and strings, depth first, widened
// so that the wire needs one function per family of kinds: the receiving side
// knows the real type. A slice sends its length first, -1 if it is nil. Values
// the host can't follow into guest memory are refused here, before anything
// about them crosses.
func (ffi *API) Encode(dst Structure, value reflect.Value) {
	switch value.Kind() {
	case reflect.Bool:
		ffi.Structure.Encode.Bool(dst, value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ffi.Structure.Encode.Int64(dst, value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		ffi.Structure.Encode.Uint64(dst, value.Uint())
	case reflect.Float32, reflect.Float64:
		ffi.Structure.Encode.Float64(dst, value.Float())
	case reflect.Complex64, reflect.Complex128:
		ffi.Structure.Encode.Complex128(dst, value.Complex())
	case reflect.String:
		str := ffi.String.New(strings.NewReader(value.String()), uint32(value.Len()))
		defer ffi.String.Free(str)
		ffi.Structure.Encode.String(dst, str)
	case reflect.Array:
		for i := range value.Len() {
			ffi.Encode(dst, value.Index(i))
		}
	case reflect.Struct:
		for i := range value.NumField() {
			ffi.Encode(dst, value.Field(i))
		}
	case reflect.Slice:
		if value.IsNil() {
			ffi.Structure.Encode.Int64(dst, -1)
			return
		}
		ffi.Structure.Encode.Int64(dst, int64(value.Len()))
		for i := range value.Len() {
			ffi.Encode(dst, value.Index(i))
		}
	default:
		panic(fmt.Errorf("ffi: a %s can't cross to the host, only scalars, strings and the structs, arrays and slices of those can", value.Type()))
	}
}

func encode(structure reflect.Value, b reflect.Value) {
	switch structure.Kind() {
	case reflect.Chan:
		structure.Send(b.Convert(structure.Type().Elem()))
	case reflect.Interface:
		structure.Set(b)
	case reflect.Func:
		panic("ffi.encode: func not implemented")
	case reflect.Map:
		panic("ffi.encode: map not implemented")
	case reflect.Pointer:
		if encoder, ok := structure.Interface().(*statefulEncoder); ok {
			encoder.put(b)
			return
		}
		structure.Elem().Set(b.Convert(structure.Type().Elem()))
	case reflect.Slice:
		panic("ffi.encode: slice not implemented")
	default:
		panic(fmt.Sprintf("ffi.encode: illegal encode %v", structure.Type()))
	}
}

// statefulEncoder builds a function's arguments from what [API.Encode] sends.
type statefulEncoder struct {
	args  []reflect.Value // one per parameter, without a leading context.Context
	slots []slot          // where the next values go, the very next one last
}

// slot is somewhere a value goes. A slice whose length has arrived stays on the
// stack until its elements have too, and only grows as they do: a guest can
// claim any length, but has to pay a call for every element it wants allocated.
type slot struct {
	value    reflect.Value
	filling  bool
	elements int
}

func newEncoder(fn reflect.Type) *statefulEncoder {
	encoder := new(statefulEncoder)
	for i := range fn.NumIn() {
		if i > 0 || fn.In(0) != reflect.TypeFor[context.Context]() {
			encoder.args = append(encoder.args, reflect.New(fn.In(i)).Elem())
		}
	}
	for _, arg := range slices.Backward(encoder.args) {
		encoder.slots = append(encoder.slots, slot{value: arg})
	}
	return encoder
}

func (e *statefulEncoder) put(value reflect.Value) {
	target := e.next()
	if target.Kind() != reflect.Slice {
		target.Set(value.Convert(target.Type()))
		return
	}
	length := value.Int()
	switch {
	case length < 0:
	case target.Type().Elem().Size() == 0:
		target.Set(reflect.MakeSlice(target.Type(), int(length), int(length)))
	default:
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		e.slots = append(e.slots, slot{value: target, filling: true, elements: int(length)})
	}
}

func (e *statefulEncoder) next() reflect.Value {
	for len(e.slots) > 0 {
		top := &e.slots[len(e.slots)-1]
		if top.elements > 0 {
			top.elements--
			slice := top.value
			slice.Set(reflect.Append(slice, reflect.Zero(slice.Type().Elem())))
			e.slots = append(e.slots, slot{value: slice.Index(slice.Len() - 1)})
			continue
		}
		current := *top
		e.slots = e.slots[:len(e.slots)-1]
		switch {
		case current.filling:
		case current.value.Kind() == reflect.Struct:
			for i := current.value.NumField() - 1; i >= 0; i-- {
				e.slots = append(e.slots, slot{value: reachable(current.value.Field(i))})
			}
		case current.value.Kind() == reflect.Array:
			for i := current.value.Len() - 1; i >= 0; i-- {
				e.slots = append(e.slots, slot{value: current.value.Index(i)})
			}
		default:
			return current.value
		}
	}
	panic("ffi: more arguments than the function takes")
}

func (e *statefulEncoder) call(ctx context.Context, fn reflect.Value) []reflect.Value {
	args := e.args
	if fn.Type().NumIn() > len(args) {
		args = append([]reflect.Value{reflect.ValueOf(ctx)}, args...)
	}
	if fn.Type().IsVariadic() {
		return fn.CallSlice(args)
	}
	return fn.Call(args)
}

// reachable returns a usable view of an addressable field, which may be
// unexported: a struct crosses whole, the way an assignment copies it.
func reachable(field reflect.Value) reflect.Value {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

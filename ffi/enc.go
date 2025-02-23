package ffi

import (
	"fmt"
	"reflect"
)

func (ffi *API) Encode(dst Structure, val any) {
	rtype := reflect.TypeOf(val)
	value := reflect.ValueOf(val)
	switch rtype.Kind() {
	case reflect.Bool:
		ffi.Structure.Encode.Bool(dst, value.Bool())
	case reflect.Int:
		ffi.Structure.Encode.Int(dst, int(value.Int()))
	case reflect.Int8:
		ffi.Structure.Encode.Int8(dst, int8(value.Int()))
	case reflect.Int16:
		ffi.Structure.Encode.Int16(dst, int16(value.Int()))
	case reflect.Int32:
		ffi.Structure.Encode.Int32(dst, int32(value.Int()))
	case reflect.Int64:
		ffi.Structure.Encode.Int64(dst, value.Int())
	case reflect.Uint:
		ffi.Structure.Encode.Uint(dst, uint(value.Uint()))
	case reflect.Uint8:
		ffi.Structure.Encode.Uint8(dst, uint8(value.Uint()))
	case reflect.Uint16:
		ffi.Structure.Encode.Uint16(dst, uint16(value.Uint()))
	case reflect.Uint32:
		ffi.Structure.Encode.Uint32(dst, uint32(value.Uint()))
	case reflect.Uint64:
		ffi.Structure.Encode.Uint64(dst, value.Uint())
	case reflect.Uintptr:
		ffi.Structure.Encode.Uintptr(dst, uintptr(value.Uint()))
	case reflect.Float32:
		ffi.Structure.Encode.Float32(dst, float32(value.Float()))
	case reflect.Float64:
		ffi.Structure.Encode.Float64(dst, value.Float())
	case reflect.Complex64:
		ffi.Structure.Encode.Complex64(dst, complex64(value.Complex()))
	case reflect.Complex128:
		ffi.Structure.Encode.Complex128(dst, value.Complex())
	case reflect.Array:
		for i := range rtype.Len() {
			ffi.Encode(dst, value.Index(i).Interface())
		}
	case reflect.Chan:
		ffi.Structure.Encode.Chan(dst, value.Interface().(Channel))
	case reflect.Func:
		ffi.Structure.Encode.Func(dst, value.Interface().(Function))
	case reflect.Interface:
		ffi.Encode(dst, value.Interface())
	case reflect.Map:
		ffi.Structure.Encode.Map(dst, value.Interface().(Map))
	case reflect.Ptr:
		ffi.Structure.Encode.Pointer(dst, value.Interface().(Pointer))
	case reflect.Slice:
		ffi.Structure.Encode.Slice(dst, value.Interface().(Slice))
	case reflect.String:
		ffi.Structure.Encode.String(dst, value.Interface().(String))
	case reflect.Struct:
		ffi.Structure.Encode.Structure(dst, value.Interface().(Structure))
	case reflect.UnsafePointer:
		ffi.Structure.Encode.Pointer(dst, value.Interface().(Pointer))
	}
}

func encode(structure reflect.Value, b reflect.Value) {
	switch structure.Kind() {
	case reflect.Chan:
		structure.Send(reflect.ValueOf(b).Convert(structure.Type().Elem()))
	case reflect.Interface:
		structure.Set(reflect.ValueOf(b))
	case reflect.Func:
		panic("ffi.encode: func not implemented")
	case reflect.Map:
		panic("ffi.encode: map not implemented")
	case reflect.Pointer:
		structure.Elem().Set(reflect.ValueOf(b).Convert(structure.Type().Elem()))
	case reflect.Slice:
		panic("ffi.encode: slice not implemented")
	default:
		panic(fmt.Sprintf("ffi.encode: illegal encode %v", structure.Type()))
	}
}

package ffi

import (
	"fmt"
	"reflect"
	"strings"
)

type statefulDecoder struct {
	value reflect.Value
	state int
}

// Decode fills dst from val, reading what [flatten] lists on the other side:
// scalars widened as in [API.Encode], and strings.
func (ffi *API) Decode(dst any, val Structure) {
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Ptr {
		panic("ffi.Decode: dst must be a pointer")
	}
	ffi.decode(value.Elem(), val)
}

func (ffi *API) decode(value reflect.Value, val Structure) {
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(ffi.Structure.Decode.Bool(val))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(ffi.Structure.Decode.Int64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value.SetUint(ffi.Structure.Decode.Uint64(val))
	case reflect.Float32, reflect.Float64:
		value.SetFloat(ffi.Structure.Decode.Float64(val))
	case reflect.Complex64, reflect.Complex128:
		re := ffi.Structure.Decode.Float64(val)
		im := ffi.Structure.Decode.Float64(val)
		value.SetComplex(complex(re, im))
	case reflect.Array:
		for i := range value.Len() {
			ffi.decode(value.Index(i), val)
		}
	case reflect.Struct:
		for i := range value.NumField() {
			ffi.decode(reachable(value.Field(i)), val)
		}
	case reflect.String:
		str := ffi.Structure.Decode.String(val)
		defer ffi.String.Free(str)
		data := ffi.String.Data(str)
		defer ffi.Structure.Free(data)
		var b strings.Builder
		for range ffi.String.Len(str) {
			b.WriteByte(ffi.Structure.Decode.Uint8(data))
		}
		value.SetString(b.String())
	default:
		panic(fmt.Errorf("ffi: a %s can't cross from the host, only scalars, strings and the structs and arrays of those can", value.Type()))
	}
}

// flatten appends value to leaves as [API.Decode] reads it, depth first. Complex
// numbers come apart because no wasm result can hold one.
func flatten(leaves []reflect.Value, value reflect.Value) []reflect.Value {
	switch value.Kind() {
	case reflect.Struct:
		fields := reflect.New(value.Type()).Elem()
		fields.Set(value)
		for i := range fields.NumField() {
			leaves = flatten(leaves, reachable(fields.Field(i)))
		}
		return leaves
	case reflect.Array:
		for i := range value.Len() {
			leaves = flatten(leaves, value.Index(i))
		}
		return leaves
	case reflect.Complex64, reflect.Complex128:
		c := value.Complex()
		return append(leaves, reflect.ValueOf(real(c)), reflect.ValueOf(imag(c)))
	default:
		return append(leaves, value)
	}
}

// flattenResults lists what fn returned. A trailing error crosses as whether
// there was one, then its message, because a guest can't hold a host error.
func flattenResults(fn reflect.Type, results []reflect.Value) []reflect.Value {
	var leaves []reflect.Value
	for i, result := range results {
		if i == len(results)-1 && result.Type().Implements(reflect.TypeFor[error]()) {
			err, failed := result.Interface().(error)
			var message string
			if failed {
				message = err.Error()
			}
			return append(leaves, reflect.ValueOf(failed), reflect.ValueOf(message))
		}
		leaves = flatten(leaves, result)
	}
	return leaves
}

func decode[T any](structure reflect.Value) T {
	switch structure.Kind() {
	case reflect.Chan:
		val, ok := structure.Recv()
		if !ok {
			panic("ffi.decode: chan closed")
		}
		return val.Interface().(T)
	case reflect.Interface:
		return structure.Interface().(T)
	case reflect.Func:
		panic("ffi.decode: func not implemented")
	case reflect.Map:
		panic("ffi.decode: map not implemented")
	case reflect.Pointer:
		if structure.Type() == reflect.TypeFor[*statefulDecoder]() {
			decoder := structure.Interface().(*statefulDecoder)
			index := decoder.state
			decoder.state++
			item := decoder.value.Index(index)
			if item.Type() == reflect.TypeFor[reflect.Value]() {
				item = item.Interface().(reflect.Value)
			}
			return item.Convert(reflect.TypeFor[T]()).Interface().(T)
		}
		return structure.Elem().Interface().(T)
	case reflect.Slice:
		panic("ffi.decode: stateless slice not implemented")
	case reflect.String:
		panic("ffi.decode: string not implemented")
	default:
		panic(fmt.Sprintf("ffi.decode: illegal decode %v", structure.Type()))
	}
}

func decodeRef(refs *references, structure reflect.Value, state int) uint64 {
	switch structure.Kind() {
	case reflect.Chan:
		val, ok := structure.Recv()
		if !ok {
			panic("ffi.decode: chan closed")
		}
		return new_ref(refs, val)
	case reflect.Func:
		panic("ffi.decode: func not implemented")
	case reflect.Map:
		panic("ffi.decode: map not implemented")
	case reflect.Pointer:
		if structure.Type() == reflect.TypeFor[*statefulDecoder]() {
			decoder := structure.Interface().(*statefulDecoder)
			defer func() {
				decoder.state++
			}()
			return decodeRef(refs, decoder.value, decoder.state)
		}
		return new_ref(refs, structure.Elem())
	case reflect.Slice:
		if elemType := structure.Type().Elem(); elemType == reflect.TypeFor[reflect.Value]() {
			return new_ref(refs, structure.Index(state).Interface().(reflect.Value))
		}
		return new_ref(refs, structure.Index(state))
	case reflect.String:
		panic("ffi.decode: string not implemented")
	case reflect.Struct:
		panic("ffi.decode: struct not implemented")
	default:
		panic(fmt.Sprintf("ffi.decodeRef: illegal decode %v", structure.Type()))
	}
}

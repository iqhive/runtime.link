package ffi

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ref_reflect handle[reflect.Value, uint64]

	ref_structures = (*handle[reflect.Value, Structure])(&ref_reflect)
	ref_strings    = (*handle[reflect.Value, String])(&ref_reflect)
	ref_slices     = (*handle[reflect.Value, Slice])(&ref_reflect)
	ref_types      = (*handle[reflect.Type, Type])(&ref_reflect)
	ref_functions  = (*handle[reflect.Value, Function])(&ref_reflect)
	ref_pointers   = (*handle[reflect.Value, Pointer])(&ref_reflect)
	ref_channels   = (*handle[reflect.Value, Channel])(&ref_reflect)
	ref_maps       = (*handle[reflect.Value, Map])(&ref_reflect)
	ref_fields     = (*handle[reflect.StructField, Field])(&ref_reflect)
)

func NewFunction(val any) Function {
	fn := ref_functions.New(reflect.ValueOf(val))
	return Function(fn)
}

func NewString(val string) String {
	return String(ref_strings.New(reflect.ValueOf(val)))
}

func new_ref(val reflect.Value) uint64 {
	rvalue := val
	switch rvalue.Kind() {
	case reflect.String:
		return uint64(ref_strings.New(reflect.ValueOf(rvalue.String())))
	default:
		return uint64(ref_reflect.New(rvalue))
	}
}

type handle[T any, ID ~uint64] struct {
	m sync.Map
	n atomic.Uint64
}

func (h *handle[T, ID]) New(v T) ID {
	p := h.n.Add(1)
	h.m.Store(p, v)
	return ID(p)
}

func (h *handle[T, ID]) Get(id ID) T {
	v, ok := h.m.Load(uint64(id))
	if !ok {
		panic(fmt.Errorf("ffi.handle.Get: invalid id %d", id))
	}
	return v.(T)
}

func (h *handle[T, ID]) End(id ID) { h.m.Delete(uintptr(id)) }

// New returns an new FFI API implementation in Go.
func New() API {
	return API{
		String: Strings{
			New: func(r io.Reader, n uint32) String {
				var b strings.Builder
				io.CopyN(&b, r, int64(n))
				return String(ref_strings.New(reflect.ValueOf(b.String())))
			},
			Len: func(s String) uint32 {
				return uint32(ref_strings.Get(s).Len())
			},
			Data: func(s String) Structure {
				return Structure(new_ref(reflect.ValueOf(&statefulDecoder{
					value: ref_strings.Get(s),
				})))
			},
			Free: func(s String) { ref_strings.End(s) },
		},
		Slice: Slices{
			Make: func(t Type, l, c uint32) Slice {
				return Slice(ref_slices.New(reflect.MakeSlice(ref_types.Get(t), int(l), int(c))))
			},
			Nil: func(s Slice) bool { return ref_slices.Get(s).IsNil() },
			Cap: func(s Slice) uint32 { return uint32(ref_slices.Get(s).Cap()) },
			Len: func(s Slice) uint32 { return uint32(ref_slices.Get(s).Len()) },
			From: func(s Slice, idx uint32, end uint32) Slice {
				return Slice(ref_slices.New(ref_slices.Get(s).Slice(int(idx), int(end))))
			},
			Data: func(s Slice) Structure { return Structure(s) },
			Copy: func(dst Slice, src Slice) { reflect.Copy(ref_slices.Get(dst), ref_slices.Get(src)) },
			Elem: func(s Slice, idx uint32) Pointer {
				return Pointer(ref_reflect.New(ref_slices.Get(s).Index(int(idx))))
			},
			Free: func(s Slice) { ref_slices.End(s) },
		},
		Pointer: Pointers{
			New: func(rtype Type) Pointer {
				return Pointer(ref_reflect.New(reflect.New(ref_types.Get(rtype)).Elem()))
			},
			Data: func(p Pointer) Structure { return Structure(p) },
			Nil:  func(p Pointer) bool { return ref_pointers.Get(p).IsNil() },
			Free: func(p Pointer) { ref_pointers.End(p) },
		},
		Channel: Channels{
			Make: func(t Type, n uint32) Channel {
				return Channel(ref_channels.New(reflect.MakeChan(ref_types.Get(t), int(n))))
			},
			Nil:   func(c Channel) bool { return ref_channels.Get(c).IsNil() },
			Len:   func(c Channel) int { return ref_channels.Get(c).Len() },
			Cap:   func(c Channel) int { return ref_channels.Get(c).Cap() },
			Data:  func(c Channel) Structure { return Structure(c) },
			Close: func(c Channel) { ref_channels.Get(c).Close() },
			Free:  func(c Channel) { ref_channels.End(c) },
		},
		Function: Functions{
			Nil:  func(f Function) bool { return ref_functions.Get(f).IsNil() },
			Free: func(f Function) { ref_functions.End(f) },
			Args: func(f Function) Structure {
				fn := ref_functions.Get(f)
				return Structure(new_ref(reflect.ValueOf(make([]reflect.Value, 0, fn.Type().NumIn()))))
			},
			Call: func(f Function, args Structure) Structure {
				fn := ref_functions.Get(f)
				if fn.Type().NumIn() == 0 && fn.Type().NumOut() == 0 {
					fn.Call(nil)
					return 0
				}
				results := fn.Call(ref_reflect.Get(uint64(args)).Interface().([]reflect.Value))
				return Structure(new_ref(reflect.ValueOf(&statefulDecoder{
					value: reflect.ValueOf(results),
					state: 0,
				})))
			},
		},
		Map: Maps{
			Make: func(t Type) Map { return Map(ref_maps.New(reflect.MakeMap(ref_types.Get(t)))) },
			Nil:  func(m Map) bool { return ref_maps.Get(m).IsNil() },
			Len: func(m Map) int {
				return ref_maps.Get(m).Len()
			},
			Free:  func(m Map) { ref_maps.End(m) },
			Clear: func(m Map) { ref_maps.Get(m).Clear() },
			Key: func(m Map) Structure {
				mtype := ref_maps.Get(m).Type()
				return Structure(ref_reflect.New(reflect.New(mtype.Key()).Elem()))
			},
			Delete: func(m Map, key Structure) { ref_maps.Get(m).SetMapIndex(ref_reflect.Get(uint64(key)), reflect.Value{}) },
			Data:   func(m Map) Structure { return Structure(m) },
		},
		Structure: Structures{
			Zero: func(s Structure) bool { return ref_structures.Get(s).IsZero() },
			Type: func(s Structure) Type { return Type(ref_types.New(ref_structures.Get(s).Type())) },
			Free: func(s Structure) {
				structure := ref_structures.Get(s)
				if structure.Type() == reflect.TypeFor[[]reflect.Value]() {
					ref_structures.End(s)
				}
			},
			Encode: Encoding{
				Bool: func(s Structure, b bool) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Bool {
						structure.SetBool(b)
					} else {
						encode(structure, reflect.ValueOf(b))
					}
				},
				Int: func(s Structure, i int) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int {
						structure.SetInt(int64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Int8: func(s Structure, i int8) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int8 {
						structure.SetInt(int64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Int16: func(s Structure, i int16) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int16 {
						structure.SetInt(int64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Int32: func(s Structure, i int32) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int32 {
						structure.SetInt(int64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Int64: func(s Structure, i int64) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int64 {
						structure.SetInt(i)
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uint: func(s Structure, i uint) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint {
						structure.SetUint(uint64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uint8: func(s Structure, i uint8) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint8 {
						structure.SetUint(uint64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uint16: func(s Structure, i uint16) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint16 {
						structure.SetUint(uint64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uint32: func(s Structure, i uint32) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint32 {
						structure.SetUint(uint64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uint64: func(s Structure, i uint64) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint64 {
						structure.SetUint(i)
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Uintptr: func(s Structure, i uintptr) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uintptr {
						structure.SetUint(uint64(i))
					} else {
						encode(structure, reflect.ValueOf(i))
					}
				},
				Float32: func(s Structure, f float32) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Float32 {
						structure.SetFloat(float64(f))
					} else {
						encode(structure, reflect.ValueOf(f))
					}
				},
				Float64: func(s Structure, f float64) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Float64 {
						structure.SetFloat(f)
					} else {
						encode(structure, reflect.ValueOf(f))
					}
				},
				Complex64: func(s Structure, c complex64) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Complex64 {
						structure.SetComplex(complex128(c))
					} else {
						encode(structure, reflect.ValueOf(c))
					}
				},
				Complex128: func(s Structure, c complex128) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Complex128 {
						structure.SetComplex(c)
					} else {
						encode(structure, reflect.ValueOf(c))
					}
				},
				Chan: func(s Structure, c Channel) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Chan {
						structure.Set(reflect.ValueOf(ref_channels.Get(c)))
					} else {
						encode(structure, reflect.ValueOf(c))
					}
				},
				Func: func(s Structure, f Function) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Func {
						structure.Set(reflect.ValueOf(ref_functions.Get(f)))
					} else {
						encode(structure, reflect.ValueOf(f))
					}
				},
				Structure: func(s Structure, t Structure) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Struct || kind == reflect.Interface {
						structure.Set(ref_structures.Get(t))
					} else {
						encode(structure, reflect.ValueOf(t))
					}
				},
				Map: func(s Structure, m Map) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Map {
						structure.Set(reflect.ValueOf(ref_maps.Get(m)))
					} else {
						encode(structure, reflect.ValueOf(m))
					}
				},
				Pointer: func(s Structure, p Pointer) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Ptr {
						structure.Set(reflect.ValueOf(ref_pointers.Get(p)))
					} else {
						encode(structure, reflect.ValueOf(p))
					}
				},
				Slice: func(s Structure, sl Slice) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Slice {
						structure.Set(reflect.ValueOf(ref_slices.Get(sl)))
					} else {
						encode(structure, reflect.ValueOf(sl))
					}
				},
				String: func(s Structure, str String) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.String {
						structure.SetString(ref_strings.Get(str).String())
					} else {
						encode(structure, reflect.ValueOf(str))
					}
				},
				Type: func(s Structure, t Type) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Interface {
						structure.Set(reflect.ValueOf(ref_types.Get(t)))
					} else {
						encode(structure, reflect.ValueOf(t))
					}
				},
				Field: func(s Structure, f Field) {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Struct {
						structure.FieldByIndex(ref_fields.Get(f).Index).Set(reflect.ValueOf(ref_fields.Get(f)))
					} else {
						encode(structure, reflect.ValueOf(f))
					}
				},
			},
			Decode: Decoding{
				Bool: func(s Structure) bool {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Bool {
						return structure.Bool()
					} else {
						return decode[bool](structure)
					}
				},
				Int: func(s Structure) int {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int {
						return int(structure.Int())
					} else {
						return decode[int](structure)
					}
				},
				Int8: func(s Structure) int8 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int8 {
						return int8(structure.Int())
					} else {
						return decode[int8](structure)
					}
				},
				Int16: func(s Structure) int16 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int16 {
						return int16(structure.Int())
					} else {
						return decode[int16](structure)
					}
				},
				Int32: func(s Structure) int32 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int32 {
						return int32(structure.Int())
					} else {
						return decode[int32](structure)
					}
				},
				Int64: func(s Structure) int64 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Int64 {
						return structure.Int()
					} else {
						return decode[int64](structure)
					}
				},
				Uint: func(s Structure) uint {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint {
						return uint(structure.Uint())
					} else {
						return decode[uint](structure)
					}
				},
				Uint8: func(s Structure) uint8 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint8 {
						return uint8(structure.Uint())
					} else {
						return decode[uint8](structure)
					}
				},
				Uint16: func(s Structure) uint16 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint16 {
						return uint16(structure.Uint())
					} else {
						return decode[uint16](structure)
					}
				},
				Uint32: func(s Structure) uint32 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint32 {
						return uint32(structure.Uint())
					} else {
						return decode[uint32](structure)
					}
				},
				Uint64: func(s Structure) uint64 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uint64 {
						return structure.Uint()
					} else {
						return decode[uint64](structure)
					}
				},
				Uintptr: func(s Structure) uintptr {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Uintptr {
						return uintptr(structure.Uint())
					} else {
						return decode[uintptr](structure)
					}
				},
				Float32: func(s Structure) float32 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Float32 {
						return float32(structure.Float())
					} else {
						return decode[float32](structure)
					}
				},
				Float64: func(s Structure) float64 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Float64 {
						return structure.Float()
					} else {
						return decode[float64](structure)
					}
				},
				Complex64: func(s Structure) complex64 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Complex64 {
						return complex64(structure.Complex())
					} else {
						return decode[complex64](structure)
					}
				},
				Complex128: func(s Structure) complex128 {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Complex128 {
						return structure.Complex()
					} else {
						return decode[complex128](structure)
					}
				},
				Chan: func(s Structure) Channel {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Chan {
						return Channel(ref_channels.New(structure.Interface().(reflect.Value)))
					} else {
						return Channel(decodeRef(structure, 0))
					}
				},
				Func: func(s Structure) Function {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Func {
						return Function(ref_functions.New(structure.Interface().(reflect.Value)))
					} else {
						return Function(decodeRef(structure, 0))
					}
				},
				Map: func(s Structure) Map {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Map {
						return Map(ref_maps.New(structure.Interface().(reflect.Value)))
					} else {
						return Map(decodeRef(structure, 0))
					}
				},
				Pointer: func(s Structure) Pointer {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Ptr {
						return Pointer(ref_pointers.New(structure.Interface().(reflect.Value)))
					} else {
						return Pointer(decodeRef(structure, 0))
					}
				},
				Slice: func(s Structure) Slice {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Slice {
						return Slice(ref_slices.New(structure.Interface().(reflect.Value)))
					} else {
						return Slice(decodeRef(structure, 0))
					}
				},
				String: func(s Structure) String {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.String {
						return String(ref_strings.New(reflect.ValueOf(structure.String())))
					} else {
						return String(decodeRef(structure, 0))
					}
				},
				Struct: func(s Structure) Structure {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Struct || kind == reflect.Interface {
						return s
					} else {
						return Structure(decodeRef(structure, 0))
					}
				},
				Type: func(s Structure) Type {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Interface {
						return Type(ref_types.New(structure.Interface().(reflect.Type)))
					} else {
						return Type(decodeRef(structure, 0))
					}
				},
				Field: func(s Structure) Field {
					structure := ref_structures.Get(s)
					if kind := structure.Type().Kind(); kind == reflect.Struct {
						return Field(ref_fields.New(structure.Interface().(reflect.StructField)))
					} else {
						return Field(decodeRef(structure, 0))
					}
				},
			},
		},
		Type: Types{
			Arg: func(t Type, idx uint32) Type {
				return Type(ref_types.New(ref_types.Get(t).In(int(idx))))
			},
			Args: func(t Type) uint32 {
				return uint32(ref_types.Get(t).NumIn())
			},
			Outs: func(t Type) uint32 {
				return uint32(ref_types.Get(t).NumOut())
			},
			Out: func(t Type, idx uint32) Type {
				return Type(ref_types.New(ref_types.Get(t).Out(int(idx))))
			},
			Elem: func(t Type) Type {
				return Type(ref_types.New(ref_types.Get(t).Elem()))
			},
			Key: func(t Type) Type {
				return Type(ref_types.New(ref_types.Get(t).Key()))
			},
			Dir: func(t Type) reflect.ChanDir {
				return ref_types.Get(t).ChanDir()
			},
			Kind:       func(t Type) reflect.Kind { return ref_types.Get(t).Kind() },
			FieldAlign: func(t Type) uint32 { return uint32(ref_types.Get(t).FieldAlign()) },
			Align:      func(t Type) uint32 { return uint32(ref_types.Get(t).Align()) },
			Field: func(t Type, idx uint32) Field {
				return Field(ref_fields.New(ref_types.Get(t).Field(int(idx))))
			},
			Size: func(t Type) uintptr { return ref_types.Get(t).Size() },
			Len: func(t Type) uint32 {
				switch ref_types.Get(t).Kind() {
				case reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
					return uint32(ref_types.Get(t).Len())
				}
				return uint32(ref_types.Get(t).NumField())
			},
			String: func(t Type) String { return String(ref_strings.New(reflect.ValueOf(ref_types.Get(t).String()))) },
			Name: func(t Type) String {
				return String(ref_strings.New(reflect.ValueOf(ref_types.Get(t).Name())))
			},
			Package: func(t Type) String {
				return String(ref_strings.New(reflect.ValueOf(ref_types.Get(t).PkgPath())))
			},
			Free: func(t Type) { ref_types.End(t) },
		},
		Field: Fields{
			Tag: func(f Field) String {
				return String(ref_strings.New(reflect.ValueOf(ref_fields.Get(f).Tag)))
			},
			Name: func(f Field) String {
				return String(ref_strings.New(reflect.ValueOf(ref_fields.Get(f).Name)))
			},
			Free: func(f Field) { ref_fields.End(f) },
			Embedded: func(f Field) bool {
				return ref_fields.Get(f).Anonymous
			},
			Type: func(f Field) Type {
				return Type(ref_types.New(ref_fields.Get(f).Type))
			},
			Offset: func(f Field) uintptr {
				return ref_fields.Get(f).Offset
			},
			Package: func(f Field) String {
				return String(ref_strings.New(reflect.ValueOf(ref_fields.Get(f).PkgPath)))
			},
		},
	}
}

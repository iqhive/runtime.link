package ffi

import (
	"encoding/binary"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ref_strings handle[string, String]
	ref_reflect handle[reflect.Value, uint64]
	ref_readers handle[io.ReaderAt, Bytes]
	ref_field   handle[reflect.StructField, Field]
)

var (
	ref_slices     = (*handle[reflect.Value, Slice])(&ref_reflect)
	ref_types      = (*handle[reflect.Type, Type])(&ref_reflect)
	ref_functions  = (*handle[reflect.Value, Function])(&ref_reflect)
	ref_interfaces = (*handle[reflect.Value, Any])(&ref_reflect)
	ref_pointers   = (*handle[reflect.Value, Pointer])(&ref_reflect)
	ref_channels   = (*handle[reflect.Value, Channel])(&ref_reflect)
	ref_maps       = (*handle[reflect.Value, Map])(&ref_reflect)
)

func new_ref(val reflect.Value) uintptr {
	rvalue := val
	switch rvalue.Kind() {
	case reflect.String:
		return uintptr(ref_strings.New(rvalue.String()))
	default:
		return uintptr(ref_reflect.New(rvalue))
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
	v, _ := h.m.Load(uintptr(id))
	return v.(T)
}

func (h *handle[T, ID]) End(id ID) { h.m.Delete(uintptr(id)) }

// New returns an new FFI API implementation in Go.
func New() API {
	return API{
		String: Strings{
			New: func(r io.Reader, n int) String {
				var b strings.Builder
				io.CopyN(&b, r, int64(n))
				return String(ref_strings.New(b.String()))
			},
			Len: func(s String) int { return len(ref_strings.Get(s)) },
			Data: func(s String) Bytes {
				return ref_readers.New(strings.NewReader(ref_strings.Get(s)))
			},
			Iter: func(s String, fn Function) {
				for _, r := range ref_strings.Get(s) {
					result := ref_functions.Get(fn).Call([]reflect.Value{reflect.ValueOf(r)})
					if !result[0].Bool() {
						break
					}
				}
			},
			Free: func(s String) { ref_strings.End(s) },
		},
		Slice: Slices{
			Make: func(t Type, l, c int) Slice { return Slice(ref_slices.New(reflect.MakeSlice(ref_types.Get(t), l, c))) },
			Nil:  func(s Slice) bool { return ref_slices.Get(s).IsNil() },
			Cap:  func(s Slice) int { return ref_slices.Get(s).Cap() },
			Len:  func(s Slice) int { return ref_slices.Get(s).Len() },
			Type: func(s Any) Type { return Type(ref_types.New(ref_interfaces.Get(s).Type().Elem())) },
			Data: func(s Slice) Bytes {
				slice := ref_slices.Get(s)
				return ref_readers.New(newSliceReader(slice))
			},
			Iter: func(s Slice, yield Function) {
				slice := ref_slices.Get(s)
				for i := range slice.Len() {
					result := ref_functions.Get(yield).Call([]reflect.Value{slice.Index(i)})
					if !result[0].Bool() {
						break
					}
				}
			},
			Copy: func(dst Slice, src Slice) { reflect.Copy(ref_slices.Get(dst), ref_slices.Get(src)) },
			Get: func(s Slice, idx uint32) Pointer {
				return Pointer(ref_reflect.New(ref_slices.Get(s).Index(int(idx))))
			},
			Free: func(s Slice) { ref_slices.End(s) },
		},
		Pointer: Pointers{
			New: func(rtype Type) Pointer {
				return Pointer(ref_reflect.New(reflect.New(ref_types.Get(rtype)).Elem()))
			},
			Type: func(p Any) Type {
				return Type(ref_types.New(ref_interfaces.Get(p).Type().Elem()))
			},
			Data: func(p Pointer) Bytes {
				return ref_readers.New(newPointerBytes(ref_pointers.Get(p)))
			},
			Nil:  func(p Pointer) bool { return ref_pointers.Get(p).IsNil() },
			Free: func(p Pointer) { ref_pointers.End(p) },
		},
		Channel: Channels{
			Make: func(t Type, n int) Channel {
				return Channel(ref_channels.New(reflect.MakeChan(ref_types.Get(t), n)))
			},
			Nil:  func(c Channel) bool { return ref_channels.Get(c).IsNil() },
			Len:  func(c Channel) int { return ref_channels.Get(c).Len() },
			Dir:  func(c Type) reflect.ChanDir { return ref_types.Get(c).ChanDir() },
			Cap:  func(c Channel) int { return ref_channels.Get(c).Cap() },
			Type: func(c Any) Type { return Type(ref_types.New(ref_interfaces.Get(c).Type().Elem())) },
			Iter: func(c Channel, yield Function) {
				channel := ref_channels.Get(c)
				for {
					val, ok := channel.Recv()
					result := ref_functions.Get(yield).Call([]reflect.Value{val})
					if !ok || !result[0].Bool() {
						return
					}
				}
			},
			Close: func(c Channel) { ref_channels.Get(c).Close() },
			Recv: func(c Channel, dst Bytes) bool {
				val, ok := ref_channels.Get(c).Recv()
				if !ok {
					return false
				}
				writeValue(ref_readers.Get(dst).(io.WriterAt), 0, val)
				return true
			},
			Send: func(c Channel, val Bytes) {
				ch := ref_channels.Get(c)
				tmp := reflect.New(ch.Type().Elem()).Elem()
				writeBytes(tmp, 0, ref_readers.Get(val))
				ch.Send(tmp)
			},
			Free: func(c Channel) { ref_channels.End(c) },
		},
		Bytes: Memory{
			Set8: func(addr Bytes, off int, val uint64) {
				var buf [8]byte
				binary.LittleEndian.PutUint64(buf[:], val)
				ref_readers.Get(addr).(io.WriterAt).WriteAt(buf[:], int64(off))
			},
			Set4: func(addr Bytes, off int, val uint32) {
				var buf [4]byte
				binary.LittleEndian.PutUint32(buf[:], val)
				ref_readers.Get(addr).(io.WriterAt).WriteAt(buf[:], int64(off))
			},
			Set2: func(addr Bytes, off int, val uint16) {
				var buf [2]byte
				binary.LittleEndian.PutUint16(buf[:], val)
				ref_readers.Get(addr).(io.WriterAt).WriteAt(buf[:], int64(off))
			},
			Set1: func(addr Bytes, off int, val byte) {
				ref_readers.Get(addr).(io.WriterAt).WriteAt([]byte{val}, int64(off))
			},
			Get1: func(addr Bytes, off int) byte {
				var buf [1]byte
				ref_readers.Get(addr).(io.ReaderAt).ReadAt(buf[:], int64(off))
				return buf[0]
			},
			Get2: func(addr Bytes, off int) uint16 {
				var buf [2]byte
				ref_readers.Get(addr).(io.ReaderAt).ReadAt(buf[:], int64(off))
				return binary.LittleEndian.Uint16(buf[:])
			},
			Get4: func(addr Bytes, off int) uint32 {
				var buf [4]byte
				ref_readers.Get(addr).(io.ReaderAt).ReadAt(buf[:], int64(off))
				return binary.LittleEndian.Uint32(buf[:])
			},
			Get8: func(addr Bytes, off int) uint64 {
				var buf [8]byte
				ref_readers.Get(addr).(io.ReaderAt).ReadAt(buf[:], int64(off))
				return binary.LittleEndian.Uint64(buf[:])
			},
		},
		Function: Functions{
			Free: func(f Function) { ref_functions.End(f) },
			Outs: func(f Any) int { return ref_interfaces.Get(f).Type().NumOut() },
			Args: func(f Any) int { return ref_interfaces.Get(f).Type().NumIn() },
			Nil:  func(f Function) bool { return ref_functions.Get(f).IsNil() },
			Type: func(f Any, n int) Type {
				rtype := ref_interfaces.Get(f).Type()
				if n < rtype.NumIn() {
					return Type(ref_types.New(rtype.In(n)))
				}
				return Type(ref_types.New(rtype.Out(n - rtype.NumIn())))
			},
			Call: func(f Function, args Bytes, rets Bytes) {
				panic("not implemented")
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
			Key: func(m Any) Type {
				return Type(ref_types.New(ref_interfaces.Get(m).Type().Key()))
			},
			Val: func(m Any) Type {
				return Type(ref_types.New(ref_interfaces.Get(m).Type().Elem()))
			},
			Iter: func(m Map, yield Function) {
				for _, key := range ref_maps.Get(m).MapKeys() {
					val := ref_maps.Get(m).MapIndex(key)
					result := ref_functions.Get(yield).Call([]reflect.Value{key, val})
					if !result[0].Bool() {
						break
					}
				}
			},
			Delete: func(m Map, b Bytes) {
				var table = ref_maps.Get(m)
				var mtype = table.Type()
				var key = reflect.New(mtype.Key()).Elem()
				writeBytes(key, 0, ref_readers.Get(b))
				table.SetMapIndex(key, reflect.Value{})
			},
			Set: func(m Map, src Bytes, key_bytes Bytes) {
				var table = ref_maps.Get(m)
				var mtype = table.Type()
				var key = reflect.New(mtype.Key()).Elem()
				var val = reflect.New(mtype.Elem()).Elem()
				writeBytes(key, 0, ref_readers.Get(key_bytes))
				writeBytes(val, 0, ref_readers.Get(src))
				table.SetMapIndex(key, val)
			},
			Get: func(m Map, dst Bytes, key_bytes Bytes) bool {
				var table = ref_maps.Get(m)
				var mtype = table.Type()
				var key = reflect.New(mtype.Key()).Elem()
				writeBytes(key, 0, ref_readers.Get(key_bytes))
				val := table.MapIndex(key)
				if !val.IsValid() {
					return false
				}
				writeValue(ref_readers.Get(dst).(io.WriterAt), 0, val)
				return true
			},
		},
		Any: Interfaces{
			Nil: func(a Any) bool { return ref_interfaces.Get(a).IsNil() },
			Data: func(a Any) Bytes {
				value := ref_reflect.Get(uint64(a))
				if !value.CanAddr() {
					ptr := reflect.New(value.Type()).Elem()
					ptr.Set(value)
					value = ptr
				}
				value = value.Addr()
				return ref_readers.New(newPointerBytes(value))
			},
			Type: func(a Any) Type {
				return Type(ref_types.New(ref_interfaces.Get(a).Type()))
			},
			Free: func(a Any) { ref_interfaces.End(a) },
		},
		Type: Types{
			Kind:       func(t Type) reflect.Kind { return ref_types.Get(t).Kind() },
			FieldAlign: func(t Type) int { return ref_types.Get(t).FieldAlign() },
			Align:      func(t Type) int { return ref_types.Get(t).Align() },
			Field: func(t Type, idx int) Field {
				return Field(ref_field.New(ref_types.Get(t).Field(idx)))
			},
			Size: func(t Type) uintptr { return ref_types.Get(t).Size() },
			Len: func(t Type) int {
				switch ref_types.Get(t).Kind() {
				case reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
					return ref_types.Get(t).Len()
				}
				return ref_types.Get(t).NumField()
			},
			String: func(t Type) String { return String(ref_strings.New(ref_types.Get(t).String())) },
			Name: func(t Type) String {
				return String(ref_strings.New(ref_types.Get(t).Name()))
			},
			Package: func(t Type) String {
				return String(ref_strings.New(ref_types.Get(t).PkgPath()))
			},
			Free: func(t Type) { ref_types.End(t) },
		},
		Field: Fields{
			Tag: func(f Field) String {
				return String(ref_strings.New(ref_field.Get(f).Tag.Get("json")))
			},
			Name: func(f Field) String {
				return String(ref_strings.New(ref_field.Get(f).Name))
			},
			Free: func(f Field) { ref_field.End(f) },
			Embedded: func(f Field) bool {
				return ref_field.Get(f).Anonymous
			},
			Type: func(f Field) Type {
				return Type(ref_types.New(ref_field.Get(f).Type))
			},
			Offset: func(f Field) uintptr {
				return ref_field.Get(f).Offset
			},
			Package: func(f Field) String {
				return String(ref_strings.New(ref_field.Get(f).PkgPath))
			},
		},
	}
}

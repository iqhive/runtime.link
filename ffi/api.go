// Package ffi provides a bridge for data types that cross runtime boundaries.
package ffi

import (
	"io"
	"reflect"

	"runtime.link/api"
)

type API struct {
	api.Specification `api:"runtime.link/ffi"`

	String    Strings
	Slice     Slices
	Pointer   Pointers
	Channel   Channels
	Function  Functions
	Map       Maps
	Structure Structures

	Type  Types
	Field Fields
}

type Strings struct {
	New  func(io.Reader, uint32) String `api:"string_new"`
	Len  func(String) uint32            `api:"string_len"`
	Data func(String) Structure         `api:"string_data"`
	Free func(String)                   `api:"string_free"`
}

type Slices struct {
	Make func(Type, uint32, uint32) Slice     `api:"slice_make"`
	Data func(s Slice) Structure              `api:"slice_data"`
	From func(s Slice, idx, end uint32) Slice `api:"slice_from"`
	Nil  func(s Slice) bool                   `api:"slice_nil"`
	Len  func(s Slice) uint32                 `api:"slice_len"`
	Cap  func(s Slice) uint32                 `api:"slice_cap"`
	Elem func(s Slice, idx uint32) Pointer    `api:"slice_elem"`
	Copy func(dst, src Slice)                 `api:"slice_copy"`
	Free func(s Slice)                        `api:"slice_free"`
}

type Pointers struct {
	New  func(Type) Pointer        `api:"pointer_new"`
	Nil  func(p Pointer) bool      `api:"pointer_nil"`
	Data func(p Pointer) Structure `api:"pointer_data"`
	Free func(p Pointer)           `api:"pointer_free"`
}

type Channels struct {
	Make  func(Type, uint32) Channel `api:"chan_make"`
	Nil   func(c Channel) bool       `api:"chan_nil"`
	Len   func(c Channel) int        `api:"chan_len"`
	Cap   func(c Channel) int        `api:"chan_cap"`
	Data  func(c Channel) Structure  `api:"chan_data"`
	Close func(c Channel)            `api:"chan_close"`
	Free  func(c Channel)            `api:"chan_free"`
}

type Functions struct {
	Nil  func(f Function) bool                      `api:"func_nil"`
	Args func(f Function) Structure                 `api:"func_args"`
	Call func(f Function, args Structure) Structure `api:"func_call"`
	Free func(f Function)                           `api:"func_free"`
}

type Maps struct {
	Make   func(Type) Map                       `api:"map_make"`
	Nil    func(m Map) bool                     `api:"map_nil"`
	Len    func(m Map) int                      `api:"map_len"`
	Key    func(m Map) Structure                `api:"map_key"`
	Data   func(m Map) Structure                `api:"map_data"`
	Get    func(m Map, key Structure) Structure `api:"map_read"`
	Delete func(m Map, key Structure)           `api:"map_delete"`
	Clear  func(m Map)                          `api:"map_clear"`
	Free   func(m Map)                          `api:"map_free"`
}

type Structures struct {
	Zero func(a Structure) bool `api:"struct_zero"`
	Type func(a Structure) Type `api:"struct_type"`
	Free func(a Structure)      `api:"struct_free"`

	Encode Encoding
	Decode Decoding
}

type Types struct {
	Kind       func(t Type) reflect.Kind      `api:"type_kind"`
	Align      func(t Type) uint32            `api:"type_align"`
	FieldAlign func(t Type) uint32            `api:"type_field_align"`
	Arg        func(t Type, idx uint32) Type  `api:"type_arg"`
	Out        func(t Type, idx uint32) Type  `api:"type_out"`
	Dir        func(t Type) reflect.ChanDir   `api:"type_dir"`
	Key        func(t Type) Type              `api:"type_key"`
	Elem       func(t Type) Type              `api:"type_elem"`
	Args       func(t Type) uint32            `api:"type_args"`
	Outs       func(t Type) uint32            `api:"type_outs"`
	Size       func(t Type) uintptr           `api:"type_size"`
	Name       func(t Type) String            `api:"type_name"`
	Package    func(t Type) String            `api:"type_package"`
	String     func(t Type) String            `api:"type_string"`
	Len        func(t Type) uint32            `api:"type_len"`
	Field      func(t Type, idx uint32) Field `api:"type_field"`
	Free       func(t Type)                   `api:"type_free"`
}

type Fields struct {
	Name     func(f Field) String  `api:"field_name"`
	Package  func(f Field) String  `api:"field_package"`
	Offset   func(f Field) uintptr `api:"field_offset"`
	Type     func(f Field) Type    `api:"field_type"`
	Tag      func(f Field) String  `api:"field_tag"`
	Embedded func(f Field) bool    `api:"field_embedded"`
	Free     func(f Field)         `api:"field_free"`
}

type Encoding struct {
	Bool       func(Structure, bool)       `api:"encode_bool"`
	Int        func(Structure, int)        `api:"encode_int"`
	Int8       func(Structure, int8)       `api:"encode_int8"`
	Int16      func(Structure, int16)      `api:"encode_int16"`
	Int32      func(Structure, int32)      `api:"encode_int32"`
	Int64      func(Structure, int64)      `api:"encode_int64"`
	Uint       func(Structure, uint)       `api:"encode_uint"`
	Uint8      func(Structure, uint8)      `api:"encode_uint8"`
	Uint16     func(Structure, uint16)     `api:"encode_uint16"`
	Uint32     func(Structure, uint32)     `api:"encode_uint32"`
	Uint64     func(Structure, uint64)     `api:"encode_uint64"`
	Uintptr    func(Structure, uintptr)    `api:"encode_uintptr"`
	Float32    func(Structure, float32)    `api:"encode_float32"`
	Float64    func(Structure, float64)    `api:"encode_float64"`
	Complex64  func(Structure, complex64)  `api:"encode_complex64"`
	Complex128 func(Structure, complex128) `api:"encode_complex128"`
	Chan       func(Structure, Channel)    `api:"encode_chan"`
	Func       func(Structure, Function)   `api:"encode_func"`
	Structure  func(Structure, Structure)  `api:"encode_struct"`
	Map        func(Structure, Map)        `api:"encode_map"`
	Pointer    func(Structure, Pointer)    `api:"encode_pointer"`
	Slice      func(Structure, Slice)      `api:"encode_slice"`
	String     func(Structure, String)     `api:"encode_string"`
	Field      func(Structure, Field)      `api:"encode_field"`
	Type       func(Structure, Type)       `api:"encode_type"`
}

type Decoding struct {
	Bool       func(Structure) bool       `api:"decode_bool"`
	Int        func(Structure) int        `api:"decode_int"`
	Int8       func(Structure) int8       `api:"decode_int8"`
	Int16      func(Structure) int16      `api:"decode_int16"`
	Int32      func(Structure) int32      `api:"decode_int32"`
	Int64      func(Structure) int64      `api:"decode_int64"`
	Uint       func(Structure) uint       `api:"decode_uint"`
	Uint8      func(Structure) uint8      `api:"decode_uint8"`
	Uint16     func(Structure) uint16     `api:"decode_uint16"`
	Uint32     func(Structure) uint32     `api:"decode_uint32"`
	Uint64     func(Structure) uint64     `api:"decode_uint64"`
	Uintptr    func(Structure) uintptr    `api:"decode_uintptr"`
	Float32    func(Structure) float32    `api:"decode_float32"`
	Float64    func(Structure) float64    `api:"decode_float64"`
	Complex64  func(Structure) complex64  `api:"decode_complex64"`
	Complex128 func(Structure) complex128 `api:"decode_complex128"`
	Chan       func(Structure) Channel    `api:"decode_chan"`
	Func       func(Structure) Function   `api:"decode_func"`
	Struct     func(Structure) Structure  `api:"decode_struct"`
	Map        func(Structure) Map        `api:"decode_map"`
	Pointer    func(Structure) Pointer    `api:"decode_pointer"`
	Slice      func(Structure) Slice      `api:"decode_slice"`
	String     func(Structure) String     `api:"decode_string"`
	Field      func(Structure) Field      `api:"decode_field"`
	Type       func(Structure) Type       `api:"decode_type"`
}

type (
	Function  uint64
	String    uint64
	Slice     uint64
	Any       uint64
	Type      uint64
	Channel   uint64
	Pointer   uint64
	Bytes     uint64
	Map       uint64
	Field     uint64
	Structure uint64
)

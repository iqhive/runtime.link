// Package ffi provides a bridge for data types that cross runtime boundaries.
package ffi

import (
	"io"
	"reflect"

	"runtime.link/api"
)

type API struct {
	api.Specification

	Bytes    Memory
	String   Strings
	Slice    Slices
	Pointer  Pointers
	Channel  Channels
	Function Functions
	Map      Maps

	Any  Interfaces
	Type Types

	Field Fields
}

type Strings struct {
	New  func(io.Reader, int) String `api:"string_new"`
	Len  func(String) int            `api:"string_len"`
	Iter func(String, Function)      `api:"string_iter"`
	Data func(String) Bytes          `api:"string_data"`
	Free func(String)                `api:"string_free"`
}

type Slices struct {
	Make func(Type, int, int) Slice        `api:"slice_make"`
	Data func(s Slice) Bytes               `api:"slice_data"`
	Nil  func(s Slice) bool                `api:"slice_nil"`
	Len  func(s Slice) int                 `api:"slice_len"`
	Cap  func(s Slice) int                 `api:"slice_cap"`
	Get  func(s Slice, idx uint32) Pointer `api:"slice_get"`
	Type func(s Any) Type                  `api:"slice_type"`
	Copy func(dst, src Slice)              `api:"slice_copy"`
	Iter func(s Slice, yield Function)     `api:"slice_iter"`
	Free func(s Slice)                     `api:"slice_free"`
}

type Pointers struct {
	New  func(Type) Pointer    `api:"pointer_new"`
	Nil  func(p Pointer) bool  `api:"pointer_nil"`
	Type func(p Any) Type      `api:"pointer_type"`
	Data func(p Pointer) Bytes `api:"pointer_data"`
	Free func(p Pointer)       `api:"pointer_free"`
}

type Memory struct {
	Get1 func(addr Bytes, off int) byte        `api:"bytes_get1"`
	Get2 func(addr Bytes, off int) uint16      `api:"bytes_get2"`
	Get4 func(addr Bytes, off int) uint32      `api:"bytes_get4"`
	Get8 func(addr Bytes, off int) uint64      `api:"bytes_get8"`
	Set1 func(addr Bytes, off int, val byte)   `api:"bytes_set1"`
	Set2 func(addr Bytes, off int, val uint16) `api:"bytes_set2"`
	Set4 func(addr Bytes, off int, val uint32) `api:"bytes_set4"`
	Set8 func(addr Bytes, off int, val uint64) `api:"bytes_set8"`
}

type Channels struct {
	Make  func(Type, int) Channel         `api:"chan_make"`
	Nil   func(c Channel) bool            `api:"chan_nil"`
	Len   func(c Channel) int             `api:"chan_len"`
	Dir   func(c Type) reflect.ChanDir    `api:"chan_dir"`
	Cap   func(c Channel) int             `api:"chan_cap"`
	Type  func(c Any) Type                `api:"chan_type"`
	Iter  func(c Channel, yield Function) `api:"chan_iter"`
	Send  func(c Channel, val Bytes)      `api:"chan_send"`
	Recv  func(c Channel, dst Bytes) bool `api:"chan_recv"`
	Close func(c Channel)                 `api:"chan_close"`
	Free  func(c Channel)                 `api:"chan_free"`
}

type Functions struct {
	Nil  func(f Function) bool                    `api:"func_nil"`
	Args func(f Any) int                          `api:"func_args"`
	Outs func(f Any) int                          `api:"func_outs"`
	Type func(f Any, n int) Type                  `api:"func_type"`
	Call func(f Function, args Bytes, rets Bytes) `api:"func_call"`
	Free func(f Function)                         `api:"func_free"`
}

type Maps struct {
	Make   func(Type) Map                         `api:"map_make"`
	Nil    func(m Map) bool                       `api:"map_nil"`
	Len    func(m Map) int                        `api:"map_len"`
	Key    func(m Any) Type                       `api:"map_key"`
	Val    func(m Any) Type                       `api:"map_val"`
	Iter   func(m Map, yield Function)            `api:"map_iter"`
	Get    func(m Map, dst Bytes, key Bytes) bool `api:"map_read"`
	Set    func(m Map, src Bytes, key Bytes)      `api:"map_write"`
	Delete func(m Map, key Bytes)                 `api:"map_delete"`
	Clear  func(m Map)                            `api:"map_clear"`
	Free   func(m Map)                            `api:"map_free"`
}

type Interfaces struct {
	Nil  func(a Any) bool  `api:"any_nil"`
	Data func(a Any) Bytes `api:"any_data"`
	Type func(a Any) Type  `api:"any_type"`
	Free func(a Any)       `api:"any_free"`
}

type Types struct {
	Kind       func(t Type) reflect.Kind   `api:"type_kind"`
	Align      func(t Type) int            `api:"type_align"`
	FieldAlign func(t Type) int            `api:"type_field_align"`
	Size       func(t Type) uintptr        `api:"type_size"`
	Name       func(t Type) String         `api:"type_name"`
	Package    func(t Type) String         `api:"type_package"`
	String     func(t Type) String         `api:"type_string"`
	Len        func(t Type) int            `api:"type_len"`
	Field      func(t Type, idx int) Field `api:"type_field"`
	Free       func(t Type)                `api:"type_free"`
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

type (
	Function uint64
	String   uint64
	Slice    uint64
	Any      uint64
	Type     uint64
	Channel  uint64
	Pointer  uint64
	Bytes    uint64
	Map      uint64
	Field    uint64
)

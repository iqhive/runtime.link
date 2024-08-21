package xyz

import (
	"reflect"
	"unsafe"
)

type move struct {
	from uintptr
	into uintptr
}

type cast struct {
	from reflect.StructField
	into reflect.StructField
}

type packing struct {
	moves []move
	casts []cast
}

func (p *packing) pack(val any, ram any) {
	a := reflect.ValueOf(val).UnsafePointer()
	b := reflect.ValueOf(ram).UnsafePointer()
	for _, m := range p.moves {
		*(*uintptr)(unsafe.Add(b, m.into)) = *(*uintptr)(unsafe.Add(a, m.from))
	}
	for _, c := range p.casts {
		reflect.NewAt(c.into.Type, unsafe.Add(b, c.into.Offset)).Elem().Set(reflect.NewAt(c.from.Type, unsafe.Add(a, c.from.Offset)).Elem())
	}
}

func (p *packing) unpack(val any, ram any) {
	a := reflect.ValueOf(val).UnsafePointer()
	b := reflect.ValueOf(ram).UnsafePointer()
	for _, m := range p.moves {
		*(*uintptr)(unsafe.Add(a, m.from)) = *(*uintptr)(unsafe.Add(b, m.into))
	}
	for _, c := range p.casts {
		reflect.NewAt(c.from.Type, unsafe.Add(a, c.from.Offset)).Elem().Set(reflect.NewAt(c.into.Type, unsafe.Add(b, c.into.Offset)).Elem())
	}
}

type slot struct {
	reflect.Type

	from uintptr
	size uintptr
}

type packer struct {
	ptrSlots map[reflect.Type][]slot
	valSlots []slot
}

func (p *packer) fill(t reflect.Type, offset uintptr) {
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			p.fill(t.Field(i).Type, offset+t.Field(i).Offset)
		}
	case reflect.Array:
		for i := 0; i < t.Len(); i++ {
			p.fill(t.Elem(), offset+uintptr(i)*t.Elem().Size())
		}
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		p.ptrSlots[t] = append(p.ptrSlots[t], slot{
			Type: t,

			from: offset,
			size: t.Size(),
		})
	default:
		p.valSlots = append(p.valSlots, slot{
			Type: t,

			from: offset,
			size: t.Size(),
		})
	}
}

func (p *packing) make(from reflect.Type, into reflect.Type) {
	var packer = packer{
		ptrSlots: make(map[reflect.Type][]slot),
	}
	packer.fill(into, 0)

}

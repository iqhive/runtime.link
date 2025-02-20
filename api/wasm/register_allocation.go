package wasm

import (
	"reflect"
	"unsafe"
)

type RegisterMapping struct {
	offset int
	spills bool
}

func (m RegisterMapping) Write(stack, spill []byte, value reflect.Value) {
	if !value.CanAddr() {
		ptr := reflect.New(value.Type())
		ptr.Elem().Set(value)
		value = ptr.Elem()
	}
	raw := unsafe.Slice((*byte)(value.Addr().UnsafePointer()), value.Type().Size())
	if m.spills {
		copy(spill[m.offset:], raw)
	} else {
		copy(stack[m.offset:], raw)
	}
}

func (m RegisterMapping) Read(stack uint64, spill []byte, value reflect.Value) {
	stack_raw := unsafe.Slice((*byte)(unsafe.Pointer(&stack)), 8)
	raw := unsafe.Slice((*byte)(value.Addr().UnsafePointer()), value.Type().Size())
	if m.spills {
		copy(raw, spill[m.offset:])
	} else {
		copy(raw, stack_raw[m.offset:])
	}
}

func sizeToMask(size int) uint64 {
	switch size {
	case 1:
		return 0xFF
	case 2:
		return 0xFFFF
	case 3:
		return 0xFFFFFF
	case 4:
		return 0xFFFFFFFF
	case 5:
		return 0xFFFFFFFFFF
	case 6:
		return 0xFFFFFFFFFFFF
	case 7:
		return 0xFFFFFFFFFFFFFF
	case 8:
		return 0xFFFFFFFFFFFFFFFF
	default:
		panic("invalid size")
	}
}

func jumpType(rtype reflect.Type) int {
	var args int
	for i := 0; i < rtype.NumIn(); i++ {
		args += int(rtype.In(i).Size())
	}
	if rtype.NumOut() > 0 {
		return -1
	}
	switch {
	case args == 0:
		return 0
	case args <= 8:
		return 1
	case args <= 16:
		return 2
	case args <= 32:
		return 4
	case args <= 64:
		return 8
	default:
		return -1
	}
}

func registersCount(rtype reflect.Type) int {
	switch rtype.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return 1
	case reflect.Complex64, reflect.Complex128, reflect.String, reflect.Interface:
		return 2
	case reflect.Array:
		return rtype.Len() * registersCount(rtype.Elem())
	case reflect.Struct:
		var count int
		for i := 0; i < rtype.NumField(); i++ {
			count += registersCount(rtype.Field(i).Type)
		}
		return count
	case reflect.Uintptr, reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return 1
	default:
		return 0
	}
}

func defaultRegistersFor(rtype reflect.Type) (args, results []RegisterMapping) {
	var resultSize = 8
	var resultRegisterUsed = false
	var resultsOffset = 0
	for i := 0; i < rtype.NumOut(); i++ {
		size := int(rtype.Out(i).Size())
		if size == 0 {
			continue
		}
		if resultRegisterUsed || registersCount(rtype.Out(i)) > 1 {
			results = append(results, RegisterMapping{
				offset: resultsOffset,
				spills: true,
			})
			resultsOffset += size
			continue
		}
		results = append(results, RegisterMapping{
			offset: 0,
		})
		resultSize -= int(rtype.Out(i).Size())
		resultRegisterUsed = true
	}
	var n int
	for i := 0; i < rtype.NumIn(); i++ {
		size := int(rtype.In(i).Size())
		if size == 0 {
			continue
		}
		count := registersCount(rtype.In(i))
		args = append(args, RegisterMapping{
			offset: n,
		})
		n += count
	}
	return
}

func squeezeRegistersFor(rtype reflect.Type, max_registers int) (args, results []RegisterMapping, registerOffset, resultsOffset int) {
	var resultSize = 8
	var resultsSpillOver = false
	for i := 0; i < rtype.NumOut(); i++ {
		size := int(rtype.Out(i).Size())
		if size == 0 {
			continue
		}
		if size > resultSize {
			resultsSpillOver = true
			results = append(results, RegisterMapping{
				offset: resultsOffset,
				spills: true,
			})
			resultsOffset += size
			continue
		}
		results = append(results, RegisterMapping{
			offset: 8 - resultSize,
		})
		resultSize -= int(rtype.Out(i).Size())
	}
	if max_registers == 0 {
		return
	}
	if resultsSpillOver {
		max_registers-- // reserve results pointer
	}
	if max_registers == 0 {
		return
	}
	var total int
	for i := 0; i < rtype.NumIn(); i++ {
		size := int(rtype.In(i).Size())
		if size == 0 {
			continue
		}
		total += size
	}
	if total > max_registers*8 {
		max_registers-- // reserve arguments pointer
	}
	if max_registers == 0 {
		return
	}
	var registerSpace int = max_registers * 8
	for i := 0; i < rtype.NumIn(); i++ {
		size := int(rtype.In(i).Size())
		if size == 0 {
			continue
		}
		if size > registerSpace {
			args = append(args, RegisterMapping{
				offset: registerOffset,
				spills: true,
			})
			registerOffset += size
			continue
		}
		args = append(args, RegisterMapping{
			offset: (max_registers * 8) - registerSpace,
		})
		registerSpace -= size
	}
	return
}

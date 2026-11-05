package cpu

import (
	"reflect"
	"runtime"
	"unsafe"
)

// part is a piece of a Go value that fills one register.
type part struct {
	offset  uintptr
	size    uintptr
	float   bool
	pointer bool
}

// assignment says which registers Go's internal
// ABI gives to a value, if there are any.
type assignment struct {
	parts []part
	r, x  int // the first of each register kind.
}

// assign reserves the registers for a value of type t, r and x are the numbers
// of each kind in use. A value that can't be assigned entirely to registers
// goes on the stack, and leaves them for the values after it.
func assign(t reflect.Type, r, x *int) assignment {
	parts, ok := flatten(t, 0, nil)
	var integers, floats int
	for _, part := range parts {
		switch {
		case part.float:
			floats++
		default:
			integers++
		}
	}
	if !ok || *r+integers > generalRegisters || *x+floats > floatingRegisters {
		return assignment{}
	}
	to := assignment{parts: parts, r: *r, x: *x}
	*r += integers
	*x += floats
	return to
}

// flatten follows the register-assignment algorithm in abi-internal.md.
func flatten(t reflect.Type, offset uintptr, parts []part) ([]part, bool) {
	word := unsafe.Sizeof(uintptr(0))
	switch t.Kind() {
	case reflect.Bool, reflect.Uintptr,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return append(parts, part{offset: offset, size: t.Size()}), true
	case reflect.Float32, reflect.Float64:
		return append(parts, part{offset: offset, size: t.Size(), float: true}), true
	case reflect.Complex64, reflect.Complex128:
		half := t.Size() / 2
		return append(parts, part{offset: offset, size: half, float: true}, part{offset: offset + half, size: half, float: true}), true
	case reflect.Pointer, reflect.UnsafePointer, reflect.Func, reflect.Map, reflect.Chan:
		return append(parts, part{offset: offset, size: word, pointer: true}), true
	case reflect.String:
		return append(parts, part{offset: offset, size: word, pointer: true}, part{offset: offset + word, size: word}), true
	case reflect.Slice:
		return append(parts, part{offset: offset, size: word, pointer: true}, part{offset: offset + word, size: word}, part{offset: offset + 2*word, size: word}), true
	case reflect.Interface:
		return append(parts, part{offset: offset, size: word, pointer: true}, part{offset: offset + word, size: word, pointer: true}), true
	case reflect.Struct:
		ok := true
		for i := 0; ok && i < t.NumField(); i++ {
			field := t.Field(i)
			parts, ok = flatten(field.Type, offset+field.Offset, parts)
		}
		return parts, ok
	case reflect.Array:
		switch t.Len() {
		case 0:
			return parts, true
		case 1:
			return flatten(t.Elem(), offset, parts)
		}
	}
	return parts, false
}

// general returns the general register at index i.
func general(r *RegistersArch, i int) *Register {
	switch i {
	case 0:
		return r.R0()
	case 1:
		return r.R1()
	case 2:
		return r.R2()
	case 3:
		return r.R3()
	case 4:
		return r.R4()
	case 5:
		return r.R5()
	case 6:
		return r.R6()
	case 7:
		return r.R7()
	case 8:
		return r.R8()
	case 9:
		return r.R9()
	case 10:
		return r.R10()
	case 11:
		return r.R11()
	case 12:
		return r.R12()
	case 13:
		return r.R13()
	case 14:
		return r.R14()
	default:
		return r.R15()
	}
}

// floating returns the floating-point register at index i.
func floating(r *RegistersArch, i int) *FloatingPointRegister {
	switch i {
	case 0:
		return r.X0()
	case 1:
		return r.X1()
	case 2:
		return r.X2()
	case 3:
		return r.X3()
	case 4:
		return r.X4()
	case 5:
		return r.X5()
	case 6:
		return r.X6()
	case 7:
		return r.X7()
	case 8:
		return r.X8()
	case 9:
		return r.X9()
	case 10:
		return r.X10()
	case 11:
		return r.X11()
	case 12:
		return r.X12()
	case 13:
		return r.X13()
	case 14:
		return r.X14()
	default:
		return r.X15()
	}
}

// load the value into its registers.
func (a assignment) load(reg *RegistersArch, value reflect.Value) {
	memory := reflect.New(value.Type())
	memory.Elem().Set(value)
	base := memory.UnsafePointer()
	r, x := a.r, a.x
	for _, part := range a.parts {
		var bits uint64
		address := unsafe.Add(base, part.offset)
		switch part.size {
		case 1:
			bits = uint64(*(*uint8)(address))
		case 2:
			bits = uint64(*(*uint16)(address))
		case 4:
			bits = uint64(*(*uint32)(address))
		default:
			bits = *(*uint64)(address)
		}
		if part.float {
			floating(reg, x).SetUint64(bits)
			x++
			continue
		}
		general(reg, r).SetUint64(bits)
		r++
	}
	runtime.KeepAlive(memory)
}

// store returns a value of type t that is read from the registers. A value
// that was assigned to the stack is a program's to deal with, so it's zero here.
func (a assignment) store(out *RegistersArch, t reflect.Type) reflect.Value {
	memory := reflect.New(t)
	base := memory.UnsafePointer()
	r, x := a.r, a.x
	for _, part := range a.parts {
		address := unsafe.Add(base, part.offset)
		if part.pointer { // as a pointer, because the garbage collector must be told about the write.
			*(*unsafe.Pointer)(address) = general(out, r).UnsafePointer()
			r++
			continue
		}
		var bits uint64
		switch {
		case part.float:
			bits = floating(out, x).Uint64()
			x++
		default:
			bits = general(out, r).Uint64()
			r++
		}
		switch part.size {
		case 1:
			*(*uint8)(address) = uint8(bits)
		case 2:
			*(*uint16)(address) = uint16(bits)
		case 4:
			*(*uint32)(address) = uint32(bits)
		default:
			*(*uint64)(address) = bits
		}
	}
	return memory.Elem()
}

package arm64

import (
	"math"
	"reflect"
	"runtime"
	"strconv"
	"unsafe"
)

// Operation is either positive for a load from the named location
// to the main register, or negative to write the value in the main
// register to the named location. Writes should not be treated as
// observable.
type Operation uint8

// Locations names.
const (
	// general purpose registers
	Noop Operation = iota

	R0
	R1
	R2
	R3
	R4
	R5
	R6
	R7

	// floating point registers
	X0
	X1
	X2
	X3
	X4
	X5
	X6
	X7

	// Jump to the function provided by the current context.
	Jump

	Ret // return value from call

	// stack (is consumed by each access).
	// reads read from the caller's stack.
	// writes write to the callee's stack.
	Stack8
	Stack16
	Stack32
	Stack64
	Reset // reset the stack back to its original value.

	Write // write versions of all of the above.
)

// String implements [fmt.Stringer] and returns the name of the instruction.
func (op Operation) String() (s string) {
	if op > Write {
		defer func() {
			s = "Write" + s
		}()
		op -= Write
	}
	switch op {
	case Noop:
		return "Noop"
	case R0, R1, R2, R3, R4, R5, R6, R7:
		return "R" + strconv.Itoa(int(op-R0))
	case X0, X1, X2, X3, X4, X5, X6, X7:
		return "X" + strconv.Itoa(int(op-X0))
	case Jump:
		return "Call"
	case Stack8, Stack16, Stack32, Stack64:
		return "Stack" + strconv.Itoa(int(op-Stack8)*8)
	case Reset:
		return "Reset"
	default:
		return "INVALID"
	}
}

// Registers provides access to the raw internal Go calling
// convention registers on arm64.
type Registers struct {
	R0, R1, R2, R3, R4, R5, R6, R7 uint64
	X0, X1, X2, X3, X4, X5, X6, X7 float64
}

// Function with raw access to argument and result arm64
// registers.
type Function func(Registers) Registers

// MakeFunc returns a function that calls a function with
// direct access to the arm64 registers.
func MakeFunc(rtype reflect.Type, fn Function) reflect.Value {
	if runtime.GOARCH != "arm64" {
		panic("arm64.MakeFunc called on non-arm64 architecture")
	}
	return reflect.NewAt(rtype, reflect.ValueOf(&fn).UnsafePointer()).Elem()
}

func Prepare()
func Restore()

func Call(rtype reflect.Type, call unsafe.Pointer, src []Operation) reflect.Value {
	return MakeFunc(rtype, func(reg Registers) Registers {
	loop:
		out := reg
		var g float64      // runtime.g
		var normal float64 // main register
		for i, op := range src {
			switch op {
			case R0, R1, R2, R3, R4, R5, R6, R7:
				normal = *(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(&reg.R0)) + uintptr(op)*8))
			case X0, X1, X2, X3, X4, X5, X6, X7:
				normal = *(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(&reg.X0)) + uintptr(op-X0)*8))
			case Jump:
				src = src[i+1:]
				goto call
			case Ret:
			case Stack8, Stack16, Stack32, Stack64:
				panic("stack access not implemented")
			case R0 + Write, R1 + Write, R2 + Write, R3 + Write, R4 + Write, R5 + Write, R6 + Write, R7 + Write:
				*(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(&out.R0)) + uintptr(op-Write)*8)) = normal
			case X0 + Write, X1 + Write, X2 + Write, X3 + Write, X4 + Write, X5 + Write, X6 + Write, X7 + Write:
				*(*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(&out.X0)) + uintptr(op-X0-Write)*8)) = normal
			}
		}
		return reg
	call:
		prepare := Prepare
		restore := Restore
		closure := &call
		r0 := reg.R0
		g = math.Float64frombits((*(*func() uint64)(unsafe.Pointer(&prepare)))()) // save runtime.g, and grow stack.
		reg.R0 = r0
		reg.R0, reg.X0 = (*(*func(Registers) (uint64, float64))(unsafe.Pointer(&closure)))(out)
		r0 = reg.R0
		reg.R0 = math.Float64bits(g)
		(*(*func(Registers))(unsafe.Pointer(&restore)))(reg) // restore runtime.g
		reg.R0 = r0
		goto loop
	})
}

type MemoryLayout int

const (
	StandardLayout MemoryLayout = iota
)

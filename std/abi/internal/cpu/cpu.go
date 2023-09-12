package cpu

import (
	"reflect"
	"runtime"
	"strconv"
	"unsafe"

	"runtime.link/std/abi/internal/cgo"
)

/*
	Helpful links:

	https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
*/

// Instruction is either positive for a load from the named location
// to the main register, or negative to write the value in the main
// register to the named location. Writes should not be treated as
// observable.
type Instruction uint8

// Locations names.
const (
	// general purpose registers
	Noop Instruction = iota

	R0
	R1
	R2
	R3
	R4
	R5
	R6
	R7
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15

	// floating point registers
	X0
	X1
	X2
	X3
	X4
	X5
	X6
	X7
	X8
	X9
	X10
	X11
	X12
	X13
	X14
	X15

	// scratch registers
	S0
	S1
	S2
	S3

	Set0 // set S0 to 0
	Set1 // set S0 to 1

	Swap1 // swap S0 and S1
	Swap2 // swap S0 and S2
	Swap3 // swap S0 and S3

	// Jump to the function provided by the current context.
	Jump

	Ret // return value from call

	// heap writes
	Heap // write to reset, read to $main
	Heap8
	Heap16
	Heap32
	Heap64

	// stack (is consumed by each access).
	// reads read from the caller's stack.
	// writes write to the callee's stack.
	Stack8
	Stack16
	Stack32
	Stack64
	Reset // reset the stack back to its original value.

	MemoryCopy
	MemoryNull

	AssertLess
	AssertMore

	Error
	ErrorValue

	WriteR3
	WriteR8
	WriteR9

	WriteX3
	WriteX8

	Write // write versions of all of the above.

)

// String implements [fmt.Stringer] and returns the name of the instruction.
func (op Instruction) String() (s string) {
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
	case S0, S1, S2, S3:
		return "S" + strconv.Itoa(int(op-S0))
	case Set0:
		return "Set0"
	case Set1:
		return "Set1"
	case Heap:
		return "Heap"
	case Heap8:
		return "Heap8"
	case Heap16:
		return "Heap16"
	case Heap32:
		return "Heap32"
	case Heap64:
		return "Heap64"
	case Swap1, Swap2, Swap3:
		return "Swap" + strconv.Itoa(int(op-Swap1)+1)
	case MemoryCopy:
		return "MemoryCopy"
	case MemoryNull:
		return "MemoryNull"
	case AssertLess:
		return "AssertLess"
	case AssertMore:
		return "AssertMore"
	case Error:
		return "Error"
	case ErrorValue:
		return "ErrorValue"
	case Jump:
		return "Call"
	case Stack8, Stack16, Stack32, Stack64:
		return "Stack" + strconv.Itoa(int(op-Stack8)*8)
	case Reset:
		return "Reset"
	case WriteR3:
		return "LoadR3"
	case WriteR8:
		return "LoadR8"
	case WriteX3:
		return "LoadX3"
	case WriteX8:
		return "LoadX8"
	default:
		return "INVALID"
	}
}

// function has to reserve enough registers so that any calls
// made before the ABI call are not overwritten. Additional
// registers can be fetched on demand.
type function func(r0, r1 Register, x0 FloatingPointRegister) (Register, Register, FloatingPointRegister)

// MakeFunc returns a function that calls a function with
// direct access to the arm64 registers.
func makeFunc(rtype reflect.Type, fn function) reflect.Value {
	return reflect.NewAt(rtype, reflect.ValueOf(&fn).UnsafePointer()).Elem()
}

func Prepare()
func Restore()
func PushFunc()
func CallFunc()

func Nothing() {}

var err error = new(cgo.Error)

var (
	nothing  = Nothing
	prepare  = Prepare
	restore  = Restore
	pushfunc = PushFunc
	callfunc = CallFunc
)

func Call(rtype reflect.Type, call unsafe.Pointer, src []Instruction) reflect.Value {
	closure := &call
	return makeFunc(rtype, func(r0, r1 Register, x0 FloatingPointRegister) (Register, Register, FloatingPointRegister) {
		//fmt.Println(src)
		var r2, r3, r4, r5, r6, r7 Register
		var x1, x2 FloatingPointRegister

		var r *[8]Register
		var x *[13]FloatingPointRegister

		var pc int
		var g FloatingPointRegister              // runtime.g
		var s0, s1, s2, s3 FloatingPointRegister // main registers
		var heap []byte
		var pins runtime.Pinner

		switch runtime.GOARCH {
		case "amd64":
			Prepare()
		case "arm64":
			g.SetUintptr((*(*func() uintptr)(unsafe.Pointer(&prepare)))()) // save runtime.g, and grow stack.
		}

		for ; pc < len(src); pc++ {
			switch src[pc] {
			case WriteX3:
				(*(*func(x0, x1, x2 FloatingPointRegister))(unsafe.Pointer(&nothing)))(x0, x1, x2)
			case WriteR3:
				(*(*func(r0, r1, r2 Register))(unsafe.Pointer(&nothing)))(r0, r1, r2)

			case WriteR8:
				(*(*func(r0, r1, r2, r3, r4, r5, r6, r7 Register))(unsafe.Pointer(&nothing)))(r0, r1, r2, r3, r4, r5, r6, r7)
			case WriteR9:
				(*(*func(r0, r1, r2, r3, r4, r5, r6, r7, r8 Register))(unsafe.Pointer(&nothing)))(r0, r1, r2, r3, r4, r5, r6, r7, r[0])
			case WriteX8:
				(*(*func(x0, x1, x2, x3, x4, x5, x6, x7 FloatingPointRegister))(unsafe.Pointer(&nothing)))(x0, x1, x2, x[0], x[1], x[2], x[3], x[4])

			/*case 100:
				r0, r1, r2, r3, r4, r5, r6, r7 = (*(*func() (r0, r1, r2, r3, r4, r5, r6, r7 Register))(unsafe.Pointer(&nothing)))()
			case 101:
				(*(*func(r0, r1, r2, r3, r4, r5, r6, r7, r8, r9, r10, r11, r12, r13, r14, r15 Register))(unsafe.Pointer(&nothing)))(r0, r1, r2, r3, r4, r5, r6, r7, r8, r9, r10, r11, r12, r13, r14, r15)
			case 102:
				x0, x1, x2, x3, x4, x5, x6, x7 = (*(*func() (x0, x1, x2, x3, x4, x5, x6, x7 FloatingPointRegister))(unsafe.Pointer(&nothing)))()
			case 103:
				(*(*func(x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15 FloatingPointRegister))(unsafe.Pointer(&nothing)))(x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15)*/
			case R0:
				s0.SetUint(r0.Uint())
			case R1:
				s0.SetUint(r1.Uint())
			case X0:
				s0 = x0
			case X1:
				_, x1 := (*(*func() (x0, x1 FloatingPointRegister))(unsafe.Pointer(&nothing)))()
				s0 = x1
			case S1:
				s0 = s1
			case S2:
				s0 = s2
			case S3:
				s0 = s3
			case Swap2:
				s0, s2 = s2, s0
			case Set0:
				s0 = 0
			case Set1:
				s0 = 1
			case Error:
				if s3.Uint64() != 0 {
					ptr := *(*unsafe.Pointer)(unsafe.Pointer(&err))
					s0.SetUnsafePointer(ptr)
				} else {
					s0 = 0
				}
			case ErrorValue:
				if err := s3.Uint64(); err != 0 {
					err := cgo.Error(err)
					ptr := &err
					pins.Pin(ptr)
					s0.SetUnsafePointer(unsafe.Pointer(ptr))
				} else {
					s0 = 0
				}
			case AssertMore:
				if !(s0.Uint64() > s2.Uint64()) {
					s3.SetUint64(1)
				}
			case AssertLess:
				if !(s0.Uint64() < s2.Uint64()) {
					s3.SetUint64(1)
				}
			case Heap:
				ptr := unsafe.Pointer(unsafe.SliceData(heap))
				s0.SetUnsafePointer(ptr)
				s1.SetInt(len(heap))
			case Jump:
				switch runtime.GOARCH {
				case "amd64":
					_, _, _ = (*(*func(Register, Register, FloatingPointRegister) (r0, r1 Register, x0 FloatingPointRegister))(unsafe.Pointer(&pushfunc)))(Register(uintptr(call)), r1, x0)
					r0, r1, x0 = (*(*func(Register, Register, FloatingPointRegister) (r0, r1 Register, x0 FloatingPointRegister))(unsafe.Pointer(&callfunc)))(Register(uintptr(call)), r1, x0)
				case "arm64":
					r0, r1, x0 = (*(*func(Register, Register, FloatingPointRegister) (Register, Register, FloatingPointRegister))(unsafe.Pointer(&closure)))(r0, r1, x0)
					(*(*func(g uintptr))(unsafe.Pointer(&restore)))(g.Uintptr())
				}

			case Stack8, Stack16, Stack32, Stack64:
				panic("stack access not implemented")
			case R0 + Write:
				r0 = Register(s0.Uint())
			case R1 + Write:
				r1 = Register(s0.Uint())
			case X0 + Write:
				x0 = s0
			case X1 + Write:
				(*(*func(x0, x1 FloatingPointRegister))(unsafe.Pointer(&nothing)))(x0, s0)
			case Write + S1:
				s1 = s0
			case Write + S2:
				s2 = s0
			case Write + S3:
				s3 = s0
			case Write + Heap:
				heap = nil
			case Write + Heap8:
				heap = append(heap, s0.Uint8())
			case Write + Heap16:
				i := s0.Uint16()
				heap = append(heap, byte(i), byte(i>>8))
			case Write + Heap32:
				i := s0.Uint32()
				heap = append(heap, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
			case Write + Heap64:
				i := s0.Uint64()
				heap = append(heap, byte(i), byte(i>>8), byte(i>>16), byte(i>>24), byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
			case MemoryCopy:
				len := s1.Int()
				ptr := (*byte)(s0.UnsafePointer())
				heap = append(heap, unsafe.Slice(ptr, uintptr(len))...)
				heap = append(heap, 0)
			case MemoryNull:
				ptr := (*byte)(s0.UnsafePointer())
				itr := s1.Int()
				s := unsafe.String(ptr, int(itr))
				if len(s) > 0 && s[len(s)-1] != 0 {
					s += "\x00"
					ptr = unsafe.StringData(s)
					pins.Pin(ptr)
					itr++
					s0.SetUnsafePointer(unsafe.Pointer(ptr))
					s1.SetInt(itr)
				}
			default:
				panic("unsupported operation " + src[pc].String())
			}
		}
		if pins != (runtime.Pinner{}) {
			pins.Unpin()
		}
		// set return registers.
		(*(*func(Register, Register, FloatingPointRegister) (Register, FloatingPointRegister))(unsafe.Pointer(&nothing)))(r0, r1, x0)
		return r0, r1, x0
	})
}

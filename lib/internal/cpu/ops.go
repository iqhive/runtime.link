package cpu

import "strconv"

// Instruction is a single instruction that can be executed by the
// cpu inside of a [MakeFunc] function.
type Instruction uint8

// Mode is the highest 3-bits of an instruction that defined how to
// interpret the remaining lower 5 bits of the instruction.
type Mode uint8

func (m Mode) New(args Args) Instruction {
	return Instruction(m<<5) | Instruction(args)
}

// Decode returns the mode and arguments of the instruction.
func (op Instruction) Decode() (Mode, Args) {
	return Mode(op >> 5), Args(op & 0b11111)
}

// Modes supported.
const (
	Func Mode = iota
	Arch      // architecture-specific registers and instructions.
	Load      // register N into the $normal register.
	Copy      // the $normal register to register N.
	Move      // the $normal register into write-only output register N.
	Math      // math operations.
	Bits      // load lowest five bits of $normal into register N.
	Data      // load static data slot N.
)

// Math operations, operate on $normal and $assert, result is
// stored in $result.
const (
	Flip Args = iota // invert $result as a boolean

	// assertions
	Less // write 1 to $result if $normal !< $assert
	More // write 1 to $result if $normal !> $assert
	Same // write 1 to $result if $normal != $assert
	Diff // write 1 to $result if $normal = $assert

	Add // add $normal and $assert
	Sub // subtract $assert from $normal
	Mul // multiply $normal and $assert
	Div // divide $normal by $assert
	Mod // modulo $normal by $assert

	Addi // signed add $normal and $assert
	Subi // signed subtract $assert from $normal
	Muli // signed multiply $normal and $assert
	Divi // signed divide $normal by $assert
	Modi // signed modulo $normal by $assert

	And // bitwise and $normal and $assert
	Or  // bitwise or $normal and $assert
	Xor // bitwise xor $normal and $assert
	Shl // shift $normal left by $assert
	Shr // shift $normal right by $assert

	Addf // add $normal and $assert as floating point
	Subf // subtract $assert from $normal as floating point
	Mulf // multiply $normal and $assert as floating point
	Divf // divide $normal by $assert as floating point

	Conv  // cast integer $normal to $result as a float
	Convf // cast float $normal to $result as an integer
)

// Args to a [Mode]
type Args uint8

// Func names.
const (
	Noop Args = iota

	Jump // to instruction $normal if $assert is not zero.
	Call // call the function loaded into the context.

	SwapLength
	SwapAssert
	SwapResult

	HeapMake
	HeapPush8
	HeapPush16
	HeapPush32
	HeapPush64
	HeapCopy
	HeapLoad

	StackCaller // switch to caller stack pointer
	StackCallee // switch to callee stack pointer
	Stack       // reset stack pointer
	Stack8
	Stack16
	Stack32
	Stack64

	ClosureMake

	PointerMake // make $normal pointer a [Pointer].
	PointerFree
	PointerKeep
	PointerLoad

	ErrorMake // make $result into an error in $normal and $assert.

	StringSize // null-terminated string size of $normal
	StringCopy // copy $normal as a null-terminated string and store in $normal.
	StringMake // copy $normal + $length as a null-terminated string if needed.

	AssertArgs
)

// Registers
const (
	R0 Args = iota
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
)

// String implements [fmt.Stringer] and returns a readable representation
// of the instruction.
func (op Instruction) String() (s string) {
	mode, data := op.Decode()
	reg := func() string {
		if data > R15 {
			return "X" + strconv.Itoa(int(data-X0))
		} else {
			return "R" + strconv.Itoa(int(data-R0))
		}
	}
	switch mode {
	case Load:
		return "LOAD(" + reg() + ")"
	case Move:
		return "MOVE(" + reg() + ")"
	case Bits:
		return "BITS(" + strconv.Itoa(int(data)) + ")"
	case Data:
		return "DATA(" + strconv.Itoa(int(data)) + ")"
	case Copy:
		return "COPY(" + reg() + ")"
	case Math:
		switch data {
		case Flip:
			return "FLIP"
		case Less:
			return "LESS"
		case More:
			return "MORE"
		case Same:
			return "SAME"
		case Add:
			return "ADD"
		case Sub:
			return "SUB"
		case Mul:
			return "MUL"
		case Div:
			return "DIV"
		case Mod:
			return "MOD"
		case And:
			return "AND"
		case Or:
			return "OR"
		case Xor:
			return "XOR"
		case Shl:
			return "SHL"
		case Shr:
			return "SHR"
		case Addf:
			return "ADDF"
		case Subf:
			return "SUBF"
		case Mulf:
			return "MULF"
		case Divf:
			return "DIVF"
		default:
			return "MATH(" + strconv.Itoa(int(data)) + ")"
		}
	case Func:
		switch data {
		case Noop:
			return "NOOP"
		case Jump:
			return "JUMP"
		case Call:
			return "CALL"
		case SwapLength:
			return "SWAP(LENGTH)"
		case SwapAssert:
			return "SWAP(ASSERT)"
		case SwapResult:
			return "SWAP(RESULT)"
		case HeapMake:
			return "HEAP(MAKE)"
		case HeapPush8:
			return "HEAP(PUSH8)"
		case HeapPush16:
			return "HEAP(PUSH16)"
		case HeapPush32:
			return "HEAP(PUSH32)"
		case HeapPush64:
			return "HEAP(PUSH64)"
		case HeapCopy:
			return "HEAP(COPY)"
		case HeapLoad:
			return "HEAP(LOAD)"
		case StackCaller:
			return "STACK(CALLER)"
		case StackCallee:
			return "STACK(CALLEE)"
		case Stack:
			return "STACK"
		case Stack8:
			return "STACK8"
		case Stack16:
			return "STACK16"
		case Stack32:
			return "STACK32"
		case Stack64:
			return "STACK64"
		case ClosureMake:
			return "CLOSURE(MAKE)"
		case PointerMake:
			return "POINTER(MAKE)"
		case PointerFree:
			return "POINTER(FREE)"
		case PointerKeep:
			return "POINTER(KEEP)"
		case PointerLoad:
			return "POINTER(LOAD)"
		case StringSize:
			return "STRING(SIZE)"
		case StringMake:
			return "STRING(MAKE)"
		case StringCopy:
			return "STRING(COPY)"
		case ErrorMake:
			return "ERROR(MAKE)"
		case AssertArgs:
			return "ASSERT(ARGS)"
		default:
			return "FUNC(" + strconv.Itoa(int(data)) + ")"
		}
	default:
		return "NOOP(" + strconv.Itoa(int(data)) + ")"
	}
}

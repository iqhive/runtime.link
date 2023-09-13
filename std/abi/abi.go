/*
Package abi provides an interface to the platform-standard ABI calling
conventions and type system (typically C).

Helpful links:
https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
https://dyncall.org/docs/manual/manualse11.html
*/
package abi

import (
	"reflect"

	"runtime.link/std/abi/internal/cgo"
)

// CallOperation used to transform arguments from Go to the platform-native
// calling convention. The instructions operate on a 64-instruction virtual
// machine with the following five registers:
//
//   - $normal 64bit register: the register used for most operations.
//   - $length 64bit register: the register used to store length.
//   - $assert 64bit register: the register used to make assertions.
//   - $result 64bit register: the register set by assertions.
//
// The following counters are also available:
//
//   - program counter (PC): the current instruction.
//   - caller counter: the Nth current caller value.
//   - callee counter: the Nth current callee value.
//   - loop counter: the Nth current loop iteration.
//
// If the $result register is set to a non-zero value before the function is
// called, the function panics.
type Operation byte

func (op Operation) IsCopy() bool {
	switch op {
	case CopyValByt, CopyValU16, CopyValU32, CopyValU64,
		CopyValF32, CopyValF64, CopyValPtr, CopyValLen, CopyValVar:
		return true
	default:
		return false
	}
}

func (op Operation) IsMove() bool {
	switch op {
	case MoveValByt, MoveValU16, MoveValU32, MoveValU64,
		MoveValF32, MoveValF64, MoveValPtr, MoveValStr, MoveValMap,
		MoveValArr, MoveValAny, MoveValTim:
		return true
	default:
		return false
	}
}

const (
	Operations Operation = 64 - iota // no operation, used for padding.

	// special instructions.
	JumpToCall // calls the calling context's function, second time calls the error function, third panics. swaps meaning of SendArg* and RecvArg*. functions.

	// data
	LookupType // load the type of the current argument into the $assert register.
	LookupData // load the constant associated with the current argument into the $assert register.

	// general purpose.
	NormalSet0 // sets the $normal register to 0.
	NormalSet1 // sets the $normal register to 1.
	SwapLength // swap the $normal and $length registers.
	SwapAssert // swap the $normal and $assert registers.

	// Move Go values to registers, before [NormalCall] these refer
	// to the caller's arguments, after [NormalCall] they refer to
	// the return values to pass back to the caller.
	MoveValByt // move 8bit into $normal
	MoveValU16 // move 16bit into $normal
	MoveValU32 // move 32bit into $normal
	MoveValU64 // move 64bit into $normal
	MoveValF32 // move float32 into $normal
	MoveValF64 // move float64 into $normal
	MoveValPtr // move uintptr into $normal
	MoveValStr // move string into $normal + $length
	MoveValMap // move map into $normal + $length
	MoveValArr // move slice into $normal + $length, capacity is ignored.
	MoveValAny // move interface into $normal + $length (type)
	MoveValTim // move time.Time into $normal + $length, timezone is ignored.
	MoveValErr // move error into $normal as integer, or $assert into results.
	MoveNewVal // reset the caller's value counter back to zero.

	CopyValByt // copy $normal as byte
	CopyValU16 // copy $normal as uint16
	CopyValU32 // copy $normal as uint32
	CopyValU64 // copy $normal as uint64
	CopyValF32 // copy $normal as float32
	CopyValF64 // copy $normal as float64
	CopyValPtr // copy $normal as uintptr
	CopyValStr // copy $normal + $length as null-terminated string
	CopyValArr // copy $normal + $length as array pointer.
	CopyValLen // copy $length as uintptr
	CopyValVar // copy $normal + $length as varargs
	CopyNewVal // reset the callee's value counter back to zero.

	MoveStruct // subsequent move operations are apply to struct members.
	CopyStruct // subsequent copy operations are aggreated as a struct.
	DoneStruct // terminates [MoveStruct] and [CopyStruct] ands returns move and copy to normal.

	// iterators
	ForEachMap // push PC and repeat $normal, moves relate to the current map key and value, as if they were a struct.
	ForEachLen // push the program counter onto the stack and repeats the next instructions in [ForEachEnd] $length times.
	ForEachIdx // writes the current loop counter into the $normal register.
	ForEachEnd // pop the $length register from the stack and returns to normal execution.

	// push the $normal register onto the stack and then dereferences it as $normal
	// if the $normal register is zero, the stack is not pushed and execution is
	// advanced to the next matching [DoneOffset] instruction. cylical references
	// are resolved. subsequent SendArg* instructions will add perform the copy
	PushOffset
	PushPtrPtr // like [PushOffset] but for [Pointer] types.
	DoneOffset // pop the $normal register from the stack. Move ahead ptrsize bytes.

	// strings
	SizeString // set $length to the length of the null-terminated [String] pointed to by $normal.
	NullString // asserts that the $normal + $length string has a null terminator, copying it as neccasary.

	// allocation.
	MakeLength // subsequent copy instructions will add their sizes to the $assert register.
	DoneLength // pointer to empty memory of size $assert is written into the $length register, move and copy return to default behaviour.
	MakeMemory // subsequent copy instructions will write to dynamic heap memory.
	DoneMemory // pointer to heap memory from [MakeMemory] into the $normal register, , move and copy return to default behaviour.

	// garbage collection.
	FreeMemory // release ownership of the current argument.
	KeepMemory // maintain ownership of the current argument (keepalive)..

	// pointer instructions.
	MakePtrPtr // make the pointer inside the $normal register, a [Pointer] (or [Uintptr]).
	LoadPtrPtr // load the pointer inside the $normal register, a [Pointer] (or [Uintptr]) into $normal.

	// printf support.
	MakeFormat // load $normal + $length as a C string into the assert register.
	NewClosure // convert the $normal register into an ABI-compatible function pointer of type $assert.

	// time instructions.
	NanoTiming // swap the $normal nano time with the $format register.
	UnixTiming // swap the $normal unix milli with into the $format register.

	// assertions.
	AssertFlip // flips inverts the $failed register between 1 and 0.
	AssertLess // write sets the $failed register if the $normal register is not less than the $assert register.
	AssertSame // write sets the $failed register if the $normal register is not equal to the $assert register.
	AssertMore // write sets the $failed register if the $normal register is not greater than the $assert register.

	// write 1 into the $assert register if the $normal + $length register does
	// not have enough capacity to store the result of the C printf format string
	// specified by the $format register. Also checks if the types are correct.
	AssertArgs
)

// String implements [fmt.Stringer] and returns the name of the instruction.
func (op Operation) String() string {
	switch op {
	case Operations:
		return "Noop"
	case JumpToCall:
		return "JumpToCall"
	case LookupType:
		return "LookupType"
	case LookupData:
		return "LookupData"
	case FreeMemory:
		return "FreeMemory"
	case KeepMemory:
		return "KeepMemory"
	case NormalSet0:
		return "NormalSet0"
	case NormalSet1:
		return "NormalSet1"
	case SwapLength:
		return "SwapLength"
	case SwapAssert:
		return "SwapAssert"
	case MoveValByt:
		return "MoveValByt"
	case MoveValU16:
		return "MoveValU16"
	case MoveValU32:
		return "MoveValU32"
	case MoveValU64:
		return "MoveValU64"
	case MoveValF32:
		return "MoveValF32"
	case MoveValF64:
		return "MoveValF64"
	case MoveValPtr:
		return "MoveValPtr"
	case MoveValStr:
		return "MoveValStr"
	case MoveValMap:
		return "MoveValMap"
	case MoveValArr:
		return "MoveValArr"
	case MoveValAny:
		return "MoveValAny"
	case MoveValTim:
		return "MoveValTim"
	case MoveNewVal:
		return "MoveNewVal"
	case CopyValByt:
		return "CopyValByt"
	case CopyValU16:
		return "CopyValU16"
	case CopyValU32:
		return "CopyValU32"
	case CopyValU64:
		return "CopyValU64"
	case CopyValF32:
		return "CopyValF32"
	case CopyValF64:
		return "CopyValF64"
	case CopyValPtr:
		return "CopyValPtr"
	case CopyValLen:
		return "CopyValLen"
	case CopyValStr:
		return "CopyValStr"
	case CopyValArr:
		return "CopyValArr"
	case CopyValVar:
		return "CopyValVar"
	case CopyNewVal:
		return "CopyNewVal"
	case MoveStruct:
		return "MoveStruct"
	case CopyStruct:
		return "CopyStruct"
	case DoneStruct:
		return "DoneStruct"
	case ForEachMap:
		return "ForEachMap"
	case ForEachLen:
		return "ForEachLen"
	case ForEachIdx:
		return "ForEachIdx"
	case ForEachEnd:
		return "ForEachEnd"
	case PushOffset:
		return "PushOffset"
	case PushPtrPtr:
		return "PushPtrPtr"
	case DoneOffset:
		return "DoneOffset"
	case SizeString:
		return "SizeString"
	case NullString:
		return "NullString"
	case MakeLength:
		return "MakeLength"
	case DoneLength:
		return "DoneLength"
	case MakeMemory:
		return "MakeMemory"
	case DoneMemory:
		return "DoneMemory"
	case MakePtrPtr:
		return "MakePtrPtr"
	case LoadPtrPtr:
		return "LoadPtrPtr"
	case MakeFormat:
		return "MakeFormat"
	case NewClosure:
		return "NewClosure"
	case NanoTiming:
		return "NanoTiming"
	case UnixTiming:
		return "UnixTiming"
	case AssertFlip:
		return "AssertFlip"
	case AssertLess:
		return "AssertLess"
	case AssertSame:
		return "AssertSame"
	case AssertMore:
		return "AssertMore"
	case AssertArgs:
		return "AssertArgs"
	case MoveValErr:
		return "MoveValErr"
	case 0, 1:
		return "RESERVED"
	default:
		return "INVALID"
	}
}

// Sizeof returns the size of the given ABI type in bytes.
func Sizeof(name string) uintptr {
	return cgo.Sizeof(name)
}

// Const returns the value of the given standard ABI constant.
func Const(name string) string {
	return cgo.Const(name)
}

// Kind returns the kind of the given standard ABI type.
func Kind(name string) reflect.Kind {
	return cgo.Kind(name)
}

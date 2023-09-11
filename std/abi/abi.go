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
// calling convention. The instructions operate on a virtual machine with
// the following five registers:
//
//   - $normal 64bit register: the register used for most operations.
//   - $backup 64bit register: free-form register used for temporary storage.
//   - $length 64bit register: the register used to store length.
//   - $format 128bit register: the register used for printf format strings.
//   - $assert 64bit register: the register used to make assertions.
//   - $failed 64bit register: the register set by assertions when they fail.
//
// The following peuseo-pointers are also available:
//
//   - caller argument pointer: the pointer to the recv argument.
//   - callee argument pointer: the pointer to the send argument.
//
// If the failed register is set before the function is called, the function
// panics.
type Operation byte

const (
	Noop Operation = iota // no operation, used for padding.

	// special instructions.
	Call // calls the calling context's function, second time calls the error function, third panics.
	Type // load the type of the current argument into the $assert register.
	Copy // creates a copy buffer, all future RecvArgs* and SendArg* instructions will copy into buffer until the next [Done].
	Done // copy complete.
	Load // load the constant associated with the current argument into the $assert register.
	Free // release ownership of the current argument.
	Keep // maintain ownership of the current argument (keepalive).
	Recv // reset the argument pointer back to the first argument.

	// Recv instructions load arguments passed to the Go function being
	// called in left to right order and sets the current argument.
	RecvArgByt
	RecvArgU16
	RecvArgU32
	RecvArgU64
	RecvArgF32
	RecvArgF64
	RecvArgPtr
	RecvArgStr // string $normal + $length
	RecvArgArr // slice $normal + $length, capacity is ignored.
	RecvArgAny // interface $normal + $length (type)
	RecvArgTim // time.Time $format, timezone is ignored.
	RecvArgNew // resets the argument index back to zero.

	// Send arguments to the function about to be called,
	// arguments are sent in left to right order.
	SendArgByt
	SendArgU16
	SendArgU32
	SendArgU64
	SendArgF32
	SendArgF64
	SendArgPtr
	SendArgVar // varargs $normal + $length

	// struct boundary markers TODO
	RecvStruct
	SendStruct
	DoneStruct // needs to appear after SendStruct or RecvStruct

	// push the $normal register onto the stack and then dereferences it as $normal
	// if the $normal register is zero, the stack is not pushed and execution is
	// advanced to the next matching [DoneOffset] instruction. cylical references
	// are resolved. subsequent SendArg* instructions will add perform the copy
	PushOffset
	PushPtrPtr // like [PushOffset] but for [Pointer] types.
	DoneOffset // pop the $normal register from the stack. Move ahead ptrsize bytes.

	SizeString // set $length to the length of the null-terminated [String] pointed to by $normal.
	CopyMemory // copy $length bytes pointed to by the $normal register into dst as a pointer.

	// memory allocation.
	SendLength // subsequent SendArg* instructions will add sizes to the $length register.
	MakeMemory // set $normal register to a pointer to allocated memory on the heap of $length size.
	MakePtrPtr // make the pointer inside the $normal register, a [Pointer] (or [Uintptr]).
	LoadPtrPtr // load the pointer inside the $normal register, a [Pointer] (or [Uintptr]).

	// printf support.
	MakeFormat // load into $format string from the $normal + $length registers.

	// time instructions.
	NanoTiming // swap the $normal nano time with the $format register.
	UnixTiming // swap the $normal unix milli with into the $format register.

	// moves
	SetNormal0 // sets the $normal register to 0.
	SetNormal1 // sets the $normal register to 1.
	SwapLength // swap the $normal and $length registers.
	SwapAssert // swap the $normal and $assert registers.
	SwapBackup // swap the $normal and $backup registers.
	CopyBackup // copy the $normal register into the $backup register.

	// assertions.
	AssertFlip // flips inverts the $failed register between 1 and 0.
	AssertLess // write sets the $failed register if the $normal register is not less than the $assert register.
	AssertSame // write sets the $failed register if the $normal register is not equal to the $assert register.
	AssertMore // write sets the $failed register if the $normal register is not greater than the $assert register.

	// write 1 into the $assert register if the $normal + $length register does
	// not have enough capacity to store the result of the C printf format string
	// specified by the $format register. Also checks if the types are correct.
	AssertArgs

	// receive the return value from the function being called.
	RecvRetU64
	RecvRetF64
	RecvRetPtr

	// send the return value back to the calling context.
	SendRetU8
	SendRetU16
	SendRetU32
	SendRetU64
	SendRetF32
	SendRetF64
	SendRetPtr
	SendRetAny // interface $normal + $length (type)
	SendRetStr // string $normal + $length
	SendRetArr // slice $normal + $length, capacity set to $length.
	SendRetErr // return the current value of the $failed register as an error.
	SendRetTim // time.Time $format, timezone is ignored.

	Operations // the number of operations.
)

// String implements [fmt.Stringer] and returns the name of the instruction.
func (op Operation) String() string {
	switch op {
	case Noop:
		return "Noop"
	case Call:
		return "Call"
	case Copy:
		return "Copy"
	case Type:
		return "Type"
	case Load:
		return "Load"
	case Free:
		return "Free"
	case Keep:
		return "Keep"
	case RecvArgByt:
		return "RecvArgByt"
	case RecvArgU16:
		return "RecvArgU16"
	case RecvArgU32:
		return "RecvArgU32"
	case RecvArgU64:
		return "RecvArgU64"
	case RecvArgF32:
		return "RecvArgF32"
	case RecvArgF64:
		return "RecvArgF64"
	case RecvArgStr:
		return "RecvArgStr"
	case RecvArgPtr:
		return "RecvArgPtr"
	case RecvArgTim:
		return "RecvArgTim"
	case RecvArgArr:
		return "RecvArgArr"
	case RecvArgAny:
		return "RecvArgAny"
	case RecvArgNew:
		return "RecvArg"
	case SendArgByt:
		return "SendArgByt"
	case SendArgU16:
		return "SendArgU16"
	case SendArgU32:
		return "SendArgU32"
	case SendArgU64:
		return "SendArgU64"
	case SendArgF32:
		return "SendArgF32"
	case SendArgF64:
		return "SendArgF64"
	case SendArgPtr:
		return "SendArgPtr"
	case RecvStruct:
		return "RecvStruct"
	case SendStruct:
		return "SendStruct"
	case DoneStruct:
		return "DoneStruct"
	case SendLength:
		return "SendLength"
	case MakeMemory:
		return "MakeMemory"
	case NanoTiming:
		return "NanoTiming"
	case UnixTiming:
		return "UnixTiming"
	case MakePtrPtr:
		return "MakePtrPtr"
	case SwapLength:
		return "SwapLength"
	case SwapAssert:
		return "SwapAssert"
	case MakeFormat:
		return "MakeFormat"
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
	case RecvRetU64:
		return "RecvRetU64"
	case RecvRetF64:
		return "RecvRetF64"
	case SendRetU8:
		return "SendRetU8"
	case SendRetU16:
		return "SendRetU16"
	case SendRetU32:
		return "SendRetU32"
	case SendRetU64:
		return "SendRetU64"
	case SendRetF32:
		return "SendRetF32"
	case SendRetF64:
		return "SendRetF64"
	case SendRetAny:
		return "SendRetAny"
	case SendRetStr:
		return "SendRetStr"
	case SendRetArr:
		return "SendRetArr"
	case SendRetErr:
		return "SendRetErr"
	case SetNormal0:
		return "SetNormal0"
	case SetNormal1:
		return "SetNormal1"
	case RecvRetPtr:
		return "RecvRetPtr"
	case SendRetPtr:
		return "SendRetPtr"
	case SendRetTim:
		return "SendRetTim"
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

//go:build cgo

package dyncall

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"
)

// place is where an argument is expected: in general register x, in floating
// point register d, or in size bytes of the stack at offset s.
type place struct {
	in   byte
	at   int
	size int
}

func x(n int) place { return place{in: 'x', at: n} }
func d(n int) place { return place{in: 'd', at: n} }

// s is an argument in a whole slot of the stack, and packed is one that only
// takes up its own size, which is how Apple packs the named arguments.
func s(offset int) place            { return place{in: 's', at: offset, size: 8} }
func packed(offset, size int) place { return place{in: 's', at: offset, size: size} }

// bits returns what the place holds when an argument is in it, and which of
// its bits the argument defines: an integer is widened to fill a register or
// a slot, where a float32 leaves the high half of its register as it was.
func bits(arg any, where place) (value, defined uint64) {
	defined = math.MaxUint64
	if where.in == 's' && where.size < 8 {
		defined = 1<<(8*where.size) - 1
	}
	switch arg := arg.(type) {
	case bool:
		if arg {
			return 1, defined
		}
		return 0, defined
	case int8:
		return uint64(int64(arg)) & defined, defined
	case uint8:
		return uint64(arg), defined
	case uint16:
		return uint64(arg), defined
	case int32:
		return uint64(int64(arg)) & defined, defined
	case int64:
		return uint64(arg), defined
	case float32:
		return uint64(math.Float32bits(arg)), math.MaxUint32
	case float64:
		return math.Float64bits(arg), defined
	case unsafe.Pointer:
		return uint64(uintptr(arg)), defined
	}
	panic("unsupported argument")
}

// read returns what the frame has in a place.
func read(t *testing.T, frame Frame, where place) uint64 {
	t.Helper()
	switch where.in {
	case 'x':
		return frame.X[where.at]
	case 'd':
		return frame.D[where.at]
	}
	if where.at+where.size > len(frame.Stack) {
		t.Fatalf("the stack is %d bytes, which ends before an argument at %d", len(frame.Stack), where.at)
	}
	var slot [8]byte
	copy(slot[:], frame.Stack[where.at:where.at+where.size])
	return binary.NativeEndian.Uint64(slot[:])
}

// laidOut pushes the arguments as a call does, and returns where they would be put.
// The function has variable arguments, after the first fixed, unless fixed is negative.
func laidOut(t *testing.T, apple bool, fixed int, args []any) Frame {
	t.Helper()
	vm := NewVM(4096)
	defer vm.Free()
	if fixed >= 0 {
		vm.Variadic(fixed)
	}
	for _, arg := range args {
		switch arg := arg.(type) {
		case bool:
			vm.PushBool(arg)
		case int8:
			vm.PushChar(arg)
		case uint8:
			vm.PushUnsignedChar(arg)
		case uint16:
			vm.PushUnsignedShort(arg)
		case int32:
			vm.PushSignedInt(arg)
		case int64:
			vm.PushSignedLongLong(arg)
		case float32:
			vm.PushFloat(arg)
		case float64:
			vm.PushDouble(arg)
		case unsafe.Pointer:
			vm.PushPointer(arg)
		default:
			t.Fatalf("unsupported argument %T", arg)
		}
	}
	frame, err := vm.LayoutARM64(apple)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

// Each argument has a value of its own, so that one found in a place can only
// have been put there as that argument.
const (
	count  = int32(3)
	long1  = int64(0x1111111111111111)
	long2  = int64(-0x2222222222222222)
	long3  = int64(0x3333333333333333)
	real1  = 1.5
	real2  = -2.25
	int1   = int32(-7)
	int2   = int32(0x44444444)
	char1  = int8(-3)
	single = float32(0.75)
	// with the top bit set, so that sign-extending them would show.
	uchar1  = uint8(200)
	uchar2  = uint8(100)
	ushort1 = uint16(40000)
	ushort2 = uint16(50000)
)

// pointee is somewhere for a pointer argument to point.
var pointee byte

func longs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = int64(0x0101010101010101 * (i + 1))
	}
	return args
}

func TestWhereArgumentsArePut(t *testing.T) {
	for _, call := range []struct {
		name  string
		fixed int // negative for a function without variable arguments.
		args  []any
		linux []place // linux/arm64, and every other platform that follows AAPCS64.
		apple []place // darwin/arm64.
	}{
		{
			name: "named integers and doubles", fixed: -1, args: []any{long1, real1, long2, real2},
			linux: []place{x(0), d(0), x(1), d(1)},
			apple: []place{x(0), d(0), x(1), d(1)},
		},
		{
			name: "variable integers", fixed: 1, args: []any{count, long1, long2, long3},
			linux: []place{x(0), x(1), x(2), x(3)},
			apple: []place{x(0), s(0), s(8), s(16)},
		},
		{
			name: "variable doubles", fixed: 1, args: []any{count, real1, real2},
			linux: []place{x(0), d(0), d(1)},
			apple: []place{x(0), s(0), s(8)},
		},
		{
			name: "variable integers and doubles", fixed: 1, args: []any{count, long1, real1, long2, real2},
			linux: []place{x(0), x(1), d(0), x(2), d(1)},
			apple: []place{x(0), s(0), s(8), s(16), s(24)},
		},
		{
			name: "variable arguments after named ones of both kinds", fixed: 3, args: []any{long1, real1, count, long2, real2},
			linux: []place{x(0), d(0), x(1), x(2), d(1)},
			apple: []place{x(0), d(0), x(1), s(0), s(8)},
		},
		{
			name: "every argument is a variable one", fixed: 0, args: []any{long1, real1},
			linux: []place{x(0), d(0)},
			apple: []place{s(0), s(8)},
		},
		{
			name: "a variadic function called with its named arguments alone", fixed: 2, args: []any{long1, real1},
			linux: []place{x(0), d(0)},
			apple: []place{x(0), d(0)},
		},
		{
			name: "variable integers narrower than a slot fill it", fixed: 1, args: []any{count, int1, char1, int2},
			linux: []place{x(0), x(1), x(2), x(3)},
			apple: []place{x(0), s(0), s(8), s(16)},
		},
		{
			name: "more variable arguments than registers", fixed: 1, args: append([]any{count}, longs(9)...),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8)},
			apple: []place{x(0), s(0), s(8), s(16), s(24), s(32), s(40), s(48), s(56), s(64)},
		},
		{
			name: "a variable argument after named ones on the stack", fixed: 9, args: append(longs(9), long1),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8)},
		},
		{
			// Apple packs the named arguments on the stack, and starts the variable ones on the next slot.
			name: "a variable argument after a packed named one", fixed: 10, args: append(longs(8), int1, char1, long1, int2),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8), s(16), s(24)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), packed(0, 4), packed(4, 1), s(8), s(16)},
		},
		{
			name: "named arguments on the stack without any variable ones", fixed: -1, args: append(longs(8), int1, int2, long1),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8), s(16)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), packed(0, 4), packed(4, 4), s(8)},
		},
		{
			name: "a named float keeps its size", fixed: 2, args: []any{single, count, real1},
			linux: []place{d(0), x(0), d(1)},
			apple: []place{d(0), x(0), s(0)},
		},
		{
			// as the callback given to curl_easy_setopt(handle, option, ...) is.
			name: "a pointer that is the first variable argument", fixed: 2, args: []any{long1, count, unsafe.Pointer(&pointee), long2},
			linux: []place{x(0), x(1), x(2), x(3)},
			apple: []place{x(0), x(1), s(0), s(8)},
		},
		{
			// the caller widens them, with zeroes as they are unsigned, and a bool is 0 or 1.
			name: "unsigned narrow integers and bools in registers", fixed: -1, args: []any{uchar1, ushort1, true, false},
			linux: []place{x(0), x(1), x(2), x(3)},
			apple: []place{x(0), x(1), x(2), x(3)},
		},
		{
			// Apple gives each of them its own size, as it does a signed char, and aligns the int after them.
			name: "unsigned chars on the stack", fixed: -1, args: append(longs(8), uchar1, uchar2, int2),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8), s(16)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), packed(0, 1), packed(1, 1), packed(4, 4)},
		},
		{
			name: "unsigned shorts on the stack", fixed: -1, args: append(longs(8), uchar1, ushort1, ushort2, char1),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8), s(16), s(24)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), packed(0, 1), packed(2, 2), packed(4, 2), packed(6, 1)},
		},
		{
			name: "bools on the stack", fixed: -1, args: append(longs(8), true, false, true, char1, ushort1),
			linux: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), s(0), s(8), s(16), s(24), s(32)},
			apple: []place{x(0), x(1), x(2), x(3), x(4), x(5), x(6), x(7), packed(0, 1), packed(1, 1), packed(2, 1), packed(3, 1), packed(4, 2)},
		},
		{
			name: "variable unsigned narrow integers and bools fill a slot", fixed: 1, args: []any{count, uchar1, ushort1, true},
			linux: []place{x(0), x(1), x(2), x(3)},
			apple: []place{x(0), s(0), s(8), s(16)},
		},
	} {
		for platform, want := range map[string][]place{"linux/arm64": call.linux, "darwin/arm64": call.apple} {
			if len(want) != len(call.args) {
				t.Fatalf("%s on %s: %d places for %d arguments", call.name, platform, len(want), len(call.args))
			}
			frame := laidOut(t, platform == "darwin/arm64", call.fixed, call.args)
			for i, where := range want {
				value, defined := bits(call.args[i], where)
				if got := read(t, frame, where) & defined; got != value {
					t.Errorf("%s on %s: argument %d is not in %c%d, which has %#x, want %#x", call.name, platform, i, where.in, where.at, got, value)
				}
			}
		}
	}
}

// The stack is what the variable arguments of Apple's convention take up, and
// the registers are all that the others need for as many.
func TestStackOnlyHoldsWhatIsPassedOnIt(t *testing.T) {
	args := []any{count, long1, long2, long3}
	if frame := laidOut(t, true, 1, args); len(frame.Stack) != 32 {
		t.Errorf("darwin/arm64: 3 variable arguments take %d bytes of stack, want 24 rounded up to 32", len(frame.Stack))
	}
	if frame := laidOut(t, false, 1, args); len(frame.Stack) != 0 {
		t.Errorf("linux/arm64: %d bytes of stack for arguments that fit in registers", len(frame.Stack))
	}
}

// A VM is used for one call after another, and only some are of variadic functions.
func TestResetForgetsTheVariableArguments(t *testing.T) {
	vm := NewVM(4096)
	defer vm.Free()
	vm.Variadic(0)
	vm.PushSignedLongLong(long1)
	variadic, err := vm.LayoutARM64(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(variadic.Stack) == 0 {
		t.Fatalf("a variable argument was not put on the stack")
	}
	vm.Reset()
	vm.PushSignedLongLong(long2)
	named, err := vm.LayoutARM64(true)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := bits(long2, x(0)); len(named.Stack) != 0 || named.X[0] != want {
		t.Errorf("after Reset, a named argument was put on the stack as a variable one is: x0 = %#x", named.X[0])
	}
}

// signature returns the types that a function called back by C reads the arguments as.
func signature(t *testing.T, args []any) []rune {
	t.Helper()
	sig := make([]rune, len(args))
	for i, arg := range args {
		switch arg.(type) {
		case bool:
			sig[i] = Bool
		case int8:
			sig[i] = Char
		case uint8:
			sig[i] = UnsignedChar
		case uint16:
			sig[i] = UnsignedShort
		case int32:
			sig[i] = Int
		case int64:
			sig[i] = LongLong
		case float32:
			sig[i] = Float
		case float64:
			sig[i] = Double
		default:
			t.Fatalf("unsupported argument %T", arg)
		}
	}
	return sig
}

// A Go function that C calls back on Apple's arm64 takes its arguments from
// where a C caller puts them, which TestWhereArgumentsArePut shows is where a
// call from here puts them too. So what is read back from the layout of a call
// has to be what was passed, wherever the arguments are packed on the stack.
func TestAppleCallbackReadsWhatACallerPassed(t *testing.T) {
	for name, args := range map[string][]any{
		"registers":                      {long1, real1, uchar1, single, true, char1, ushort1, int1},
		"a false bool before a true one": append(longs(8), false, true, char1, uchar1),
		"bools between wider arguments":  append(longs(8), int1, true, ushort1, false, long1, true, real1, false, single),
		"every narrow type":              append(longs(8), char1, uchar1, ushort1, uchar2, int2, ushort2, true, long2),
		"floats on the stack":            {real1, real2, real1, real2, real1, real2, real1, real2, single, true, real1, single, false, char1},
	} {
		frame := laidOut(t, true, -1, args)
		got, err := frame.ReadAppleARM64(signature(t, args))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for i, arg := range args {
			want, defined := bits(arg, x(0))
			if got[i]&defined != want {
				t.Errorf("%s: argument %d (%T %v) was read as %#x, want %#x, from the stack % x", name, i, arg, arg, got[i]&defined, want, frame.Stack)
			}
		}
	}
}

// The registers of a caller that isn't this package may hold more than the
// argument: a C bool is one byte, and nothing is promised about the rest.
func TestAppleCallbackReadsABoolAsOneByte(t *testing.T) {
	frame := Frame{Stack: []byte{0, 1, 0xff, 0xff, 0, 0xff, 0xff, 0xff}}
	for i := range frame.X {
		frame.X[i] = 0xffffffffffffff00 | uint64(i%2)
	}
	sig := make([]rune, 13)
	for i := range sig {
		sig[i] = Bool
	}
	sig[10], sig[11] = Char, Char
	got, err := frame.ReadAppleARM64(sig)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, math.MaxUint64, math.MaxUint64, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argument %d was read as %#x, want %#x", i, got[i], want[i])
		}
	}
	if _, err := frame.ReadAppleARM64(append(sig, LongLong)); err == nil {
		t.Errorf("an argument past the end of the stack: want an error")
	}
}

func TestLayoutRefusesMoreThanItCanRecord(t *testing.T) {
	if _, err := laidOutErr(true, 0, longs(100)); err == nil {
		t.Errorf("800 bytes of variable arguments: want an error")
	}
}

func laidOutErr(apple bool, fixed int, args []any) (Frame, error) {
	vm := NewVM(4096)
	defer vm.Free()
	vm.Variadic(fixed)
	for _, arg := range args {
		vm.PushSignedLongLong(arg.(int64))
	}
	return vm.LayoutARM64(apple)
}

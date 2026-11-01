package cpu

import (
	"errors"
	"math"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/iqhive/runtime.link/api/call/internal/dll"
)

func libc(t *testing.T, name string) unsafe.Pointer {
	t.Helper()
	table, err := dll.Open("libc.so.6")
	if err != nil {
		t.Skip("requires glibc: ", err)
	}
	ptr, err := dll.Sym(table, name)
	if err != nil {
		t.Fatal(err)
	}
	return ptr
}

func execute(t *testing.T, ops ...Instruction) RegistersArch {
	t.Helper()
	p := new(Program)
	p.Add(ops...)
	var r RegistersArch
	return p.callArch(r)
}

func TestJumpRunsItsTarget(t *testing.T) {
	out := execute(t,
		NewBits(1), NewFunc(SwapAssert),
		NewBits(6),
		NewFunc(Jump),
		NewBits(9),
		NewBits(10),
		NewBits(7),
		NewMove(R0),
	)
	if got := out.R0().Uint64(); got != 7 {
		t.Errorf("R0 = %d", got)
	}
}

func TestMathReadsAssert(t *testing.T) {
	for op, want := range map[MathFunc]uint64{Add: 13, Sub: 7, Mul: 30, Div: 3, Mod: 1, And: 2, Or: 11, Xor: 9, Shl: 80, Shr: 1, Addi: 13, Subi: 7, Muli: 30, Divi: 3, Modi: 1} {
		out := execute(t,
			NewBits(3), NewFunc(SwapAssert),
			NewBits(2), NewFunc(SwapLength),
			NewBits(10),
			NewMath(op),
			NewFunc(SwapResult),
			NewMove(R0),
		)
		if got := out.R0().Uint64(); got != want {
			t.Errorf("%s: got %d, want %d", NewMath(op), got, want)
		}
	}
}

func TestComparisonsAreOneWhenTheyHold(t *testing.T) {
	for op, want := range map[MathFunc]uint64{Less: 1, More: 0, Same: 0, Diff: 1, Lessi: 1, Morei: 0} {
		out := execute(t,
			NewBits(3), NewFunc(SwapAssert),
			NewBits(2),
			NewMath(op),
			NewFunc(SwapResult),
			NewMove(R0),
		)
		if got := out.R0().Uint64(); got != want {
			t.Errorf("%s: got %d, want %d", NewMath(op), got, want)
		}
	}
}

func TestSignedComparisonsSeeNegativeValues(t *testing.T) {
	var r RegistersArch
	r.R0().SetInt64(-1)
	p := new(Program)
	p.Add(NewBits(1), NewFunc(SwapAssert), NewLoad(R0), NewMath(Lessi), NewFunc(SwapResult), NewMove(R1))
	out := p.callArch(r)
	if !out.R1().Bool() {
		t.Errorf("-1 < 1 as signed integers")
	}
	q := new(Program)
	q.Add(NewBits(1), NewFunc(SwapAssert), NewLoad(R0), NewMath(Less), NewFunc(SwapResult), NewMove(R1))
	out = q.callArch(r)
	if out.R1().Bool() {
		t.Errorf("-1 isn't less than 1 as unsigned integers")
	}
}

func TestBitsStaysInItsMode(t *testing.T) {
	if mode, data := NewBits(32).Decode(); mode != Bits || data != 0 {
		t.Errorf("NewBits(32) decodes to mode %d data %d", mode, data)
	}
	if mode, data := NewBits(31).Decode(); mode != Bits || data != 31 {
		t.Errorf("NewBits(31) decodes to mode %d data %d", mode, data)
	}
}

func TestDataLoadsFromTheProgram(t *testing.T) {
	p := &Program{Data: []uint{0, math.MaxUint}}
	p.Add(NewBits(1), NewFunc(Data), NewMove(R0))
	var r RegistersArch
	out := p.callArch(r)
	if got := out.R0().Uint(); got != math.MaxUint {
		t.Errorf("got %d", got)
	}
}

func TestCasts(t *testing.T) {
	for _, c := range []struct {
		cast CastFunc
		in   uint64
		want uint64
	}{
		{SignExtend8, 0xff, math.MaxUint64},
		{SignExtend8, 0xffffff7f, 0x7f},
		{SignExtend16, 0xffff, math.MaxUint64},
		{SignExtend32, 0xffffffff, math.MaxUint64},
		{ZeroExtend8, math.MaxUint64, 0xff},
		{ZeroExtend16, math.MaxUint64, 0xffff},
		{ZeroExtend32, math.MaxUint64, 0xffffffff},
		{FloatToDouble, uint64(math.Float32bits(1.5)), math.Float64bits(1.5)},
		{DoubleToFloat, math.Float64bits(1.5), uint64(math.Float32bits(1.5))},
	} {
		var r RegistersArch
		r.R0().SetUint64(c.in)
		p := new(Program)
		p.Add(NewLoad(R0), NewCast(c.cast), NewMove(R0))
		out := p.callArch(r)
		if got := out.R0().Uint64(); got != c.want {
			t.Errorf("%s of %#x = %#x, want %#x", NewCast(c.cast), c.in, got, c.want)
		}
	}
}

func TestErrorMakeBuildsARealError(t *testing.T) {
	out := execute(t, NewBits(5), NewFunc(ErrorMake), NewMove(R0), NewFunc(SwapLength), NewMove(R1))
	words := [2]uintptr{out.R0().Uintptr(), out.R1().Uintptr()}
	err := *(*error)(unsafe.Pointer(&words))
	var target Error
	if !errors.As(err, &target) || target != 5 {
		t.Errorf("error = %v", err)
	}
}

func TestErrorMakeIsNilWhenTheAssertionHeld(t *testing.T) {
	out := execute(t, NewBits(1), NewFunc(SwapResult), NewBits(5), NewFunc(ErrorMake), NewMove(R0), NewFunc(SwapLength), NewMove(R1))
	if out.R0().Uintptr() != 0 || out.R1().Uintptr() != 0 {
		t.Errorf("got %#x and %#x", out.R0().Uintptr(), out.R1().Uintptr())
	}
}

func TestStringMakeTerminatesEmptyStrings(t *testing.T) {
	sized := func(s string) (r RegistersArch) {
		r.R0().SetUnsafePointer(unsafe.Pointer(unsafe.StringData(s)))
		r.R1().SetInt(len(s))
		return r
	}
	p := new(Program)
	p.Add(NewLoad(R1), NewFunc(SwapLength), NewLoad(R0), NewFunc(StringMake), NewMove(R2), NewFunc(StringSize), NewFunc(SwapLength), NewMove(R0))
	for _, s := range []string{"", "hello"[:0], "hello"[:2], "hi\x00"} {
		out := p.callArch(sized(s))
		if got, want := out.R0().Int(), len(strings.TrimRight(s, "\x00")); got != want {
			t.Errorf("%q: C sees %d bytes, want %d", s, got, want)
		}
		if out.R2().Uintptr() == 0 {
			t.Errorf("%q: C sees NULL", s)
		}
	}
}

func TestStringSizeOfNull(t *testing.T) {
	out := execute(t, NewFunc(StringSize), NewFunc(SwapLength), NewMove(R0))
	if got := out.R0().Int(); got != 0 {
		t.Errorf("got %d", got)
	}
}

func TestSmallRegisterWritesDoNotReadPastTheirArgument(t *testing.T) {
	var r Register
	r = ^Register(0)
	r.SetInt8(-1)
	if r != 0xff {
		t.Errorf("SetInt8: %#x", uint64(r))
	}
	r = ^Register(0)
	r.SetInt32(-1)
	if r != 0xffffffff {
		t.Errorf("SetInt32: %#x", uint64(r))
	}
	r = ^Register(0)
	r.SetFloat32(1.5)
	if uint64(r) != uint64(math.Float32bits(1.5)) || r.Float32() != 1.5 {
		t.Errorf("SetFloat32: %#x", uint64(r))
	}
	var f FloatingPointRegister
	f.SetInt8(-1)
	if f.Uint64() != 0xff {
		t.Errorf("SetInt8: %#x", f.Uint64())
	}
	f.SetInt32(-1)
	if f.Uint64() != 0xffffffff {
		t.Errorf("SetInt32: %#x", f.Uint64())
	}
	f.SetUint8(0xfe)
	if f.Uint64() != 0xfe {
		t.Errorf("SetUint8: %#x", f.Uint64())
	}
	f.SetUint16(0xfffe)
	if f.Uint64() != 0xfffe {
		t.Errorf("SetUint16: %#x", f.Uint64())
	}
	f.SetUint32(0xfffffffe)
	if f.Uint64() != 0xfffffffe {
		t.Errorf("SetUint32: %#x", f.Uint64())
	}
	f.SetFloat32(1.5)
	if f.Uint64() != uint64(math.Float32bits(1.5)) || f.Float32() != 1.5 {
		t.Errorf("SetFloat32: %#x", f.Uint64())
	}
}

func TestSlowInstructionsRunWithoutAFrameSize(t *testing.T) {
	p := &Program{Call: libc(t, "labs")}
	p.Add(NewLoad(R0), NewSlow(PushBytes8), NewSlow(CallBytes8), NewMove(R0))
	var r RegistersArch
	r.R0().SetInt64(-5)
	out := p.callArch(r)
	if got := out.R0().Int64(); got != 5 {
		t.Errorf("labs(-5) = %d", got)
	}
}

func TestProgramCanCallMoreThanOnce(t *testing.T) {
	p := &Program{Call: libc(t, "labs")}
	p.Add(NewLoad(R0), NewSlow(PushBytes8), NewSlow(CallBytes8), NewSlow(PushBytes8), NewSlow(CallBytes8), NewMove(R0))
	var r RegistersArch
	r.R0().SetInt64(-5)
	out := p.callArch(r)
	if got := out.R0().Int64(); got != 5 {
		t.Errorf("labs(labs(-5)) = %d", got)
	}
}

func TestBothInterpretersAgree(t *testing.T) {
	p := &Program{Size: 4096, Call: libc(t, "labs")}
	p.Add(NewLoad(R0), NewSlow(PushBytes8), NewSlow(CallBytes8), NewMove(R0))
	var arch RegistersArch
	arch.R0().SetInt64(-5)
	var fast RegistersFast
	fast.R0().SetInt64(-5)
	a, f := p.callArch(arch), p.callFast(fast)
	if a.R0().Int64() != 5 || f.R0().Int64() != 5 {
		t.Errorf("callArch = %d, callFast = %d", a.R0().Int64(), f.R0().Int64())
	}
}

func residentSize(t *testing.T) uint64 {
	t.Helper()
	statm, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		t.Skip("requires /proc: ", err)
	}
	size, err := strconv.ParseUint(strings.Fields(string(statm))[0], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return size * uint64(os.Getpagesize())
}

func TestProgramFreesItsVM(t *testing.T) {
	const frame, rounds = 1 << 20, 2048
	p := &Program{Size: frame, Call: libc(t, "labs")}
	p.Add(NewLoad(R0), NewSlow(PushBytes8), NewSlow(CallBytes8), NewMove(R0))
	var r RegistersArch
	before := residentSize(t)
	for range rounds {
		p.callArch(r)
	}
	if grown := residentSize(t) - before; grown > frame*rounds/2 {
		t.Errorf("address space grew by %d MiB", grown>>20)
	}
}

func TestStringCopySurvivesUntilTheResultIsVisible(t *testing.T) {
	p := new(Program)
	p.Add(NewLoad(R0), NewFunc(StringCopy), NewFunc(SwapLength), NewMove(R1), NewFunc(SwapLength), NewMove(R0))
	copied := p.MakeFunc(reflect.TypeFor[func(*byte) string]()).Interface().(func(*byte) string)
	source := append([]byte("hello world, this is a long enough string"), 0)
	for range 64 {
		runtime.GC()
		if got := copied(&source[0]); got != "hello world, this is a long enough string" {
			t.Fatalf("got %q", got)
		}
	}
}

//go:noinline
func smallCaller(fn func(int64) int64) int64 { return fn(-5) + 1 }

func TestMakeFuncDoesNotOverwriteItsCaller(t *testing.T) {
	p := &Program{Call: libc(t, "labs")}
	p.Add(NewLoad(R0), NewSlow(PushBytes8), NewSlow(CallBytes8), NewMove(R0))
	labs := p.MakeFunc(reflect.TypeFor[func(int64) int64]()).Interface().(func(int64) int64)
	for range 10 {
		if got := smallCaller(labs); got != 6 {
			t.Fatalf("got %d", got)
		}
	}
}

func TestMakeFuncPassesRegistersThrough(t *testing.T) {
	// nothing touches the registers, so what comes in has to be what goes out.
	fn := new(Program).MakeFunc(reflect.TypeFor[func(int32, float64, int32, float64) (int32, float64, int32, float64)]()).
		Interface().(func(int32, float64, int32, float64) (int32, float64, int32, float64))
	a, b, c, d := fn(1, 2, 3, 4)
	if a != 1 || b != 2 || c != 3 || d != 4 {
		t.Errorf("got %v %v %v %v", a, b, c, d)
	}
}

func TestMakeFuncAssignsRegistersLikeGo(t *testing.T) {
	type pair struct {
		A int8
		B float32
	}
	type wrapped struct{ V [1]string }
	fn := new(Program).MakeFunc(reflect.TypeFor[func(pair, wrapped, []int32, error, complex128) (pair, wrapped, []int32, error, complex128)]()).
		Interface().(func(pair, wrapped, []int32, error, complex128) (pair, wrapped, []int32, error, complex128))
	failure := errors.New("failure")
	a, b, c, d, e := fn(pair{-1, 1.5}, wrapped{[1]string{"hello"}}, []int32{1, 2, 3}, failure, 1+2i)
	if a != (pair{-1, 1.5}) || b.V[0] != "hello" || len(c) != 3 || c[2] != 3 || d != failure || e != 1+2i {
		t.Errorf("got %v %v %v %v %v", a, b, c, d, e)
	}
}

func TestMakeFuncLeavesStackValuesOutOfRegisters(t *testing.T) {
	p := new(Program)
	p.Add(NewLoad(R1), NewMove(R0))
	fn := p.MakeFunc(reflect.TypeFor[func([2]int64, int64) int64]()).Interface().(func([2]int64, int64) int64)
	if got := fn([2]int64{1, 2}, 3); got != 0 {
		t.Errorf("R1 holds %d, the array belongs on the stack and the int in R0", got)
	}
	q := new(Program)
	q.Add(NewLoad(R0), NewMove(R0))
	gn := q.MakeFunc(reflect.TypeFor[func([2]int64, int64) int64]()).Interface().(func([2]int64, int64) int64)
	if got := gn([2]int64{1, 2}, 3); got != 3 {
		t.Errorf("R0 holds %d", got)
	}
}

func TestMakeFuncPinsWhatItIsTold(t *testing.T) {
	p := new(Program)
	p.Pin(R2)
	fn := p.MakeFunc(reflect.TypeFor[func(a, b, c *byte)]()).Interface().(func(a, b, c *byte))
	var x byte
	fn(&x, &x, &x)
}

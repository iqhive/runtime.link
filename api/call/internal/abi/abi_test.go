package abi_test

import (
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
)

func TestVariant(t *testing.T) {
	var register = abi.HardwareLocations.Register.As(0)
	var location = abi.Locations.Hardware.As(register)
	if location.String() != "0" {
		t.Errorf("location.String() == %q", location.String())
	}
}

func TestOperationToString(t *testing.T) {

	for op := abi.Operation(0); op < abi.Operations; op++ {
		if op.String() == "INVALID" {
			t.Errorf("Operation(%d).String() == 'INVALID'", op)
		}
	}
}

type withArray struct {
	A [2]float32
	B int32
}

func TestValueOfKeepsTheLayoutOfArrays(t *testing.T) {
	for _, rtype := range []reflect.Type{
		reflect.TypeFor[[1]int32](),
		reflect.TypeFor[[2]int32](),
		reflect.TypeFor[[2]float32](),
		reflect.TypeFor[withArray](),
		reflect.TypeFor[struct{ V [4]uint64 }](),
		reflect.TypeFor[[16]byte](),
		reflect.TypeFor[[2]struct {
			A int8
			B int32
		}](),
	} {
		value := abi.ValueOf(rtype)
		if value.Size() != rtype.Size() || value.Align() != uintptr(rtype.Align()) {
			t.Errorf("%s: size %d align %d, want size %d align %d", rtype, value.Size(), value.Align(), rtype.Size(), rtype.Align())
		}
	}
}

func TestFunctionOfAcceptsArrays(t *testing.T) {
	fn := abi.FunctionOf(reflect.TypeFor[func([2]int32, [1]float64) [3]byte]())
	if len(fn.Args) != 2 || len(fn.Rets) != 1 {
		t.Errorf("got %+v", fn)
	}
}

func TestInternalOnlyRegisterAssignsArraysOfOne(t *testing.T) {
	cc, err := abi.Internal(abi.FunctionOf(reflect.TypeFor[func([2]int32, [1]int32, [0]int32, struct{ A, B int32 })]()))
	if err != nil {
		t.Fatal(err)
	}
	stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(0))
	if !cc.Args[0].Equals(stack) {
		t.Errorf("[2]int32: %v, go puts it on the stack", cc.Args[0])
	}
	first := abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(cpu.Location(0)))
	if !cc.Args[1].Equals(abi.Locations.Multiple.As([]abi.Location{first})) {
		t.Errorf("[1]int32: %v, go puts it in the first register", cc.Args[1])
	}
	if len(abi.Locations.Multiple.Get(cc.Args[3])) != 2 {
		t.Errorf("struct of two ints: %v", cc.Args[3])
	}
}

func TestSizeIncludesPadding(t *testing.T) {
	for _, rtype := range []reflect.Type{
		reflect.TypeFor[struct {
			A int8
			B int64
		}](),
		reflect.TypeFor[struct {
			A int8
			B int64
			C int8
		}](),
		reflect.TypeFor[struct {
			A int8
			B int16
			C int8
		}](),
	} {
		if got := abi.ValueOf(rtype).Size(); got != rtype.Size() {
			t.Errorf("%s: size %d, want %d", rtype, got, rtype.Size())
		}
	}
}

func TestScalarsAreFlattenedWithTheirOffsets(t *testing.T) {
	type inner struct {
		A int8
		B float32
	}
	got := abi.ValueOf(reflect.TypeFor[struct {
		I inner
		C [2]int16
		_ [0]int64
	}]()).Scalars()
	want := []abi.Scalar{
		{Offset: 0, Value: abi.Values.Bytes1},
		{Offset: 4, Value: abi.Values.Float4},
		{Offset: 8, Value: abi.Values.Bytes2},
		{Offset: 10, Value: abi.Values.Bytes2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestInternalAlignsStackArguments(t *testing.T) {
	args := make([]abi.Value, 9, 11)
	for i := range args {
		args[i] = abi.Values.Bytes8
	}
	cc, err := abi.Internal(abi.Function{Args: append(args, abi.Values.Bytes1, abi.Values.Bytes8)})
	if err != nil {
		t.Fatal(err)
	}
	if got := abi.HardwareLocations.StackRtl.Get(abi.Locations.Hardware.Get(cc.Args[10])); got != 8 {
		t.Errorf("int64 after a byte is at %d", got)
	}
}

func TestLocationEquals(t *testing.T) {
	reg := abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(cpu.Location(1)))
	other := abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(cpu.Location(2)))
	one := abi.Locations.Multiple.As([]abi.Location{reg})
	two := abi.Locations.Multiple.As([]abi.Location{reg, reg})
	for _, c := range []struct {
		a, b abi.Location
		want bool
	}{
		{reg, reg, true},
		{reg, other, false},
		{one, one, true},
		{two, one, false},
		{one, two, false},
		{one, abi.Locations.Multiple.As([]abi.Location{other}), false},
		{abi.Locations.Returned, abi.Locations.Returned, true},
		{abi.Locations.Returned, reg, false},
		{abi.Locations.Indirect.As(abi.IndirectLocation{Location: reg}), abi.Locations.Indirect.As(abi.IndirectLocation{Location: reg}), true},
		{abi.Locations.Indirect.As(abi.IndirectLocation{Location: reg}), abi.Locations.Indirect.As(abi.IndirectLocation{Location: other}), false},
	} {
		if got := c.a.Equals(c.b); got != c.want {
			t.Errorf("%v.Equals(%v) = %v", c.a, c.b, got)
		}
	}
}

func TestZeroLaysOutTheStack(t *testing.T) {
	cc, err := abi.Zero(abi.Function{
		Args: []abi.Value{abi.Values.Bytes1, abi.Values.Bytes8},
		Rets: []abi.Value{abi.Values.Bytes4},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uintptr{0, 8} {
		if got := abi.HardwareLocations.StackRtl.Get(abi.Locations.Hardware.Get(cc.Args[i])); got != want {
			t.Errorf("argument %d is at %d, want %d", i, got, want)
		}
	}
	if got := abi.HardwareLocations.StackRtl.Get(abi.Locations.Hardware.Get(cc.Rets[0])); got != 16 {
		t.Errorf("result is at %d", got)
	}
}

func TestPinIgnoresASpilledValue(t *testing.T) {
	str := abi.ValueOf(reflect.TypeFor[string]())
	args := make([]abi.Value, 8, 9)
	for i := range args {
		args[i] = abi.Values.Bytes8
	}
	cc, err := abi.Internal(abi.Function{Args: append(args, str)})
	if err != nil {
		t.Fatal(err)
	}
	if pins := str.Pin(cc.Args[8]); len(pins) != 0 {
		t.Errorf("got %v", pins)
	}
}

func TestPinFollowsPointersInRegisters(t *testing.T) {
	str := abi.ValueOf(reflect.TypeFor[string]())
	cc, err := abi.Internal(abi.Function{Args: []abi.Value{abi.Values.Bytes8, str}})
	if err != nil {
		t.Fatal(err)
	}
	if pins := str.Pin(cc.Args[1]); len(pins) != 1 || pins[0] != cpu.R1 {
		t.Errorf("got %v", pins)
	}
}

func TestLocationsThatCannotBeUsedAreErrors(t *testing.T) {
	stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(8))
	if _, err := stack.Read(); err == nil {
		t.Errorf("Read of a stack location: want an error")
	}
	if _, err := stack.Send(); err == nil {
		t.Errorf("Send to a stack location: want an error")
	}
	three := abi.Locations.Multiple.As([]abi.Location{stack, stack, stack})
	if _, err := three.Read(); err == nil {
		t.Errorf("Read of three parts: want an error")
	}
}

package amd64

import (
	"runtime"
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
	"github.com/iqhive/runtime.link/xyz"
)

// requireLinux skips where ABI refuses to lay anything out: it only knows the
// System V convention as linux uses it, though the tests run on any host.
func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("ABI only supports linux")
	}
}

func args(t *testing.T, values ...abi.Value) []abi.Location {
	t.Helper()
	requireLinux(t)
	cc, err := ABI(abi.Function{Args: values})
	if err != nil {
		t.Fatal(err)
	}
	return cc.Args
}

func repeat(value abi.Value, n int) []abi.Value {
	values := make([]abi.Value, n)
	for i := range values {
		values[i] = value
	}
	return values
}

func register(l cpu.Location) abi.Location {
	return abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(l))
}

func floating(l cpu.Location) abi.Location {
	return abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(l))
}

func stack(offset uintptr) abi.Location {
	return abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(offset))
}

func rdx() abi.Location {
	return abi.Locations.Hardware.As(abi.HardwareLocations.Specific.As(RDX))
}

func expect(t *testing.T, got []abi.Location, want ...abi.Location) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d locations, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equals(want[i]) {
			t.Errorf("argument %d is at %v, want %v", i, got[i], want[i])
		}
	}
}

func TestIntegersUseSuccessiveRegisters(t *testing.T) {
	expect(t, args(t, repeat(abi.Values.Bytes8, 7)...),
		register(RDI), register(RSI), rdx(), register(RCX), register(R8), register(R9), stack(0))
}

func TestFloatsUseSuccessiveRegisters(t *testing.T) {
	expect(t, args(t, repeat(abi.Values.Float8, 9)...),
		floating(0), floating(1), floating(2), floating(3), floating(4), floating(5), floating(6), floating(7), stack(0))
}

func TestIntegerAndFloatRegistersAreCountedApart(t *testing.T) {
	expect(t, args(t, abi.Values.Float8, abi.Values.Bytes8, abi.Values.Float4, abi.Values.Bytes4),
		floating(0), register(RDI), floating(1), register(RSI))
	expect(t, args(t, append(repeat(abi.Values.Float8, 6), abi.Values.Bytes8)...),
		floating(0), floating(1), floating(2), floating(3), floating(4), floating(5), register(RDI))
}

func TestStructsInMemoryTakeTheirOwnStackSpace(t *testing.T) {
	big := abi.Values.Struct.As(repeat(abi.Values.Bytes8, 3))
	expect(t, args(t, big, big, abi.Values.Bytes4), stack(0), stack(24), register(RDI))
}

func TestStackArgumentsAreEightByteSlots(t *testing.T) {
	byteStruct := abi.Values.Struct.As(repeat(abi.Values.Bytes1, 17))
	expect(t, args(t, byteStruct, byteStruct), stack(0), stack(24))
}

func TestStructsArePassedByEightbyte(t *testing.T) {
	two := func(a, b abi.Value) abi.Value { return abi.Values.Struct.As([]abi.Value{a, b}) }
	for _, c := range []struct {
		name  string
		value abi.Value
		want  []abi.Location
	}{
		{"struct{int,int}", two(abi.Values.Bytes4, abi.Values.Bytes4), []abi.Location{register(RDI)}},
		{"struct{float,float}", two(abi.Values.Float4, abi.Values.Float4), []abi.Location{floating(0)}},
		{"struct{int,float}", two(abi.Values.Bytes4, abi.Values.Float4), []abi.Location{register(RDI)}},
		{"struct{long,long}", two(abi.Values.Bytes8, abi.Values.Bytes8), []abi.Location{register(RDI), register(RSI)}},
		{"struct{double,long}", two(abi.Values.Float8, abi.Values.Bytes8), []abi.Location{floating(0), register(RDI)}},
		{"struct{long,double}", two(abi.Values.Bytes8, abi.Values.Float8), []abi.Location{register(RDI), floating(0)}},
		{"int[2]", abi.Values.Array.As(repeat(abi.Values.Bytes4, 2)), []abi.Location{register(RDI)}},
		{"float[4]", abi.Values.Array.As(repeat(abi.Values.Float4, 4)), []abi.Location{floating(0), floating(1)}},
	} {
		got := args(t, c.value)
		if !got[0].Equals(abi.Locations.Multiple.As(c.want)) {
			t.Errorf("%s: got %v, want %v", c.name, got[0], c.want)
		}
	}
}

func TestStructIsNotSplitBetweenRegistersAndStack(t *testing.T) {
	pair := abi.Values.Struct.As(repeat(abi.Values.Bytes8, 2))
	got := args(t, append(repeat(abi.Values.Bytes8, 5), pair, abi.Values.Bytes8)...)
	expect(t, got[5:], stack(0), register(R9))
}

func TestPaddingCountsTowardsTheSizeLimit(t *testing.T) {
	v := abi.Values.Struct.As([]abi.Value{abi.Values.Bytes1, abi.Values.Bytes8, abi.Values.Bytes1})
	if v.Size() != 24 {
		t.Errorf("sizeof(struct{char,long,char}) = %d", v.Size())
	}
	expect(t, args(t, v), stack(0))
}

func TestResultsAreInTheirRegisters(t *testing.T) {
	requireLinux(t)
	cc, err := ABI(abi.Function{Rets: []abi.Value{abi.Values.Bytes8}})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, cc.Rets, register(RAX))
	cc, err = ABI(abi.Function{Rets: []abi.Value{abi.Values.Float8}})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, cc.Rets, floating(0))
}

func TestStructResultsAreRefused(t *testing.T) {
	if _, err := ABI(abi.Function{Rets: []abi.Value{abi.Values.Struct.As(repeat(abi.Values.Bytes8, 2))}}); err == nil {
		t.Errorf("want an error")
	}
}

func TestZeroSizedArgumentsTakeNothing(t *testing.T) {
	got := args(t, abi.Values.Bytes0, abi.Values.Bytes8)
	if xyz.ValueOf(got[1]) != abi.Locations.Hardware || !got[1].Equals(register(RDI)) {
		t.Errorf("got %v", got[1])
	}
}

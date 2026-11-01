package arm64

import (
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
)

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
	return abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(l - D0))
}

func stack(offset uintptr) abi.Location {
	return abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(offset))
}

func multiple(locations ...abi.Location) abi.Location {
	return abi.Locations.Multiple.As(locations)
}

func expect(t *testing.T, apple bool, values []abi.Value, want ...abi.Location) {
	t.Helper()
	cc, err := classify(abi.Function{Args: values}, apple)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if !cc.Args[i].Equals(want[i]) {
			t.Errorf("argument %d is at %v, want %v", i, cc.Args[i], want[i])
		}
	}
}

func TestSmallCompositeUsesGeneralRegisters(t *testing.T) {
	long2 := abi.Values.Struct.As(repeat(abi.Values.Bytes8, 2))
	expect(t, false, []abi.Value{long2}, multiple(register(X0), register(X1)))
	int2 := abi.Values.Struct.As(repeat(abi.Values.Bytes4, 2))
	expect(t, false, []abi.Value{int2}, multiple(register(X0)))
	mixed := abi.Values.Struct.As([]abi.Value{abi.Values.Bytes8, abi.Values.Float8})
	expect(t, false, []abi.Value{mixed}, multiple(register(X0), register(X1)))
}

func TestFloatAggregateUpToFourMembersUsesFloatRegisters(t *testing.T) {
	hfa := abi.Values.Struct.As(repeat(abi.Values.Float8, 4))
	expect(t, false, []abi.Value{hfa}, multiple(floating(D0), floating(D1), floating(D2), floating(D3)))
	nested := abi.Values.Struct.As([]abi.Value{abi.Values.Array.As(repeat(abi.Values.Float4, 2)), abi.Values.Float4})
	expect(t, false, []abi.Value{nested}, multiple(floating(D0), floating(D1), floating(D2)))
	five := abi.Values.Struct.As(repeat(abi.Values.Float4, 5))
	expect(t, false, []abi.Value{five}, abi.Locations.Indirect.As(abi.IndirectLocation{Location: register(X0)}))
	different := abi.Values.Struct.As([]abi.Value{abi.Values.Float4, abi.Values.Float8})
	expect(t, false, []abi.Value{different}, multiple(register(X0), register(X1)))
}

func TestLargeCompositeIsPassedByReference(t *testing.T) {
	big := abi.Values.Struct.As(repeat(abi.Values.Bytes8, 5))
	expect(t, false, []abi.Value{abi.Values.Bytes8, big},
		register(X0), abi.Locations.Indirect.As(abi.IndirectLocation{Location: register(X1)}))
}

func TestSpilledFloatAggregateFillsTheRemainingRegisters(t *testing.T) {
	hfa := abi.Values.Struct.As(repeat(abi.Values.Float8, 3))
	args := append(repeat(abi.Values.Float8, 6), hfa, abi.Values.Float8)
	expect(t, false, args, floating(D0), floating(D1), floating(D2), floating(D3), floating(D4), floating(D5), stack(0), stack(24))
}

func TestSpilledCompositeFillsTheRemainingRegisters(t *testing.T) {
	long2 := abi.Values.Struct.As(repeat(abi.Values.Bytes8, 2))
	args := append(repeat(abi.Values.Bytes8, 7), long2, abi.Values.Bytes8)
	cc, err := classify(abi.Function{Args: args}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !cc.Args[7].Equals(stack(0)) || !cc.Args[8].Equals(stack(16)) {
		t.Errorf("got %v and %v", cc.Args[7], cc.Args[8])
	}
}

func TestLargeResultGoesThroughX8(t *testing.T) {
	cc, err := classify(abi.Function{Rets: []abi.Value{abi.Values.Struct.As(repeat(abi.Values.Bytes8, 5))}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := abi.Locations.Indirect.As(abi.IndirectLocation{Location: register(X8)}); !cc.Rets[0].Equals(want) {
		t.Errorf("got %v", cc.Rets[0])
	}
}

func TestSmallResultsUseArgumentRegisters(t *testing.T) {
	cc, err := classify(abi.Function{Rets: []abi.Value{abi.Values.Struct.As(repeat(abi.Values.Float8, 2))}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := multiple(floating(D0), floating(D1)); !cc.Rets[0].Equals(want) {
		t.Errorf("got %v", cc.Rets[0])
	}
}

func TestStackSlotsDependOnThePlatform(t *testing.T) {
	args := append(repeat(abi.Values.Bytes8, 8), abi.Values.Bytes1, abi.Values.Bytes1, abi.Values.Bytes4, abi.Values.Bytes8)
	cc, err := classify(abi.Function{Args: args}, false)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uintptr{0, 8, 16, 24} {
		if !cc.Args[8+i].Equals(stack(want)) {
			t.Errorf("AAPCS64: argument %d is at %v, want %d", 8+i, cc.Args[8+i], want)
		}
	}
	cc, err = classify(abi.Function{Args: args}, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uintptr{0, 1, 4, 8} {
		if !cc.Args[8+i].Equals(stack(want)) {
			t.Errorf("Apple: argument %d is at %v, want %d", 8+i, cc.Args[8+i], want)
		}
	}
}

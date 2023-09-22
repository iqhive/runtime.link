package abi_test

import (
	"testing"

	"runtime.link/lib/internal/abi"
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

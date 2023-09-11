package abi_test

import (
	"testing"

	"runtime.link/std/abi"
)

func TestOperationToString(t *testing.T) {
	for op := abi.Operation(0); op < abi.Operations; op++ {
		if op.String() == "INVALID" {
			t.Errorf("Operation(%d).String() == 'INVALID'", op)
		}
	}
}

package abi

import (
	"fmt"
	"reflect"
	"unsafe"

	"runtime.link/std/abi/arm64"
)

type CallingConvention int

const (
	Default CallingConvention = iota
)

// MakeCall returns a native-assembly function that calls the provided
// function pointer using the provided assembly calling instructions.
func (cc CallingConvention) Call(rtype reflect.Type, call unsafe.Pointer, src []Operation) (reflect.Value, error) {
	ops, err := compile(src)
	if err != nil {
		return reflect.Value{}, err
	}
	switch cc {
	case Default:
		return arm64.Call(rtype, call, ops), nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported calling convention '%d'", cc)
	}
}

// compile general asm calling instructions to arm64-specific
// instructions.
func compile(src []Operation) (ops []arm64.Operation, err error) {
	peek := func(i int) Operation {
		if i+1 < len(src) {
			return src[i+1]
		}
		return Noop
	}
	var fromU, fromF arm64.Operation // caller argument register counters.
	var intoU, intoF arm64.Operation // callee argument register counters.
	for i, op := range src {
		switch op {
		case SendArgByt, SendArgU16, SendArgU32, SendArgU64:
			if peek(i) < SendArgByt {
				ops = append(ops, arm64.R0+intoU+arm64.Write)
			}
			intoU++
		case RecvArgByt, RecvArgU16, RecvArgU32, RecvArgU64:
			if peek(i) > RecvArgNew || peek(i) < RecvArgByt {
				ops = append(ops, arm64.R0+fromU)
			}
			fromU++
		case RecvArgF32, RecvArgF64:
			if peek(i) > RecvArgNew || peek(i) < RecvArgF32 {
				ops = append(ops, arm64.X0+fromF)
			}
			fromF++
		case SendArgF32, SendArgF64:
			if peek(i) < SendArgF32 {
				ops = append(ops, arm64.X0+intoF+arm64.Write)
			}
			intoF++
		case RecvRetF64:
			ops = append(ops, arm64.X0+fromU)
		case RecvRetU64, RecvArgPtr:
			ops = append(ops, arm64.R0+fromU)
		case SendRetF64:
			ops = append(ops, arm64.X0+fromU+arm64.Write)
		case SendRetU64:
			ops = append(ops, arm64.R0+fromU+arm64.Write)
		case Call:
			ops = append(ops, arm64.Jump)
		default:
			return nil, fmt.Errorf("unsupported 'asm' instruction '%s' on arm64", op)
		}
		if fromF >= 8 || fromU >= 8 || intoF >= 8 || intoU >= 8 {
			return nil, fmt.Errorf("too many arguments to call using arm64 registers") // FIXME stack.
		}
	}
	return ops, nil
}

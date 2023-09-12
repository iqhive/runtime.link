package abi

import (
	"fmt"
	"reflect"
	"unsafe"

	"runtime.link/std/abi/internal/cpu"
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
		return cpu.Call(rtype, call, ops), nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported calling convention '%d'", cc)
	}
}

// compile general asm calling instructions to arm64-specific
// instructions.
func compile(src []Operation) (ops []cpu.Instruction, err error) {
	peek := func(i int) Operation {
		if i+1 < len(src) {
			return src[i+1]
		}
		return Operations
	}
	var moveR, moveX cpu.Instruction // caller argument register counters.
	var copyR, copyX cpu.Instruction // callee argument register counters.

	//var normal = cpu.S0
	var length = cpu.S1
	var assert = cpu.S2
	//var failed = cpu.S3

	var heap bool
	var call = true

	// optimisation
	checkRedundantWrite := func() {
		if len(ops) >= 2 {
			if ops[len(ops)-1] == ops[len(ops)-2]+cpu.Write {
				ops = ops[:len(ops)-2]
			}
		}
	}

	for i := 0; i < len(src); i++ {
		op := src[i]

		switch op {
		case MoveValByt, MoveValU16, MoveValU32, MoveValU64:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				ops = append(ops, cpu.R0+moveR)
			} else {
				ops = append(ops, cpu.Write+cpu.R0+moveR)
			}
			moveR++
		case MoveValF32, MoveValF64:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				ops = append(ops, cpu.X0+moveX)
			} else {
				ops = append(ops, cpu.Write+cpu.X0+moveX)
			}
			moveR++
		case MoveValStr:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				ops = append(ops, cpu.R1+moveR)
				ops = append(ops, cpu.Write+length)
				ops = append(ops, cpu.R0+moveR)
				moveR += 2
			} else {
				return nil, fmt.Errorf("non-recv move not supported on arm64, %s", op)
			}
		case MoveValErr:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				ops = append(ops, cpu.R0+moveR)
				ops = append(ops, cpu.Write+assert)
				moveR++
				ops = append(ops, cpu.R0+moveR)
				moveR++
			} else {
				checkRedundantWrite()
				ops = append(ops, assert)
				ops = append(ops, cpu.ErrorValue)
				ops = append(ops, cpu.Write+cpu.R1+moveR)
				ops = append(ops, cpu.Error)
				ops = append(ops, cpu.Write+cpu.R0+moveR)
				moveR += 2
			}
		case CopyValByt, CopyValU16, CopyValU32, CopyValU64, CopyValPtr:
			if peek(i).IsCopy() {
				continue
			}
			if call {
				ops = append(ops, cpu.Write+cpu.R0+copyR)
			} else {
				checkRedundantWrite()
				ops = append(ops, cpu.R0+copyR)
			}
			copyR++
		case CopyValF32, CopyValF64:
			if peek(i).IsCopy() {
				continue
			}
			if call {
				ops = append(ops, cpu.Write+cpu.X0+copyX)
			} else {
				checkRedundantWrite()
				ops = append(ops, cpu.X0+copyX)
			}
			copyX++
		case CopyValStr:
			if heap {
				ops = append(ops, cpu.MemoryCopy)
			} else {
				return nil, fmt.Errorf("non-heap string copy not supported on arm64")
			}
		case NormalSet0:
			checkRedundantWrite()
			ops = append(ops, cpu.Set0)
		case NormalSet1:
			checkRedundantWrite()
			ops = append(ops, cpu.Set1)
		case SwapAssert:
			ops = append(ops, cpu.Swap2)
		case AssertLess:
			ops = append(ops, cpu.AssertLess)
		case AssertMore:
			ops = append(ops, cpu.AssertMore)
		case MakeMemory:
			ops = append(ops, cpu.Write+cpu.Heap)
			heap = true
		case DoneMemory:
			ops = append(ops, cpu.Heap)
			heap = false
		case NullString:
			ops = append(ops, cpu.MemoryNull)
		case JumpToCall:
			checkRedundantWrite()
			//ops = append(ops, cpu.WriteR3)
			//ops = append(ops, cpu.WriteX3)
			ops = append(ops, cpu.Jump)
			call = false
			moveR = 0
			moveX = 0
			copyR = 0
			copyX = 0
		default:
			//fmt.Println(src)
			return nil, fmt.Errorf("unsupported 'asm' instruction '%s' on arm64", op)
		}
		if moveX >= 8 || moveR >= 8 || copyX >= 8 || copyR >= 8 {
			return nil, fmt.Errorf("too many arguments to call using arm64 registers") // FIXME stack.
		}
	}
	checkRedundantWrite()

	return ops, nil
}

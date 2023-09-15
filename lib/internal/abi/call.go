package abi

import (
	"fmt"

	"runtime.link/lib/internal/cpu"
)

// compile general asm calling instructions to arm64-specific
// instructions.
func compile(src []Operation) (program cpu.Program, err error) {
	peek := func(i int) Operation {
		if i+1 < len(src) {
			return src[i+1]
		}
		return Operations
	}
	var (
		// caller argument register counters.
		loadR = cpu.R0
		loadX = cpu.X0

		// callee argument register counters.
		moveR = cpu.R0
		moveX = cpu.X0

		heap bool
		call = true
	)
	// optimisation
	checkRedundantWrite := func() {
		ops := program.Text
		if len(ops) >= 2 {
			mode1, data1 := ops[len(ops)-1].Decode()
			mode2, data2 := ops[len(ops)-2].Decode()
			if mode1 == cpu.Move && mode2 == cpu.Load && data1 == data2 {
				program.Text = ops[:len(ops)-2]
			}
		}
	}
	for i := 0; i < len(src); i++ {
		var (
			op = src[i]
		)
		switch op {
		case MoveValByt, MoveValU16, MoveValU32, MoveValU64:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				program.Add(cpu.Load.New(loadR))
			} else {
				program.Add(cpu.Move.New(loadR))
			}
			loadR++
		case MoveValF32, MoveValF64:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				program.Add(cpu.Load.New(loadX))
			} else {
				program.Add(cpu.Move.New(loadX))
			}
			loadR++
		case MoveValStr:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				program.Add(cpu.Load.New(loadR + 1))
				program.Add(cpu.Func.New(cpu.SwapLength))
				program.Add(cpu.Load.New(loadR))
				loadR += 2
			} else {
				return program, fmt.Errorf("non-recv move not supported on arm64, %s", op)
			}
		case MoveValErr:
			if call {
				if peek(i).IsMove() {
					continue
				}
				checkRedundantWrite()
				program.Add(cpu.Load.New(loadR))
				program.Add(cpu.Func.New(cpu.SwapAssert))
				loadR++
				program.Add(cpu.Load.New(loadR))
				loadR++
			} else {
				checkRedundantWrite()
				program.Add(cpu.Func.New(cpu.ErrorMake))
				program.Add(cpu.Move.New(loadR + 1))
				program.Add(cpu.Func.New(cpu.SwapAssert))
				program.Add(cpu.Move.New(loadR))
				loadR += 2
			}
		case CopyNewVal:
			moveR = cpu.R0
			moveX = cpu.X0
		case CopyValByt, CopyValU16, CopyValU32, CopyValU64, CopyValPtr:
			if peek(i).IsCopy() {
				continue
			}
			if call {
				program.Add(cpu.Move.New(moveR))
			} else {
				checkRedundantWrite()
				program.Add(cpu.Load.New(moveR))
			}
			moveR++
		case CopyValF32, CopyValF64:
			if peek(i).IsCopy() {
				continue
			}
			if call {
				program.Add(cpu.Move.New(moveX))
			} else {
				checkRedundantWrite()
				program.Add(cpu.Load.New(moveX))
			}
			moveX++
		case CopyValStr:
			if heap {
				program.Add(cpu.Func.New(cpu.StringCopy))
			} else {
				return program, fmt.Errorf("non-heap string copy not supported on arm64")
			}
		case NormalSet0:
			checkRedundantWrite()
			program.Add(cpu.Bits.New(0))
		case NormalSet1:
			checkRedundantWrite()
			program.Add(cpu.Bits.New(1))
		case SwapAssert:
			program.Add(cpu.Func.New(cpu.SwapAssert))
		case AssertLess:
			program.Add(cpu.Math.New(cpu.Less))
		case AssertMore:
			program.Add(cpu.Math.New(cpu.More))
		case MakeMemory:
			program.Add(cpu.Math.New(cpu.HeapMake))
			heap = true
		case DoneMemory:
			program.Add(cpu.Math.New(cpu.HeapLoad))
			heap = false
		case NullString:
			program.Add(cpu.Func.New(cpu.StringMake))
		case JumpToCall:
			checkRedundantWrite()
			program.Add(cpu.Func.New(cpu.Call))
			call = false
			loadR = cpu.R0
			loadX = cpu.X0
			moveR = cpu.R0
			moveX = cpu.X0
		default:
			//fmt.Println(src)
			return program, fmt.Errorf("unsupported 'asm' instruction '%s' on arm64", op)
		}
	}
	checkRedundantWrite()
	program.Dump = func() {
		fmt.Println(src)
	}
	return program, nil
}

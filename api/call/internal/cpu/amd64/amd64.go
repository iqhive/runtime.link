package amd64

import (
	"errors"
	"runtime"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
	"github.com/iqhive/runtime.link/xyz"
)

const (
	X0  = cpu.X0
	X1  = cpu.X1
	X2  = cpu.X2
	X3  = cpu.X3
	X4  = cpu.X4
	X5  = cpu.X5
	X6  = cpu.X6
	X7  = cpu.X7
	X8  = cpu.X8
	X9  = cpu.X9
	X10 = cpu.X10
	X11 = cpu.X11
	X12 = cpu.X12
	X13 = cpu.X13
	X14 = cpu.X14

	RAX = cpu.R0
	RBX = cpu.R1
	RCX = cpu.R2
	RDI = cpu.R3
	RSI = cpu.R4
	R8  = cpu.R5
	R9  = cpu.R6
	R10 = cpu.R7
	R11 = cpu.R8

	RDX = cpu.ArchFunc(0)
)

// ABI (AMD64)
// https://dyncall.org/docs/manual/manualse11.html#x12-55000D.2
// https://refspecs.linuxbase.org/elf/x86_64-abi-0.99.pdf
func ABI(fn abi.Function) (cc abi.CallingConvention, err error) {
	if runtime.GOOS != "linux" {
		return cc, errors.New("unsupported operating system " + runtime.GOOS + " for amd64")
	}
	if fn.Vars {
		return cc, errors.New("variadic functions are not supported on amd64")
	}
	var (
		gpr = []abi.Location{
			abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(RDI)),
			abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(RSI)),
			abi.Locations.Hardware.As(abi.HardwareLocations.Specific.As(RDX)),
			abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(RCX)),
			abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(R8)),
			abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(R9)),
		}
		fpr cpu.Location = 8
		x0  cpu.Location
		r0  int
		sp  uintptr
	)
	cc.Args = make([]abi.Location, len(fn.Args))
	cc.Rets = make([]abi.Location, len(fn.Rets))
	onStack := func(arg abi.Value) abi.Location {
		stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(sp))
		sp = (sp + arg.Size() + 7) &^ 7
		return stack
	}
	assign := func(arg abi.Value) abi.Location {
		if arg.Size() == 0 {
			return abi.Location{}
		}
		eightbytes, ok := classify(arg)
		if !ok {
			return onStack(arg)
		}
		var integers, floats int
		for _, sse := range eightbytes {
			switch {
			case sse:
				floats++
			default:
				integers++
			}
		}
		if r0+integers > len(gpr) || x0+cpu.Location(floats) > fpr {
			return onStack(arg) // an argument can't be part way into the registers.
		}
		var multiple []abi.Location
		for _, sse := range eightbytes {
			switch {
			case sse:
				multiple = append(multiple, abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(x0)))
				x0++
			default:
				multiple = append(multiple, gpr[r0])
				r0++
			}
		}
		if xyz.ValueOf(arg) == abi.Values.Struct || xyz.ValueOf(arg) == abi.Values.Array {
			return abi.Locations.Multiple.As(multiple)
		}
		return multiple[0]
	}
	for i, arg := range fn.Args {
		cc.Args[i] = assign(arg)
	}
	if len(fn.Rets) == 0 {
		return cc, nil
	}
	switch fn.Rets[0] {
	case abi.Values.Bytes1, abi.Values.Bytes2, abi.Values.Bytes4, abi.Values.Bytes8, abi.Values.Memory, abi.Values.Sizing:
		cc.Rets[0] = abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(RAX))
	case abi.Values.Float4, abi.Values.Float8:
		cc.Rets[0] = abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(0))
	default:
		return cc, errors.New("amd64 unsupported return type " + fn.Rets[0].String())
	}
	return cc, nil
}

// classify returns whether each eightbyte of the value is passed in an SSE
// register, or false if the value is too large to be passed in registers at all.
func classify(arg abi.Value) (eightbytes []bool, ok bool) {
	size := arg.Size()
	if size > 16 {
		return nil, false
	}
	eightbytes = make([]bool, (size+7)/8)
	for i := range eightbytes {
		eightbytes[i] = true
	}
	for _, scalar := range arg.Scalars() {
		if scalar.Value != abi.Values.Float4 && scalar.Value != abi.Values.Float8 {
			eightbytes[scalar.Offset/8] = false // INTEGER wins when an eightbyte holds both.
		}
	}
	return eightbytes, true
}

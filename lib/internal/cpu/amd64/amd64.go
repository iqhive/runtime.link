package amd64

import (
	"errors"
	"runtime"

	"runtime.link/lib/internal/abi"
	"runtime.link/lib/internal/cpu"
	"runtime.link/xyz"
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
	align := func(n uintptr) {
		sp = (sp + n - 1) &^ (n - 1)
	}
	var assignRegister func(arg abi.Value) (abi.Location, bool)
	assignRegister = func(arg abi.Value) (abi.Location, bool) {
		var loc abi.Location
		switch arg {
		case abi.Values.Bytes1, abi.Values.Bytes2, abi.Values.Bytes4, abi.Values.Bytes8, abi.Values.Memory, abi.Values.Sizing:
			loc = gpr[x0]
			r0++
		case abi.Values.Float4, abi.Values.Float8:
			loc = abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(x0))
			x0++
		default:
			var (
				multiple  []abi.Location
				structure = abi.Values.Struct.Get(arg)
			)
			for _, value := range structure {
				loc, ok := assignRegister(value)
				if !ok {
					return loc, false
				}
				multiple = append(multiple, loc)
			}
			loc = abi.Locations.Multiple.As(multiple)
		}
		if r0 >= len(gpr) || x0 >= fpr {
			return loc, false
		}
		return loc, true
	}
	assignValue := func(arg abi.Value) abi.Location {
		if arg.Size() == 0 {
			align(uintptr(arg.Align()))
			return abi.Location{}
		}
		br, bx := r0, x0
		reg, ok := assignRegister(arg)
		if ok {
			return reg
		}
		r0, x0 = br, bx
		stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(sp))
		sp += arg.Size()
		align(8)
		return stack
	}
	assign := func(arg abi.Value) abi.Location {
		if xyz.ValueOf(arg) == abi.Values.Struct.Value {
			if arg.Size() > 16 {
				return abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(sp))
			}
		}
		return assignValue(arg)
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

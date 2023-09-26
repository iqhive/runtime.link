package arm64

import (
	"errors"

	"runtime.link/lib/internal/abi"
	"runtime.link/lib/internal/cpu"
	"runtime.link/xyz"
)

const (
	X0  = cpu.R0
	X1  = cpu.R1
	X2  = cpu.R2
	X3  = cpu.R3
	X4  = cpu.R4
	X5  = cpu.R5
	X6  = cpu.R6
	X7  = cpu.R7
	X8  = cpu.R8
	X9  = cpu.R9
	X10 = cpu.R10
	X11 = cpu.R11
	X12 = cpu.R12
	X13 = cpu.R13
	X14 = cpu.R14
	X15 = cpu.R15

	D0  = cpu.X0
	D1  = cpu.X1
	D2  = cpu.X2
	D3  = cpu.X3
	D4  = cpu.X4
	D5  = cpu.X5
	D6  = cpu.X6
	D7  = cpu.X7
	D8  = cpu.X8
	D9  = cpu.X9
	D10 = cpu.X10
	D11 = cpu.X11
	D12 = cpu.X12
	D13 = cpu.X13
	D14 = cpu.X14
	D15 = cpu.X15
)

// ABI (AArch64)
// https://dyncall.org/docs/manual/manualse11.html#x12-74000D.6
// https://github.com/ARM-software/abi-aa/blob/844a79fd4c77252a11342709e3b27b2c9f590cf1/aapcs64/aapcs64.rst
func ABI(fn abi.Function) (cc abi.CallingConvention, err error) {
	if fn.Vars {
		return cc, errors.New("variadic functions are not supported on arm64")
	}
	var (
		gpr, fpr cpu.Location = 8, 8
		x0, d0   cpu.Location
		sp       uintptr
	)
	cc.Args = make([]abi.Location, len(fn.Args))
	cc.Rets = make([]abi.Location, len(fn.Rets))
	align := func(n uintptr) {
		sp = (sp + n - 1) &^ (n - 1)
	}
	var assignRegister func(arg abi.Value) (abi.Location, bool)
	assignRegister = func(arg abi.Value) (abi.Location, bool) {
		loc := abi.Location{}
		switch arg {
		case abi.Values.Bytes1, abi.Values.Bytes2, abi.Values.Bytes4, abi.Values.Bytes8, abi.Values.Memory, abi.Values.Sizing:
			loc = abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(x0))
			x0++
		case abi.Values.Float4, abi.Values.Float8:
			loc = abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(d0))
			d0++
		default:
			var (
				multiple  []abi.Location
				size      = arg.Size()
				structure = abi.Values.Struct.Get(arg)
			)
			if allFloats := true; len(structure) < 4 {
				for i, value := range structure {
					if value != abi.Values.Float4 && value != abi.Values.Float8 {
						allFloats = false
						break
					}
					if i > 0 && value != structure[i-1] {
						allFloats = false
						break
					}
				}
				if allFloats {
					for _, value := range structure {
						loc, ok := assignRegister(value)
						if !ok {
							return loc, false
						}
						multiple = append(multiple, loc)
					}
					loc = abi.Locations.Multiple.As(multiple)
				}
			} else if size <= 16 {
				for _, value := range structure {
					loc, ok := assignRegister(value)
					if !ok {
						return loc, false
					}
					multiple = append(multiple, loc)
				}
				loc = abi.Locations.Multiple.As(multiple)
			} else {
				return loc, false
			}
		}
		if x0 > gpr || d0 > fpr {
			return loc, false
		}
		return loc, true
	}
	assignValue := func(arg abi.Value) abi.Location {
		if arg.Size() == 0 {
			align(uintptr(arg.Align()))
			return abi.Location{}
		}
		br, bx := x0, d0
		reg, ok := assignRegister(arg)
		if ok {
			return reg
		}
		x0, d0 = br, bx
		stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(sp))
		sp += arg.Size()
		align(8)
		return stack
	}
	assign := func(arg abi.Value) abi.Location {
		allFloats := true
		if xyz.ValueOf(arg) == abi.Values.Struct.Value {
			structure := abi.Values.Struct.Get(arg)
			if len(structure) < 4 {
				for i, value := range structure {
					if value != abi.Values.Float4 && value != abi.Values.Float8 {
						allFloats = false
						break
					}
					if i > 0 && value != structure[i-1] {
						allFloats = false
						break
					}
				}
			}
		}
		if arg.Size() > 16 && !allFloats {
			return abi.Locations.Indirect.As(abi.IndirectLocation{
				Location: assignValue(abi.Values.Bytes8),
			})
		}
		return assignValue(arg)
	}
	for i, arg := range fn.Args {
		cc.Args[i] = assign(arg)
	}
	x0, d0 = 0, 0
	for i, ret := range fn.Rets {
		cc.Rets[i] = assign(ret)
	}
	return cc, nil
}

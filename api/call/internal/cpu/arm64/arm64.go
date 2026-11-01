package arm64

import (
	"errors"
	"runtime"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
	"github.com/iqhive/runtime.link/xyz"
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
	return classify(fn, runtime.GOOS == "darwin" || runtime.GOOS == "ios")
}

// classify is passed whether to follow Apple's variation of AAPCS64, where
// arguments on the stack are packed by their natural alignment, they each
// take a slot of 8 bytes everywhere else.
func classify(fn abi.Function, apple bool) (cc abi.CallingConvention, err error) {
	if fn.Vars {
		return cc, errors.New("variadic functions are not supported on arm64")
	}
	const gpr, fpr = 8, 8
	var (
		x0, d0 cpu.Location // the next general and floating-point register, 'NGRN' and 'NSRN'.
		sp     uintptr
	)
	cc.Args = make([]abi.Location, len(fn.Args))
	cc.Rets = make([]abi.Location, len(fn.Rets))
	align := func(n uintptr) {
		sp = (sp + n - 1) &^ (n - 1)
	}
	onStack := func(arg abi.Value) abi.Location {
		switch {
		case apple:
			align(arg.Align())
		default:
			align(max(arg.Align(), 8))
		}
		stack := abi.Locations.Hardware.As(abi.HardwareLocations.StackRtl.As(sp))
		sp += arg.Size()
		if !apple {
			align(8)
		}
		return stack
	}
	inRegisters := func(arg abi.Value, locations []abi.Location) abi.Location {
		if isComposite(arg) {
			return abi.Locations.Multiple.As(locations)
		}
		return locations[0]
	}
	var assign func(arg abi.Value) abi.Location
	assign = func(arg abi.Value) abi.Location {
		if arg.Size() == 0 {
			return abi.Location{}
		}
		if members, ok := homogeneous(arg); ok {
			if d0+cpu.Location(members) > fpr {
				d0 = fpr // C.3: nothing later may fill in the registers that are left over.
				return onStack(arg)
			}
			locations := make([]abi.Location, members)
			for i := range locations {
				locations[i] = abi.Locations.Hardware.As(abi.HardwareLocations.Floating.As(d0))
				d0++
			}
			return inRegisters(arg, locations)
		}
		if isComposite(arg) && arg.Size() > 16 {
			return abi.Locations.Indirect.As(abi.IndirectLocation{Location: assign(abi.Values.Bytes8)})
		}
		words := cpu.Location((arg.Size() + 7) / 8)
		if x0+words > gpr {
			x0 = gpr // C.13: the same rule as for floats.
			return onStack(arg)
		}
		locations := make([]abi.Location, words)
		for i := range locations {
			locations[i] = abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(x0))
			x0++
		}
		return inRegisters(arg, locations)
	}
	for i, arg := range fn.Args {
		cc.Args[i] = assign(arg)
	}
	x0, d0 = 0, 0
	for i, ret := range fn.Rets {
		if _, ok := homogeneous(ret); !ok && isComposite(ret) && ret.Size() > 16 {
			cc.Rets[i] = abi.Locations.Indirect.As(abi.IndirectLocation{
				Location: abi.Locations.Hardware.As(abi.HardwareLocations.Register.As(X8)),
			})
			continue
		}
		cc.Rets[i] = assign(ret)
	}
	return cc, nil
}

func isComposite(arg abi.Value) bool {
	return xyz.ValueOf(arg) == abi.Values.Struct || xyz.ValueOf(arg) == abi.Values.Array
}

// homogeneous returns the number of floating-point registers needed for
// a float, or for a homogeneous floating-point aggregate of up to 4 members.
func homogeneous(arg abi.Value) (members int, ok bool) {
	scalars := arg.Scalars()
	if len(scalars) == 0 || len(scalars) > 4 {
		return 0, false
	}
	for _, scalar := range scalars {
		if scalar.Value != abi.Values.Float4 && scalar.Value != abi.Values.Float8 {
			return 0, false
		}
		if scalar.Value != scalars[0].Value {
			return 0, false
		}
	}
	return len(scalars), true
}

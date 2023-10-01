package abi

import (
	"errors"
	"runtime"
	"unsafe"

	"runtime.link/api/link/internal/cpu"
)

// Zero is the Go ABI0 calling convention. All arguments are
// passed on the stack in right to left order.
func Zero(fn Function) (cc CallingConvention, err error) {
	if fn.Vars {
		return cc, errors.New("abi: variadic functions are not supported for ABI0")
	}
	var (
		sp uintptr
	)
	align := func(n uintptr) {
		sp = (sp + n - 1) &^ (n - 1)
	}
	assign := func(arg Value) Location {
		if arg.Size() == 0 {
			align(uintptr(arg.Align()))
			return Location{}
		}
		stack := Locations.Hardware.As(HardwareLocations.StackRtl.As(sp))
		sp += arg.Size()
		return stack
	}
	for i, arg := range fn.Args {
		cc.Args[i] = assign(arg)
	}
	align(unsafe.Alignof(uintptr(0)))
	for i, ret := range fn.Rets {
		cc.Rets[i] = assign(ret)
	}
	align(unsafe.Alignof(uintptr(0)))
	return cc, nil
}

// Internal Go ABI calling convention (requirement for direct cpu register access) any
// arguments that fit entirely into the remaining registers are passed in registers.
// The rest are passed on the stack, as per ABI0.
// https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
func Internal(fn Function) (cc CallingConvention, err error) {
	if fn.Vars {
		return cc, errors.New("abi: variadic functions are not supported for ABIInternal")
	}
	var (
		gpr, fpr cpu.Location
		r0, x0   cpu.Location
		sp       uintptr
	)
	cc.Args = make([]Location, len(fn.Args))
	cc.Rets = make([]Location, len(fn.Rets))
	switch runtime.GOARCH {
	case "amd64":
		gpr = 9
		fpr = 15
	case "arm64":
		gpr = 16
		fpr = 16
	case "ppc64":
		gpr = 12
		fpr = 13
	case "riscv64":
		gpr = 16
		fpr = 16
	default:
		panic("abi.Internal: not implemented for architecture " + runtime.GOARCH)
	}
	align := func(n uintptr) {
		sp = (sp + n - 1) &^ (n - 1)
	}
	var assignRegister func(arg Value) (Location, bool)
	assignRegister = func(arg Value) (Location, bool) {
		loc := Location{}
		switch arg {
		case Values.Bytes0:
			return loc, true
		case Values.Bytes1, Values.Bytes2, Values.Bytes4, Values.Bytes8, Values.Memory, Values.Sizing:
			loc = Locations.Hardware.As(HardwareLocations.Register.As(r0))
			r0++
		case Values.Float4, Values.Float8:
			loc = Locations.Hardware.As(HardwareLocations.Floating.As(x0))
			x0++
		default:
			var multiple []Location
			structure := Values.Struct.Get(arg)
			for _, value := range structure {
				loc, ok := assignRegister(value)
				if !ok {
					return loc, false
				}
				multiple = append(multiple, loc)
			}
			loc = Locations.Multiple.As(multiple)
		}
		if r0 > gpr || x0 > fpr {
			return loc, false
		}
		return loc, true
	}
	assign := func(arg Value) Location {
		if arg.Size() == 0 {
			align(uintptr(arg.Align()))
			return Location{}
		}
		br, bx := r0, x0
		reg, ok := assignRegister(arg)
		if ok {
			return reg
		}
		r0, x0 = br, bx
		stack := Locations.Hardware.As(HardwareLocations.StackRtl.As(sp))
		sp += arg.Size()
		return stack
	}
	for i, arg := range fn.Args {
		cc.Args[i] = assign(arg)
	}
	align(unsafe.Alignof(uintptr(0)))
	spill := r0 + x0
	r0, x0 = 0, 0
	for i, ret := range fn.Rets {
		cc.Rets[i] = assign(ret)
	}
	align(unsafe.Alignof(uintptr(0)))
	spill += r0 + x0
	sp += uintptr(spill)
	align(unsafe.Alignof(uintptr(0)))
	return cc, nil
}

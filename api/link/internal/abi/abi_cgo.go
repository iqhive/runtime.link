package abi

import (
	"errors"

	"runtime.link/api/link/internal/cpu"
)

// CGO implementation is used to call the function. ie. with dyncall.
func CGO(fn Function) (cc CallingConvention, err error) {
	cc.Args = make([]Location, len(fn.Args))
	cc.Rets = make([]Location, len(fn.Rets))
	for i, arg := range fn.Args {
		switch arg {
		case Values.Bytes1:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushBytes1))
		case Values.Bytes2:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushBytes2))
		case Values.Bytes4:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushBytes4))
		case Values.Bytes8:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushBytes8))
		case Values.Float4:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushFloat4))
		case Values.Float8:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushFloat8))
		case Values.Memory:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushMemory))
		case Values.Sizing:
			cc.Args[i] = Locations.Hardware.As(HardwareLocations.Software.As(cpu.PushSizing))
		default:
			return cc, errors.New("cgo ABI structs not supported")
		}
	}
	if len(fn.Rets) == 0 {
		cc.Call = cpu.NewSlow(cpu.CallIgnore)
		return cc, nil
	}
	if len(fn.Rets) > 1 {
		return cc, errors.New("cgo ABI multiple return values not supported")
	}
	switch fn.Rets[0] {
	case Values.Bytes1:
		cc.Call = cpu.NewSlow(cpu.CallBytes1)
	case Values.Bytes2:
		cc.Call = cpu.NewSlow(cpu.CallBytes2)
	case Values.Bytes4:
		cc.Call = cpu.NewSlow(cpu.CallBytes4)
	case Values.Bytes8:
		cc.Call = cpu.NewSlow(cpu.CallBytes8)
	case Values.Float4:
		cc.Call = cpu.NewSlow(cpu.CallFloat4)
	case Values.Float8:
		cc.Call = cpu.NewSlow(cpu.CallFloat8)
	case Values.Memory:
		cc.Call = cpu.NewSlow(cpu.CallMemory)
	case Values.Sizing:
		cc.Call = cpu.NewSlow(cpu.CallSizing)
	default:
		return cc, errors.New("cgo ABI structs not supported")
	}
	cc.Rets[0] = Locations.Returned
	return cc, nil
}

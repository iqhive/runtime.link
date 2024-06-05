package jit

import (
	"reflect"
	"unsafe"

	"runtime.link/api/call/internal/bin/std/cpu"
	"runtime.link/xyz"
)

// Value represents an underlying Go value.
type Value struct {
	direct reflect.Value // when sourced from [reflect.MakeFunc]
	locate Location      // information about the hardware location
}

func (val Value) UnsafePointer() Value {
	if val.direct.Kind() == reflect.String {
		return Value{direct: reflect.ValueOf(unsafe.Pointer(unsafe.StringData(val.direct.String())))}
	}
	if val.direct.Kind() == reflect.Slice {
		return Value{direct: reflect.ValueOf(unsafe.Pointer(val.direct.Pointer()))}
	}
	return Value{direct: reflect.ValueOf(val.direct.UnsafePointer())}
}

// Lifetime for a value.
type Lifetime struct {
	direct func() // free the value.
}

// Location of a function's argument or return value.
type Location xyz.Switch[any, struct {
	Physical xyz.Case[Location, HardwareLocation] // value is passed directly.
	Indirect xyz.Case[Location, HardwareLocation] // value is passed indirectly.
	Multiple xyz.Case[Location, []Location]       // multiple values (ie. struct).
}]

func (loc Location) Equals(rhs Location) bool {
	a, b := xyz.ValueOf(loc), xyz.ValueOf(rhs)
	if a != b {
		return false
	}
	switch {
	case a == Locations.Physical:
		return Locations.Physical.Get(loc) == Locations.Physical.Get(rhs)
	case a == Locations.Indirect:
		return Locations.Indirect.Get(loc) == Locations.Indirect.Get(rhs)
	case a == Locations.Multiple:
		for i, loc := range Locations.Multiple.Get(loc) {
			if !loc.Equals(Locations.Multiple.Get(rhs)[i]) {
				return false
			}
		}
	}
	return false
}

var Locations = xyz.AccessorFor(Location.Values)

// HardwareLocation describes the physical and direct location of a value.
type HardwareLocation xyz.Switch[uint64, struct {
	Register xyz.Case[HardwareLocation, cpu.GPR] // standard register.
	Floating xyz.Case[HardwareLocation, cpu.FPR] // standard floating-point register.
	Stack    xyz.Case[HardwareLocation, uintptr] // offset to the parameter on the stack.
}]

var HardwareLocations = xyz.AccessorFor(HardwareLocation.Values)

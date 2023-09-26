/*
Package abi provides an interface to the platform-standard ABI calling
conventions and type system (typically C).

Helpful links:
https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
https://dyncall.org/docs/manual/manualse11.html
https://github.com/ziglang/zig/blob/master/src/arch
*/
package abi

import (
	"reflect"
	"unsafe"

	"runtime.link/lib/internal/cpu"
	"runtime.link/xyz"
)

// Location of a function's argument or return value.
type Location xyz.Switch[any, struct {
	Returned Location                             // value is returned by the call.
	Software xyz.Case[Location, cpu.SlowFunc]     // software location.
	Hardware xyz.Case[Location, HardwareLocation] //physical location.
	Multiple xyz.Case[Location, []Location]       // multiple values passed by value.
	Indirect xyz.Case[Location, IndirectLocation] // value passed by reference.
}]

func (loc Location) Equals(rhs Location) bool {
	a, b := xyz.ValueOf(loc), xyz.ValueOf(rhs)
	if a != b {
		return false
	}
	switch {
	case a == Locations.Hardware.Value:
		return Locations.Hardware.Get(loc) == Locations.Hardware.Get(rhs)
	case a == Locations.Indirect.Value:
		return Locations.Indirect.Get(loc) == Locations.Indirect.Get(rhs)
	case a == Locations.Multiple.Value:
		for i, loc := range Locations.Multiple.Get(loc) {
			if !loc.Equals(Locations.Multiple.Get(rhs)[i]) {
				return false
			}
		}
	}
	return false
}

var Locations = new(Location).Values()

// IndirectLocation describes a value passed by reference.
type IndirectLocation struct {
	Location Location // hardware location of the pointer.
	Relative uintptr  // relative offset to the pointer.
}

// HardwareLocation describes the physical and direct location of a value.
type HardwareLocation xyz.Switch[[8]byte, struct {
	Register xyz.Case[HardwareLocation, cpu.Location] // standard register.
	Floating xyz.Case[HardwareLocation, cpu.Location] // standard floating-point register.
	Software xyz.Case[HardwareLocation, cpu.SlowFunc] // platform-native ABI push/pop.
	Specific xyz.Case[HardwareLocation, cpu.ArchFunc] // architecture-specific hardware location.
	StackRtl xyz.Case[HardwareLocation, uintptr]      // offset to the parameter on a right to left stack.
	StackLtr xyz.Case[HardwareLocation, uintptr]      // offset to the parameter on a left to right stack.
}]

var HardwareLocations = new(HardwareLocation).Values()

// CallingConvention describes the calling convention argument and return value
// locations for a function.
type CallingConvention struct {
	Call cpu.Instruction
	Args []Location
	Rets []Location
}

// Type returns the calling convention locations for the given argument
// and return types.
type Type func(Function) (CallingConvention, error)

// Value represents the fixed-sized value type.
type Value xyz.Switch[any, struct {
	Bytes0 Value
	Bytes1 Value
	Bytes2 Value
	Bytes4 Value
	Bytes8 Value
	Float4 Value
	Float8 Value
	Memory Value // pointer
	Sizing Value
	Struct xyz.Case[Value, []Value]
}]

var Values = new(Value).Values()

// Function describes the low-level fixed-size values for a function.
type Function struct {
	Vars bool // varargs/variadic
	Args []Value
	Rets []Value
}

// FunctionOf returns the [Function] of the given reflect function type.
func FunctionOf(rtype reflect.Type) Function {
	if rtype.Kind() != reflect.Func {
		panic("abi.FunctionOf called with " + rtype.String() + " (not a function)")
	}
	var fn Function
	fn.Args = make([]Value, rtype.NumIn())
	fn.Rets = make([]Value, rtype.NumOut())
	if rtype.IsVariadic() {
		fn.Vars = true
	}
	for i := 0; i < rtype.NumIn(); i++ {
		fn.Args[i] = ValueOf(rtype.In(i))
	}
	for i := 0; i < rtype.NumOut(); i++ {
		fn.Rets[i] = ValueOf(rtype.Out(i))
	}
	return fn
}

func ValueOf(rtype reflect.Type) Value {
	if rtype.Size() == 0 {
		return Values.Bytes0
	}
	var (
		values []Value
	)
	switch rtype.Kind() {
	case reflect.Array:
		for i := 0; i < rtype.Len(); i++ {
			values = append(values, ValueOf(rtype.Elem()))
		}
	case reflect.Struct:
		for i := 0; i < rtype.NumField(); i++ {
			values = append(values, ValueOf(rtype.Field(i).Type))
		}
		return Values.Struct.As(values)
	case reflect.Float32:
		return Values.Float4
	case reflect.Float64:
		return Values.Float8
	case reflect.Complex64:
		values = append(values, Values.Float4, Values.Float4)
		return Values.Struct.As(values)
	case reflect.Complex128:
		values = append(values, Values.Float8, Values.Float8)
		return Values.Struct.As(values)
	case reflect.Slice:
		values = append(values, Values.Memory, Values.Sizing, Values.Sizing)
		return Values.Struct.As(values)
	case reflect.String:
		values = append(values, Values.Memory, Values.Sizing)
		return Values.Struct.As(values)
	case reflect.Interface:
		values = append(values, Values.Memory, Values.Memory)
		return Values.Struct.As(values)
	case reflect.Func, reflect.Chan, reflect.Map:
		return Values.Memory
	case reflect.Pointer, reflect.UnsafePointer:
		return Values.Memory
	default:
		switch rtype.Size() {
		case 1:
			return Values.Bytes1
		case 2:
			return Values.Bytes2
		case 4:
			return Values.Bytes4
		case 8:
			return Values.Bytes8
		}
	}
	panic("abi.valuesOf called with " + rtype.String() + " (not a fixed-size value)")
}

func (val Value) Size() uintptr {
	switch val {
	case Values.Bytes0:
		return 0
	case Values.Bytes1:
		return 1
	case Values.Bytes2:
		return 2
	case Values.Bytes4:
		return 4
	case Values.Bytes8:
		return 8
	case Values.Float4:
		return 4
	case Values.Float8:
		return 8
	case Values.Memory, Values.Sizing:
		return unsafe.Sizeof(uintptr(0))
	default:
		structure := Values.Struct.Get(val)
		var size uintptr
		for _, field := range structure {
			size += field.Size()
		}
		return size
	}
}

func (val Value) Align() uintptr {
	switch val {
	case Values.Bytes0:
		return 1
	case Values.Bytes1:
		return unsafe.Alignof(uint8(0))
	case Values.Bytes2:
		return unsafe.Alignof(uint16(0))
	case Values.Bytes4:
		return unsafe.Alignof(uint32(0))
	case Values.Bytes8:
		return unsafe.Alignof(uint64(0))
	case Values.Float4:
		return unsafe.Alignof(float32(0))
	case Values.Float8:
		return unsafe.Alignof(float64(0))
	case Values.Memory, Values.Sizing:
		return unsafe.Alignof(uintptr(0))
	default:
		structure := Values.Struct.Get(val)
		var align uintptr
		for _, field := range structure {
			align = max(align, field.Align())
		}
		return align
	}
}

func (val Value) Pin(location Location) []cpu.Location {
	switch val {
	case Values.Memory:
		if xyz.ValueOf(location) == Locations.Hardware.Value && xyz.ValueOf(Locations.Hardware.Get(location)) == HardwareLocations.Register.Value {
			return []cpu.Location{cpu.R0 + HardwareLocations.Register.Get(Locations.Hardware.Get(location))}
		}
	default:
		if xyz.ValueOf(val) == Values.Struct.Value {
			var structure = Values.Struct.Get(val)
			var multiple = Locations.Multiple.Get(location)
			var pins []cpu.Location
			for i, field := range structure {
				pins = append(pins, field.Pin(multiple[i])...)
			}
			return pins
		}
	}
	return nil
}

func (val Location) Read() []cpu.Instruction {
	switch xyz.ValueOf(val) {
	case Locations.Multiple.Value:
		var multiple = Locations.Multiple.Get(val)
		switch len(multiple) {
		case 2:
			return append(append(multiple[1].Read(), cpu.NewFunc(cpu.SwapLength)), multiple[0].Read()...)
		}
	case Locations.Hardware.Value:
		physical := Locations.Hardware.Get(val)
		switch xyz.ValueOf(physical) {
		case HardwareLocations.Register.Value:
			register := HardwareLocations.Register.Get(physical)
			return []cpu.Instruction{cpu.NewLoad(cpu.R0 + register)}
		case HardwareLocations.Floating.Value:
			floating := HardwareLocations.Floating.Get(physical)
			return []cpu.Instruction{cpu.NewLoad(cpu.X0 + floating)}
		default:
			panic("abi.Locations.Physical.Read: not implemented for non-register locations")
		}
	}
	panic("abi.Locations.Indirect.Read: not implemented")
}

func (val Location) Send() []cpu.Instruction {
	switch xyz.ValueOf(val) {
	case Locations.Multiple.Value:
		var multiple = Locations.Multiple.Get(val)
		switch len(multiple) {
		case 2:
			return append(append(multiple[0].Send(), cpu.NewFunc(cpu.SwapLength)), multiple[1].Send()...)
		}
	case Locations.Hardware.Value:
		physical := Locations.Hardware.Get(val)
		switch xyz.ValueOf(physical) {
		case HardwareLocations.Register.Value:
			register := HardwareLocations.Register.Get(physical)
			return []cpu.Instruction{cpu.NewMove(cpu.R0 + register)}
		case HardwareLocations.Floating.Value:
			floating := HardwareLocations.Floating.Get(physical)
			return []cpu.Instruction{cpu.NewMove(cpu.X0 + floating)}
		default:
			panic("abi.Locations.Physical.Send: not implemented for non-register locations")
		}
	}
	panic("abi.Locations.Indirect.Send: not implemented")
}

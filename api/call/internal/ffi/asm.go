package ffi

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"

	"runtime.link/api/call/internal/abi"
	"runtime.link/api/call/internal/cpu"
	"runtime.link/api/call/internal/cpu/amd64"
	"runtime.link/api/call/internal/cpu/arm64"
	"runtime.link/api/xray"
)

func functionOf(lookup reflect.Type, foreign Type) abi.Function {
	var fn abi.Function
	fn.Vars = foreign.More
	fn.Args = make([]abi.Value, len(foreign.Args))
	for i, arg := range foreign.Args {
		fn.Args[i] = valueOf(arg)
	}
	if foreign.Func != nil {
		fn.Rets = append(fn.Rets, valueOf(*foreign.Func))
	}
	return fn
}

func check(src *cpu.Program, from, into abi.CallingConvention, ctype Type, assert Assertions, arg Argument) error {
	if arg.Index > 0 {
		if ctype.Free != 0 && assert.Capacity {
			src.Add(
				cpu.NewFunc(cpu.SwapLength),
				cpu.NewFunc(cpu.SwapAssert),
			)
		}
		src.Add(cpu.NewFunc(cpu.SwapAssert))
		src.Add(from.Args[ctype.Args[arg.Index-1].Maps].Read()...)
		if ctype.Free == 0 && assert.Capacity {
			src.Add(cpu.NewFunc(cpu.SwapLength))
		}
		src.Add(cpu.NewFunc(cpu.SwapAssert))
		if ctype.Free == 0 && assert.Capacity {
			src.Add(cpu.NewFunc(cpu.SwapLength))
		}
	} else if arg.Const == "" && arg.Value >= 0 && arg.Value < 32 {
		src.Add(cpu.NewFunc(cpu.SwapAssert))
		src.Add(cpu.NewBits(uint8(arg.Value)))
		src.Add(cpu.NewFunc(cpu.SwapAssert))
	} else {
		return fmt.Errorf("lib.compile currently unsupports constants and literals '%s'", arg.Const)
	}
	return nil
}

func valueOf(foreign Type) abi.Value {
	if foreign.Free != 0 {
		return abi.Values.Memory
	}
	switch kind := Kind(foreign.Name); kind {
	case reflect.Float32:
		return abi.Values.Float4
	case reflect.Float64:
		return abi.Values.Float8
	default:
		switch size := Sizeof(foreign.Name); size {
		case 0:
			panic("unsupported value type " + foreign.Name)
		case 1:
			return abi.Values.Bytes1
		case 2:
			return abi.Values.Bytes2
		case 4:
			return abi.Values.Bytes4
		case 8:
			return abi.Values.Bytes8
		default:
			var values []abi.Value
			for i := uintptr(0); i < size; i++ {
				values = append(values, abi.Values.Bytes1)
			}
			return abi.Values.Struct.As(values)
		}
	}
}

func assert(src *cpu.Program, from, into abi.CallingConvention, ctype Type) (ok bool, err error) {
	var inverted = ctype.Test.Inverted
	if ctype.Test.Indirect != 0 {
		return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "indirect")
	}
	if a := ctype.Test.Equality; a.Check {
		ok = true
		if err := check(src, from, into, ctype, ctype.Test, a); err != nil {
			return false, xray.New(err)
		}
		src.Add(cpu.NewMath(cpu.Same))
	}
	if a := ctype.Test.LessThan; a.Check {
		ok = true
		if err := check(src, from, into, ctype, ctype.Test, a); err != nil {
			return false, xray.New(err)
		}
		src.Add(cpu.NewMath(cpu.Less))
	}
	if a := ctype.Test.MoreThan; a.Check {
		ok = true
		if err := check(src, from, into, ctype, ctype.Test, a); err != nil {
			return false, xray.New(err)
		}
		src.Add(cpu.NewMath(cpu.More))
	}
	if a := ctype.Test.OfFormat; a.Check {
		return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "format")
	}
	if a := ctype.Test.SameType; a.Check {
		return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "type")
	}
	if a := ctype.Test.Lifetime; a.Check {
		return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "lifetime")
	}
	if a := ctype.Test.Overlaps; a.Check {
		return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "overlaps")
	}
	if inverted {
		ok = true
		src.Add(cpu.NewMath(cpu.Flip))
	}
	return
}

func CompileReliably(fn reflect.Type, foreign Type) (src *cpu.Program, err error) {
	abifn := abi.FunctionOf(fn)
	internal, err := abi.Internal(abifn)
	if err != nil {
		return src, xray.New(err)
	}
	external, err := abi.CGO(functionOf(fn, foreign))
	if err != nil {
		return src, xray.New(err)
	}
	return compile(fn, foreign, internal, external)
}

func CompileForSpeed(fn reflect.Type, foreign Type) (*cpu.Program, error) {
	abifn := abi.FunctionOf(fn)
	internal, err := abi.Internal(abifn)
	if err != nil {
		return CompileReliably(fn, foreign)
	}
	var ABI abi.Type
	switch runtime.GOARCH {
	case "arm64":
		ABI = arm64.ABI
	case "amd64":
		ABI = amd64.ABI
	default:
		return CompileReliably(fn, foreign)
	}
	external, err := ABI(functionOf(fn, foreign))
	if err != nil {
		return nil, xray.New(err)
	}
	src, err := compile(fn, foreign, internal, external)
	if err != nil {
		return CompileReliably(fn, foreign)
	}
	return src, nil
}

func compile(fn reflect.Type, foreign Type, internal, external abi.CallingConvention) (src *cpu.Program, err error) {
	src = new(cpu.Program)
	for i, arg := range abi.FunctionOf(fn).Args {
		src.Pin(arg.Pin(internal.Args[i])...)
	}
	for i, into := range foreign.Args {
		from := fn.In(into.Maps - 1)
		read := internal.Args[into.Maps-1]
		send := external.Args[i]
		if read.Equals(send) && into.Free == 0 {
			continue // no translation needed.
		}
		if into.Free == 0 { // value types.
			src.Add(read.Read()...)
			src.Add(send.Send()...)
			continue
		}
		switch from.Kind() {
		case reflect.String:
			if into.Free == '&' {
				src.Add(read.Read()...)
				src.Add(cpu.NewFunc(cpu.StringMake))
				src.Add(send.Send()...)
				continue
			}
		case reflect.Func: // FIXME resources used by function parameters are never freed.
			wrap, err := callback(from, into)
			if err != nil {
				return src, xray.New(err)
			}
			if len(src.Func) >= 255 {
				return src, xray.New(errors.New("too many callbacks"))
			}
			// we are creating a wrapper function that will convert
			// the Go function pointer in the register to an ABI-compatible one.
			src.Add(cpu.NewBits(uint8(len(src.Func))))
			src.Add(cpu.NewFunc(cpu.SwapLength))
			src.Func = append(src.Func, wrap)
			src.Add(read.Read()...)
			src.Add(cpu.NewFunc(cpu.Wrap))
			src.Add(send.Send()...)
			continue
		}
		return src, xray.New(errors.New("only value arguments are supported"))
	}
	src.Add(cpu.NewFunc(cpu.Call))
	if foreign.Func == nil {
		return src, nil
	}
	from := *foreign.Func
	into := fn.Out(0)
	read := external.Rets[0]
	send := internal.Rets[0]
	if read.Equals(send) && foreign.Func.Free == 0 { // value types
		if into.Kind() == reflect.Bool {
			src.Add(read.Read()...)
			src.Add(cpu.NewFunc(cpu.Bool))
			src.Add(send.Send()...)
		}
		return src, nil // no translation needed.
	}
	switch into.Kind() {
	case reflect.Interface:
		if into == reflect.TypeOf([0]error{}).Elem() {
			src.Add(read.Read()...)
			checked, err := assert(src, internal, external, from)
			if err != nil {
				return nil, xray.New(err)
			}
			if checked {
				src.Add(read.Read()...)
			}
			src.Add(cpu.NewFunc(cpu.ErrorMake))
			src.Add(send.Send()...)
			return src, nil
		}
	}
	return src, xray.New(errors.New("only value return types are supported"))
}

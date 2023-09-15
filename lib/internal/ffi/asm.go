package ffi

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"runtime.link/lib/internal/abi"
	"runtime.link/lib/internal/cpu"
	"runtime.link/lib/internal/cpu/arm64"
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
				cpu.Func.New(cpu.SwapLength),
				cpu.Func.New(cpu.SwapAssert),
			)
		}
		src.Add(cpu.Func.New(cpu.SwapAssert))
		src.Add(from.Args[ctype.Args[arg.Index-1].Maps].Read()...)
		if ctype.Free == 0 && assert.Capacity {
			src.Add(cpu.Func.New(cpu.SwapLength))
		}
		src.Add(cpu.Func.New(cpu.SwapAssert))
		if ctype.Free == 0 && assert.Capacity {
			src.Add(cpu.Func.New(cpu.SwapLength))
		}
	} else if arg.Const == "" && arg.Value >= 0 && arg.Value < 32 {
		src.Add(cpu.Func.New(cpu.SwapAssert))
		src.Add(cpu.Bits.New(cpu.Args(arg.Value)))
		src.Add(cpu.Func.New(cpu.SwapAssert))
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
			return false, err
		}
		src.Add(cpu.Math.New(cpu.Same))
	}
	if a := ctype.Test.LessThan; a.Check {
		ok = true
		if err := check(src, from, into, ctype, ctype.Test, a); err != nil {
			return false, err
		}
		src.Add(cpu.Math.New(cpu.Less))
	}
	if a := ctype.Test.MoreThan; a.Check {
		ok = true
		if err := check(src, from, into, ctype, ctype.Test, a); err != nil {
			return false, err
		}
		src.Add(cpu.Math.New(cpu.More))
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
		src.Add(cpu.Math.New(cpu.Flip))
	}
	return
}

func Assemble(fn reflect.Type, foreign Type) (src *cpu.Program, err error) {
	src = new(cpu.Program)

	internal, err := abi.Internal(abi.FunctionOf(fn))
	if err != nil {
		return src, err
	}
	var ABI abi.Type
	switch runtime.GOARCH {
	case "arm64":
		ABI = arm64.ABI
	default:
		return src, errors.New("runtime.link/lib: unsupported architecture " + runtime.GOARCH)
	}
	external, err := ABI(functionOf(fn, foreign))
	if err != nil {
		return src, err
	}
	for i, into := range foreign.Args {
		from := fn.In(i)
		read := internal.Args[into.Maps-1]
		send := external.Args[i]
		if read.Equals(send) && into.Free == 0 {
			continue // no translation needed.
		}
		switch from.Kind() {
		case reflect.String:
			if into.Free == '&' {
				src.Add(read.Read()...)
				src.Add(cpu.Func.New(cpu.StringMake))
				src.Add(send.Send()...)
				continue
			}
		}
		return src, errors.New("only value arguments are supported")
	}
	src.Add(cpu.Func.New(cpu.Call))
	if foreign.Func == nil {
		return src, nil
	}
	from := *foreign.Func
	into := fn.Out(0)
	read := external.Rets[0]
	send := internal.Rets[0]
	if read.Equals(send) && foreign.Func.Free == 0 { // value types
		return src, nil // no translation needed.
	}
	switch into.Kind() {
	case reflect.Interface:
		if into == reflect.TypeOf([0]error{}).Elem() {
			src.Add(read.Read()...)
			checked, err := assert(src, internal, external, from)
			if err != nil {
				return nil, err
			}
			if checked {
				src.Add(read.Read()...)
			}
			src.Add(cpu.Func.New(cpu.ErrorMake))
			src.Add(send.Send()...)
			return src, nil
		}
	}
	return src, errors.New("only value return types are supported")
}

func compile(gtype reflect.Type, ctype Type) (ops []abi.Operation, err error) {
	var gargs int
	var recv func(int) error
	recv = func(garg int) error {
		if garg == gargs {
			var op abi.Operation
			rtype := gtype.In(0)
			kind := rtype.Kind()
			switch kind {
			case reflect.Bool, reflect.Int8, reflect.Uint8:
				op = abi.MoveValByt
			case reflect.Int16, reflect.Uint16:
				op = abi.MoveValU16
			case reflect.Int32, reflect.Uint32:
				op = abi.MoveValU32
			case reflect.Int64, reflect.Uint64:
				op = abi.MoveValU64
			case reflect.Float32:
				op = abi.MoveValF32
			case reflect.Float64:
				op = abi.MoveValF64
			case reflect.Ptr, reflect.UnsafePointer, reflect.Uintptr, reflect.Chan, reflect.Map, reflect.Func:
				op = abi.MoveValPtr
			case reflect.String:
				op = abi.MoveValStr
			case reflect.Slice:
				op = abi.MoveValArr
			case reflect.Interface:
				op = abi.MoveValAny
			case reflect.Struct:
				switch rtype {
				case reflect.TypeOf([0]abi.File{}).Elem():
					op = abi.MoveValPtr
				case reflect.TypeOf([0]time.Time{}).Elem():
					op = abi.MoveValTim
				default:
					return fmt.Errorf("lib.compile.recv unsupported Go type '%s'", kind)
				}
			default:
				return fmt.Errorf("lib.compile.recv unsupported Go type '%s'", kind)
			}
			ops = append(ops, op)
			if kind == reflect.Func {
				ops = append(ops, abi.LoadPtrPtr)
			}
			gargs++
		} else {
			gargs = 0
			ops = append(ops, abi.MoveNewVal)
			for i := 0; i < garg-gargs; i++ {
				if err := recv(garg); err != nil {
					return err
				}
			}
		}
		return nil
	}

	send := func(from reflect.Type, into Type) error {
		switch into.Name {
		case "double":
			switch from.Kind() {
			case reflect.Float32, reflect.Float64:
			default:
				return fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'double'", from)
			}
			ops = append(ops, abi.CopyValF64)
		case "int", "long":
			switch from.Kind() {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			default:
				return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'int'", from)
			}
			ops = append(ops, abi.CopyValU64)
		case "unsigned_int":
			switch from.Kind() {
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			default:
				return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'int'", from)
			}
			ops = append(ops, abi.CopyValU64)
		case "time_t":
			switch from.Kind() {
			case reflect.Int64:
			default:
				if from != reflect.TypeOf(time.Time{}) {
					return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'time_t'", from)
				}
				ops = append(ops, abi.UnixTiming)
			}
			ops = append(ops, abi.CopyValU64)
		default:
			return fmt.Errorf("lib.compile unsupported ABI type '%s'", into.Name)
		}
		return nil
	}

	for _, into := range ctype.Args {
		garg := into.Maps - 1
		from := gtype.In(garg)
		if err := recv(garg); err != nil {
			return nil, err
		}
		if into.Free == 0 {
			if err := send(from, into); err != nil {
				return nil, err
			}
		} else {
			if into.Free == '$' || (into.Free == '&' && from.Kind() == reflect.String && !into.Hash) {
				ops = append(ops, abi.MakeMemory)
				switch from.Kind() {
				case reflect.String:
					ops = append(ops, abi.CopyValStr)
				default:
					if err := send(from, into); err != nil {
						return nil, err
					}
				}
				ops = append(ops, abi.DoneMemory)
			} else if into.Free == '&' {
				ops = append(ops, abi.KeepMemory)
			}

			switch into.Name {
			case "int", "double":
				ops = append(ops, abi.CopyValPtr)
			case "FILE":
				if from != reflect.TypeOf(abi.File{}) {
					return nil, fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'FILE'", from)
				}
				ops = append(ops, abi.LoadPtrPtr)
				ops = append(ops, abi.CopyValPtr)
			case "char":
				if from.Kind() != reflect.String {
					return nil, fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'char'", from)
				}
				ops = append(ops, abi.NullString)
				ops = append(ops, abi.CopyValPtr)
			case "fpos_t":
				if from != reflect.TypeOf(abi.FilePosition{}) {
					return nil, fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'fpos_t'", from)
				}
				ops = append(ops, abi.LoadPtrPtr)
				ops = append(ops, abi.CopyValPtr)
			default:
				return nil, fmt.Errorf("lib.compile currently unsupported ABI type '%s%s'", string(into.Free), into.Name)
			}
		}
	}

	ops = append(ops, abi.JumpToCall)

	check := func(assert Assertions, arg Argument) error {
		if arg.Index > 0 {
			if ctype.Free != 0 && assert.Capacity {
				ops = append(ops, abi.SwapLength)
				ops = append(ops, abi.SwapAssert)
			}
			ops = append(ops, abi.SwapAssert)
			if err := recv(int(arg.Index) - 1); err != nil {
				return err
			}
			if ctype.Free == 0 && assert.Capacity {
				ops = append(ops, abi.SwapLength)
			}
			ops = append(ops, abi.SwapAssert)
			if ctype.Free == 0 && assert.Capacity {
				ops = append(ops, abi.SwapLength)
			}
		} else if arg.Const == "" && arg.Value == 0 {
			ops = append(ops, abi.SwapAssert)
			ops = append(ops, abi.NormalSet0)
			ops = append(ops, abi.SwapAssert)
		} else if arg.Const == "" && arg.Value == 1 {
			ops = append(ops, abi.SwapAssert)
			ops = append(ops, abi.NormalSet1)
			ops = append(ops, abi.SwapAssert)
		} else {
			return fmt.Errorf("lib.compile currently unsupports constants and literals '%s'", arg.Const)
		}
		return nil
	}

	// assert the normal register with the requested assertions.
	assert := func(ctype Type) (ok bool, err error) {
		var inverted = ctype.Test.Inverted
		if ctype.Test.Indirect != 0 {
			return false, fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", "indirect")
		}
		if a := ctype.Test.Equality; a.Check {
			ok = true
			if err := check(ctype.Test, a); err != nil {
				return false, err
			}
			ops = append(ops, abi.AssertSame)
		}
		if a := ctype.Test.LessThan; a.Check {
			ok = true
			if err := check(ctype.Test, a); err != nil {
				return false, err
			}
			ops = append(ops, abi.AssertLess)
		}
		if a := ctype.Test.MoreThan; a.Check {
			ok = true
			if err := check(ctype.Test, a); err != nil {
				return false, err
			}
			ops = append(ops, abi.AssertMore)
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
			ops = append(ops, abi.AssertFlip)
		}
		return
	}

	switch gtype.NumOut() {
	case 0:
		break
	case 1:
		if ctype.Func == nil {
			return nil, fmt.Errorf("lib.compile Go function returns a value but ABI doesn't")
		}
		ctype := *ctype.Func
		rtype := gtype.Out(0)
		kind := gtype.Out(0).Kind()
		switch ctype.Name {
		case "char":
			if ctype.Free != 0 {
				if kind != reflect.String {
					return nil, fmt.Errorf("lib.compile cannot return ABI 'char' as Go '%s'", rtype)
				}
				ops = append(ops, abi.CopyValPtr)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.CopyNewVal, abi.CopyValPtr)
				}
				ops = append(ops, abi.SizeString)
				ops = append(ops, abi.MoveValStr)
			} else {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'char' as Go '%s'", rtype)
			}
		case "double":
			if kind != reflect.Float64 && kind != reflect.Float32 {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'double' as Go '%s'", rtype)
			}
			ops = append(ops, abi.CopyValF64)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.CopyNewVal, abi.CopyValF64)
			}
			ops = append(ops, abi.MoveValF64)
		case "int", "long":
			switch kind {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int, reflect.Bool:
				ops = append(ops, abi.CopyValU64)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.CopyNewVal, abi.CopyValU64)
				}
				ops = append(ops, abi.MoveValU64)
			case reflect.Interface:
				if rtype != reflect.TypeOf([0]error{}).Elem() {
					return nil, fmt.Errorf("lib.compile cannot return ABI 'int' as Go '%s'", rtype)
				}
				ops = append(ops, abi.CopyValU64)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if !checked {
					return nil, fmt.Errorf("lib.compile error result requires assertion(s) '%s'", rtype)
				}
				ops = append(ops, abi.CopyNewVal, abi.CopyValU64)
				ops = append(ops, abi.MoveValErr)
			default:
				return nil, fmt.Errorf("lib.compile cannot return ABI 'int' as Go '%s'", rtype)
			}

		case "fpos_t":
			if rtype != reflect.TypeOf(abi.FilePosition{}) || ctype.Free == 0 {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'FILE' as Go '%s'", rtype)
			}
			ops = append(ops, abi.CopyValPtr)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.CopyNewVal, abi.CopyValPtr)
			}
			ops = append(ops, abi.MakePtrPtr)
			ops = append(ops, abi.MoveValPtr)

		case "FILE":
			if rtype != reflect.TypeOf(abi.File{}) || ctype.Free == 0 {
				return nil, fmt.Errorf("lib.compile cannot return ABI '%s' as Go '%s'", string(ctype.Free)+ctype.Func.Name, rtype)
			}
			ops = append(ops, abi.CopyValPtr)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.CopyNewVal, abi.CopyValPtr)
			}
			ops = append(ops, abi.MakePtrPtr)
			ops = append(ops, abi.MoveValPtr)
		case "time_t":
			ops = append(ops, abi.CopyValU64)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.CopyNewVal, abi.CopyValU64)
			}
			switch kind {
			case reflect.Int64:
				ops = append(ops, abi.MoveValU64)
			case reflect.Struct:
				if rtype != reflect.TypeOf(time.Time{}) {
					return nil, fmt.Errorf("lib.compile return pass ABI 'time_t' as Go '%s'", rtype)
				}
				ops = append(ops, abi.UnixTiming)
				ops = append(ops, abi.MoveValTim)
			default:
				return nil, fmt.Errorf("lib.compile return pass ABI 'time_t' as Go '%s'", rtype)
			}
		default:
			return nil, fmt.Errorf("lib.compile unsupported ABI function return type '%s'", ctype.Name)
		}
	case 2:
		return nil, fmt.Errorf("lib.compile currently unsupported Go function return count '%d'", gtype.NumOut())
	default:
		return nil, fmt.Errorf("lib.compile currently unsupported Go function return count '%d'", gtype.NumOut())
	}
	return ops, nil
}

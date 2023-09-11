package lib

import (
	"fmt"
	"reflect"
	"time"

	"runtime.link/std/abi"
)

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
				op = abi.RecvArgByt
			case reflect.Int16, reflect.Uint16:
				op = abi.RecvArgU16
			case reflect.Int32, reflect.Uint32:
				op = abi.RecvArgU32
			case reflect.Int64, reflect.Uint64:
				op = abi.RecvArgU64
			case reflect.Float32:
				op = abi.RecvArgF32
			case reflect.Float64:
				op = abi.RecvArgF64
			case reflect.Ptr, reflect.UnsafePointer, reflect.Uintptr, reflect.Chan, reflect.Map, reflect.Func:
				op = abi.RecvArgPtr
			case reflect.String:
				op = abi.RecvArgStr
			case reflect.Slice:
				op = abi.RecvArgArr
			case reflect.Interface:
				op = abi.RecvArgAny
			case reflect.Struct:
				switch rtype {
				case reflect.TypeOf([0]abi.File{}).Elem():
					op = abi.RecvArgPtr
				case reflect.TypeOf([0]time.Time{}).Elem():
					op = abi.RecvArgTim
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
			ops = append(ops, abi.RecvArgNew)
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
			ops = append(ops, abi.SendArgF64)
		case "int", "long":
			switch from.Kind() {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			default:
				return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'int'", from)
			}
			ops = append(ops, abi.SendArgU64)
		case "unsigned_int":
			switch from.Kind() {
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			default:
				return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'int'", from)
			}
			ops = append(ops, abi.SendArgU64)
		case "time_t":
			switch from.Kind() {
			case reflect.Int64:
			default:
				if from != reflect.TypeOf(time.Time{}) {
					return fmt.Errorf("lib.compile cannot pass Go '%s' to ABI 'time_t'", from)
				}
				ops = append(ops, abi.UnixTiming)
			}
			ops = append(ops, abi.SendArgU64)
		default:
			return fmt.Errorf("lib.compile unsupported ABI type '%s'", into.Name)
		}
		return nil
	}

	for _, into := range ctype.Args {
		garg := into.Maps - 1
		if err := recv(garg); err != nil {
			return nil, err
		}
		from := gtype.In(garg)
		if into.Free == 0 {
			if err := send(from, into); err != nil {
				return nil, err
			}
		} else {
			switch into.Name {
			case "int", "double":
				switch into.Free {
				case '$':
					ops = append(ops, abi.Copy)
					if err := send(from, into); err != nil {
						return nil, err
					}
					ops = append(ops, abi.Done)
				case '&':
					ops = append(ops, abi.Keep)
				default:
					return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
				}
			case "FILE":
				if from != reflect.TypeOf(abi.File{}) {
					return nil, fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'FILE'", from)
				}
				switch into.Free {
				case '$':
					ops = append(ops, abi.Free)
				case '&':
					ops = append(ops, abi.Keep)
				default:
					return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
				}
				ops = append(ops, abi.LoadPtrPtr)
				ops = append(ops, abi.SendArgPtr)
			case "fpos_t":
				if from != reflect.TypeOf(abi.FilePosition{}) {
					return nil, fmt.Errorf("lib.compile cannot pass Go '%s' as ABI 'fpos_t'", from)
				}
				switch into.Free {
				case '$':
					ops = append(ops, abi.Free)
				case '&':
					ops = append(ops, abi.Keep)
				default:
					return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
				}
				ops = append(ops, abi.LoadPtrPtr)
				ops = append(ops, abi.SendArgPtr)
			case "char":
				switch from.Kind() {
				case reflect.String:
					switch into.Free {
					case '$':
						ops = append(ops, abi.Copy)
						ops = append(ops, abi.CopyMemory)
						ops = append(ops, abi.Done)
					case '&':
						if into.Hash {
							ops = append(ops, abi.Keep)
						} else {
							ops = append(ops, abi.CopyMemory)
						}
					default:
						return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
					}
				default:
					switch from {
					case reflect.TypeOf(([]uint8)(nil)):
						switch into.Free {
						case '$':
							ops = append(ops, abi.Copy)
							ops = append(ops, abi.CopyMemory)
							ops = append(ops, abi.Done)
						case '&':
							ops = append(ops, abi.Keep)
						default:
							return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
						}
					case reflect.TypeOf(abi.String{}):
						ops = append(ops, abi.LoadPtrPtr)
						switch into.Free {
						case '$':
							ops = append(ops, abi.Free)
						case '&':
							ops = append(ops, abi.Keep)
						default:
							return nil, fmt.Errorf("lib.compile unsupported ABI free type for %s '%s'", from, string(into.Free))
						}
					default:
						return nil, fmt.Errorf("lib.compile currently unsupported Go type '%s' for ABI type '%s%s'", from, string(into.Free), into.Name)
					}
				}
			default:
				return nil, fmt.Errorf("lib.compile currently unsupported ABI type '%s%s'", string(into.Free), into.Name)
			}
		}
	}

	ops = append(ops, abi.Call)

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
			check(ctype.Test, a)
			ops = append(ops, abi.AssertSame)
		}
		if a := ctype.Test.LessThan; a.Check {
			ok = true
			check(ctype.Test, a)
			ops = append(ops, abi.AssertLess)
		}
		if a := ctype.Test.MoreThan; a.Check {
			ok = true
			check(ctype.Test, a)
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
				ops = append(ops, abi.RecvRetPtr)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.Recv, abi.RecvRetPtr)
				}
				ops = append(ops, abi.SizeString)
				ops = append(ops, abi.SendRetStr)
			} else {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'char' as Go '%s'", rtype)
			}
		case "double":
			if kind != reflect.Float64 && kind != reflect.Float32 {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'double' as Go '%s'", rtype)
			}
			ops = append(ops, abi.RecvRetF64)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.Recv, abi.RecvRetF64)
			}
			ops = append(ops, abi.SendRetF64)
		case "int", "long":
			switch kind {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int, reflect.Bool:
				ops = append(ops, abi.RecvRetU64)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.Recv, abi.RecvRetU64)
				}
				ops = append(ops, abi.SendRetU64)
			case reflect.Interface:
				if rtype != reflect.TypeOf([0]error{}).Elem() {
					return nil, fmt.Errorf("lib.compile cannot return ABI 'int' as Go '%s'", rtype)
				}
			default:
				return nil, fmt.Errorf("lib.compile cannot return ABI 'int' as Go '%s'", rtype)
			}

		case "fpos_t":
			if rtype != reflect.TypeOf(abi.FilePosition{}) || ctype.Free == 0 {
				return nil, fmt.Errorf("lib.compile cannot return ABI 'FILE' as Go '%s'", rtype)
			}
			ops = append(ops, abi.RecvRetPtr)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.Recv, abi.RecvRetPtr)
			}
			ops = append(ops, abi.MakePtrPtr)
			ops = append(ops, abi.SendRetPtr)

		case "FILE":
			if rtype != reflect.TypeOf(abi.File{}) || ctype.Free == 0 {
				return nil, fmt.Errorf("lib.compile cannot return ABI '%s' as Go '%s'", string(ctype.Free)+ctype.Func.Name, rtype)
			}
			ops = append(ops, abi.RecvRetPtr)
			checked, err := assert(ctype)
			if err != nil {
				return nil, err
			}
			if checked {
				ops = append(ops, abi.Recv, abi.RecvRetPtr)
			}
			ops = append(ops, abi.MakePtrPtr)
			ops = append(ops, abi.SendRetPtr)
		case "time_t":
			switch kind {
			case reflect.Int64:
				ops = append(ops, abi.RecvRetU64)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.Recv, abi.RecvRetU64)
				}
				ops = append(ops, abi.SendRetU64)
			case reflect.Struct:
				if rtype != reflect.TypeOf(time.Time{}) {
					return nil, fmt.Errorf("lib.compile return pass ABI 'time_t' as Go '%s'", rtype)
				}
				ops = append(ops, abi.RecvRetU64)
				checked, err := assert(ctype)
				if err != nil {
					return nil, err
				}
				if checked {
					ops = append(ops, abi.Recv, abi.RecvRetU64)
				}
				ops = append(ops, abi.UnixTiming)
				ops = append(ops, abi.SendRetTim)
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

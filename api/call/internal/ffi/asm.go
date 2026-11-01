package ffi

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"

	"github.com/iqhive/runtime.link/api/call/internal/abi"
	"github.com/iqhive/runtime.link/api/call/internal/cpu"
	"github.com/iqhive/runtime.link/api/call/internal/cpu/amd64"
	"github.com/iqhive/runtime.link/api/call/internal/cpu/arm64"
	"github.com/iqhive/runtime.link/api/xray"
)

func functionOf(lookup reflect.Type, foreign Type) (abi.Function, error) {
	var fn abi.Function
	fn.Vars = foreign.More
	fn.Args = make([]abi.Value, len(foreign.Args))
	for i, arg := range foreign.Args {
		value, err := valueOf(arg)
		if err != nil {
			return fn, err
		}
		fn.Args[i] = value
	}
	if returnsValue(foreign) {
		value, err := valueOf(*foreign.Func)
		if err != nil {
			return fn, err
		}
		fn.Rets = append(fn.Rets, value)
	}
	return fn, nil
}

func returnsValue(foreign Type) bool {
	return foreign.Func != nil && (foreign.Func.Name != "void" || foreign.Func.Free != 0)
}

func valueOf(foreign Type) (abi.Value, error) {
	if foreign.Free != 0 || foreign.Name == "func" {
		return abi.Values.Memory, nil
	}
	switch kind := Kind(foreign.Name); kind {
	case reflect.Float32:
		return abi.Values.Float4, nil
	case reflect.Float64:
		return abi.Values.Float8, nil
	case reflect.Uint8, reflect.Uint16:
		// C promotes these to int. Passing one as such zero-extends it, where the
		// dyncall VM takes the smaller sizes as signed, and sign-extends them.
		return abi.Values.Bytes4, nil
	default:
		switch size := Sizeof(foreign.Name); size {
		case 0:
			return abi.Value{}, errors.New("unsupported value type " + foreign.Name)
		case 1:
			return abi.Values.Bytes1, nil
		case 2:
			return abi.Values.Bytes2, nil
		case 4:
			return abi.Values.Bytes4, nil
		case 8:
			return abi.Values.Bytes8, nil
		default:
			var values []abi.Value
			for i := uintptr(0); i < size; i++ {
				values = append(values, abi.Values.Bytes1)
			}
			return abi.Values.Struct.As(values), nil
		}
	}
}

func isFloat(kind reflect.Kind) bool {
	return kind == reflect.Float32 || kind == reflect.Float64
}

func isSigned(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

// widened returns the instructions that fill a register with a narrower
// value of the given kind, as neither Go nor C defines the rest of the register.
func widened(kind reflect.Kind) []cpu.Instruction {
	var cast cpu.CastFunc
	switch kind {
	case reflect.Int8:
		cast = cpu.SignExtend8
	case reflect.Int16:
		cast = cpu.SignExtend16
	case reflect.Int32:
		cast = cpu.SignExtend32
	case reflect.Bool, reflect.Uint8:
		cast = cpu.ZeroExtend8
	case reflect.Uint16:
		cast = cpu.ZeroExtend16
	case reflect.Uint32:
		cast = cpu.ZeroExtend32
	default:
		return nil
	}
	return []cpu.Instruction{cpu.NewCast(cast)}
}

// floatCast returns the instructions that convert between float sizes.
func floatCast(from, into reflect.Kind) []cpu.Instruction {
	switch {
	case from == reflect.Float32 && into == reflect.Float64:
		return []cpu.Instruction{cpu.NewCast(cpu.FloatToDouble)}
	case from == reflect.Float64 && into == reflect.Float32:
		return []cpu.Instruction{cpu.NewCast(cpu.DoubleToFloat)}
	}
	return nil
}

// argumentCast returns the instructions that turn a Go value into a value of
// a C type. C expects a type narrower than int to have been promoted to one.
func argumentCast(from reflect.Kind, into Type) ([]cpu.Instruction, error) {
	kind := Kind(into.Name)
	if isFloat(from) != isFloat(kind) {
		return nil, fmt.Errorf("unsupported conversion from %s to %s", from, into.Name)
	}
	if isFloat(from) {
		return floatCast(from, kind), nil
	}
	casts := widened(from)
	if kind != from && Sizeof(into.Name) < 4 {
		casts = append(casts, widened(kind)...)
	}
	return casts, nil
}

// resultCast returns the instructions that turn a C result into a Go value.
func resultCast(from Type, into reflect.Kind) ([]cpu.Instruction, error) {
	kind := Kind(from.Name)
	if from.Free != 0 {
		kind = reflect.Uintptr
	}
	if isFloat(kind) != isFloat(into) {
		return nil, fmt.Errorf("unsupported conversion from %s to %s", from.Name, into)
	}
	if isFloat(kind) {
		return floatCast(kind, into), nil
	}
	casts := widened(kind)
	if into == reflect.Bool {
		casts = append(casts, cpu.NewFunc(cpu.Bool))
	}
	return casts, nil
}

// constantOf returns the integer that an assertion argument
// refers to, as it would appear in a register.
func constantOf(arg Argument) (uint, error) {
	if arg.Const == "" {
		return uint(arg.Value), nil
	}
	text := Const(arg.Const)
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return uint(value), nil
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("lib.compile needs an integer constant, not '%s'", arg.Const)
	}
	return uint(value), nil
}

// unsupportedAssertion returns the name of the first assertion that a [cpu.Program]
// cannot check, one that's dropped would look like it had been checked.
func unsupportedAssertion(test Assertions) string {
	switch {
	case test.Capacity:
		return "capacity"
	case test.Indirect != 0:
		return "indirect"
	case test.OfFormat.Check:
		return "format"
	case test.SameType.Check:
		return "type"
	case test.Lifetime.Check:
		return "lifetime"
	case test.Overlaps.Check:
		return "overlaps"
	}
	return ""
}

func hasAssertion(test Assertions) bool {
	return unsupportedAssertion(test) != "" || test.Inverted ||
		test.Equality.Check || test.LessThan.Check || test.MoreThan.Check
}

// compared returns which side of the comparison an assertion
// refers to, and the instructions that write the outcome to $result.
func compared(test Assertions, kind reflect.Kind) (Argument, []cpu.Instruction, error) {
	less, more := cpu.NewMath(cpu.Less), cpu.NewMath(cpu.More)
	if isSigned(kind) {
		less, more = cpu.NewMath(cpu.Lessi), cpu.NewMath(cpu.Morei)
	}
	flip := cpu.NewMath(cpu.Flip)
	var (
		arg Argument
		ops []cpu.Instruction
	)
	switch orEqual := test.Equality.Check; {
	case test.LessThan.Check && orEqual:
		arg, ops = test.LessThan, []cpu.Instruction{more, flip}
	case test.MoreThan.Check && orEqual:
		arg, ops = test.MoreThan, []cpu.Instruction{less, flip}
	case test.LessThan.Check:
		arg, ops = test.LessThan, []cpu.Instruction{less}
	case test.MoreThan.Check:
		arg, ops = test.MoreThan, []cpu.Instruction{more}
	case orEqual:
		arg, ops = test.Equality, []cpu.Instruction{cpu.NewMath(cpu.Same)}
	default:
		return arg, nil, errors.New("expected a comparison")
	}
	if test.Inverted {
		ops = append(ops, flip)
	}
	return arg, ops, nil
}

func CompileReliably(fn reflect.Type, foreign Type) (src *cpu.Program, err error) {
	abifn := abi.FunctionOf(fn)
	internal, err := abi.Internal(abifn)
	if err != nil {
		return src, xray.New(err)
	}
	function, err := functionOf(fn, foreign)
	if err != nil {
		return src, xray.New(err)
	}
	external, err := abi.CGO(function)
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
	function, err := functionOf(fn, foreign)
	if err != nil {
		return nil, xray.New(err)
	}
	external, err := ABI(function)
	if err != nil {
		return CompileReliably(fn, foreign)
	}
	src, err := compile(fn, foreign, internal, external)
	if err != nil {
		return CompileReliably(fn, foreign)
	}
	return src, nil
}

// add the instructions, unless reading them has already failed.
func add(src *cpu.Program, instructions []cpu.Instruction, err error) error {
	if err == nil {
		src.Add(instructions...)
	}
	return err
}

func compile(fn reflect.Type, foreign Type, internal, external abi.CallingConvention) (src *cpu.Program, err error) {
	src = new(cpu.Program)
	if fn.NumOut() > 1 {
		return src, xray.New(errors.New("lib.compile only supports a single result"))
	}
	if foreign.Call.Name != "" {
		return src, xray.New(errors.New("lib.compile doesn't support calling a symbol on failure"))
	}
	for i, arg := range abi.FunctionOf(fn).Args {
		src.Pin(arg.Pin(internal.Args[i])...)
	}
	for i, into := range foreign.Args {
		if into.Maps == 0 {
			return src, xray.New(errors.New("lib.compile doesn't support inferred arguments"))
		}
		if err := compileArgument(src, fn.In(into.Maps-1), into, internal.Args[into.Maps-1], external.Args[i], external.Call == 0); err != nil {
			return src, err
		}
	}
	wantsError := fn.NumOut() == 1 && fn.Out(0) == reflect.TypeOf([0]error{}).Elem()
	check := Assertions{}
	if returnsValue(foreign) {
		check = foreign.Func.Test
	}
	if hasAssertion(check) && !wantsError {
		return src, xray.New(errors.New("lib.compile only supports assertions on a result that is an error"))
	}
	if wantsError {
		if err := compileOperand(src, fn, foreign, internal, check); err != nil {
			return src, xray.New(err)
		}
	}
	switch external.Call {
	case 0:
		src.Add(cpu.NewFunc(cpu.Call))
	default:
		src.Add(external.Call)
	}
	if fn.NumOut() == 0 {
		return src, nil
	}
	if !returnsValue(foreign) {
		return src, xray.New(errors.New("lib.compile needs a C result for the Go function to return"))
	}
	read, send := external.Rets[0], internal.Rets[0]
	if wantsError {
		return src, compileError(src, *foreign.Func, read, send)
	}
	if foreign.Func.Free != 0 {
		return src, xray.New(errors.New("only value return types are supported"))
	}
	casts, err := resultCast(*foreign.Func, fn.Out(0).Kind())
	if err != nil {
		return src, xray.New(err)
	}
	if read.Equals(send) && len(casts) == 0 {
		return src, nil // no translation needed.
	}
	instructions, err := read.Read()
	if err := add(src, instructions, err); err != nil {
		return src, xray.New(err)
	}
	src.Add(casts...)
	instructions, err = send.Send()
	return src, add(src, instructions, err)
}

// compileArgument is told if the call will be direct: without going through cgo,
// foreign code can't call back into Go, as the runtime doesn't know it left.
func compileArgument(src *cpu.Program, from reflect.Type, into Type, read, send abi.Location, direct bool) error {
	if hasAssertion(into.Test) {
		return xray.New(errors.New("lib.compile doesn't support assertions on arguments"))
	}
	if into.Free == 0 && into.Name != "func" { // value types.
		casts, err := argumentCast(from.Kind(), into)
		if err != nil {
			return xray.New(err)
		}
		if read.Equals(send) && len(casts) == 0 {
			return nil // no translation needed.
		}
		instructions, err := read.Read()
		if err := add(src, instructions, err); err != nil {
			return xray.New(err)
		}
		src.Add(casts...)
		instructions, err = send.Send()
		return add(src, instructions, err)
	}
	switch from.Kind() {
	case reflect.String:
		if into.Free == '&' {
			instructions, err := read.Read()
			if err := add(src, instructions, err); err != nil {
				return xray.New(err)
			}
			src.Add(cpu.NewFunc(cpu.StringMake))
			instructions, err = send.Send()
			return add(src, instructions, err)
		}
	case reflect.Func: // FIXME resources used by function parameters are never freed.
		if direct {
			return xray.New(errors.New("lib.compile cannot pass a Go function to a direct call"))
		}
		wrap, err := callback(from, into)
		if err != nil {
			return xray.New(err)
		}
		if len(src.Func) >= 32 {
			return xray.New(errors.New("too many callbacks"))
		}
		// we are creating a wrapper function that will convert
		// the Go function pointer in the register to an ABI-compatible one.
		src.Add(cpu.NewBits(uint8(len(src.Func))))
		src.Add(cpu.NewFunc(cpu.SwapLength))
		src.Func = append(src.Func, wrap)
		instructions, err := read.Read()
		if err := add(src, instructions, err); err != nil {
			return xray.New(err)
		}
		src.Add(cpu.NewFunc(cpu.Wrap))
		instructions, err = send.Send()
		return add(src, instructions, err)
	}
	return xray.New(errors.New("only value arguments are supported"))
}

// compileOperand puts what the result is to be compared with into $assert. It's
// done ahead of the call, as a call leaves nothing of the Go arguments behind,
// though it doesn't touch $assert.
func compileOperand(src *cpu.Program, fn reflect.Type, foreign Type, internal abi.CallingConvention, test Assertions) error {
	if name := unsupportedAssertion(test); name != "" {
		return fmt.Errorf("lib.compile currently unsupported ABI assertion '%s'", name)
	}
	var arg Argument
	switch {
	case !hasAssertion(test):
		if foreign.Func != nil && foreign.Func.Name == "bool" && foreign.Func.Free == 0 {
			arg.Value = 1 // a C function returning a bool says whether it succeeded.
		}
	default:
		var err error
		if arg, _, err = compared(test, reflect.Invalid); err != nil {
			return err
		}
	}
	if arg.Index == 0 {
		value, err := constantOf(arg)
		if err != nil {
			return err
		}
		src.Add(cpu.NewBits(uint8(len(src.Data))), cpu.NewFunc(cpu.Data), cpu.NewFunc(cpu.SwapAssert))
		src.Data = append(src.Data, value)
		return nil
	}
	if int(arg.Index) > len(foreign.Args) || foreign.Args[arg.Index-1].Maps == 0 {
		return fmt.Errorf("lib.compile cannot refer to argument %d", arg.Index)
	}
	index := foreign.Args[arg.Index-1].Maps - 1
	instructions, err := internal.Args[index].Read()
	if err := add(src, instructions, err); err != nil {
		return err
	}
	src.Add(widened(fn.In(index).Kind())...)
	src.Add(cpu.NewFunc(cpu.SwapAssert))
	return nil
}

// compileError turns the C result in read into the Go error in send, which is nil when
// the result meets the comparison with $assert, by default that it's equal.
func compileError(src *cpu.Program, from Type, read, send abi.Location) error {
	kind := Kind(from.Name)
	if from.Free != 0 {
		kind = reflect.Uintptr
	}
	if isFloat(kind) {
		return xray.New(errors.New("lib.compile cannot turn a float into an error"))
	}
	instructions, err := read.Read()
	if err := add(src, instructions, err); err != nil {
		return xray.New(err)
	}
	src.Add(widened(kind)...)
	switch {
	case hasAssertion(from.Test):
		_, ops, err := compared(from.Test, kind)
		if err != nil {
			return xray.New(err)
		}
		src.Add(ops...)
	case from.Free != 0: // a NULL pointer is what a failure looks like.
		src.Add(cpu.NewMath(cpu.Diff))
	default:
		src.Add(cpu.NewMath(cpu.Same))
	}
	src.Add(cpu.NewFunc(cpu.ErrorMake))
	instructions, err = send.Send()
	return add(src, instructions, err)
}

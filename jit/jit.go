/*
Package jit provides a safe interface for generating executable code at runtime.
*/
package jit

import (
	"reflect"
	"unsafe"

	"runtime.link/cpu"
)

// Make a new function of type 'T' using the given JIT implementation
// function. The implementation function must have the JIT equivalent for
// each argument and return value. The Implementation function may be
// called each time the JIT function is called, in order to compile it,
// or it may only be called once. Therefore the behaviour of the
// implementation should not depend on any side effects or mutability.
func Make[T any](src **Program, impl any) T {
	return MakeFunc(reflect.TypeOf([0]T{}).Elem(), src, impl).Interface().(T)
}

// MakeFunc is like [Make], but it can be used to create a function value
// from a [reflect.Type] instead of one known at compile time.
func MakeFunc(ftype reflect.Type, src **Program, impl any) reflect.Value {
	rtype := reflect.TypeOf(impl)
	value := reflect.ValueOf(impl)
	if ftype.Kind() != reflect.Func || rtype.Kind() != reflect.Func {
		panic("jit: MakeFunc called with non-func type")
	}
	if !(*src).CompileOnce && isDirect(ftype, rtype) {
		*src = nil
		copy := reflect.New(rtype).Elem()
		copy.Set(value)
		ptr := copy.Addr().UnsafePointer()
		return reflect.NewAt(ftype, ptr).Elem()
	}

	args := make([]reflect.Value, rtype.NumIn())
	for i := range args {
		args[i] = reflect.New(rtype.In(i)).Elem()
		equivalent, ok := args[i].Addr().Interface().(reader)
		if !ok {
			panic("jit: MakeFunc called with non-jit type " + rtype.In(i).String())
		}
		equivalent.read(*src)
	}
	rets := value.Call(args)
	for i := range rets {
		equivalent, ok := rets[i].Interface().(sender)
		if !ok {
			panic("jit: MakeFunc called with non-jit type " + rtype.Out(i).String())
		}
		equivalent.send(*src)
	}

	code, err := (*src).compile()
	if err != nil {
		return reflect.MakeFunc(ftype, func([]reflect.Value) []reflect.Value {
			panic(err)
		})
	}

	code, err = compile(code)
	if err != nil {
		return reflect.MakeFunc(ftype, func([]reflect.Value) []reflect.Value {
			panic(err)
		})
	}

	main := &code[0]
	jump := &main

	return reflect.NewAt(ftype, unsafe.Pointer(&jump)).Elem()
}

type Program struct {
	CompileOnce bool

	asm  cpu.InstructionSet
	code []op
	gprs []cpu.GPR
	fprs []cpu.FPR
}

type (
	gpr uint
	fpr float64

	gprIndex int32
	fprIndex int32
)

type reader interface {
	read(*Program)
}

func (r *gpr) read(src *Program) { *r = gpr(src.gpr()) }

type sender interface {
	send(*Program)
}

func (r gpr) send(src *Program) {
	src.gprs[r] = 0
	return
	src.code = append(src.code, ops.Mov.As(opMov{
		dst: 0,
		src: gprIndex(r),
	}))
}

// gpr allocates and returns a new general purpose register
// for use.
func (src *Program) gpr() gprIndex {
	gpr := gprIndex(len(src.gprs))
	src.gprs = append(src.gprs, cpu.GPR(gpr))
	return gpr
}

func (src *Program) mapGPR(val gprValue) gprIndex {
	return gprIndex(gprValue(val).gpr)
}

type register struct {
	index int
}

type gprValue = struct{ gpr }

type fprValue = struct{ fpr }

type ptrValue[T any] struct {
	isptr [0]*T
	value unsafe.Pointer
}

// JIT equivalents to all Go types.
type (
	Bool      struct{ gpr }
	Int       struct{ gpr }
	Int8      struct{ gpr }
	Int16     struct{ gpr }
	Int32     struct{ gpr }
	Int64     struct{ gpr }
	Uint      struct{ gpr }
	Uint8     struct{ gpr }
	Uint16    struct{ gpr }
	Uint32    struct{ gpr }
	Uint64    struct{ gpr }
	Uintptr   struct{ gpr }
	Float32   fprValue
	Float64   fprValue
	Complex64 struct {
		real fprValue
		imag fprValue
	}
	Complex128 struct {
		real fprValue
		imag fprValue
	}
	Array[T any]     ptrValue[T]
	Chan[T any]      ptrValue[chan T]
	Func[T any]      ptrValue[Func[T]]
	Interface[T any] struct {
		rtype ptrValue[reflect.Type]
		value ptrValue[unsafe.Pointer]
	}
	Map[K comparable, V any] ptrValue[map[K]V]
	Pointer[T any]           ptrValue[T]
	Slice[T any]             struct {
		ptr ptrValue[T]
		len struct{ gpr }
		cap struct{ gpr }
	}
	String struct {
		ptr ptrValue[byte]
		len struct{ gpr }
	}
	UnsafePointer ptrValue[unsafe.Pointer]
)

func Add[T ~gprValue](src *Program, a, b T) T {
	if src == nil {
		return T{gpr: gprValue(a).gpr + gprValue(b).gpr} // fast path, hopefully inlined.
	}
	reg := src.gpr()
	src.code = append(src.code, ops.Add.As(opAdd{
		dst: reg,
		a:   src.mapGPR(a),
		b:   src.mapGPR(b),
	}))
	return T(gprValue{
		gpr: gpr(reg),
	})
}

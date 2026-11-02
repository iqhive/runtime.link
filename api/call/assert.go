package call

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"

	"github.com/iqhive/runtime.link/api/call/internal/cgo"
	"github.com/iqhive/runtime.link/api/call/internal/ffi"
	"github.com/iqhive/runtime.link/api/call/internal/jit"
)

// check is a safety assertion from a tag, compiled for
// one function. test returns a description of any failure.
type check func(*frame) string

// firstFailure runs the checks, so that a failed assertion stops the call.
func firstFailure(f *frame, name string, checks []check) error {
	for _, test := range checks {
		if failure := test(f); failure != "" {
			return fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: %s", name, failure)
		}
	}
	return nil
}

// subject is the value that an assertion is about.
type subject struct {
	from reflect.Type
	read func(*frame) jit.Value
	base func(*frame) jit.Value // the pointer that C sees, nil if read returns it.
}

// number is an integer of any type, compared by value: a size_t of
// 2^64-1 isn't -1, and neither fits the other's type.
type number struct {
	negative  bool
	magnitude uint64
}

func numberOf(i int64) number {
	if i < 0 {
		return number{negative: true, magnitude: -uint64(i)}
	}
	return number{magnitude: uint64(i)}
}

func (n number) String() string {
	if n.negative {
		return fmt.Sprintf("-%d", n.magnitude)
	}
	return fmt.Sprint(n.magnitude)
}

func (n number) compare(rhs number) int {
	switch {
	case n.negative && !rhs.negative:
		return -1
	case !n.negative && rhs.negative:
		return 1
	case n.negative:
		return cmp.Compare(rhs.magnitude, n.magnitude)
	}
	return cmp.Compare(n.magnitude, rhs.magnitude)
}

func isInteger(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

// operand reads one side of a comparison during a call.
type operand func(*frame) number

func constantOperand(value int64) operand {
	return func(*frame) number { return numberOf(value) }
}

// valueOperand reads the number that a value stands for in a tag,
// which for anything with a length is that length.
func valueOperand(from reflect.Type, read func(*frame) jit.Value) (operand, error) {
	kind := from.Kind()
	switch {
	case kind == reflect.Bool:
		return func(f *frame) number {
			if read(f).Reflect().Bool() {
				return number{magnitude: 1}
			}
			return number{}
		}, nil
	case isInteger(kind):
		return func(f *frame) number {
			value := read(f).Reflect()
			if value.CanInt() {
				return numberOf(value.Int())
			}
			return number{magnitude: value.Uint()}
		}, nil
	case kind == reflect.Slice, kind == reflect.String:
		return func(f *frame) number { return number{magnitude: uint64(read(f).Reflect().Len())} }, nil
	case kind == reflect.Pointer && from.Elem().Kind() == reflect.Array:
		return constantOperand(int64(from.Elem().Len())), nil
	case kind == reflect.Pointer, kind == reflect.UnsafePointer:
		return func(f *frame) number { return number{magnitude: uint64(read(f).Reflect().Pointer())} }, nil
	}
	return nil, fmt.Errorf("assertions cannot be made on %s", from)
}

// capacityOperand reads how many elements a value has room for.
func capacityOperand(from reflect.Type, read func(*frame) jit.Value) (operand, error) {
	kind := from.Kind()
	switch {
	case kind == reflect.Slice:
		return func(f *frame) number { return number{magnitude: uint64(read(f).Reflect().Cap())} }, nil
	case kind == reflect.String:
		return func(f *frame) number { return number{magnitude: uint64(read(f).Reflect().Len())} }, nil
	case kind == reflect.Pointer && from.Elem().Kind() == reflect.Array:
		return constantOperand(int64(from.Elem().Len())), nil
	case kind == reflect.Pointer:
		return constantOperand(1), nil
	}
	return nil, fmt.Errorf("the capacity of %s is unknown, so assertions about it can't be checked", from)
}

// argument returns the subject for the C parameter at index i.
func (p *plan) argument(i int, arg ffi.Type) subject {
	if arg.Maps == 0 {
		return subject{
			from: normal(cgo.Types.LookupKind(arg.Name)),
			read: func(f *frame) jit.Value { return f.send[i] },
		}
	}
	index := arg.Maps - 1
	return subject{
		from: p.goType.In(index),
		read: func(f *frame) jit.Value { return f.args[index] },
		base: func(f *frame) jit.Value { return f.send[i] },
	}
}

// operand returns the side of a comparison that an [ffi.Argument] refers to.
func (p *plan) operand(arg ffi.Argument) (operand, error) {
	switch {
	case arg.Index > 0:
		if int(arg.Index) > len(p.ldType.Args) {
			return nil, fmt.Errorf("@%d is out of range", arg.Index)
		}
		i := int(arg.Index) - 1
		target := p.argument(i, p.ldType.Args[i])
		return valueOperand(target.from, target.read)
	case arg.Const != "":
		value, err := constant(arg.Const)
		if err != nil {
			return nil, err
		}
		return constantOperand(value), nil
	}
	return constantOperand(arg.Value), nil
}

// assertions compiles the safety assertions of a type that are checked. Those about
// memory, on capacity, size, overlap and lifetime, are checked wherever they can be.
// 840f71d checked no assertion at all, so the comparison of a value is only checked
// when values is set, and an assertion that can't be checked, such as one on the
// capacity of an unsafe.Pointer or a printf-style one, is left out, not refused.
func (p *plan) assertions(rule ffi.Type, target subject, values bool) []check {
	test := rule.Test
	var checks []check
	keep := func(c check, err error) {
		if err == nil {
			checks = append(checks, c)
		}
	}
	if test.Overlaps.Check {
		keep(p.overlaps(target, test))
	}
	if test.Lifetime.Check {
		keep(p.within(target, test.Lifetime))
	}
	compared := test.Equality.Check || test.MoreThan.Check || test.LessThan.Check
	ofMemory := test.Capacity || test.Indirect > 0
	if compared && (ofMemory || values) {
		keep(p.comparison(rule, target))
	}
	return checks
}

// comparison compiles '=', '>', '<', the forms of them that are combined with '=',
// and the same comparisons within '[x]' or '*', which are about capacity and size.
func (p *plan) comparison(rule ffi.Type, target subject) (check, error) {
	test := rule.Test
	active := test.Equality
	if test.MoreThan.Check {
		active = test.MoreThan
	}
	if test.LessThan.Check {
		active = test.LessThan
	}
	var (
		left, right operand
		err         error
	)
	label := rule.Name
	switch {
	case test.Capacity:
		label = "capacity"
		left, err = capacityOperand(target.from, target.read)
	default:
		left, err = valueOperand(target.from, target.read)
	}
	if err != nil {
		return nil, err
	}
	switch {
	case test.Indirect > 0 && active.Index > 0:
		from, _, err := p.source(active)
		if err != nil {
			return nil, err
		}
		size, err := elementSize(from)
		if err != nil {
			return nil, err
		}
		right = constantOperand(int64(size))
	default:
		right, err = p.operand(active)
		if err != nil {
			return nil, err
		}
	}
	return func(f *frame) string {
		a, b := left(f), right(f)
		order := a.compare(b)
		holds := (test.Equality.Check && order == 0) || (test.MoreThan.Check && order > 0) || (test.LessThan.Check && order < 0)
		if holds != test.Inverted {
			return ""
		}
		return fmt.Sprintf("%s %s fails the assertion in its tag when compared with %s", label, a, b)
	}, nil
}

// memory is a range of addresses that a value occupies.
type memory func(*frame) (start, end uintptr)

func (p *plan) memoryOf(target subject) (memory, error) {
	var size func(*frame) uintptr
	switch kind := target.from.Kind(); {
	case kind == reflect.Slice:
		element := target.from.Elem().Size()
		size = func(f *frame) uintptr { return uintptr(target.read(f).Reflect().Cap()) * element }
	case kind == reflect.String:
		size = func(f *frame) uintptr { return uintptr(target.read(f).Reflect().Len()) + 1 }
	case kind == reflect.Pointer:
		size = func(*frame) uintptr { return target.from.Elem().Size() }
	default:
		return nil, fmt.Errorf("the memory behind %s is unknown, so assertions about it can't be checked", target.from)
	}
	base := target.base
	if base == nil {
		base = target.read
	}
	return func(f *frame) (uintptr, uintptr) {
		start := base(f).Reflect().Pointer()
		return start, start + size(f)
	}, nil
}

// memoryOperand returns the memory of the argument that '@n' refers to.
func (p *plan) memoryOperand(arg ffi.Argument) (memory, error) {
	if arg.Index == 0 || int(arg.Index) > len(p.ldType.Args) {
		return nil, errors.New("expected an argument number")
	}
	i := int(arg.Index) - 1
	return p.memoryOf(p.argument(i, p.ldType.Args[i]))
}

func (p *plan) overlaps(target subject, test ffi.Assertions) (check, error) {
	own, err := p.memoryOf(target)
	if err != nil {
		return nil, err
	}
	other, err := p.memoryOperand(test.Overlaps)
	if err != nil {
		return nil, err
	}
	return func(f *frame) string {
		ownStart, ownEnd := own(f)
		otherStart, otherEnd := other(f)
		overlapping := ownStart < otherEnd && otherStart < ownEnd
		if overlapping != test.Inverted {
			return ""
		}
		if test.Inverted {
			return "memory must not overlap the buffer given in its tag"
		}
		return "memory must overlap the buffer given in its tag"
	}, nil
}

// within checks that a pointer points into the buffer of '@n'. A NULL
// pointer is left alone, since it's how C says that nothing was found.
func (p *plan) within(target subject, arg ffi.Argument) (check, error) {
	buffer, err := p.memoryOperand(arg)
	if err != nil {
		return nil, err
	}
	base := target.base
	if base == nil {
		base = target.read
	}
	switch target.from.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Slice, reflect.String:
	default:
		return nil, fmt.Errorf("%s cannot point into a buffer", target.from)
	}
	return func(f *frame) string {
		pointer := base(f).Reflect().Pointer()
		start, end := buffer(f)
		if pointer == 0 || (pointer >= start && pointer < end) {
			return ""
		}
		return fmt.Sprintf("pointer %#x is outside of the buffer given in its tag", pointer)
	}, nil
}

func elementType(from reflect.Type) (reflect.Type, error) {
	switch from.Kind() {
	case reflect.Slice:
		return from.Elem(), nil
	case reflect.String:
		return reflect.TypeFor[byte](), nil
	case reflect.Pointer:
		if from.Elem().Kind() == reflect.Array {
			return from.Elem().Elem(), nil
		}
		return from.Elem(), nil
	}
	return nil, fmt.Errorf("the element type of %s is unknown", from)
}

func elementSize(from reflect.Type) (uintptr, error) {
	element, err := elementType(from)
	if err != nil {
		return 0, err
	}
	return element.Size(), nil
}

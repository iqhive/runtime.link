package call

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"

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

// number is an integer or a float of any type, compared by value: a size_t of
// 2^64-1 isn't -1, and neither fits the other's type, nor does every integer
// have a float that is equal to it.
type number struct {
	negative  bool
	magnitude uint64

	floating bool    // the value is real, not the integer above.
	real     float64 // which may be an infinity or a NaN.

	absent bool // the 0 is how many elements there are behind a nil pointer.
}

func numberOf(i int64) number {
	if i < 0 {
		return number{negative: true, magnitude: -uint64(i)}
	}
	return number{magnitude: uint64(i)}
}

func (n number) String() string {
	switch {
	case n.floating:
		return strconv.FormatFloat(n.real, 'g', -1, 64)
	case n.negative:
		return fmt.Sprintf("-%d", n.magnitude)
	}
	return fmt.Sprint(n.magnitude)
}

// compare returns the order of n and rhs. They have none when either is a NaN,
// which is not more than, less than or equal to anything, so ordered is false.
func (n number) compare(rhs number) (order int, ordered bool) {
	switch {
	case n.floating || rhs.floating:
		a, b := n.exact(), rhs.exact()
		if a == nil || b == nil {
			return 0, false
		}
		return a.Cmp(b), true
	case n.negative && !rhs.negative:
		return -1, true
	case !n.negative && rhs.negative:
		return 1, true
	case n.negative:
		return cmp.Compare(rhs.magnitude, n.magnitude), true
	}
	return cmp.Compare(n.magnitude, rhs.magnitude), true
}

// exact returns the value with nothing rounded away, so that an integer
// can be compared with a float. A NaN has no value, which is nil.
func (n number) exact() *big.Float {
	switch {
	case n.floating && math.IsNaN(n.real):
		return nil
	case n.floating:
		return big.NewFloat(n.real)
	}
	value := new(big.Float).SetUint64(n.magnitude)
	if n.negative {
		value.Neg(value)
	}
	return value
}

// value returns the number as the Go value that holds it.
func (n number) value(asm jit.Assembly) jit.Value {
	switch {
	case n.floating:
		return asm.Float64(n.real)
	case n.negative:
		return asm.Int64(int64(-n.magnitude))
	case n.magnitude > math.MaxInt64:
		return asm.Uint64(n.magnitude)
	}
	return asm.Int64(int64(n.magnitude))
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

// room returns how many elements there are behind a pointer of the given type.
// A nil pointer has none, whatever its type says, just as a nil slice has none:
// counted by its type, NULL passed every check of the size that C is given with
// it, and '!=NULL' could not keep it from C.
func room(from reflect.Type, read func(*frame) jit.Value) operand {
	elements := uint64(1)
	if from.Elem().Kind() == reflect.Array {
		elements = uint64(from.Elem().Len())
	}
	return func(f *frame) number {
		if read(f).Reflect().IsNil() {
			return number{absent: true}
		}
		return number{magnitude: elements}
	}
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
	case kind == reflect.Float32, kind == reflect.Float64:
		return func(f *frame) number { return number{floating: true, real: read(f).Reflect().Float()} }, nil
	case kind == reflect.Slice, kind == reflect.String:
		return func(f *frame) number { return number{magnitude: uint64(read(f).Reflect().Len())} }, nil
	case kind == reflect.Pointer && from.Elem().Kind() == reflect.Array:
		return room(from, read), nil
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
	case kind == reflect.Pointer:
		return room(from, read), nil
	}
	return nil, fmt.Errorf("the capacity of %s is unknown, so assertions about it can't be checked", from)
}

// argument returns the subject for the C parameter at index i. A number is read
// as C is passed it, which is after its conversion to the C type, since that is
// the value that a tag is about: a Go int of -1 is not the size_t it turns into,
// and a capacity that is more than -1 says nothing of the size that C is given.
func (p *plan) argument(i int, arg ffi.Type) subject {
	sent := func(f *frame) jit.Value { return f.send[i] }
	if arg.Maps == 0 {
		return subject{from: normal(cgo.Types.LookupKind(arg.Name)), read: sent}
	}
	index := arg.Maps - 1
	from := p.goType.In(index)
	if passed := passedAs(arg, from); passed != nil {
		return subject{from: passed, read: sent}
	}
	return subject{
		from: from,
		read: func(f *frame) jit.Value { return f.args[index] },
		base: sent,
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
		return func(*frame) number { return value }, nil
	}
	return constantOperand(arg.Value), nil
}

// assertions compiles the safety assertions of a type that are checked. Those about
// memory, on capacity, size, overlap and lifetime, are checked wherever they can be,
// and the comparison of a value is checked when values is set. An assertion that
// can't be checked returns the error, so that a tag that promises a check that
// cannot be made is refused when it is linked: 840f71d left such an assertion out
// instead, which a caller could not tell from one that was checked.
func (p *plan) assertions(rule ffi.Type, target subject, values bool) ([]check, error) {
	test := rule.Test
	var checks []check
	if test.Overlaps.Check {
		overlap, err := p.overlaps(target, test)
		if err != nil {
			return nil, err
		}
		checks = append(checks, overlap)
	}
	if test.Lifetime.Check {
		inside, err := p.within(target, test)
		if err != nil {
			return nil, err
		}
		checks = append(checks, inside)
	}
	ofMemory := test.Capacity || test.Indirect > 0
	if compares(rule) && (ofMemory || values) {
		comparison, err := p.comparison(rule, target)
		if err != nil {
			return nil, err
		}
		checks = append(checks, comparison)
	}
	return checks, nil
}

// compares reports whether a tag says what its value must compare as.
func compares(rule ffi.Type) bool {
	test := rule.Test
	return test.Equality.Check || test.MoreThan.Check || test.LessThan.Check
}

// asserts reports whether a tag makes a safety assertion about its value that
// is checked. '?@n' and ':@n' are not, they only document what is intended.
func asserts(rule ffi.Type) bool {
	test := rule.Test
	return test.Capacity || test.Lifetime.Check || test.Overlaps.Check || compares(rule)
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
		order, ordered := a.compare(b)
		holds := ordered && ((test.Equality.Check && order == 0) || (test.MoreThan.Check && order > 0) || (test.LessThan.Check && order < 0))
		if ordered && nothingAsked(test, active, a, b, order) {
			holds = true
		}
		if holds != test.Inverted {
			return ""
		}
		return fmt.Sprintf("%s %s fails the assertion in its tag when compared with %s", label, a, b)
	}, nil
}

// nothingAsked reports whether a comparison is of a nil pointer with a size of
// 0, in a tag that asks for more room than the size, as '[@n]' does. NULL with
// a size of 0 is how C is told that there is no buffer, and nothing is read or
// written through it, so it keeps meeting the assertion as it did when a nil
// pointer was counted by its type. Only a size from '@n' is one that C is
// passed: more room than a literal 0 is still more than a nil pointer has.
func nothingAsked(test ffi.Assertions, size ffi.Argument, a, b number, order int) bool {
	if order != 0 || size.Index == 0 {
		return false
	}
	return (test.MoreThan.Check && a.absent) || (test.LessThan.Check && b.absent)
}

// memory is a range of addresses that a value occupies.
type memory func(*frame) (start, end uintptr)

func (p *plan) memoryOf(target subject) (memory, error) {
	var size func(f *frame, start uintptr) uintptr
	switch kind := target.from.Kind(); {
	case kind == reflect.Slice:
		element := target.from.Elem().Size()
		size = func(f *frame, _ uintptr) uintptr { return uintptr(target.read(f).Reflect().Cap()) * element }
	case kind == reflect.String:
		size = func(f *frame, start uintptr) uintptr {
			text := target.read(f).Reflect()
			if text.Len() > 0 && start == uintptr(text.UnsafePointer()) {
				// a string that is passed where it is ends in a NUL of its own. Only
				// a copy has one after it, which is a byte more than the string has.
				return uintptr(text.Len())
			}
			return uintptr(text.Len()) + 1
		}
	case kind == reflect.Pointer:
		size = func(*frame, uintptr) uintptr { return target.from.Elem().Size() }
	default:
		return nil, fmt.Errorf("the memory behind %s is unknown, so assertions about it can't be checked", target.from)
	}
	base := target.base
	if base == nil {
		base = target.read
	}
	return func(f *frame) (uintptr, uintptr) {
		start := base(f).Reflect().Pointer()
		if start == 0 {
			// there is no memory at NULL, however much of it the type of a nil pointer has.
			return 0, 0
		}
		return start, start + size(f, start)
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

// within checks that a pointer points into the buffer of '@n', or with a '!'
// that it doesn't. A NULL pointer is left alone, since it's how C says that
// nothing was found.
func (p *plan) within(target subject, test ffi.Assertions) (check, error) {
	buffer, err := p.memoryOperand(test.Lifetime)
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
		inside := pointer >= start && pointer < end
		switch {
		case pointer == 0, inside != test.Inverted:
			return ""
		case test.Inverted:
			return fmt.Sprintf("pointer %#x is inside of the buffer given in its tag, which it must not be", pointer)
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

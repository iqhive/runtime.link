// Package xyz provides switch types, a way to represent named unions, enums and other variants.
package xyz

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Switch on the underlying storage in order
// to represent a restricted set of values.
// Can be used as the underlying value for
// a named type.
type Switch[Storage any, Values any] struct {
	switchMethods[Storage, Values] // export methods.
}

type varWith[Storage any, Values any] interface {
	~struct {
		switchMethods[Storage, Values]
	}
	accessor() accessor
	Values() Values
}

// ValueOf returns the value of the switch.
func ValueOf[Storage any, Values any, Variant varWith[Storage, Values]](variant Variant) Value[Variant] {
	return Value[Variant]{variant.accessor()}
}

// Value represents the type of a field within a variant.
type Value[T any] struct {
	accessor
}

func (k Value[T]) String() string {
	return k.name
}

// switchMethods can be embedded into a struct to
// provide methods for interacting with a variant.
type switchMethods[Storage any, Values any] struct {
	tag uint16
	ram Storage
}

func (v switchMethods[Storage, Values]) String() string {
	access := v.accessor()
	if access.text != "" || access.zero {
		if access.fmts {
			return fmt.Sprintf(access.text, access.get(&v))
		}
		return access.text
	}
	if access.void {
		return access.name
	}
	return fmt.Sprint(access.get(&v))
}

func (v switchMethods[Storage, Values]) variant() {}

func (v *switchMethods[Storage, Values]) storage() (any, uint16) {
	return &v.ram, v.tag
}

func (v *switchMethods[Storage, Values]) setTag(tag uint16) {
	v.tag = tag
}

func (v switchMethods[Storage, Values]) typeOf(field reflect.StructField) reflect.Type {
	type isVary interface {
		vary() reflect.Type
	}
	if field.Type.Implements(reflect.TypeOf([0]isVary{}).Elem()) {
		return reflect.Zero(field.Type).Interface().(isVary).vary()
	}
	if field.Type.Kind() == reflect.Struct && field.Type.NumField() > 0 && field.Type.Field(0).Type == reflect.TypeOf(v) {
		return nil
	}
	panic(fmt.Sprintf("invalid variant field: %s", field.Type))
}

func (v switchMethods[Storage, Values]) accessor() accessor {
	var stype = reflect.TypeOf([0]Storage{}).Elem()
	var sptrs = hasPointers(stype)

	var values Values
	var rtype = reflect.TypeOf(values)
	field := rtype.Field(int(v.tag))
	text, hasText := field.Tag.Lookup("text")
	if !hasText && stype.Kind() == reflect.String {
		panic(fmt.Sprintf("missing text tag for string variant field '%s'", field.Name))
	}
	enum := uint64(0)
	if s, ok := field.Tag.Lookup("enum"); ok {
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("invalid enum tag '%s': %s", field.Tag.Get("enum"), err))
		}
		enum = u
	} else {
		enum = uint64(v.tag)
	}
	void := false
	ftype := v.typeOf(field)
	if ftype == nil {
		ftype = reflect.TypeOf(v.tag)
		void = true
	}
	ptrs := hasPointers(ftype)
	safe := stype.Kind() == reflect.Interface || stype.Kind() == reflect.UnsafePointer ||
		stype.Kind() == reflect.String || (stype.Kind() == reflect.Slice && !ptrs) || (stype.Size() >= ftype.Size() && !sptrs && !ptrs)
	access := accessor{
		name: field.Name,
		chck: uint16(v.tag),
		enum: enum,
		void: void,
		text: text,
		zero: text == "" && hasText,
		fmts: strings.Contains(text, "%"),
		safe: safe,
		rtyp: ftype,
	}
	if !safe {
		panic(fmt.Sprintf("unsafe use of variant accessor '%s': incompatible with storage", field.Name))
	}
	return access
}

func (v switchMethods[Storage, Values]) Values() Values {
	var zero Values
	var rtype = reflect.TypeOf(zero)
	var rvalue = reflect.ValueOf(&zero).Elem()
	for i := 0; i < rtype.NumField(); i++ {
		if i > math.MaxUint16 {
			panic("too many variant values")
		}
		v.tag = uint16(i)
		var (
			access = v.accessor()
		)
		if access.void {
			access.as(rvalue.Field(i).Addr().Interface(), uint16(i))
		} else {
			type settable interface {
				set(accessor)
			}
			rvalue.Field(i).Addr().Interface().(settable).set(access)
		}
	}
	return zero
}

type isVariant interface {
	variant()
}

type hasStorage interface {
	storage() (any, uint16)
	setTag(uint16)
}

type accessor struct {
	chck uint16
	void bool
	fmts bool
	zero bool // is a zero value
	safe bool
	enum uint64
	name string
	text string
	rtyp reflect.Type
}

func (v accessor) get(ram any) any {
	if !v.safe {
		panic("unintialized variant")
	}
	storage, check := any(ram).(hasStorage).storage()
	if check != v.chck {
		panic("variant access violation")
	}
	var (
		rvalue = reflect.ValueOf(storage).Elem()
	)
	switch rvalue.Kind() {
	case reflect.String:
		var s = reflect.New(v.rtyp).Elem()
		fmt.Sscanf(rvalue.String(), v.text, s.Addr().Interface())
		return s.Interface()
	case reflect.Slice, reflect.UnsafePointer:
		return reflect.NewAt(v.rtyp, rvalue.UnsafePointer()).Elem().Interface()
	case reflect.Interface:
		return rvalue.Interface()
	default:
		return reflect.NewAt(v.rtyp, rvalue.Addr().UnsafePointer()).Elem().Interface()
	}
}

// as is unsafe,
func (v accessor) as(ram any, val any) {
	if !v.safe {
		panic("unintialized variant")
	}
	if reflect.TypeOf(val) != v.rtyp {
		panic("unsafe use of variant accessor")
	}
	storage, _ := any(ram).(hasStorage).storage()
	var (
		rvalue = reflect.ValueOf(storage).Elem()
	)
	switch rvalue.Kind() {
	case reflect.Bool:
		if v.enum > 0 {
			rvalue.SetBool(true)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rvalue.SetInt(int64(v.enum))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		rvalue.SetUint(v.enum)
	case reflect.Float32, reflect.Float64:
		rvalue.SetFloat(float64(v.enum))
	case reflect.Complex64, reflect.Complex128:
		rvalue.SetComplex(complex(float64(v.enum), 0))
	case reflect.Array:
		reflect.NewAt(reflect.TypeOf(val), rvalue.Addr().UnsafePointer()).Elem().Set(reflect.ValueOf(val))
	case reflect.String:
		if v.fmts {
			rvalue.SetString(fmt.Sprintf(v.text, val))
		} else {
			rvalue.SetString(v.text)
		}
	case reflect.Interface:
		rvalue.Set(reflect.ValueOf(val))
	case reflect.Slice:
		var length = int(reflect.TypeOf(val).Size() / rvalue.Type().Elem().Size())
		rvalue.Set(reflect.MakeSlice(rvalue.Type(), length, length))
		reflect.NewAt(reflect.TypeOf(val), rvalue.UnsafePointer()).Set(reflect.ValueOf(val))
	case reflect.Struct:
		reflect.NewAt(reflect.TypeOf(val), rvalue.Addr().UnsafePointer()).Set(reflect.ValueOf(val))
	case reflect.UnsafePointer:
		value := reflect.ValueOf(val)
		if !value.CanAddr() {
			copy := reflect.New(value.Type()).Elem()
			copy.Set(value)
			value = copy
		}
		rvalue.SetPointer(value.Addr().UnsafePointer())
	default:
		panic("unreachable")
	}
	any(ram).(hasStorage).setTag(v.chck)
}

// Case indicates that a value within a variant can vary
// in value, constrained by a particular type.
type Case[Variant isVariant, Constraint any] struct {
	_     [0]*Constraint
	Value Value[Variant]
}

func (v *Case[Variant, Constraint]) set(to accessor) {
	v.Value = Value[Variant]{to}
}

func (v Case[Variant, Constraint]) vary() reflect.Type {
	return reflect.TypeOf([0]Constraint{}).Elem()
}

// As returns the value of the variant as the given type.
func (v Case[Variant, Constraint]) As(val Constraint) Variant {
	var zero Variant
	v.Value.as(&zero, val)
	return zero
}

// Get returns the value of the variant as the given type.
func (v Case[Variant, Constraint]) Get(variant Variant) Constraint {
	return v.Value.get(&variant).(Constraint)
}

func hasPointers(value reflect.Type) bool {
	switch value.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return false
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if hasPointers(value.Field(i).Type) {
				return true
			}
		}
	case reflect.Array:
		return hasPointers(value.Elem())

	}
	return true
}
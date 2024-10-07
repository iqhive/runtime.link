// Package oop provides a pattern for representing objects and classes in Go (for the rare cases where a representation is needed).
//
// The primary utility of this package, is to assist in translating Object Oriented code from other languages into Go, enabling
// such a translation to be fairly direct and with less risk of error from attempting to use a more idiomatic Go approach.
//
// Example
//
//	type Animal struct {
//		oop.Object
//	}
//
//	func (animal *Animal) Class() ClassAnimal { return animal }
//	func (animal *Animal) MakeSound() string { return "" }
//
//	type ClassAnimal interface {
//		MakeSound() string
//	}
//
//	type Dog struct {
//		Animal
//	}
//
//	func (dog *Dog) Class() ClassAnimal { return dog }
//	func (dog *Dog) MakeSound() string { return "Woof!" }
package oop

import "sync"

// Object should be embedded inside of a base class struct.
type Object struct {
	_       [0]sync.RWMutex
	derived any
}

var _ isClass[any] = &Object{}

func (i *Object) Class() any              { return i }
func (i *Object) getDerivedClass() any    { return i.derived }
func (i *Object) setDerivedClass(val any) { i.derived = val }

type isClass[I any] interface {
	getDerivedClass() any
	setDerivedClass(any)

	Class() I
}

// Make instantiates a class correctly, call this before returning the object from
// a constructor, or pass new(T) if the type has no constructor function.
func Make[T isClass[I], I any](class T) T {
	class.setDerivedClass(class)
	return class
}

// Derived returns the derived class from the current class, methods called
// on the dervied class will call the derived class's methods.
func Derived[T isClass[I], I any](class T) I {
	return class.getDerivedClass().(I)
}

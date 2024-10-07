package oop_test

import (
	"fmt"
	"reflect"
	"testing"

	"runtime.link/oop"
)

type Animal struct {
	oop.Object

	Name    string
	Species string
}

func NewAnimal(name, species string) *Animal {
	return oop.Make(&Animal{Name: name, Species: species})
}

type ClassAnimal interface {
	MakeSound() string
}

func (a *Animal) Class() ClassAnimal { return a }
func (a *Animal) MakeSound() string  { return "" }

type Dog struct {
	Animal

	Breed string
}

func NewDog(name, breed string) *Dog {
	var dog Dog
	dog.Animal = *NewAnimal(name, "Dog")
	dog.Breed = breed
	return oop.Make(&dog)
}

func (d *Dog) Class() ClassAnimal { return d }
func (d *Dog) MakeSound() string  { return "Woof!" }

type Cat struct {
	Animal

	Color string
}

func NewCat(name, color string) *Cat {
	var cat Cat
	cat.Animal = *NewAnimal(name, "Cat")
	cat.Color = color
	return oop.Make(&cat)
}

func (c *Cat) Class() ClassAnimal { return c }
func (c *Cat) MakeSound() string  { return "Meow!" }

func TestOOP(t *testing.T) {
	dog := NewDog("Buddy", "Labrador")
	cat := NewCat("Whiskers", "White")

	// Calling methods
	fmt.Println(dog.Name, "is a", dog.Species, "and says:", dog.MakeSound())
	fmt.Println(cat.Name, "is a", cat.Species, "and says:", cat.MakeSound())

	if reflect.TypeOf(oop.Derived(&dog.Animal)) != reflect.TypeOf(&Dog{}) {
		t.Error("Derived(&dog.Animal) != &Dog{}")
	}
}

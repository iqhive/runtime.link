package xyz_test

import (
	"testing"

	"runtime.link/xyz"
)

func TestSwitch(t *testing.T) {
	type Animal xyz.Switch[int, struct {
		Cat Animal
		Dog Animal
	}]
	var Animals = xyz.AccessorFor(Animal.Values)

	var animal = Animals.Cat

	if animal.String() != "Cat" {
		t.Fatal("unexpected value")
	}

	switch animal {
	case Animals.Cat:
	case Animals.Dog:
		t.Fatal("unexpected value")
	default:
		t.Fatal("unexpected value")
	}

	var decoded Animal
	if err := decoded.UnmarshalJSON([]byte(`"Dog"`)); err != nil {
		t.Fatal(err)
	}
	if decoded != Animals.Dog {
		t.Fatal("unexpected value")
	}
}

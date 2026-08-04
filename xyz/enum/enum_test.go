package enum_test

import (
	"testing"

	"runtime.link/xyz/enum"
)

func TestEnum(t *testing.T) {
	type Animal string
	var Animals = enum.Register[Animal, struct {
		Dog Animal
		Cat Animal
	}]()

	if Animals.Dog != "Dog" {
		t.Errorf("Expected Dog to be 'Dog', got '%s'", Animals.Dog)
	}
	if Animals.Cat != "Cat" {
		t.Errorf("Expected Cat to be 'Cat', got '%s'", Animals.Cat)
	}

	if err := enum.Validate(Animals.Dog); err != nil {
		t.Errorf("expected Dog to be valid, got %v", err)
	}
	if err := enum.Validate(Animal("Fish")); err == nil {
		t.Errorf("expected Fish to be invalid, got nil error")
	}

	// A string type that was never registered has no known value set, so
	// validating it is itself an error.
	type Unregistered string
	if err := enum.Validate(Unregistered("anything")); err == nil {
		t.Errorf("expected unregistered type to error, got nil")
	}
	// Non-string values are never registered enums.
	if err := enum.Validate(42); err == nil {
		t.Errorf("expected non-string to error, got nil")
	}

	// ValuesJSON returns the value names, JSON-encoded, in declaration order.
	values := enum.ValuesJSON(Animals.Dog)
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if string(values[0]) != `"Dog"` || string(values[1]) != `"Cat"` {
		t.Errorf("unexpected values order: %q, %q", values[0], values[1])
	}
	// An unregistered type has no values.
	if enum.ValuesJSON(Unregistered("x")) != nil {
		t.Errorf("expected nil values for unregistered type")
	}
}

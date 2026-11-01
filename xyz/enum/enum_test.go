package enum_test

import (
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"github.com/iqhive/runtime.link/xyz/enum"
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

type color string

var colors = enum.Register[color, struct {
	Red   color
	Blue  color `json:"blue"`
	Green color `json:"green,omitempty"`
	Pink  color `json:",omitempty"`
	Other color `json:"an <other> colour"
		keeps its documentation in the tag.`
}]()

// A value is the whole json tag, options and all, as it is stored in callers' data.
func TestRegisterDerivesValuesFromNamesAndTags(t *testing.T) {
	for want, got := range map[string]color{
		"Red":               colors.Red,
		"blue":              colors.Blue,
		"green,omitempty":   colors.Green,
		",omitempty":        colors.Pink,
		"an <other> colour": colors.Other,
	} {
		if string(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if err := enum.Validate(got); err != nil {
			t.Errorf("%q is one of the values: %v", got, err)
		}
	}
}

func TestIs(t *testing.T) {
	type unregistered string
	if !enum.Is(colors.Red) || !enum.Is(color("not one of the values")) {
		t.Error("the type is registered")
	}
	for _, val := range []any{"Red", unregistered("Red"), 5, nil, []string{"Red"}, new(color)} {
		if enum.Is(val) {
			t.Errorf("%#v is not a value of a registered type", val)
		}
	}
}

func TestValidateRejectsWhatIsNotAKnownValue(t *testing.T) {
	type unregistered string
	for _, val := range []any{color("red"), color(""), color("green"), color("Pink"), "Red", unregistered("Red"), 5, nil} {
		if err := enum.Validate(val); err == nil {
			t.Errorf("%#v was valid", val)
		}
	}
}

func TestValuesJSONListsValuesInOrder(t *testing.T) {
	type unregistered string
	var got []string
	for _, value := range enum.ValuesJSON(colors.Red) {
		var name string
		if err := json.Unmarshal(value, &name); err != nil {
			t.Fatalf("%s is not JSON: %v", value, err)
		}
		got = append(got, name)
	}
	want := []string{"Red", "blue", "green,omitempty", ",omitempty", "an <other> colour"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	for _, val := range []any{unregistered("x"), "x", 5, nil} {
		if values := enum.ValuesJSON(val); values != nil {
			t.Errorf("%#v has the values %q", val, values)
		}
	}
}

func TestRegisterWithoutValues(t *testing.T) {
	type empty string
	enum.Register[empty, struct{}]()
	if !enum.Is(empty("")) {
		t.Error("the type is registered")
	}
	if err := enum.Validate(empty("")); err == nil {
		t.Error("a type without values has no valid value")
	}
	if got := enum.ValuesJSON(empty("")); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}

func TestEnumIsSafeForConcurrentUse(t *testing.T) {
	type raced string
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			values := enum.Register[raced, struct{ A, B raced }]()
			if !enum.Is(values.A) {
				t.Error("the type is registered")
			}
			if err := enum.Validate(values.B); err != nil {
				t.Error(err)
			}
			if got := enum.ValuesJSON(values.A); len(got) != 2 {
				t.Errorf("got %q", got)
			}
		})
	}
	wg.Wait()
}

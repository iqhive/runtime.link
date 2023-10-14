package xyz_test

import (
	"encoding/json"
	"testing"

	"runtime.link/xyz"
)

func TestSwitch(t *testing.T) {
	type StringOrInt xyz.Switch[any, struct {
		String xyz.Case[StringOrInt, string]
		Number xyz.Case[StringOrInt, int]
	}]
	var StringOrInts = xyz.AccessorFor(StringOrInt.Values)

	var val StringOrInt = StringOrInts.Number.As(22)

	if val.String() != "22" {
		t.Fatal("unexpected value")
	}
	if StringOrInts.Number.Get(val) != 22 {
		t.Fatal("unexpected value")
	}

	switch xyz.ValueOf(val) {
	case StringOrInts.String:
		t.Fatal("unexpected value")
	case StringOrInts.Number:

	default:
		t.Fatal("unexpected value")
	}

	val = StringOrInts.String.As("hello")

	if val.String() != "hello" {
		t.Fatal("unexpected value")
	}
	if StringOrInts.String.Get(val) != "hello" {
		t.Fatal("unexpected value")
	}

}

func TestEnum(t *testing.T) {
	type Animal xyz.Switch[xyz.Enum, struct {
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
}

func TestOmit(t *testing.T) {
	var val xyz.Maybe[string]

	val = xyz.New("hello")

	v, ok := val.Get()
	if !ok {
		t.Fatal("unexpected value")
	}
	if v != "hello" {
		t.Fatal("unexpected value")
	}

	clear(val)

	b, err := json.Marshal(struct {
		Field xyz.Maybe[string] `json:"field,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{}" {
		t.Fatal("unexpected value")
	}
}

/*func TestGetter(t *testing.T) {
	type API struct{}

	type TestObject ref.For[API, TestObject, xyz.Struct[xyz.Standard, struct {
		Float64 xyz.Field[MyTestObject, float64]
		Int64   xyz.Field[MyTestObject, int64]
	}]]

	var NewMyTestObject = xyz.Creates(TestObject{}.Ref.Fields)
	var ForMyTestObject = xyz.Reflect(TestObject{}.Ref.Fields)

	var obj = NewMyTestObject()
	ForMyTestObject.Float64.Get(obj, 1.234)
	val := ForMyTestObject.Float64.Get(obj)
	if val != 1.234 {
		t.Fatalf("val = %v; want %v", val, 1.234)
	}
}*/

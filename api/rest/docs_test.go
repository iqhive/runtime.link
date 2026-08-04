package rest

import (
	"reflect"
	"testing"

	"runtime.link/api/internal/oas"
	"runtime.link/xyz/enum"
)

// Fruit is a runtime.link/xyz/enum type: a plain named string type whose
// values live in the enum registry rather than in a ValuesJSON method.
type Fruit string

var Fruits = enum.Register[Fruit, struct {
	Apple  Fruit `json:"apple"`
	Banana Fruit `json:"banana"`
}]()

// TestSchemaFor_Enum verifies that an enum type produces a string schema
// whose Enum lists the registered values (via enum.ValuesJSON), even though
// the type has no ValuesJSON method of its own.
func TestSchemaFor_Enum(t *testing.T) {
	schema := schemaFor(nil, Fruit(""))
	if len(schema.Type) != 1 || schema.Type[0] != oas.Types.String {
		t.Fatalf("expected string type, got %v", schema.Type)
	}
	if len(schema.Enum) != 2 {
		t.Fatalf("expected 2 enum values, got %d: %v", len(schema.Enum), schema.Enum)
	}
	if string(schema.Enum[0]) != `"apple"` || string(schema.Enum[1]) != `"banana"` {
		t.Errorf("unexpected enum values: %q, %q", schema.Enum[0], schema.Enum[1])
	}
}

// Money is a named struct type; a field of this type produces a $ref to a
// registered component.
type Money struct {
	Cents int64 `json:"cents"`
}

// Named is a named scalar type; previously it produced a $ref, which caused
// its sibling title/description to be dropped by Swagger UI. It should now be
// inlined instead.
type Named string

// Priced exercises the field description rendering for referenced schemas.
type Priced struct {
	Amount Money `json:"amount" docs:"of the thing, in cents."`
	Label  Named `json:"label" docs:"attached to the thing."`
}

// TestFieldDescriptionOnReferencedSchema verifies that a struct-typed field
// keeps its description by wrapping the $ref in an allOf, and that a named
// scalar field is inlined (no $ref) so its description survives directly.
func TestFieldDescriptionOnReferencedSchema(t *testing.T) {
	var doc oas.Document
	// The top-level struct is registered as a component; schemaFor returns a
	// $ref to it, so inspect the registered schema itself.
	schemaFor(&doc, Priced{})
	schema := doc.Components.Schemas["rest"].Defs["Priced"]
	if schema == nil {
		t.Fatal("Priced schema was not registered")
	}

	amount := schema.Properties["amount"]
	if amount == nil {
		t.Fatal("missing amount property")
	}
	if amount.Ref != "" {
		t.Errorf("expected amount to wrap the $ref, got bare $ref %q", amount.Ref)
	}
	if len(amount.AllOf) != 1 || amount.AllOf[0].Ref == "" {
		t.Errorf("expected amount.allOf to hold the $ref, got %+v", amount.AllOf)
	}
	if string(amount.Description) != "of the thing, in cents." {
		t.Errorf("amount description not carried at property level: %q", amount.Description)
	}

	label := schema.Properties["label"]
	if label == nil {
		t.Fatal("missing label property")
	}
	if label.Ref != "" {
		t.Errorf("expected named scalar to be inlined, got $ref %q", label.Ref)
	}
	if string(label.Description) != "attached to the thing." {
		t.Errorf("label description not carried: %q", label.Description)
	}
}

func TestNamespaceName(t *testing.T) {

	type Generic[T any] struct{}
	type Thing struct{}

	type Complex[A, B any] struct{}

	namespace, name := namespaceName(reflect.TypeOf(Generic[Thing]{}))
	if namespace != "rest" {
		t.Fatal("unexpected value")
	}
	if name != "Generic[rest.Thing]" {
		t.Fatal("unexpected value")
	}

	namespace, name = namespaceName(reflect.TypeOf(Complex[Thing, Thing]{}))
	if namespace != "rest" {
		t.Fatal("unexpected value")
	}
	if name != "Complex[rest.Thing, rest.Thing]" {
		t.Fatal("unexpected value")
	}
}

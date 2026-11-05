package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/internal/oas"
	"github.com/iqhive/runtime.link/xyz"
	"github.com/iqhive/runtime.link/xyz/enum"
)

// Fruit is a github.com/iqhive/runtime.link/xyz/enum type: a plain named string type whose
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

type (
	recursiveMap     map[string]recursiveMap
	recursiveSlice   []recursiveSlice
	recursiveArray   [1]recursiveMap
	recursivePointer *recursivePointer
	recursiveChan    chan recursiveChan
	recursiveAnon    []struct{ Next recursiveAnon }
	recursiveIter    func(func(recursiveIter) bool)
)

type recursiveStruct struct {
	Name   string                     `json:"name"`
	Next   *recursiveStruct           `json:"next"`
	Kids   []recursiveStruct          `json:"kids"`
	ByName map[string]recursiveStruct `json:"by_name"`
	Tree   recursiveMap               `json:"tree"`
}

type mutualA struct {
	B map[string]mutualB `json:"b"`
}

type mutualB []mutualA

type recursiveUnion xyz.Tagged[any, struct {
	Leaf xyz.Case[recursiveUnion, string]           `json:"leaf"`
	Pair xyz.Case[recursiveUnion, []recursiveUnion] `json:"pair"`
}]

// TestSchemaForRecursiveTypes must not overflow the stack, which is a fatal
// error and not a panic: nothing can recover it.
func TestSchemaForRecursiveTypes(t *testing.T) {
	for _, rtype := range []reflect.Type{
		reflect.TypeFor[recursiveMap](),
		reflect.TypeFor[recursiveSlice](),
		reflect.TypeFor[recursiveArray](),
		reflect.TypeFor[recursivePointer](),
		reflect.TypeFor[recursiveChan](),
		reflect.TypeFor[recursiveAnon](),
		reflect.TypeFor[recursiveIter](),
		reflect.TypeFor[recursiveStruct](),
		reflect.TypeFor[mutualA](),
		reflect.TypeFor[mutualB](),
		reflect.TypeFor[recursiveUnion](),
	} {
		var doc oas.Document
		for _, reg := range []oas.Registry{nil, &doc} {
			schema := schemaFor(reg, rtype)
			if _, err := json.Marshal(schema); err != nil {
				t.Errorf("%v: %v", rtype, err)
			}
		}
		if _, err := json.Marshal(doc); err != nil {
			t.Errorf("%v: %v", rtype, err)
		}
	}
}

func TestRecursiveStructKeepsItsReference(t *testing.T) {
	var doc oas.Document
	schemaFor(&doc, reflect.TypeFor[recursiveStruct]())
	schema := doc.Components.Schemas["rest"].Defs["recursiveStruct"]
	if schema == nil {
		t.Fatal("recursiveStruct was not registered")
	}
	next := schema.Properties["next"]
	if next == nil || len(next.AllOf) != 1 || next.AllOf[0].Ref == "" {
		t.Errorf("next does not refer to recursiveStruct: %+v", next)
	}
}

func TestDocumentationOfRecursiveResults(t *testing.T) {
	type API struct {
		api.Specification
		Tree func(context.Context) (recursiveMap, error)    `rest:"GET /tree"`
		List func(context.Context) (recursiveSlice, error)  `rest:"GET /list"`
		Node func(context.Context) (recursiveStruct, error) `rest:"GET /node"`
		Put  func(context.Context, recursiveMap) error      `rest:"PUT /tree"`
	}
	h, err := Handler(nil, API{
		Tree: func(context.Context) (recursiveMap, error) { return recursiveMap{}, nil },
		List: func(context.Context) (recursiveSlice, error) { return recursiveSlice{}, nil },
		Node: func(context.Context) (recursiveStruct, error) { return recursiveStruct{}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ target, accept string }{
		{"/documentation", "application/json"},
		{"/documentation", "application/javascript"},
		{"/tree", "application/json+schema"},
		{"/list", "application/json+schema"},
		{"/node", "application/json+schema"},
		{"/tree?method=PUT", "application/schema+json"},
	} {
		req := httptest.NewRequest("GET", tc.target, nil)
		req.Header.Set("Accept", tc.accept)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s as %s: %d %q", tc.target, tc.accept, rec.Code, rec.Body)
		}
	}
}

// lopsidedBound has a Min that schemaFor cannot call.
type lopsidedBound int

func (lopsidedBound) Min(a, b int) int { return min(a, b) }
func (lopsidedBound) Max() string      { return "" }

type wellBounded int

func (wellBounded) Min() wellBounded { return 1 }
func (wellBounded) Max() wellBounded { return 9 }

func TestSchemaForChecksBoundMethods(t *testing.T) {
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("schemaFor panicked: %v", p)
		}
	}()
	if schema := schemaFor(nil, reflect.TypeFor[lopsidedBound]()); schema.Minimum != nil || schema.Maximum != nil {
		t.Errorf("a method that is no bound was used as one: %v %v", schema.Minimum, schema.Maximum)
	}
	schema := schemaFor(nil, reflect.TypeFor[wellBounded]())
	if schema.Minimum == nil || *schema.Minimum != 1 || schema.Maximum == nil || *schema.Maximum != 9 {
		t.Errorf("bounds lost: %v %v", schema.Minimum, schema.Maximum)
	}
}

func TestDocumentedContentTypes(t *testing.T) {
	type thing struct {
		Name string `json:"name"`
	}
	type API struct {
		api.Specification
		Echo  func(context.Context, string) (string, error) `rest:"POST(text/plain) /echo"`
		Xml   func(context.Context, thing) error            `rest:"POST(application/xml) /xml"`
		JSON  func(context.Context, thing) error            `rest:"POST /json"`
		Page  func(context.Context) (string, error)         `rest:"GET(text/html) /page"`
		Pairs func(context.Context, string, string) error   `rest:"POST(application/xml) /pairs (a,b)"`
	}
	var doc oas.Document
	structure := api.StructureOf(API{})
	for _, fn := range structure.Functions {
		if err := addFunctionTo(&doc, fn, "default", nil); err != nil {
			t.Fatal(err)
		}
	}
	// JSON stays among the types, as the server still takes and gives it,
	// and it was documented for every body before the declared type was.
	for path, want := range map[string]oas.ContentType{
		"/echo": "text/plain", "/xml": "application/xml", "/json": "application/json", "/pairs": "application/xml",
	} {
		content := doc.Paths[path].Post.RequestBody.Content
		_, declared := content[want]
		_, json := content["application/json"]
		if !declared || !json || len(content) > 2 {
			t.Errorf("%s request body %v, want %s and application/json", path, content, want)
		}
	}
	responses := doc.Paths["/page"].Get.Responses
	for _, response := range responses {
		_, html := response.Content["text/html"]
		_, json := response.Content["application/json"]
		if !html || !json {
			t.Errorf("/page response %v, want text/html and application/json", response.Content)
		}
	}
}

func TestDocumentedBodyArrays(t *testing.T) {
	type API struct {
		api.Specification
		Pair func(context.Context, string, int) error `rest:"POST /pair"`
	}
	var doc oas.Document
	if err := addFunctionTo(&doc, api.StructureOf(API{}).Functions[0], "default", nil); err != nil {
		t.Fatal(err)
	}
	schema := doc.Paths["/pair"].Post.RequestBody.Content["application/json"].Schema
	if schema == nil || len(schema.PrefixItems) != 2 {
		t.Errorf("schema %+v, want an array of two items", schema)
	}
}

// TestServeRecoveringAnswersAPanic is what the wasm host, which has no net/http
// server to recover a handler, relies on to outlive one.
func TestServeRecoveringAnswersAPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	serveRecovering(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	serveRecovering(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }), rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("status %d", rec.Code)
	}
}

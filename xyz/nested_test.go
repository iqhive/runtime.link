package xyz_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

// leafDecodes counts the leaves that the tests of nested unions decode.
var leafDecodes atomic.Int64

// countedLeaf is a bool that counts how often it is decoded, which is how
// often the union around it was.
type countedLeaf bool

func (leaf *countedLeaf) UnmarshalJSON(data []byte) error {
	leafDecodes.Add(1)
	return json.Unmarshal(data, (*bool)(leaf))
}

// nestedNamed is a union that holds itself under the names of its cases, as
// sql/std/sodium declares its Expression.
type nestedNamed xyz.Tagged[any, struct {
	Value xyz.Case[nestedNamed, countedLeaf]   `json:"value"`
	Avoid xyz.Case[nestedNamed, nestedNamed]   `json:"avoid"`
	Cases xyz.Case[nestedNamed, []nestedNamed] `json:"cases"`
}]

var nestedNameds = xyz.AccessorFor(nestedNamed.Values)

// nestedRuled is one that selects its cases by a discriminator.
type nestedRuled xyz.Tagged[any, struct {
	Not xyz.Case[nestedRuled, struct {
		Of *nestedRuled `json:"of"`
	}] `json:"?op=not"`
	All xyz.Case[nestedRuled, []nestedRuled] `json:"?op=all"`
	Is  xyz.Case[nestedRuled, struct {
		Value countedLeaf `json:"value"`
	}] `json:"?op=is"`
}]

// nestedAsText has a case that is read as text ahead of the one that holds
// the union, so that every case after it is read as text too.
type nestedAsText xyz.Tagged[any, struct {
	Count xyz.Case[nestedAsText, int]          `json:"count,string"`
	Value xyz.Case[nestedAsText, countedLeaf]  `json:"value"`
	Avoid xyz.Case[nestedAsText, nestedAsText] `json:"avoid"`
}]

// A client chooses how deep the JSON it sends is nested. A union that holds
// itself has to decode each level once, whether or not the value at the bottom
// decodes: the two passes of UnmarshalJSON each decoded the level below, so 16
// levels with a string for a bool at the bottom, 173 bytes, decoded the bottom
// 65536 times before they were refused.
func TestNestedUnionDecodesEachLevelOnce(t *testing.T) {
	const depth = 16
	for _, test := range []struct {
		name          string
		open, close   string
		leaf          string
		into          func() any
		decodesOfLeaf int64
	}{
		{name: "by name", open: `{"avoid":`, close: `}`, leaf: `{"value":%s}`, into: func() any { return new(nestedNamed) }, decodesOfLeaf: 1},
		{name: "by name in a list", open: `{"cases":[`, close: `]}`, leaf: `{"value":%s}`, into: func() any { return new(nestedNamed) }, decodesOfLeaf: 1},
		{name: "by a discriminator", open: `{"op":"not","of":`, close: `}`, leaf: `{"op":"is","value":%s}`, into: func() any { return new(nestedRuled) }, decodesOfLeaf: 1},
		// The list is tried under "value" and as the fields of the object,
		// which is an error before anything in it is decoded.
		{name: "by a discriminator in a list", open: `{"op":"all","value":[`, close: `]}`, leaf: `{"op":"is","value":%s}`, into: func() any { return new(nestedRuled) }, decodesOfLeaf: 1},
		{name: "read as text", open: `{"avoid":`, close: `}`, leaf: `{"value":%s}`, into: func() any { return new(nestedAsText) }, decodesOfLeaf: 1},
		{name: "indented", open: "{\n\t\"avoid\" : ", close: "\n}", leaf: "{ \"value\" : %s }", into: func() any { return new(nestedNamed) }, decodesOfLeaf: 1},
		{name: "indented, by a discriminator", open: "{\n \"op\" : \"not\" ,\n \"of\" : ", close: "\n}\n", leaf: "{ \"op\" : \"is\", \"value\" : %s }", into: func() any { return new(nestedRuled) }, decodesOfLeaf: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			nested := func(leaf string) []byte {
				return []byte(strings.Repeat(test.open, depth) + strings.Replace(test.leaf, "%s", leaf, 1) + strings.Repeat(test.close, depth))
			}
			leafDecodes.Store(0)
			if err := json.Unmarshal(nested(`"x"`), test.into()); err == nil {
				t.Errorf("a string for a bool, %d levels down, decoded", depth)
			}
			if got := leafDecodes.Load(); got > test.decodesOfLeaf {
				t.Errorf("%d bytes with a string for a bool %d levels down decoded the value at the bottom %d times before they were refused, want at most %d",
					len(nested(`"x"`)), depth, got, test.decodesOfLeaf)
			}
			leafDecodes.Store(0)
			if err := json.Unmarshal(nested(`true`), test.into()); err != nil {
				t.Errorf("a bool %d levels down: %v", depth, err)
			}
			if got := leafDecodes.Load(); got > test.decodesOfLeaf {
				t.Errorf("a bool %d levels down was decoded %d times, want at most %d", depth, got, test.decodesOfLeaf)
			}
		})
	}
}

// What decodes, and what does not, is as it was when every level was decoded
// by both passes.
func TestNestedUnionDecodesToTheSameValues(t *testing.T) {
	in := nestedNameds.Avoid.As(nestedNameds.Cases.As([]nestedNamed{
		nestedNameds.Value.As(true),
		nestedNameds.Avoid.As(nestedNameds.Value.As(false)),
	}))
	wire, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"avoid":{"cases":[{"value":true},{"avoid":{"value":false}}]}}`; string(wire) != want {
		t.Fatalf("marshalled as %s, want %s", wire, want)
	}
	var out nestedNamed
	if err := json.Unmarshal(wire, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("%s came back as %v", wire, out)
	}

	// A value that the first pass refuses and the second reads: the number of
	// a ",string" case, which is written as it is.
	var text nestedAsText
	if err := json.Unmarshal([]byte(`{"avoid":{"count":5}}`), &text); err != nil {
		t.Fatal(err)
	}
	if b, err := json.Marshal(text); err != nil || string(b) != `{"avoid":{"count":5}}` {
		t.Errorf(`{"avoid":{"count":5}} came back as %s, %v`, b, err)
	}

	// Unknown data at the bottom is kept, as it is at the top.
	var unknown nestedNamed
	if err := json.Unmarshal([]byte(`{"avoid":{"later":[1,2]}}`), &unknown); err != nil {
		t.Fatal(err)
	}
	if b, err := json.Marshal(unknown); err != nil || string(b) != `{"avoid":{"later":[1,2]}}` {
		t.Errorf("unknown data in a nested union came back as %s, %v", b, err)
	}

	// The error for a value that does not decode is the one of the first pass.
	var refused nestedNamed
	before := nestedNameds.Value.As(true)
	refused = before
	err = json.Unmarshal([]byte(`{"avoid":{"value":"x"}}`), &refused)
	if err == nil {
		t.Fatal("a string decoded as a bool")
	}
	if !reflect.DeepEqual(refused, before) {
		t.Errorf("a decode that failed changed the value to %v", refused)
	}
}

// scalarUnion is the first JSON example of the package doc: each case is
// matched by its JSON type.
type scalarUnion xyz.Tagged[any, struct {
	String xyz.Case[scalarUnion, string]  `json:",string"`
	Number xyz.Case[scalarUnion, float64] `json:",number"`
	List   xyz.Case[scalarUnion, []int]   `json:",array"`
}]

var scalarUnions = xyz.AccessorFor(scalarUnion.Values)

type heldObject struct {
	Field string `json:"field"`
}

type scalarOrObject xyz.Tagged[any, struct {
	String xyz.Case[scalarOrObject, string]     `json:",string"`
	Object xyz.Case[scalarOrObject, heldObject] `json:",object"`
}]

var scalarOrObjects = xyz.AccessorFor(scalarOrObject.Values)

// valueOrText can be written as {"value":"hi"} by one case and as "hi" by
// another, which under a discriminator are the same JSON.
type valueOrText xyz.Tagged[any, struct {
	Value xyz.Case[valueOrText, string] `json:"value"`
	Text  xyz.Case[valueOrText, string] `json:",string"`
}]

var valueOrTexts = xyz.AccessorFor(valueOrText.Values)

// holder selects its cases by a discriminator, the third JSON example of the
// package doc, and holds other unions as their values.
type holder xyz.Tagged[any, struct {
	Scalar  xyz.Case[holder, scalarUnion]    `json:"?type=scalar"`
	Mixed   xyz.Case[holder, scalarOrObject] `json:"?type=mixed"`
	Pointer xyz.Case[holder, *scalarUnion]   `json:"?type=pointer"`
	Either  xyz.Case[holder, valueOrText]    `json:"?type=either"`
}]

var holders = xyz.AccessorFor(holder.Values)

// A union held by a "?key=val" case is written under "value" when its own JSON
// is not an object. It was read back from the whole object, which the union
// kept as unknown data, or took for a case of its own that ignores the fields
// it does not know: the value came back without an error as another one.
func TestUnionHeldByADiscriminatedCaseRoundTrips(t *testing.T) {
	number := scalarUnions.Number.As(2.5)
	for _, test := range []struct {
		in   holder
		wire string
	}{
		{holders.Scalar.As(scalarUnions.String.As("hi")), `{"type":"scalar","value":"hi"}`},
		{holders.Scalar.As(number), `{"type":"scalar","value":2.5}`},
		{holders.Scalar.As(scalarUnions.List.As([]int{1, 2})), `{"type":"scalar","value":[1,2]}`},
		{holders.Mixed.As(scalarOrObjects.String.As("hi")), `{"type":"mixed","value":"hi"}`},
		{holders.Mixed.As(scalarOrObjects.Object.As(heldObject{Field: "f"})), `{"field":"f","type":"mixed"}`},
		{holders.Pointer.As(&number), `{"type":"pointer","value":2.5}`},
		{holders.Either.As(valueOrTexts.Value.As("hi")), `{"value":"hi","type":"either"}`},
	} {
		wire, err := json.Marshal(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if string(wire) != test.wire {
			t.Errorf("marshalled as %s, want %s", wire, test.wire)
		}
		var out holder
		if err := json.Unmarshal(wire, &out); err != nil {
			t.Errorf("%s does not decode: %v", wire, err)
			continue
		}
		if !reflect.DeepEqual(out, test.in) {
			t.Errorf("%s came back as %#v, want %#v", wire, out.Interface(), test.in.Interface())
		}
	}
}

// An object that is both the fields of one case of the held union and another
// of its cases under "value" is read as the fields, as it was, and so is one
// that neither reading gives a case.
func TestUnionHeldByADiscriminatedCaseKeepsWhatItRead(t *testing.T) {
	var either holder
	if err := json.Unmarshal([]byte(`{"type":"either","value":"hi"}`), &either); err != nil {
		t.Fatal(err)
	}
	if got := holders.Either.Get(either); got != valueOrTexts.Value.As("hi") {
		t.Errorf("an object that could be read both ways was read as %#v, want the case with the name", got.Interface())
	}

	// No case of the held union reads a bool, as the fields of an object or
	// under "value", so the object is kept by it as unknown data.
	const unknown = `{"type":"scalar","value":true}`
	var kept holder
	if err := json.Unmarshal([]byte(unknown), &kept); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(kept) != holders.Scalar {
		t.Fatalf("%s decoded as the case %v", unknown, xyz.ValueOf(kept))
	}
	if inner := holders.Scalar.Get(kept); !inner.IsZero() {
		t.Errorf("%s gave the held union the case %v", unknown, xyz.ValueOf(inner))
	}
	if b, err := json.Marshal(kept); err != nil || string(b) != unknown {
		t.Errorf("%s came back as %s, %v", unknown, b, err)
	}

	// A held union that is null has no case either.
	var null holder
	if err := json.Unmarshal([]byte(`{"type":"pointer","value":null}`), &null); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(null) != holders.Pointer {
		t.Errorf("a null under a discriminator decoded as the case %v", xyz.ValueOf(null))
	}
}

// laterHolder has a case declared ahead of the one that holds a union, which
// takes any object first and fails on this one, so that the union is reached
// by the shape of the JSON instead of by the order of the cases.
type laterHolder xyz.Tagged[any, struct {
	Counted xyz.Case[laterHolder, struct {
		Value int `json:"value"`
	}] `json:",object"`
	Scalar xyz.Case[laterHolder, scalarOrObject] `json:"?type=scalar"`
}]

var laterHolders = xyz.AccessorFor(laterHolder.Values)

func TestUnionHeldByADiscriminatedCaseIsReadByShapeToo(t *testing.T) {
	in := laterHolders.Scalar.As(scalarOrObjects.String.As("hi"))
	wire, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out laterHolder
	if err := json.Unmarshal(wire, &out); err != nil {
		t.Fatalf("%s does not decode: %v", wire, err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("%s came back as %#v, want %#v", wire, out.Interface(), in.Interface())
	}
}

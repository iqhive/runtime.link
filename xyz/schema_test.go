package xyz_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/xyz"
)

type schemaObject struct {
	Field string `json:"field"`
	Count int    `json:"count,omitempty"`
}

// SchemaEmbedded has a method, which [reflect.StructOf] cannot embed.
type SchemaEmbedded struct {
	Inner string `json:"inner"`
}

func (SchemaEmbedded) Describe() string { return "embedded" }

type schemaPromoted struct {
	Outer string `json:"outer"`
	SchemaEmbedded
}

type schemaShapes xyz.Tagged[any, struct {
	Literal schemaShapes                           `txt:"literal"`
	Format  xyz.Case[schemaShapes, int]            `txt:"id-%d"`
	Text    xyz.Case[schemaShapes, string]         `json:",string"`
	Quoted  xyz.Case[schemaShapes, int]            `json:",string"`
	Number  xyz.Case[schemaShapes, float64]        `json:",number"`
	Object  xyz.Case[schemaShapes, schemaObject]   `json:",object"`
	Array   xyz.Case[schemaShapes, []int]          `json:",array"`
	Named   xyz.Case[schemaShapes, schemaObject]   `json:"named"`
	Both    xyz.Case[schemaShapes, schemaObject]   `json:"payload?type=both"`
	Data    xyz.Case[schemaShapes, string]         `json:"data?type=data"`
	Flat    xyz.Case[schemaShapes, schemaObject]   `json:"?type=flat"`
	Scalar  xyz.Case[schemaShapes, int]            `json:"?type=scalar"`
	Time    xyz.Case[schemaShapes, time.Time]      `json:"?type=time"`
	Ptr     xyz.Case[schemaShapes, *schemaObject]  `json:"?type=ptr"`
	Map     xyz.Case[schemaShapes, map[string]int] `json:"?type=map"`
	Clash   xyz.Case[schemaShapes, inlineType]     `json:"?type=clash"`
	Promote xyz.Case[schemaShapes, schemaPromoted] `json:"?type=promote"`
	Void    schemaShapes                           `json:"?type=void"`
	Bare    xyz.Case[schemaShapes, schemaObject]
}]

var schemaShapess = xyz.AccessorFor(schemaShapes.Values)

// checkDescribes decodes the JSON of a value into the type that TypesJSON gave
// for its case: a field that the type leaves out is an error here, and one that
// it names wrongly comes back different.
func checkDescribes(t *testing.T, value any, schema reflect.Type) {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	target := reflect.New(schema)
	decoder := json.NewDecoder(strings.NewReader(string(wire)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target.Interface()); err != nil {
		t.Errorf("%v does not describe %s: %v", schema, wire, err)
		return
	}
	again, err := json.Marshal(target.Interface())
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(wire, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(again, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%v reads %s as %s", schema, wire, again)
	}
}

func TestTypesJSONDescribesWhatIsMarshalled(t *testing.T) {
	Vs := schemaShapess
	object := schemaObject{"x", 1}
	moment := time.Unix(0, 0).UTC()
	values := []schemaShapes{
		Vs.Literal,
		Vs.Format.As(5),
		Vs.Text.As("x"),
		Vs.Quoted.As(5),
		Vs.Number.As(1.5),
		Vs.Object.As(object),
		Vs.Array.As([]int{1, 2}),
		Vs.Named.As(object),
		Vs.Both.As(object),
		Vs.Data.As("x"),
		Vs.Flat.As(object),
		Vs.Scalar.As(5),
		Vs.Time.As(moment),
		Vs.Ptr.As(&object),
		Vs.Map.As(map[string]int{"a": 1}),
		Vs.Clash.As(inlineType{Name: "x"}),
		Vs.Promote.As(schemaPromoted{"a", SchemaEmbedded{"b"}}),
		Vs.Void,
		Vs.Bare.As(object),
	}
	types := schemaShapes{}.TypesJSON()
	if len(types) != len(values) {
		t.Fatalf("%d types for %d cases", len(types), len(values))
	}
	for i, value := range values {
		// As in 840f71d, a number of a ",string" case is written as a number,
		// and described as the string that it also reads.
		if value.Tag() == `json:",string"` && i == 3 {
			continue
		}
		checkDescribes(t, value, types[i])
	}
	if got := types[3]; got != reflect.TypeFor[string]() {
		t.Errorf("a number of a string case is described as %v", got)
	}
}

func TestTypesJSONOfAVoidPresenceRule(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Key union `json:"?key"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkDescribes(t, Unions.Key, union{}.TypesJSON()[0])
}

// SchemaLinked is embedded as a pointer, which encoding/json follows like any other.
type SchemaLinked struct {
	Link string `json:"link"`
}

// schemaNamed is a field that happens to be embedded, encoding/json keeps it whole.
type SchemaNamed string

func TestTypesJSONOfEmbeddedPointersAndNonStructs(t *testing.T) {
	type embedded struct {
		*SchemaLinked
		SchemaNamed
	}
	type union xyz.Tagged[any, struct {
		Value xyz.Case[union, embedded] `json:"?type=value"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkDescribes(t, Unions.Value.As(embedded{&SchemaLinked{"x"}, "y"}), union{}.TypesJSON()[0])
}

func TestTypesJSONOfAnArrayIsTheArray(t *testing.T) {
	types := schemaShapes{}.TypesJSON()
	if got := types[6]; got != reflect.TypeFor[[]int]() {
		t.Errorf("got %v", got)
	}
}

func TestExternReflection(t *testing.T) {
	type bare xyz.Extern[bare, any]
	if got := (bare{}).TypesJSON(); len(got) != 0 {
		t.Errorf("types of no cases: %v", got)
	}
	if got := (bare{}).Reflection(); len(got) != 0 {
		t.Errorf("reflection of no cases: %v", got)
	}

	type open xyz.Extern[open, any]
	type first xyz.Extend[open, any, struct {
		Text xyz.Case[open, string] `json:",string"`
	}]
	type second xyz.Extend[open, any, struct {
		Named xyz.Case[open, int] `json:"named" http:"404"`
	}]
	Firsts := xyz.AccessorFor(first.Values)
	Seconds := xyz.AccessorFor(second.Values)

	types := open{}.TypesJSON()
	if len(types) != 2 {
		t.Fatalf("got %v", types)
	}
	checkDescribes(t, Firsts.Text.As("x"), types[0])
	checkDescribes(t, Seconds.Named.As(5), types[1])

	cases := open{}.Reflection()
	if len(cases) != 2 || cases[0].Name != "Text" || cases[1].Name != "Named" {
		t.Fatalf("got %+v", cases)
	}
	if got := cases[1].Tags.Get("http"); got != "404" {
		t.Errorf("tag is %q", got)
	}
	if cases[0].Vary != reflect.TypeFor[string]() || cases[1].Vary != reflect.TypeFor[int]() {
		t.Errorf("types are %v and %v", cases[0].Vary, cases[1].Vary)
	}
	if !cases[1].Test(Seconds.Named.As(5)) || cases[0].Test(Seconds.Named.As(5)) {
		t.Error("Test cannot tell the cases apart")
	}
}

func TestReflectionDescribesEachCase(t *testing.T) {
	cases := schemaShapes{}.Reflection()
	if len(cases) != 19 {
		t.Fatalf("got %d cases", len(cases))
	}
	literal, number := cases[0], cases[4]
	if literal.Name != "Literal" || literal.Vary != nil {
		t.Errorf("void case: %+v", literal)
	}
	if number.Name != "Number" || number.Vary != reflect.TypeFor[float64]() || number.Tags.Get("json") != ",number" {
		t.Errorf("value case: %+v", number)
	}
	if !number.Test(schemaShapess.Number.As(1)) || number.Test(schemaShapess.Text.As("x")) || number.Test(schemaShapes{}) || number.Test(5) {
		t.Error("Test cannot tell the cases apart")
	}
}

// A case without a value reads any array, and is described as an array of
// empty objects, as it was in 840f71d.
func TestTypesJSONOfAnArrayCaseWithoutAValue(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Arr union `json:",array"`
	}]
	if got := (union{}).TypesJSON()[0]; got != reflect.TypeFor[[]struct{}]() {
		t.Errorf("got %v", got)
	}
}

// A named case with a discriminator keeps the field names it was described with
// in 840f71d, when it was taken to be under "value", so that one under "value"
// is described by the same type.
func TestTypesJSONOfANamedCaseWithADiscriminator(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Value xyz.Case[union, string] `json:"value?type=value"`
	}]
	want := reflect.StructOf([]reflect.StructField{
		{Name: "UnionValue", Type: reflect.TypeFor[string](), Tag: `json:"value"`},
		{Name: "UnionType", Type: reflect.TypeFor[string](), Tag: `json:"type"`},
	})
	if got := (union{}).TypesJSON()[0]; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

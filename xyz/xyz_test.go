package xyz_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

func TestTagged(t *testing.T) {
	type StringOrInt xyz.Tagged[any, struct {
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

func TestJSON(t *testing.T) {
	shouldBe := func(data []byte, err error) func(string) {
		if err != nil {
			t.Fatal(err)
		}
		return func(expected string) {
			if string(data) != expected {
				t.Fatalf("got %q; want %q", string(data), expected)
			}
		}
	}

	type Object struct {
		Field string `json:"field"`
	}
	// Each case may be matched by JSON type, the first type
	// match that unmarshals without an error, will win.
	type MyValue xyz.Tagged[any, struct {
		String xyz.Case[MyValue, string]  `json:",string"`
		Number xyz.Case[MyValue, float64] `json:",number"`
		Object xyz.Case[MyValue, Object]  `json:",object"`
		Array  xyz.Case[MyValue, []int]   `json:",array"`
	}]
	var MyValues = xyz.AccessorFor(MyValue.Values)
	var myvalue MyValue

	shouldBe(MyValues.String.As("hello").MarshalJSON())(`"hello"`)
	shouldBe(MyValues.Number.As(22).MarshalJSON())("22")

	myvalue.UnmarshalJSON([]byte(`"hello"`))
	if myvalue.String() != "hello" {
		t.Fatal("unexpected value")
	}
	myvalue.UnmarshalJSON([]byte(`22`))
	if MyValues.Number.Get(myvalue) != 22 {
		t.Fatal("unexpected value")
	}

	// An implicit object can be used with different field names
	// for each case.
	type MyValue2 xyz.Tagged[any, struct {
		String xyz.Case[MyValue2, string]  `json:"string"`
		Number xyz.Case[MyValue2, float64] `json:"number"`
	}]
	var MyValues2 = xyz.AccessorFor(MyValue2.Values)
	var myvalue2 MyValue2

	shouldBe(MyValues2.String.As("hello").MarshalJSON())(`{"string":"hello"}`)
	shouldBe(MyValues2.Number.As(22).MarshalJSON())(`{"number":22}`)

	myvalue2.UnmarshalJSON([]byte(`{"string":"hello"}`))
	if myvalue2.String() != "hello" {
		t.Fatal("unexpected value")
	}
	myvalue2.UnmarshalJSON([]byte(`{"number":22}`))
	if MyValues2.Number.Get(myvalue2) != 22 {
		t.Fatal("unexpected value")
	}

	// A discrimator field can be specified.
	type MyValue3 xyz.Tagged[any, struct {
		String xyz.Case[MyValue3, string]  `json:"value?type=string"`
		Number xyz.Case[MyValue3, float64] `json:"value?type=number"`
		Struct xyz.Case[MyValue3, Object]  `json:"?type=struct"`
	}]
	var MyValues3 = xyz.AccessorFor(MyValue3.Values)
	var myvalue3 MyValue3

	shouldBe(MyValues3.String.As("hello").MarshalJSON())(`{"type":"string","value":"hello"}`)
	shouldBe(MyValues3.Number.As(22).MarshalJSON())(`{"type":"number","value":22}`)
	shouldBe(MyValues3.Struct.As(Object{"1234"}).MarshalJSON())(`{"field":"1234","type":"struct"}`)

	myvalue3.UnmarshalJSON([]byte(`{"type":"string","value":"hello"}`))
	if myvalue3.String() != "hello" {
		t.Fatal("unexpected value")
	}
	myvalue3.UnmarshalJSON([]byte(`{"type":"number","value":22}`))
	if MyValues3.Number.Get(myvalue3) != 22 {
		t.Fatal("unexpected value")
	}
	if err := myvalue3.UnmarshalJSON([]byte(`{"field":"1234","type":"struct"}`)); err != nil {
		t.Fatal(err)
	}
	if MyValues3.Struct.Get(myvalue3) != (Object{"1234"}) {
		t.Fatal("unexpected value")
	}
}

type is[T io.Reader] xyz.Case[MyValue, T]

type MyValue xyz.Tagged[io.Reader, struct {
	Nil   MyValue
	Bufio is[*bufio.Reader]
	Bytes is[*bytes.Reader]
}]

func TestInterface(t *testing.T) {
	var MyValues = xyz.AccessorFor(MyValue.Values)
	var myvalue = MyValues.Bufio.As(bufio.NewReader(bytes.NewReader(nil)))

	switch xyz.ValueOf(myvalue) {
	case MyValues.Bufio:
	default:
		t.Fatal("unexpected value")
	}
}

type HelloWorld xyz.Static[HelloWorld, string]

func (HelloWorld) Value() string { return "Hello, World!" }

func TestStatic(t *testing.T) {
	var Object struct {
		HelloWorld HelloWorld
	}
	b, err := json.Marshal(Object)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"HelloWorld":"Hello, World!"}` {
		t.Fatal("unexpected value")
	}
}

// TestKeyRulesMatchInAnyOrder checks that a "?key" or "?key=val" rule
// selects its case only when the rule holds, so the order of the cases
// does not decide the result: a matching case later in the list must be
// reached even when an earlier case carries a rule too.
func TestKeyRulesMatchInAnyOrder(t *testing.T) {
	type Doc struct {
		Name string `json:"name"`
	}
	type Page struct {
		Docs []Doc `json:"documents"`
	}
	type Location xyz.Tagged[any, struct {
		Document  xyz.Case[Location, Doc]  `json:"?name"`
		Documents xyz.Case[Location, Page] `json:"?documents"`
	}]
	var Locations = xyz.AccessorFor(Location.Values)

	var loc Location
	if err := json.Unmarshal([]byte(`{"documents":[{"name":"a"},{"name":"b"}]}`), &loc); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(loc) != Locations.Documents || len(Locations.Documents.Get(loc).Docs) != 2 {
		t.Fatalf("a page decoded as %v", xyz.ValueOf(loc))
	}
	if err := json.Unmarshal([]byte(`{"name":"a"}`), &loc); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(loc) != Locations.Document || Locations.Document.Get(loc).Name != "a" {
		t.Fatalf("a document decoded as %v", xyz.ValueOf(loc))
	}

	type Object struct {
		Field string `json:"field"`
	}
	type Value xyz.Tagged[any, struct {
		Struct xyz.Case[Value, Object]  `json:"?type=struct"`
		String xyz.Case[Value, string]  `json:"value?type=string"`
		Number xyz.Case[Value, float64] `json:"value?type=number"`
	}]
	var Values = xyz.AccessorFor(Value.Values)

	var val Value
	if err := json.Unmarshal([]byte(`{"type":"string","value":"hello"}`), &val); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(val) != Values.String || Values.String.Get(val) != "hello" {
		t.Fatalf("a string decoded as %v", xyz.ValueOf(val))
	}
	if err := json.Unmarshal([]byte(`{"type":"number","value":22}`), &val); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(val) != Values.Number || Values.Number.Get(val) != 22 {
		t.Fatalf("a number decoded as %v", xyz.ValueOf(val))
	}
	if err := json.Unmarshal([]byte(`{"type":"struct","field":"1234"}`), &val); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(val) != Values.Struct || Values.Struct.Get(val) != (Object{"1234"}) {
		t.Fatalf("a struct decoded as %v", xyz.ValueOf(val))
	}
}

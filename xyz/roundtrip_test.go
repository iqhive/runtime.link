package xyz_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/xyz"
)

// roundTrippable is what a round trip can compare without knowing the type. Each
// case below has its own tag, so the tag says which case a value is.
type roundTrippable interface {
	Tag() reflect.StructTag
	Interface() any
}

func checkJSONRoundTrip[V roundTrippable](t *testing.T, in V, wire string) {
	t.Helper()
	got, err := json.Marshal(in)
	if err != nil {
		t.Errorf("want %s: %v", wire, err)
		return
	}
	if string(got) != wire {
		t.Errorf("marshalled %s, want %s", got, wire)
	}
	var out V
	if err := json.Unmarshal(got, &out); err != nil {
		t.Errorf("%s does not decode: %v", got, err)
		return
	}
	if out.Tag() != in.Tag() {
		t.Errorf("%s decoded as the case tagged %q, want %q", got, out.Tag(), in.Tag())
		return
	}
	if !reflect.DeepEqual(out.Interface(), in.Interface()) {
		t.Errorf("%s decoded to %#v, want %#v", got, out.Interface(), in.Interface())
	}
}

func checkTextRoundTrip[V roundTrippable, P interface {
	*V
	UnmarshalText([]byte) error
}](t *testing.T, in V, marshal func(V) ([]byte, error), wire string) {
	t.Helper()
	got, err := marshal(in)
	if err != nil {
		t.Errorf("want %s: %v", wire, err)
		return
	}
	if string(got) != wire {
		t.Errorf("marshalled %s, want %s", got, wire)
	}
	var out V
	if err := P(&out).UnmarshalText(got); err != nil {
		t.Errorf("%s does not decode: %v", got, err)
		return
	}
	if out.Tag() != in.Tag() {
		t.Errorf("%s decoded as the case tagged %q, want %q", got, out.Tag(), in.Tag())
		return
	}
	if !reflect.DeepEqual(out.Interface(), in.Interface()) {
		t.Errorf("%s decoded to %#v, want %#v", got, out.Interface(), in.Interface())
	}
}

type wireObject struct {
	Field string `json:"field"`
}

type presence struct {
	Present int `json:"present"`
}

// inOrder and inReverse declare the same cases, so anything that only round
// trips in one of them depends on the order of the cases.
type inOrder xyz.Tagged[any, struct {
	Literal inOrder                               `txt:"literal"`
	Format  xyz.Case[inOrder, int]                `txt:"user-%d"`
	Tuple   xyz.Case[inOrder, xyz.Pair[int, int]] `txt:"%d:%d"`
	Null    inOrder                               `json:",null"`
	Text    xyz.Case[inOrder, string]             `json:",string"`
	Number  xyz.Case[inOrder, float64]            `json:",number"`
	Object  xyz.Case[inOrder, wireObject]         `json:",object"`
	Array   xyz.Case[inOrder, []int]              `json:",array"`
	Named   xyz.Case[inOrder, int]                `json:"named"`
	Rule    xyz.Case[inOrder, string]             `json:"data?type=rule"`
	Merged  xyz.Case[inOrder, wireObject]         `json:"?type=merged"`
	Wrapped xyz.Case[inOrder, int]                `json:"?type=wrapped"`
	Void    inOrder                               `json:"?type=void"`
	Present xyz.Case[inOrder, presence]           `json:"?present"`
}]

type inReverse xyz.Tagged[any, struct {
	Present xyz.Case[inReverse, presence]           `json:"?present"`
	Void    inReverse                               `json:"?type=void"`
	Wrapped xyz.Case[inReverse, int]                `json:"?type=wrapped"`
	Merged  xyz.Case[inReverse, wireObject]         `json:"?type=merged"`
	Rule    xyz.Case[inReverse, string]             `json:"data?type=rule"`
	Named   xyz.Case[inReverse, int]                `json:"named"`
	Array   xyz.Case[inReverse, []int]              `json:",array"`
	Object  xyz.Case[inReverse, wireObject]         `json:",object"`
	Number  xyz.Case[inReverse, float64]            `json:",number"`
	Text    xyz.Case[inReverse, string]             `json:",string"`
	Null    inReverse                               `json:",null"`
	Tuple   xyz.Case[inReverse, xyz.Pair[int, int]] `txt:"%d:%d"`
	Format  xyz.Case[inReverse, int]                `txt:"user-%d"`
	Literal inReverse                               `txt:"literal"`
}]

var (
	inOrders   = xyz.AccessorFor(inOrder.Values)
	inReverses = xyz.AccessorFor(inReverse.Values)
)

// Every shape round trips in reverse order. In order, the ",string" case comes
// before the cases with a format, and takes their text as it did in 840f71d.
func TestEveryShapeRoundTripsInAnyCaseOrder(t *testing.T) {
	wires := []string{
		`"literal"`,
		`"user-5"`,
		`"1:2"`,
		`null`,
		`"hello world"`,
		`22`,
		`{"field":"x"}`,
		`[1,2]`,
		`{"named":5}`,
		`{"data":"x","type":"rule"}`,
		`{"field":"x","type":"merged"}`,
		`{"type":"wrapped","value":5}`,
		`{"type":"void"}`,
		`{"present":1}`,
	}
	forwards := []inOrder{
		inOrders.Literal,
		inOrders.Format.As(5),
		inOrders.Tuple.As(xyz.NewPair(1, 2)),
		inOrders.Null,
		inOrders.Text.As("hello world"),
		inOrders.Number.As(22),
		inOrders.Object.As(wireObject{"x"}),
		inOrders.Array.As([]int{1, 2}),
		inOrders.Named.As(5),
		inOrders.Rule.As("x"),
		inOrders.Merged.As(wireObject{"x"}),
		inOrders.Wrapped.As(5),
		inOrders.Void,
		inOrders.Present.As(presence{1}),
	}
	backwards := []inReverse{
		inReverses.Literal,
		inReverses.Format.As(5),
		inReverses.Tuple.As(xyz.NewPair(1, 2)),
		inReverses.Null,
		inReverses.Text.As("hello world"),
		inReverses.Number.As(22),
		inReverses.Object.As(wireObject{"x"}),
		inReverses.Array.As([]int{1, 2}),
		inReverses.Named.As(5),
		inReverses.Rule.As("x"),
		inReverses.Merged.As(wireObject{"x"}),
		inReverses.Wrapped.As(5),
		inReverses.Void,
		inReverses.Present.As(presence{1}),
	}
	for i, wire := range wires {
		checkJSONRoundTrip(t, backwards[i], wire)
		if forwards[i].Tag().Get("txt") == "user-%d" || forwards[i].Tag().Get("txt") == "%d:%d" {
			continue
		}
		checkJSONRoundTrip(t, forwards[i], wire)
	}
	for _, text := range []string{"user-5", "1:2"} {
		var v inOrder
		quoted, _ := json.Marshal(text)
		if err := json.Unmarshal(quoted, &v); err != nil {
			t.Fatal(err)
		}
		if xyz.ValueOf(v) != inOrders.Text || inOrders.Text.Get(v) != text {
			t.Errorf("%s decoded to %v", quoted, v)
		}
	}
}

func TestTextShapesRoundTripInAnyCaseOrder(t *testing.T) {
	marshalForwards := func(v inOrder) ([]byte, error) { return v.MarshalText() }
	marshalBackwards := func(v inReverse) ([]byte, error) { return v.MarshalText() }
	checkTextRoundTrip(t, inOrders.Literal, marshalForwards, "literal")
	checkTextRoundTrip(t, inOrders.Format.As(5), marshalForwards, "user-5")
	checkTextRoundTrip(t, inOrders.Tuple.As(xyz.NewPair(1, 2)), marshalForwards, "1:2")
	checkTextRoundTrip(t, inReverses.Literal, marshalBackwards, "literal")
	checkTextRoundTrip(t, inReverses.Format.As(5), marshalBackwards, "user-5")
	checkTextRoundTrip(t, inReverses.Tuple.As(xyz.NewPair(1, 2)), marshalBackwards, "1:2")
}

// The cases are tried in the order they are declared, as in 840f71d, so a
// ",string" case declared first takes the text of any case after it.
func TestStringCaseDeclaredFirstTakesTheText(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Any   xyz.Case[union, string] `json:",string"`
		Other xyz.Case[union, string] `txt:"other: %v"`
		Rest  xyz.Case[union, string] `txt:"%v"`
		None  union                   `txt:"none"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	for _, text := range []string{"none", "other: x"} {
		var v union
		quoted, _ := json.Marshal(text)
		if err := json.Unmarshal(quoted, &v); err != nil {
			t.Fatal(err)
		}
		if xyz.ValueOf(v) != Unions.Any || Unions.Any.Get(v) != text {
			t.Errorf("%s decoded to %v", quoted, v)
		}
	}
	marshal := func(v union) ([]byte, error) { return v.MarshalText() }
	checkTextRoundTrip(t, Unions.None, marshal, "none")
	checkTextRoundTrip(t, Unions.Other.As("x"), marshal, "other: x")
}

// An object case declared first takes any object, as in 840f71d. Declared
// after the others, it leaves them to round trip.
func TestObjectCaseDeclaredFirstTakesAnyObject(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Any   xyz.Case[union, wireObject] `json:",object"`
		Named xyz.Case[union, int]        `json:"named"`
		Rule  xyz.Case[union, wireObject] `json:"?type=rule"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Any.As(wireObject{"x"}), `{"field":"x"}`)
	for input, want := range map[string]wireObject{`{"named":5}`: {}, `{"field":"x","type":"rule"}`: {"x"}} {
		var v union
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			t.Fatal(err)
		}
		if xyz.ValueOf(v) != Unions.Any || Unions.Any.Get(v) != want {
			t.Errorf("%s decoded to %v", input, v)
		}
	}

	type last xyz.Tagged[any, struct {
		Named xyz.Case[last, int]        `json:"named"`
		Rule  xyz.Case[last, wireObject] `json:"?type=rule"`
		Any   xyz.Case[last, wireObject] `json:",object"`
	}]
	Lasts := xyz.AccessorFor(last.Values)
	checkJSONRoundTrip(t, Lasts.Named.As(5), `{"named":5}`)
	checkJSONRoundTrip(t, Lasts.Rule.As(wireObject{"x"}), `{"field":"x","type":"rule"}`)
	checkJSONRoundTrip(t, Lasts.Any.As(wireObject{"x"}), `{"field":"x"}`)
}

// The kind after the comma describes the value of a named case, which sits
// under its key, not the object around it.
func TestKindOfANamedCaseIsCheckedAgainstItsValue(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Text   xyz.Case[union, string]     `json:"text,string"`
		Number xyz.Case[union, float64]    `json:"number,number"`
		Object xyz.Case[union, wireObject] `json:"object,object"`
		Array  xyz.Case[union, []int]      `json:"array,array"`
		SameS  xyz.Case[union, string]     `json:"same,string"`
		SameN  xyz.Case[union, float64]    `json:"same,number"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Text.As("x"), `{"text":"x"}`)
	checkJSONRoundTrip(t, Unions.Number.As(5), `{"number":5}`)
	checkJSONRoundTrip(t, Unions.Object.As(wireObject{"x"}), `{"object":{"field":"x"}}`)
	checkJSONRoundTrip(t, Unions.Array.As([]int{1}), `{"array":[1]}`)
	checkJSONRoundTrip(t, Unions.SameS.As("x"), `{"same":"x"}`)
	checkJSONRoundTrip(t, Unions.SameN.As(5), `{"same":5}`)
}

func TestKindOfARuleCaseDoesNotHideIt(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Number xyz.Case[union, float64]    `json:"?type=number,number"`
		Object xyz.Case[union, wireObject] `json:"?type=object,object"`
		Named  xyz.Case[union, string]     `json:"data?type=named,string"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Number.As(5), `{"type":"number","value":5}`)
	checkJSONRoundTrip(t, Unions.Object.As(wireObject{"x"}), `{"field":"x","type":"object"}`)
	checkJSONRoundTrip(t, Unions.Named.As("x"), `{"data":"x","type":"named"}`)
}

type aNumber struct {
	A int `json:"a"`
}

type aString struct {
	A string `json:"a"`
}

type twoObjects xyz.Tagged[any, struct {
	Number xyz.Case[twoObjects, aNumber] `json:",object"`
	Text   xyz.Case[twoObjects, aString] `json:",object"`
}]

var twoObjectss = xyz.AccessorFor(twoObjects.Values)

func TestFirstCaseThatDecodesWins(t *testing.T) {
	checkJSONRoundTrip(t, twoObjectss.Number.As(aNumber{1}), `{"a":1}`)
	checkJSONRoundTrip(t, twoObjectss.Text.As(aString{"x"}), `{"a":"x"}`)
}

func TestErrorWhenNoCaseThatMatchesDecodes(t *testing.T) {
	v := twoObjectss.Text.As(aString{"kept"})
	if err := json.Unmarshal([]byte(`{"a":[1]}`), &v); err == nil {
		t.Fatal("no case can hold an array")
	}
	if xyz.ValueOf(v) != twoObjectss.Text || twoObjectss.Text.Get(v).A != "kept" {
		t.Errorf("the failure changed the value to %v", v)
	}
}

func TestCasesOfOneKeyOrRuleFallThroughLikeAnyOther(t *testing.T) {
	type union xyz.Tagged[any, struct {
		KeyN  xyz.Case[union, int]     `json:"key"`
		KeyS  xyz.Case[union, string]  `json:"key"`
		RuleN xyz.Case[union, int]     `json:"data?type=rule"`
		RuleS xyz.Case[union, string]  `json:"data?type=rule"`
		MoreN xyz.Case[union, aNumber] `json:"?type=more"`
		MoreS xyz.Case[union, aString] `json:"?type=more"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.KeyS.As("x"), `{"key":"x"}`)
	checkJSONRoundTrip(t, Unions.RuleS.As("x"), `{"data":"x","type":"rule"}`)
	checkJSONRoundTrip(t, Unions.MoreS.As(aString{"x"}), `{"a":"x","type":"more"}`)
}

type stringHint xyz.Tagged[any, struct {
	A xyz.Case[stringHint, int] `json:"a,string"`
	B xyz.Case[stringHint, int] `json:"b"`
}]

var stringHints = xyz.AccessorFor(stringHint.Values)

func TestStringHintStaysOnItsOwnCase(t *testing.T) {
	checkJSONRoundTrip(t, stringHints.B.As(5), `{"b":5}`)
	checkJSONRoundTrip(t, stringHints.A.As(5), `{"a":5}`)
}

func TestNamedStringHintReadsAQuotedNumber(t *testing.T) {
	var v stringHint
	if err := json.Unmarshal([]byte(`{"a":"5"}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != stringHints.A || stringHints.A.Get(v) != 5 {
		t.Errorf("got %v", v)
	}
}

type quotedKinds xyz.Tagged[any, struct {
	Time   xyz.Case[quotedKinds, time.Time] `json:",string"`
	Number xyz.Case[quotedKinds, float64]   `json:",number"`
}]

var quotedKindss = xyz.AccessorFor(quotedKinds.Values)

func TestStringCaseOnlyMatchesAJSONString(t *testing.T) {
	checkJSONRoundTrip(t, quotedKindss.Number.As(22), `22`)
	moment := time.Unix(0, 0).UTC()
	checkJSONRoundTrip(t, quotedKindss.Time.As(moment), `"1970-01-01T00:00:00Z"`)
}

// A ",string" case of []byte reads a JSON string as the bytes of its text, as
// it did in 840f71d, though it writes them in base64.
func TestStringCaseOfBytesReadsTheText(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Bytes xyz.Case[union, []byte] `json:",string"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	b, err := json.Marshal(Unions.Bytes.As([]byte("hi")))
	if err != nil || string(b) != `"aGk="` {
		t.Fatalf("got %s, %v", b, err)
	}
	var v union
	if err := json.Unmarshal([]byte(`"hello"`), &v); err != nil {
		t.Fatal(err)
	}
	if got := Unions.Bytes.Get(v); string(got) != "hello" {
		t.Errorf("got %q", got)
	}
}

type quotedNumber xyz.Tagged[any, struct {
	Count  xyz.Case[quotedNumber, int]     `json:",string"`
	Number xyz.Case[quotedNumber, float64] `json:",number"`
}]

var quotedNumbers = xyz.AccessorFor(quotedNumber.Values)

// A number of a ",string" case is written as a number, as in 840f71d, which
// reads as a ",number" case where there is one. It reads a quoted number too.
func TestNumberOfAStringCaseIsWrittenAsANumber(t *testing.T) {
	for _, in := range []quotedNumber{quotedNumbers.Count.As(5), quotedNumbers.Number.As(5)} {
		if b, err := json.Marshal(in); err != nil || string(b) != `5` {
			t.Errorf("got %s, %v", b, err)
		}
	}
	var v quotedNumber
	if err := json.Unmarshal([]byte(`5`), &v); err != nil || xyz.ValueOf(v) != quotedNumbers.Number {
		t.Errorf("5 decoded to %v, %v", v, err)
	}
	if err := json.Unmarshal([]byte(`"5"`), &v); err != nil || xyz.ValueOf(v) != quotedNumbers.Count || quotedNumbers.Count.Get(v) != 5 {
		t.Errorf(`"5" decoded to %v, %v`, v, err)
	}
	type alone xyz.Tagged[any, struct {
		Count xyz.Case[alone, int] `json:",string"`
	}]
	Alones := xyz.AccessorFor(alone.Values)
	checkJSONRoundTrip(t, Alones.Count.As(5), `5`)
}

func TestScalarsInAStringCaseRoundTrip(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Flag xyz.Case[union, bool] `json:",string"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Flag.As(true), `true`)
	var v union
	if err := json.Unmarshal([]byte(`"true"`), &v); err != nil || !Unions.Flag.Get(v) {
		t.Errorf(`"true" decoded to %v, %v`, v, err)
	}

	type floats xyz.Tagged[any, struct {
		Float xyz.Case[floats, float64] `json:",string"`
	}]
	Floats := xyz.AccessorFor(floats.Values)
	checkJSONRoundTrip(t, Floats.Float.As(1.5), `1.5`)
}

// A quoted number is read with fmt.Sscan, as it was in 840f71d, which takes any
// form of a number that fmt reads and leaves out what follows it.
func TestQuotedNumberIsScanned(t *testing.T) {
	for input, want := range map[string]int{`"5 6"`: 5, `"5x"`: 5, `"0x10"`: 16} {
		var v quotedNumber
		if err := json.Unmarshal([]byte(input), &v); err != nil || xyz.ValueOf(v) != quotedNumbers.Count || quotedNumbers.Count.Get(v) != want {
			t.Errorf("%s decoded to %v, %v, want %d", input, v, err, want)
		}
	}
	for _, input := range []string{`"x"`, `""`, `"null"`} {
		v := quotedNumbers.Number.As(1)
		if err := json.Unmarshal([]byte(input), &v); err == nil {
			t.Errorf("%s decoded to %v", input, v)
		}
		if xyz.ValueOf(v) != quotedNumbers.Number {
			t.Errorf("%s changed the value to %v", input, v)
		}
	}
}

type userID xyz.Tagged[any, struct {
	User xyz.Case[userID, int]                `txt:"user-%d"`
	Span xyz.Case[userID, xyz.Pair[int, int]] `txt:"%d..%d"`
	Name xyz.Case[userID, string]             `txt:"name: %v"`
	Anon userID                               `txt:"anonymous"`
}]

var userIDs = xyz.AccessorFor(userID.Values)

func TestFormatCaseRoundTrip(t *testing.T) {
	marshal := func(v userID) ([]byte, error) { return v.MarshalText() }
	checkJSONRoundTrip(t, userIDs.User.As(5), `"user-5"`)
	checkTextRoundTrip(t, userIDs.User.As(5), marshal, `user-5`)
	checkJSONRoundTrip(t, userIDs.User.As(-5), `"user--5"`)
	checkJSONRoundTrip(t, userIDs.Span.As(xyz.NewPair(1, 2)), `"1..2"`)
	checkTextRoundTrip(t, userIDs.Span.As(xyz.NewPair(1, 2)), marshal, `1..2`)
	checkJSONRoundTrip(t, userIDs.Name.As("hello world"), `"name: hello world"`)
	checkTextRoundTrip(t, userIDs.Name.As("hello world"), marshal, `name: hello world`)
	checkTextRoundTrip(t, userIDs.Anon, marshal, `anonymous`)
}

// Text that formatting a value would not produce is not that case, and no case
// has it, so it leaves a value as it was, and is unknown data in JSON, as in
// 840f71d. The format itself is the text of its case, as it was in 840f71d.
func TestFormatCaseIgnoresOtherText(t *testing.T) {
	for _, input := range []string{"user-", "user-x", "user-5x", "user-05", "user-5 ", "1..", "..2", "1..2..3"} {
		v := userIDs.Anon
		if err := v.UnmarshalText([]byte(input)); err != nil {
			t.Errorf("%q: %v", input, err)
		}
		if v != userIDs.Anon {
			t.Errorf("%q changed the value to %v", input, xyz.ValueOf(v))
		}
		var decoded userID
		quoted, _ := json.Marshal(input)
		if err := json.Unmarshal(quoted, &decoded); err != nil {
			t.Errorf("%q: %v", input, err)
		}
		if !decoded.IsZero() {
			t.Errorf("%q decoded as JSON to %v", input, xyz.ValueOf(decoded))
		}
	}
	var v userID
	if err := v.UnmarshalText([]byte("user-%d")); err != nil || xyz.ValueOf(v) != userIDs.User {
		t.Errorf("the text of the case decoded to %v, %v", xyz.ValueOf(v), err)
	}
	var decoded userID
	if err := json.Unmarshal([]byte(`"user-%d"`), &decoded); err != nil || xyz.ValueOf(decoded) != userIDs.User {
		t.Errorf("the text of the case decoded as JSON to %v, %v", xyz.ValueOf(decoded), err)
	}
}

func TestFormatWithPercentAndWidth(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Share xyz.Case[union, int]                                       `txt:"%d%%"`
		Hex   xyz.Case[union, uint32]                                    `txt:"0x%08x"`
		Fixed xyz.Case[union, float64]                                   `txt:"~%.2f"`
		Flag  xyz.Case[union, bool]                                      `txt:"flag=%t"`
		Quote xyz.Case[union, string]                                    `txt:"quoted %q"`
		Bytes xyz.Case[union, []byte]                                    `txt:"bytes %s"`
		Deep  xyz.Case[union, xyz.Trio[int, xyz.Pair[int, int], string]] `txt:"%d|%d|%d|%v"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Share.As(50), `"50%"`)
	checkJSONRoundTrip(t, Unions.Hex.As(255), `"0x000000ff"`)
	checkJSONRoundTrip(t, Unions.Fixed.As(1.25), `"~1.25"`)
	checkJSONRoundTrip(t, Unions.Flag.As(true), `"flag=true"`)
	checkJSONRoundTrip(t, Unions.Quote.As("a b"), `"quoted \"a b\""`)
	checkJSONRoundTrip(t, Unions.Bytes.As([]byte("a b")), `"bytes a b"`)
	checkJSONRoundTrip(t, Unions.Deep.As(xyz.NewTrio(1, xyz.NewPair(2, 3), "a b")), `"1|2|3|a b"`)
}

type textualKey xyz.Tagged[any, struct {
	A textualKey                `txt:"a"`
	N xyz.Case[textualKey, int] `txt:"n%d"`
}]

var textualKeys = xyz.AccessorFor(textualKey.Values)

func TestTextCasesRoundTripAsMapKeys(t *testing.T) {
	in := map[textualKey]int{textualKeys.A: 1, textualKeys.N.As(5): 2}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[textualKey]int
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	if len(out) != 2 || out[textualKeys.A] != 1 || out[textualKeys.N.As(5)] != 2 {
		t.Errorf("%s decoded to %v", b, out)
	}
}

type textOrNumber xyz.Tagged[any, struct {
	Num xyz.Case[textOrNumber, int]    `json:",number"`
	A   xyz.Case[textOrNumber, string] `txt:"a"`
}]

var textOrNumbers = xyz.AccessorFor(textOrNumber.Values)

func TestZeroValueRoundTripsAsText(t *testing.T) {
	var zero textOrNumber
	text, err := zero.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var back textOrNumber
	if err := back.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if !back.IsZero() {
		t.Errorf("the zero value became %v", xyz.ValueOf(back))
	}
	v := textOrNumbers.A.As("x")
	if err := v.UnmarshalText(nil); err != nil {
		t.Fatal(err)
	}
	if !v.IsZero() {
		t.Errorf("empty text kept %v", xyz.ValueOf(v))
	}
}

// Text that no case has leaves the value as it was, without an error, as in 840f71d.
func TestUnmarshalTextLeavesUnknownTextAlone(t *testing.T) {
	v := textOrNumbers.A.As("x")
	if err := v.UnmarshalText([]byte("no-such-case")); err != nil {
		t.Error(err)
	}
	if xyz.ValueOf(v) != textOrNumbers.A || textOrNumbers.A.Get(v) != "x" {
		t.Errorf("the value changed to %v", xyz.ValueOf(v))
	}
}

func TestEmptyTextCaseIsNotTheZeroValue(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Num   xyz.Case[union, int] `json:",number"`
		Empty union                `txt:""`
	}]
	Unions := xyz.AccessorFor(union.Values)
	var v union
	if err := v.UnmarshalText(nil); err != nil {
		t.Fatal(err)
	}
	if v != Unions.Empty {
		t.Errorf("got %v", xyz.ValueOf(v))
	}
	checkJSONRoundTrip(t, Unions.Empty, `""`)
}

func TestNumericStorageKeepsTheValue(t *testing.T) {
	type u64 xyz.Tagged[uint64, struct {
		A xyz.Case[u64, uint32]
		B xyz.Case[u64, uintptr]
		F xyz.Case[u64, float64]
		I xyz.Case[u64, int8]
		T xyz.Case[u64, bool]
		L xyz.Case[u64, [2]uint16]
		S xyz.Case[u64, struct{ X, Y uint16 }]
	}]
	U64s := xyz.AccessorFor(u64.Values)
	if got := U64s.A.Get(U64s.A.As(math.MaxUint32)); got != math.MaxUint32 {
		t.Errorf("uint32: %#x", got)
	}
	if got := U64s.B.Get(U64s.B.As(0x1234)); got != 0x1234 {
		t.Errorf("uintptr: %#x", got)
	}
	if got := U64s.F.Get(U64s.F.As(1.5)); got != 1.5 {
		t.Errorf("float64: %v", got)
	}
	if got := U64s.I.Get(U64s.I.As(-5)); got != -5 {
		t.Errorf("int8: %v", got)
	}
	if got := U64s.T.Get(U64s.T.As(true)); !got {
		t.Errorf("bool: %v", got)
	}
	if got := U64s.L.Get(U64s.L.As([2]uint16{1, 2})); got != [2]uint16{1, 2} {
		t.Errorf("array: %v", got)
	}
	if got := U64s.S.Get(U64s.S.As(struct{ X, Y uint16 }{1, 2})); got != (struct{ X, Y uint16 }{1, 2}) {
		t.Errorf("struct: %v", got)
	}
	if raw, ok := U64s.B.As(0x1234).Get(); !ok || raw != 0x1234 {
		t.Errorf("the storage holds %#x, %v", raw, ok)
	}

	type i8 xyz.Tagged[int8, struct {
		A xyz.Case[i8, int8]
		B xyz.Case[i8, uint8]
	}]
	I8s := xyz.AccessorFor(i8.Values)
	if got := I8s.B.Get(I8s.B.As(200)); got != 200 {
		t.Errorf("uint8: %v", got)
	}
	if got := I8s.A.Get(I8s.A.As(-7)); got != -7 {
		t.Errorf("int8: %v", got)
	}

	type f64 xyz.Tagged[float64, struct {
		A xyz.Case[f64, float64]
		B xyz.Case[f64, int64]
	}]
	F64s := xyz.AccessorFor(f64.Values)
	if got := F64s.A.Get(F64s.A.As(2.5)); got != 2.5 {
		t.Errorf("float64: %v", got)
	}
	if got := F64s.B.Get(F64s.B.As(-9)); got != -9 {
		t.Errorf("int64: %v", got)
	}

	type c128 xyz.Tagged[complex128, struct {
		A xyz.Case[c128, complex128]
		B xyz.Case[c128, float64]
	}]
	C128s := xyz.AccessorFor(c128.Values)
	if got := C128s.A.Get(C128s.A.As(1 + 2i)); got != 1+2i {
		t.Errorf("complex128: %v", got)
	}
	if got := C128s.B.Get(C128s.B.As(3.5)); got != 3.5 {
		t.Errorf("float64: %v", got)
	}

	type flag xyz.Tagged[bool, struct {
		No  xyz.Case[flag, bool]
		Yes xyz.Case[flag, bool]
	}]
	Flags := xyz.AccessorFor(flag.Values)
	if !Flags.No.Get(Flags.No.As(true)) || Flags.Yes.Get(Flags.Yes.As(false)) {
		t.Error("bool storage kept the position of the case and not its value")
	}
}

func TestNumericStorageRoundTripsThroughJSON(t *testing.T) {
	type union xyz.Tagged[uint64, struct {
		A xyz.Case[union, uint32]  `json:"a"`
		B xyz.Case[union, uintptr] `json:"b"`
		C xyz.Case[union, float64] `json:",number"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.A.As(0x1234), `{"a":4660}`)
	checkJSONRoundTrip(t, Unions.B.As(0x1234), `{"b":4660}`)
	checkJSONRoundTrip(t, Unions.C.As(1.5), `1.5`)
}

type stringStored xyz.Tagged[string, struct {
	Point xyz.Case[stringStored, xyz.Pair[int, int]]                   `txt:"%d:%d"`
	Trio  xyz.Case[stringStored, xyz.Trio[int, int, int]]              `txt:"%d/%d/%d"`
	Quad  xyz.Case[stringStored, xyz.Quad[int, int, int, string]]      `txt:"%d;%d;%d;%v"`
	Deep  xyz.Case[stringStored, xyz.Pair[xyz.Pair[int, int], string]] `txt:"(%d,%d) %v"`
	Num   xyz.Case[stringStored, float64]                              `txt:"#%v"`
	Flag  xyz.Case[stringStored, bool]                                 `txt:"?%v"`
	Plain xyz.Case[stringStored, int]                                  `txt:"plain"`
	Whole xyz.Case[stringStored, struct{ A, B int }]                   `txt:"whole %v"`
	Text  xyz.Case[stringStored, string]                               `txt:"%v"`
}]

var stringStoreds = xyz.AccessorFor(stringStored.Values)

func TestStringStorageKeepsTheValue(t *testing.T) {
	for _, text := range []string{"hello world", "", "  padded  ", "a\nb\tc", "日本語 テキスト", "100%", "%d"} {
		v := stringStoreds.Text.As(text)
		if got := stringStoreds.Text.Get(v); got != text {
			t.Errorf("%q came back as %q", text, got)
		}
		if got := v.String(); got != text {
			t.Errorf("String of %q is %q", text, got)
		}
	}
	point := xyz.NewPair(1, 2)
	if got := stringStoreds.Point.Get(stringStoreds.Point.As(point)); got != point {
		t.Errorf("pair: %v", got)
	}
	trio := xyz.NewTrio(1, 2, 3)
	if got := stringStoreds.Trio.Get(stringStoreds.Trio.As(trio)); got != trio {
		t.Errorf("trio: %v", got)
	}
	quad := xyz.NewQuad(1, 2, 3, "a b")
	if got := stringStoreds.Quad.Get(stringStoreds.Quad.As(quad)); got != quad {
		t.Errorf("quad: %v", got)
	}
	deep := xyz.NewPair(xyz.NewPair(1, 2), "a b")
	if got := stringStoreds.Deep.Get(stringStoreds.Deep.As(deep)); got != deep {
		t.Errorf("nested tuple: %v", got)
	}
	if got := stringStoreds.Num.Get(stringStoreds.Num.As(1.5)); got != 1.5 {
		t.Errorf("float: %v", got)
	}
	if got := stringStoreds.Flag.Get(stringStoreds.Flag.As(true)); !got {
		t.Errorf("bool: %v", got)
	}
	if got := stringStoreds.Plain.Get(stringStoreds.Plain.As(5)); got != 0 {
		t.Errorf("a case whose text has no format has no value to read back, got %v", got)
	}
	if got, want := stringStoreds.Point.As(point).String(), "1:2"; got != want {
		t.Errorf("String is %q, want %q", got, want)
	}
	if got := stringStoreds.Point.As(point).Interface(); got != point {
		t.Errorf("Interface is %v", got)
	}
}

func TestStringStorageRoundTrips(t *testing.T) {
	marshal := func(v stringStored) ([]byte, error) { return v.MarshalText() }
	checkJSONRoundTrip(t, stringStoreds.Text.As("hello world"), `"hello world"`)
	checkTextRoundTrip(t, stringStoreds.Text.As("hello world"), marshal, `hello world`)
	checkJSONRoundTrip(t, stringStoreds.Point.As(xyz.NewPair(1, 2)), `"1:2"`)
	checkTextRoundTrip(t, stringStoreds.Point.As(xyz.NewPair(1, 2)), marshal, `1:2`)
	checkJSONRoundTrip(t, stringStoreds.Deep.As(xyz.NewPair(xyz.NewPair(1, 2), "a b")), `"(1,2) a b"`)
}

// A decoded value has to be the same value as the one that was encoded, down to
// what is in its storage, or the two are not equal.
func TestDecodedValueEqualsTheOneEncoded(t *testing.T) {
	for _, in := range []stringStored{
		stringStoreds.Text.As("hello world"),
		stringStoreds.Point.As(xyz.NewPair(1, 2)),
		stringStoreds.Plain.As(0),
	} {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var out stringStored
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		if out != in {
			t.Errorf("%s decoded to a different value", b)
		}
	}
}

// Text is all that string storage keeps, and a value that cannot be scanned back
// out of it still reads as the text that it was stored as.
func TestStringStorageKeepsTextThatCannotBeScanned(t *testing.T) {
	v := stringStoreds.Whole.As(struct{ A, B int }{1, 2})
	if got := v.String(); got != "whole {1 2}" {
		t.Errorf("String is %q", got)
	}
	if b, err := v.MarshalJSON(); err != nil || string(b) != `"whole {1 2}"` {
		t.Errorf("JSON is %s, %v", b, err)
	}
	if b, err := v.MarshalText(); err != nil || string(b) != "whole {1 2}" {
		t.Errorf("text is %s, %v", b, err)
	}
	if pair, err := v.MarshalPair(); err != nil || pair.X != "whole {1 2}" {
		t.Errorf("pair is %v, %v", pair, err)
	}
}

func TestInterfaceCaseInStringStorage(t *testing.T) {
	type union xyz.Tagged[string, struct {
		Any xyz.Case[union, any] `txt:"any: %v"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	v := Unions.Any.As(5)
	if got := v.String(); got != "any: 5" {
		t.Errorf("String is %q", got)
	}
	if got := Unions.Any.Get(v); got != "5" {
		t.Errorf("Get is %#v", got)
	}
	var back union
	if err := json.Unmarshal([]byte(`"any: hello world"`), &back); err != nil {
		t.Fatal(err)
	}
	if got := Unions.Any.Get(back); got != "hello world" {
		t.Errorf("decoded %#v", got)
	}
}

func TestUnknownDataOfOtherStorageIsMarshalled(t *testing.T) {
	type text xyz.Extern[text, string]
	var id text
	if err := json.Unmarshal([]byte(`"world"`), &id); err != nil {
		t.Fatal(err)
	}
	if b, err := id.MarshalJSON(); err != nil || string(b) != `"world"` {
		t.Errorf("the data was lost: %s, %v", b, err)
	}
	type number xyz.Extern[number, uint64]
	var n number
	if err := json.Unmarshal([]byte(`5`), &n); err != nil {
		t.Fatal(err)
	}
	if b, err := n.MarshalJSON(); err != nil || string(b) != `5` {
		t.Errorf("the data was lost: %s, %v", b, err)
	}
	var zero number
	if b, err := zero.MarshalJSON(); err != nil || string(b) != `null` {
		t.Errorf("the zero value is %s, %v", b, err)
	}
}

type valueField struct {
	Value string `json:"value"`
}

type mergedShape xyz.Tagged[any, struct {
	Any    xyz.Case[mergedShape, any]            `json:"?type=any"`
	Map    xyz.Case[mergedShape, map[string]int] `json:"?type=map"`
	Ptr    xyz.Case[mergedShape, *wireObject]    `json:"?type=ptr"`
	Struct xyz.Case[mergedShape, valueField]     `json:"?type=struct"`
}]

var mergedShapes = xyz.AccessorFor(mergedShape.Values)

// A map takes its fields back without the discriminator. A value that decodes
// from any object takes the whole object, as it did in 840f71d, which also
// read a nil pointer as a pointer to an empty struct.
func TestMergedValuesThatDecodeFromAnyObject(t *testing.T) {
	checkJSONRoundTrip(t, mergedShapes.Map.As(map[string]int{"a": 1}), `{"a":1,"type":"map"}`)
	checkJSONRoundTrip(t, mergedShapes.Map.As(map[string]int{"value": 1}), `{"value":1,"type":"map"}`)
	checkJSONRoundTrip(t, mergedShapes.Map.As(nil), `{"type":"map","value":null}`)
	checkJSONRoundTrip(t, mergedShapes.Ptr.As(&wireObject{"x"}), `{"field":"x","type":"ptr"}`)
	checkJSONRoundTrip(t, mergedShapes.Struct.As(valueField{"x"}), `{"value":"x","type":"struct"}`)
	for _, tt := range []struct {
		in   mergedShape
		wire string
	}{
		{mergedShapes.Any.As(float64(5)), `{"type":"any","value":5}`},
		{mergedShapes.Any.As(map[string]any{"a": 1.0}), `{"a":1,"type":"any"}`},
	} {
		b, err := json.Marshal(tt.in)
		if err != nil || string(b) != tt.wire {
			t.Errorf("got %s, %v, want %s", b, err, tt.wire)
		}
		var out mergedShape
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		var whole any
		if err := json.Unmarshal(b, &whole); err != nil {
			t.Fatal(err)
		}
		if xyz.ValueOf(out) != mergedShapes.Any || !reflect.DeepEqual(mergedShapes.Any.Get(out), whole) {
			t.Errorf("%s decoded to %#v", b, out.Interface())
		}
	}
	b, err := json.Marshal(mergedShapes.Ptr.As(nil))
	if err != nil || string(b) != `{"type":"ptr"}` {
		t.Errorf("got %s, %v", b, err)
	}
	var out mergedShape
	if err := json.Unmarshal(b, &out); err != nil || xyz.ValueOf(out) != mergedShapes.Ptr || mergedShapes.Ptr.Get(out) == nil {
		t.Errorf("%s decoded to %#v, %v", b, out.Interface(), err)
	}
}

func TestVoidCaseOfAPresenceRuleRoundTrips(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Key union `json:"?key"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Key, `{"key":null}`)
}

func TestInvalidJSONIsAnError(t *testing.T) {
	for _, input := range []string{``, ` `, `{`, `{bad`, `"abc`, `12x`, `[1,`, `nul`} {
		named := textOrNameds.S.As("kept")
		if err := named.UnmarshalJSON([]byte(input)); err == nil {
			t.Errorf("%q decoded", input)
		}
		if xyz.ValueOf(named) != textOrNameds.S || textOrNameds.S.Get(named) != "kept" {
			t.Errorf("%q changed the value", input)
		}
		text := inOrders.Text.As("kept")
		if err := text.UnmarshalJSON([]byte(input)); err == nil {
			t.Errorf("%q decoded", input)
		}
		if xyz.ValueOf(text) != inOrders.Text {
			t.Errorf("%q changed the value", input)
		}
	}
}

func TestSpaceAroundDirectlyDecodedJSON(t *testing.T) {
	var v inOrder
	if err := v.UnmarshalJSON([]byte(" 22\n")); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inOrders.Number || inOrders.Number.Get(v) != 22 {
		t.Errorf("got %v", v)
	}
	if err := v.UnmarshalJSON([]byte("\n{ \"named\" : 5 }\n")); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inOrders.Named || inOrders.Named.Get(v) != 5 {
		t.Errorf("got %v", v)
	}
}

// encoding/json differs between toolchains in what it accepts, none of which can
// be allowed to differ here.
func TestUnusualJSONDecodesTheSameOnEveryToolchain(t *testing.T) {
	var v inOrder
	if err := json.Unmarshal([]byte(`{"named":1,"named":2}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inOrders.Named || inOrders.Named.Get(v) != 2 {
		t.Errorf("repeated key: %v", v)
	}
	if err := json.Unmarshal([]byte("\"\xff\""), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inOrders.Text {
		t.Errorf("invalid UTF-8: %v", xyz.ValueOf(v))
	}
	if err := json.Unmarshal([]byte(`"\u006citeral"`), &v); err != nil {
		t.Fatal(err)
	}
	if v != inOrders.Literal {
		t.Errorf("escaped text: %v", xyz.ValueOf(v))
	}
}

func TestCaseConstructorsAgree(t *testing.T) {
	a, b, c := inOrders.Named.As(5), inOrders.Named.New(5), inOrders.Named.With(5)
	if a != b || b != c {
		t.Errorf("As %v, New %v, With %v", a, b, c)
	}
	if got := inOrders.Named.String(); got != "Named" {
		t.Errorf("String is %q", got)
	}
	if got := a.Tag().Get("json"); got != "named" {
		t.Errorf("Tag is %q", got)
	}
	if got := (inOrder{}).Tag(); got != "" {
		t.Errorf("Tag of the zero value is %q", got)
	}
	if raw, ok := a.Get(); !ok || raw != 5 {
		t.Errorf("Get is %v, %v", raw, ok)
	}
	if raw, ok := (inOrder{}).Get(); ok || raw != nil {
		t.Errorf("Get of the zero value is %v, %v", raw, ok)
	}
	if !(inOrder{}).IsZero() || a.IsZero() {
		t.Error("IsZero is wrong")
	}
	if got := a.Interface(); got != 5 {
		t.Errorf("Interface is %v", got)
	}
}

func TestStringOfEveryKindOfCase(t *testing.T) {
	for want, v := range map[string]inOrder{
		"literal": inOrders.Literal,
		"user-5":  inOrders.Format.As(5),
		"1:2":     inOrders.Tuple.As(xyz.NewPair(1, 2)),
		"Null":    inOrders.Null,
		"Void":    inOrders.Void,
		"x":       inOrders.Text.As("x"),
		"22":      inOrders.Number.As(22),
		"[1 2]":   inOrders.Array.As([]int{1, 2}),
		"":        {},
	} {
		if got := v.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestMarshalTextOfACaseWithoutText(t *testing.T) {
	if b, err := inOrders.Number.As(1).MarshalText(); err == nil {
		t.Errorf("got %s", b)
	}
	if b, err := (inOrder{}).MarshalText(); err != nil || len(b) != 0 {
		t.Errorf("the zero value is %q, %v", b, err)
	}
}

// The accessors of a type come from its fields, however early another type adds
// a case of its own to them.
type extendedLater xyz.Tagged[any, struct {
	A xyz.Case[extendedLater, int] `json:"a"`
}]

type extendsEarly xyz.Tagged[any, struct {
	B xyz.Case[extendedLater, string] `json:"b"`
}]

func TestCaseAddedBeforeTheFirstUseOfAType(t *testing.T) {
	added := xyz.AccessorFor(extendsEarly.Values)
	declared := xyz.AccessorFor(extendedLater.Values)
	checkJSONRoundTrip(t, declared.A.As(1), `{"a":1}`)
	checkJSONRoundTrip(t, added.B.As("x"), `{"b":"x"}`)
	if got := declared.A.String(); got != "A" {
		t.Errorf("the case is %q", got)
	}
}

func TestExternRoundTrip(t *testing.T) {
	type open xyz.Extern[open, any]
	type extended xyz.Extend[open, any, struct {
		Text   xyz.Case[open, string]  `json:",string"`
		Number xyz.Case[open, float64] `json:",number"`
		Named  xyz.Case[open, int]     `json:"named"`
		Rule   xyz.Case[open, string]  `json:"data?type=rule"`
		ID     xyz.Case[open, int]     `txt:"id-%d"`
		None   xyz.Case[open, int]     `txt:"none"`
	}]
	Opens := xyz.AccessorFor(extended.Values)
	checkJSONRoundTrip(t, Opens.Text.As("x"), `"x"`)
	checkJSONRoundTrip(t, Opens.Number.As(22), `22`)
	checkJSONRoundTrip(t, Opens.Named.As(5), `{"named":5}`)
	checkJSONRoundTrip(t, Opens.Rule.As("x"), `{"data":"x","type":"rule"}`)
	marshal := func(v open) ([]byte, error) { return v.MarshalText() }
	checkTextRoundTrip(t, Opens.ID.As(5), marshal, `id-5`)
	// The ",string" case is declared first, and takes the text in JSON.
	var v open
	if err := json.Unmarshal([]byte(`"id-5"`), &v); err != nil || xyz.ValueOf(v) != Opens.Text || Opens.Text.Get(v) != "id-5" {
		t.Errorf("got %v, %v", v, err)
	}
}

func TestExternInStringStorage(t *testing.T) {
	type id xyz.Extern[id, string]
	Ours := xyz.AccessorFor(xyz.Extend[id, string, struct {
		Value xyz.Case[id, string] `txt:"%v" json:"data?type=client.AccountID"`
	}].Values)
	in := Ours.Value.New("hello world")
	checkJSONRoundTrip(t, in, `"hello world"`)
	if got := Ours.Value.Get(in); got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestValuesOfAnExternHasNoCases(t *testing.T) {
	type bare xyz.Extern[bare, any]
	_ = xyz.AccessorFor(bare.Values)
	type open xyz.Extern[open, any]
	type extended xyz.Extend[open, any, struct {
		Text xyz.Case[open, string] `json:",string"`
	}]
	_ = xyz.AccessorFor(extended.Values)
	_ = xyz.AccessorFor(open.Values)
}

func TestNothingAClientSendsPanicsAnyShape(t *testing.T) {
	decodeClientJSON(t, "in order", func(v inOrder) {
		switch xyz.ValueOf(v) {
		case inOrders.Format:
			_ = inOrders.Format.Get(v)
		case inOrders.Tuple:
			_ = inOrders.Tuple.Get(v)
		case inOrders.Text:
			_ = inOrders.Text.Get(v)
		case inOrders.Number:
			_ = inOrders.Number.Get(v)
		case inOrders.Object:
			_ = inOrders.Object.Get(v)
		case inOrders.Array:
			_ = inOrders.Array.Get(v)
		case inOrders.Named:
			_ = inOrders.Named.Get(v)
		}
	})
	decodeClientJSON(t, "in reverse", func(v inReverse) {
		switch xyz.ValueOf(v) {
		case inReverses.Format:
			_ = inReverses.Format.Get(v)
		case inReverses.Tuple:
			_ = inReverses.Tuple.Get(v)
		case inReverses.Wrapped:
			_ = inReverses.Wrapped.Get(v)
		case inReverses.Present:
			_ = inReverses.Present.Get(v)
		}
	})
	decodeClientJSON(t, "text formats", func(v userID) {
		switch xyz.ValueOf(v) {
		case userIDs.User:
			_ = userIDs.User.Get(v)
		case userIDs.Span:
			_ = userIDs.Span.Get(v)
		case userIDs.Name:
			_ = userIDs.Name.Get(v)
		}
	})
	decodeClientJSON(t, "string stored", func(v stringStored) {
		switch xyz.ValueOf(v) {
		case stringStoreds.Point:
			_ = stringStoreds.Point.Get(v)
		case stringStoreds.Quad:
			_ = stringStoreds.Quad.Get(v)
		case stringStoreds.Whole:
			_ = stringStoreds.Whole.Get(v)
		case stringStoreds.Text:
			_ = stringStoreds.Text.Get(v)
		}
	})
	decodeClientJSON(t, "merged shapes", func(v mergedShape) {
		switch xyz.ValueOf(v) {
		case mergedShapes.Any:
			_ = mergedShapes.Any.Get(v)
		case mergedShapes.Map:
			_ = mergedShapes.Map.Get(v)
		case mergedShapes.Ptr:
			_ = mergedShapes.Ptr.Get(v)
		}
	})
	decodeClientJSON(t, "two objects", func(v twoObjects) {
		if xyz.ValueOf(v) == twoObjectss.Text {
			_ = twoObjectss.Text.Get(v)
		}
	})
	decodeClientJSON(t, "quoted", func(v quotedNumber) {
		if xyz.ValueOf(v) == quotedNumbers.Count {
			_ = quotedNumbers.Count.Get(v)
		}
	})
}

// Whatever a client sends, a decode that fails changes nothing.
func TestFailedDecodeOfAnyShapeLeavesValueAlone(t *testing.T) {
	inputs := append([]string{`"user-5x"`, `{"type":"wrapped","value":"x"}`, `{"named":"x"}`, `{"data":5,"type":"rule"}`}, clientJSON...)
	for _, input := range inputs {
		v := inOrders.Named.As(5)
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			if xyz.ValueOf(v) != inOrders.Named || inOrders.Named.Get(v) != 5 {
				t.Errorf("%s failed with %v but changed the value", input, err)
			}
		}
	}
}

func TestJSONOfTheWrongTypeForATextOrKeyedCaseIsAnError(t *testing.T) {
	for _, input := range []string{`true`, `[1]`, `{"a":1}`} {
		v := textOrNumbers.Num.As(5)
		if err := json.Unmarshal([]byte(input), &v); err == nil {
			t.Errorf("%s decoded to %v", input, v)
		}
		if xyz.ValueOf(v) != textOrNumbers.Num || textOrNumbers.Num.Get(v) != 5 {
			t.Errorf("%s changed the value", input)
		}
	}
	for _, input := range []string{`5`, `true`, `[1]`, `"x"`} {
		v := stringHints.B.As(5)
		if err := json.Unmarshal([]byte(input), &v); err == nil {
			t.Errorf("%s decoded to %v", input, v)
		}
		if xyz.ValueOf(v) != stringHints.B || stringHints.B.Get(v) != 5 {
			t.Errorf("%s changed the value", input)
		}
	}
}

func TestDataOfTheRightTypeThatNoCaseMatchesIsKept(t *testing.T) {
	for input, wire := range map[string]string{
		`"text no case has"`: `"text no case has"`,
		`null`:               `null`,
	} {
		var v textual
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if !v.IsZero() {
			t.Errorf("%s is the case %v", input, xyz.ValueOf(v))
		}
		if b, err := v.MarshalJSON(); err != nil || string(b) != wire {
			t.Errorf("%s came back as %s, %v", input, b, err)
		}
	}
	var keyed stringHint
	if err := json.Unmarshal([]byte(`{"c":1}`), &keyed); err != nil {
		t.Fatal(err)
	}
	if b, err := keyed.MarshalJSON(); err != nil || string(b) != `{"c":1}` {
		t.Errorf("came back as %s, %v", b, err)
	}
	// A ",string" case of a number declared first fails on what is not a string,
	// as it did in 840f71d.
	hinted := quotedNumbers.Number.As(1)
	if err := json.Unmarshal([]byte(`[1]`), &hinted); err == nil {
		t.Errorf("decoded to %v", hinted)
	}
	if xyz.ValueOf(hinted) != quotedNumbers.Number {
		t.Errorf("the failure changed the value to %v", hinted)
	}
}

type textual xyz.Tagged[any, struct {
	A textual `txt:"a"`
	B textual `txt:"b"`
}]

// A case that another type adds to this one decodes, but belongs to that type.
func TestAddedCaseDecodesButIsNotDescribedByTheType(t *testing.T) {
	added := xyz.AccessorFor(extendsEarly.Values)
	checkJSONRoundTrip(t, added.B.As("x"), `{"b":"x"}`)
	cases := extendedLater{}.Reflection()
	if len(cases) != 1 || cases[0].Name != "A" {
		t.Errorf("got %+v", cases)
	}
	if got := (extendedLater{}).TypesJSON(); len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

type keyedPair xyz.Tagged[any, struct {
	One    xyz.Case[keyedPair, wireObject] `json:"?name"`
	Many   xyz.Case[keyedPair, wireObject] `json:"?documents"`
	Typed  xyz.Case[keyedPair, wireObject] `json:"?type=typed"`
	Rule   xyz.Case[keyedPair, wireObject] `json:"data?type=rule"`
	Named  xyz.Case[keyedPair, wireObject] `json:"named"`
	Plain  xyz.Case[keyedPair, wireObject] `json:",object"`
	Text   xyz.Case[keyedPair, wireObject] `txt:"text"`
	Format xyz.Case[keyedPair, int]        `txt:"id-%d"`
	Name   keyedPair                       `json:"void"`
}]

var keyedPairs = xyz.AccessorFor(keyedPair.Values)

// The first "?key" case has the empty key it had in 840f71d, which reads back as
// that case, a later one has the key of its rule, as "" reads back as the first.
func TestPairRoundTripForEveryKindOfKey(t *testing.T) {
	object := wireObject{"x"}
	for _, tt := range []struct {
		value keyedPair
		key   string
	}{
		{keyedPairs.One.As(object), ""},
		{keyedPairs.Many.As(object), "documents"},
		{keyedPairs.Typed.As(object), "typed"},
		{keyedPairs.Rule.As(object), "rule"},
		{keyedPairs.Named.As(object), "named"},
		{keyedPairs.Plain.As(object), "Plain"},
		{keyedPairs.Name, "void"},
	} {
		pair, err := tt.value.MarshalPair()
		if err != nil {
			t.Errorf("%s: %v", tt.key, err)
			continue
		}
		if pair.X != tt.key {
			t.Errorf("the key is %q, want %q", pair.X, tt.key)
		}
		var back keyedPair
		if err := back.UnmarshalPair(pair); err != nil {
			t.Errorf("%s: %v", tt.key, err)
			continue
		}
		if back.Tag() != tt.value.Tag() {
			t.Errorf("%s came back as the case tagged %q", tt.key, back.Tag())
		}
		if !reflect.DeepEqual(back.Interface(), tt.value.Interface()) {
			t.Errorf("%s came back as %v", tt.key, back.Interface())
		}
	}
}

func TestPairOfATextCase(t *testing.T) {
	text := keyedPairs.Text.As(wireObject{"x"})
	pair, err := text.MarshalPair()
	if err != nil || pair.X != "text" {
		t.Fatalf("got %v, %v", pair, err)
	}
	var back keyedPair
	if err := back.UnmarshalPair(pair); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(back) != keyedPairs.Text {
		t.Errorf("got %v", xyz.ValueOf(back))
	}

	formatted, err := keyedPairs.Format.As(5).MarshalPair()
	if err != nil || formatted.X != "id-5" {
		t.Fatalf("got %v, %v", formatted, err)
	}
	if err := back.UnmarshalPair(formatted); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(back) != keyedPairs.Format || keyedPairs.Format.Get(back) != 5 {
		t.Errorf("got %v", back)
	}
}

func TestPairOfNoCase(t *testing.T) {
	for _, key := range []string{"bogus", "id-x", "Named", "type"} {
		v := keyedPairs.Named.As(wireObject{"x"})
		if err := v.UnmarshalPair(xyz.NewPair[string, any](key, nil)); err == nil {
			t.Errorf("%q matched the case %v", key, xyz.ValueOf(v))
		}
		if xyz.ValueOf(v) != keyedPairs.Named {
			t.Errorf("%q changed the value", key)
		}
	}
	// As in 840f71d, the empty key is the first case without a name, and the
	// text of a case with a format is that case.
	for key, want := range map[string]xyz.TypeOf[keyedPair]{"": keyedPairs.One, "id-%d": keyedPairs.Format} {
		var v keyedPair
		if err := v.UnmarshalPair(xyz.NewPair[string, any](key, nil)); err != nil || xyz.ValueOf(v) != want {
			t.Errorf("%q matched %v, %v", key, xyz.ValueOf(v), err)
		}
	}
	if _, err := inOrders.Null.MarshalPair(); err != nil {
		t.Errorf("a type case has its field name as a key: %v", err)
	}
	type untagged xyz.Tagged[any, struct {
		A xyz.Case[untagged, int]
	}]
	if _, err := xyz.AccessorFor(untagged.Values).A.As(1).MarshalPair(); err == nil {
		t.Error("a case without a tag has no key")
	}
	if _, err := xyz.AccessorFor(untagged.Values).A.Key(); err == nil {
		t.Error("a case without a tag has no key")
	}
}

func TestKeyAgreesWithMarshalPair(t *testing.T) {
	for _, tt := range []struct {
		key  func() (string, error)
		want string
	}{
		{keyedPairs.One.Key, ""},
		{keyedPairs.Many.Key, "documents"},
		{keyedPairs.Typed.Key, "typed"},
		{keyedPairs.Rule.Key, "rule"},
		{keyedPairs.Named.Key, "named"},
		{keyedPairs.Plain.Key, "Plain"},
		{keyedPairs.Text.Key, "text"},
	} {
		if got, err := tt.key(); err != nil || got != tt.want {
			t.Errorf("got %q, %v, want %q", got, err, tt.want)
		}
	}
}

func TestValuesJSONListsTheKeyOfEveryCase(t *testing.T) {
	var got []string
	for _, value := range keyedPairs.One.ValuesJSON() {
		got = append(got, string(value))
	}
	want := []string{`""`, `"documents"`, `"typed"`, `"rule"`, `"named"`, `"Plain"`, `"text"`, `"id-%d"`, `"void"`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValueOf(t *testing.T) {
	if got := xyz.ValueOf(inOrder{}); got != nil {
		t.Errorf("the zero value has the case %v", got)
	}
	value := xyz.ValueOf(keyedPairs.One.As(wireObject{"x"}))
	if value != keyedPairs.One {
		t.Errorf("got %v", value)
	}
	if got := value.String(); got != "One" {
		t.Errorf("String is %q", got)
	}
	if got, err := value.Key(); err != nil || got != "" {
		t.Errorf("Key is %q, %v", got, err)
	}
	if got := value.ValuesJSON(); len(got) != 9 {
		t.Errorf("ValuesJSON is %q", got)
	}
	// A case without a value is a value of the type, there is no case to return.
	if got := xyz.ValueOf(inOrders.Literal); got != nil {
		t.Errorf("got %v", got)
	}
}

func TestFormatOfATypeThatReadsItsOwnText(t *testing.T) {
	type union xyz.Tagged[any, struct {
		State xyz.Case[union, status]                `txt:"state: %v"`
		Both  xyz.Case[union, xyz.Pair[status, int]] `txt:"%v at %d"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.State.As(statuses.Open), `"state: in progress"`)
	checkJSONRoundTrip(t, Unions.Both.As(xyz.NewPair(statuses.Done, 5)), `"done at 5"`)
}

func TestFormatThatDoesNotFitItsValueNeverMatches(t *testing.T) {
	type union xyz.Tagged[any, struct {
		TooMany xyz.Case[union, int]                `txt:"%d-%d"`
		TooFew  xyz.Case[union, xyz.Pair[int, int]] `txt:"few %d"`
		Runs    xyz.Case[union, xyz.Pair[int, int]] `txt:"%d%d"`
		Star    xyz.Case[union, int]                `txt:"%*d"`
		Dangles xyz.Case[union, int]                `txt:"ends in %"`
		Scanned xyz.Case[union, error]              `txt:"error %v"`
	}]
	for _, input := range []string{`"1-2"`, `"few 1"`, `"12"`, `"5"`, `"ends in 5"`, `"error x"`} {
		var v union
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			t.Errorf("%s: %v", input, err)
		}
		if !v.IsZero() {
			t.Errorf("%s decoded to %v", input, xyz.ValueOf(v))
		}
	}
	// The text of a case is that case, as in 840f71d, even with a format.
	var v union
	if err := json.Unmarshal([]byte(`"ends in %"`), &v); err != nil || v.Tag().Get("txt") != "ends in %" {
		t.Errorf("got the case tagged %q, %v", v.Tag(), err)
	}
}

// The key has already selected the case, whose value can be a nil of any kind.
func TestNullUnderTheKeyOfANamedCase(t *testing.T) {
	type union xyz.Tagged[any, struct {
		Array  xyz.Case[union, []int]          `json:"array,array"`
		Object xyz.Case[union, map[string]int] `json:"object,object"`
		Number xyz.Case[union, *float64]       `json:"number,number"`
	}]
	Unions := xyz.AccessorFor(union.Values)
	checkJSONRoundTrip(t, Unions.Array.As(nil), `{"array":null}`)
	checkJSONRoundTrip(t, Unions.Object.As(nil), `{"object":null}`)
	checkJSONRoundTrip(t, Unions.Number.As(nil), `{"number":null}`)
}

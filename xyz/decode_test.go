package xyz_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"
	"unsafe"

	"github.com/iqhive/runtime.link/xyz"
)

type textOrNamed xyz.Tagged[any, struct {
	S xyz.Case[textOrNamed, string]  `json:"value?type=s"`
	N xyz.Case[textOrNamed, float64] `json:"value?type=n"`
}]

var textOrNameds = xyz.AccessorFor(textOrNamed.Values)

func TestFailedDecodeLeavesValueAlone(t *testing.T) {
	v := textOrNameds.S.As("x")
	if err := json.Unmarshal([]byte(`{"type":"n","value":"oops"}`), &v); err == nil {
		t.Fatal("a string decoded as a number")
	}
	if xyz.ValueOf(v) != textOrNameds.S || textOrNameds.S.Get(v) != "x" {
		t.Errorf("the value changed to %v", xyz.ValueOf(v))
	}
}

func TestUnknownDiscriminatorClearsTheCase(t *testing.T) {
	v := textOrNameds.S.As("x")
	if err := json.Unmarshal([]byte(`{"type":"bogus"}`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.IsZero() {
		t.Errorf("still the case %v", xyz.ValueOf(v))
	}
	if b, err := v.MarshalJSON(); err != nil || string(b) != `{"type":"bogus"}` {
		t.Errorf("the unknown data was lost: %s, %v", b, err)
	}
}

func TestUnknownDataIsCopiedFromTheDecoder(t *testing.T) {
	r := iotest.OneByteReader(strings.NewReader(`{"type":"first"}` + "\n" + `{"type":"secnd"}` + "\n" + `{"type":"third"}`))
	dec := json.NewDecoder(r)
	var vals []textOrNamed
	for range 3 {
		var v textOrNamed
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		vals = append(vals, v)
	}
	for i, want := range []string{`{"type":"first"}`, `{"type":"secnd"}`, `{"type":"third"}`} {
		if b, _ := vals[i].MarshalJSON(); string(b) != want {
			t.Errorf("value %d is now %s", i, b)
		}
	}

	buf := []byte(`{"type":"first"}`)
	var v textOrNamed
	if err := json.Unmarshal(buf, &v); err != nil {
		t.Fatal(err)
	}
	copy(buf, `{"type":"XXXXX"}`)
	if b, _ := v.MarshalJSON(); string(b) != `{"type":"first"}` {
		t.Errorf("the value changed with the caller's buffer: %s", b)
	}
}

type voidNamed xyz.Tagged[any, struct {
	None voidNamed                   `json:"none"`
	Text xyz.Case[voidNamed, string] `json:"text"`
}]

var voidNameds = xyz.AccessorFor(voidNamed.Values)

type voidRule xyz.Tagged[any, struct {
	None voidRule                   `json:"?type=none"`
	Text xyz.Case[voidRule, string] `json:"value?type=text"`
}]

var voidRules = xyz.AccessorFor(voidRule.Values)

func TestVoidCasesRoundTrip(t *testing.T) {
	var named voidNamed
	b, err := json.Marshal(voidNameds.None)
	if err != nil || string(b) != `{"none":null}` {
		t.Fatalf("got %s, %v", b, err)
	}
	if err := json.Unmarshal(b, &named); err != nil || named != voidNameds.None {
		t.Errorf("got %v, %v", named, err)
	}

	var rule voidRule
	b, err = json.Marshal(voidRules.None)
	if err != nil || string(b) != `{"type":"none"}` {
		t.Fatalf("got %s, %v", b, err)
	}
	if err := json.Unmarshal(b, &rule); err != nil || rule != voidRules.None {
		t.Errorf("got %v, %v", rule, err)
	}
}

type nullOrText xyz.Tagged[any, struct {
	Null nullOrText                   `json:",null"`
	Text xyz.Case[nullOrText, string] `json:",string"`
}]

var nullOrTexts = xyz.AccessorFor(nullOrText.Values)

type namedText xyz.Tagged[any, struct {
	A namedText                   `txt:"a"`
	B xyz.Case[namedText, string] `txt:"b"`
}]

var namedTexts = xyz.AccessorFor(namedText.Values)

// A case without a value in the JSON must not keep the value of the case before it.
func TestCaseWithoutValueClearsStorage(t *testing.T) {
	v := nullOrTexts.Text.As("x")
	if err := v.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Fatal(err)
	}
	if v != nullOrTexts.Null {
		t.Errorf("got %v", v)
	}

	text := namedTexts.B.As("x")
	if err := text.UnmarshalJSON([]byte(`"a"`)); err != nil {
		t.Fatal(err)
	}
	if text != namedTexts.A {
		t.Errorf("JSON text kept the old storage: %v", text)
	}
	text = namedTexts.B.As("x")
	if err := text.UnmarshalText([]byte(`a`)); err != nil {
		t.Fatal(err)
	}
	if text != namedTexts.A {
		t.Errorf("plain text kept the old storage: %v", text)
	}
}

// The value of a case decoded from just its text is the zero value of that case.
func TestGetAfterTextOfACaseWithValue(t *testing.T) {
	var v namedText
	if err := v.UnmarshalJSON([]byte(`"b"`)); err != nil {
		t.Fatal(err)
	}
	if got := namedTexts.B.Get(v); got != "" {
		t.Errorf("got %q", got)
	}
}

type mergedObject struct {
	Field string `json:"field"`
}

type customJSON struct{ Field string }

func (c customJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"custom": c.Field})
}

type unexportedMethod struct {
	Field string `json:"field"`
}

func (unexportedMethod) secret() {}

type inlineType struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type innerUnion xyz.Tagged[any, struct {
	S xyz.Case[innerUnion, string] `json:"s"`
}]

var innerUnions = xyz.AccessorFor(innerUnion.Values)

type merged xyz.Tagged[any, struct {
	Int    xyz.Case[merged, int]              `json:"?type=int"`
	Object xyz.Case[merged, mergedObject]     `json:"?type=object"`
	Time   xyz.Case[merged, time.Time]        `json:"?type=time"`
	Custom xyz.Case[merged, customJSON]       `json:"?type=custom"`
	Hidden xyz.Case[merged, unexportedMethod] `json:"?type=hidden"`
	Union  xyz.Case[merged, innerUnion]       `json:"?type=union"`
	Clash  xyz.Case[merged, inlineType]       `json:"?type=clash"`
	Ptr    xyz.Case[merged, *mergedObject]    `json:"?type=ptr"`
	Slice  xyz.Case[merged, []int]            `json:"?type=slice"`
	Empty  xyz.Case[merged, struct{}]         `json:"?type=empty"`
}]

var mergeds = xyz.AccessorFor(merged.Values)

func TestMergedDiscriminator(t *testing.T) {
	moment := time.Unix(0, 0).UTC()
	tests := []struct {
		value merged
		want  string
	}{
		{mergeds.Int.As(5), `{"type":"int","value":5}`},
		{mergeds.Object.As(mergedObject{"x"}), `{"field":"x","type":"object"}`},
		{mergeds.Time.As(moment), `{"type":"time","value":"1970-01-01T00:00:00Z"}`},
		{mergeds.Custom.As(customJSON{"x"}), `{"custom":"x","type":"custom"}`},
		{mergeds.Hidden.As(unexportedMethod{"x"}), `{"field":"x","type":"hidden"}`},
		{mergeds.Union.As(innerUnions.S.As("x")), `{"s":"x","type":"union"}`},
		{mergeds.Clash.As(inlineType{Type: "mine", Name: "x"}), `{"name":"x","type":"clash"}`},
		// A nil pointer to a struct has no fields, and is the discriminator alone, as in 840f71d.
		{mergeds.Ptr.As(nil), `{"type":"ptr"}`},
		{mergeds.Slice.As([]int{1}), `{"type":"slice","value":[1]}`},
		{mergeds.Empty.As(struct{}{}), `{"type":"empty"}`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.value)
		if err != nil || string(got) != tt.want {
			t.Errorf("got %s, %v, want %s", got, err, tt.want)
		}
	}
}

func TestMergedValuesDecode(t *testing.T) {
	moment := time.Unix(0, 0).UTC()
	roundTrip := func(in merged) merged {
		t.Helper()
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var out merged
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		return out
	}
	if got := roundTrip(mergeds.Int.As(5)); xyz.ValueOf(got) != mergeds.Int || mergeds.Int.Get(got) != 5 {
		t.Errorf("int: %v", got)
	}
	if got := roundTrip(mergeds.Object.As(mergedObject{"x"})); mergeds.Object.Get(got).Field != "x" {
		t.Errorf("object: %v", got)
	}
	if got := roundTrip(mergeds.Time.As(moment)); !mergeds.Time.Get(got).Equal(moment) {
		t.Errorf("time: %v", got)
	}
	if got := roundTrip(mergeds.Slice.As([]int{1, 2})); len(mergeds.Slice.Get(got)) != 2 {
		t.Errorf("slice: %v", got)
	}
	if got := roundTrip(mergeds.Union.As(innerUnions.S.As("x"))); innerUnions.S.Get(mergeds.Union.Get(got)) != "x" {
		t.Errorf("union: %v", got)
	}

	var bad merged
	if err := json.Unmarshal([]byte(`{"type":"int","value":"five"}`), &bad); err == nil {
		t.Error("a string decoded as an int")
	}
	if err := json.Unmarshal([]byte(`{"type":"int"}`), &bad); err == nil {
		t.Error("nothing decoded as an int")
	}
}

func TestExternWithoutExtensions(t *testing.T) {
	type bare xyz.Extern[bare, any]
	var v bare
	if err := json.Unmarshal([]byte(`"world"`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.IsZero() {
		t.Error("decoded a case that nothing declares")
	}
	if b, err := v.MarshalJSON(); err != nil || string(b) != `"world"` {
		t.Errorf("the data was lost: %s, %v", b, err)
	}
}

func TestZeroValueMethods(t *testing.T) {
	var v textOrNamed
	if got := v.Interface(); got != nil {
		t.Errorf("Interface() = %v", got)
	}
	if _, err := v.MarshalPair(); err == nil {
		t.Error("MarshalPair needs a case")
	}
}

type nilable xyz.Tagged[any, struct {
	Any   xyz.Case[nilable, any]            `json:"any"`
	Error xyz.Case[nilable, error]          `json:"error"`
	Ptr   xyz.Case[nilable, *int]           `json:"ptr"`
	Map   xyz.Case[nilable, map[string]int] `json:"map"`
}]

var nilables = xyz.AccessorFor(nilable.Values)

type voidKinds xyz.Tagged[any, struct {
	Str voidKinds `json:",string"`
	Num voidKinds `json:",number"`
	Obj voidKinds `json:",object"`
	Arr voidKinds `json:",array"`
}]

type voidBare xyz.Tagged[any, struct {
	Any voidBare `json:",omitempty"`
}]

type voidPresent xyz.Tagged[any, struct {
	Key voidPresent `json:"?key"`
}]

type structStorage struct{ A, B int64 }

type inStruct xyz.Tagged[structStorage, struct {
	X xyz.Case[inStruct, int32] `json:"x"`
	T xyz.Case[inStruct, int32] `txt:"t"`
}]

var inStructs = xyz.AccessorFor(inStruct.Values)

type inSlice xyz.Tagged[[]uint32, struct {
	X xyz.Case[inSlice, [2]uint32] `json:"x"`
	Y xyz.Case[inSlice, uint8]     `json:"y"`
	T xyz.Case[inSlice, [2]uint32] `txt:"t"`
}]

var inSlices = xyz.AccessorFor(inSlice.Values)

type inInt xyz.Tagged[uint64, struct {
	X xyz.Case[inInt, uint32] `json:"x"`
	T xyz.Case[inInt, uint32] `txt:"t"`
}]

var inInts = xyz.AccessorFor(inInt.Values)

type inString xyz.Tagged[string, struct {
	V xyz.Case[inString, string] `txt:"%v" json:"v"`
	A xyz.Case[inString, any]    `txt:"a" json:"a"`
}]

var inStrings = xyz.AccessorFor(inString.Values)

type inError xyz.Tagged[error, struct {
	S xyz.Case[inError, string] `json:"s"`
	E xyz.Case[inError, error]  `json:"e"`
}]

type inPointer xyz.Tagged[unsafe.Pointer, struct {
	A xyz.Case[inPointer, any] `json:"a"`
	I xyz.Case[inPointer, int] `json:"i"`
}]

var inPointers = xyz.AccessorFor(inPointer.Values)

// JSON that is valid but shaped to reach each path of every decoder below,
// where the values reach them in every order, each over what the last one left.
var clientJSON = []string{
	`null`, `true`, `false`, `0`, `-1`, `1.5`, `1e400`, `""`, `"a"`, `"b"`, `"t"`, `"%v"`, `"world"`,
	`[]`, `[1]`, `[1,2,3]`, `{}`, `{"key":null}`, `{"none":null}`, `{"none":{"x":1}}`, `{"text":"x"}`, `{"text":null}`,
	`{"type":"none"}`, `{"type":"none","value":1}`, `{"type":"text"}`, `{"type":"text","value":1}`,
	`{"type":"text","value":"x"}`, `{"type":1}`, `{"type":null}`, `{"type":"bogus"}`, `{"type":["none"]}`,
	`{"any":null}`, `{"any":1}`, `{"any":{"a":1}}`, `{"error":null}`, `{"error":"x"}`, `{"error":{"e":1}}`,
	`{"ptr":null}`, `{"ptr":1}`, `{"ptr":"x"}`, `{"map":null}`, `{"map":{"a":1}}`, `{"map":{"a":"x"}}`,
	`{"x":null}`, `{"x":7}`, `{"x":"y"}`, `{"x":[1,2]}`, `{"y":1}`, `{"y":300}`, `{"v":"x"}`, `{"a":null}`, `{"a":1}`,
	`{"s":"x"}`, `{"s":null}`, `{"e":null}`, `{"e":"x"}`, `{"i":1}`, `{"i":null}`,
}

// usable is what a server can call on a value that it decoded from a client.
type usable interface {
	fmt.Stringer
	Interface() any
	MarshalJSON() ([]byte, error)
	MarshalText() ([]byte, error)
}

func decodeClientJSON[V usable](t *testing.T, name string, inspect func(V)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		var current string
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic after decoding %s: %v", current, r)
			}
		}()
		var v V
		use := func() {
			_ = v.String()
			_ = v.Interface()
			_, _ = v.MarshalJSON()
			_, _ = v.MarshalText()
			inspect(v)
		}
		for range 2 {
			for _, input := range clientJSON {
				current = input
				_ = json.Unmarshal([]byte(input), &v)
				use()
			}
		}
		for i := len(clientJSON) - 1; i >= 0; i-- {
			current = clientJSON[i]
			var fresh V
			_ = json.Unmarshal([]byte(clientJSON[i]), &fresh)
			v = fresh
			use()
		}
	})
}

func TestNothingAClientSendsPanics(t *testing.T) {
	decodeClientJSON(t, "named", func(v voidNamed) {
		if xyz.ValueOf(v) == voidNameds.Text {
			_ = voidNameds.Text.Get(v)
		}
	})
	decodeClientJSON(t, "rule", func(v voidRule) {
		if xyz.ValueOf(v) == voidRules.Text {
			_ = voidRules.Text.Get(v)
		}
	})
	decodeClientJSON(t, "null", func(v nullOrText) {
		if xyz.ValueOf(v) == nullOrTexts.Text {
			_ = nullOrTexts.Text.Get(v)
		}
	})
	decodeClientJSON(t, "text", func(v namedText) {
		if xyz.ValueOf(v) == namedTexts.B {
			_ = namedTexts.B.Get(v)
		}
	})
	decodeClientJSON(t, "numbered", func(v textOrNamed) {
		switch xyz.ValueOf(v) {
		case textOrNameds.S:
			_ = textOrNameds.S.Get(v)
		case textOrNameds.N:
			_ = textOrNameds.N.Get(v)
		}
	})
	decodeClientJSON(t, "merged", func(v merged) {
		switch xyz.ValueOf(v) {
		case mergeds.Int:
			_ = mergeds.Int.Get(v)
		case mergeds.Object:
			_ = mergeds.Object.Get(v)
		case mergeds.Ptr:
			_ = mergeds.Ptr.Get(v)
		}
	})
	decodeClientJSON(t, "nilable", func(v nilable) {
		switch xyz.ValueOf(v) {
		case nilables.Any:
			_ = nilables.Any.Get(v)
		case nilables.Error:
			_ = nilables.Error.Get(v)
		case nilables.Ptr:
			_ = nilables.Ptr.Get(v)
		case nilables.Map:
			_ = nilables.Map.Get(v)
		}
	})
	decodeClientJSON(t, "void kinds", func(voidKinds) {})
	decodeClientJSON(t, "void bare", func(voidBare) {})
	decodeClientJSON(t, "void present", func(voidPresent) {})
	decodeClientJSON(t, "struct storage", func(v inStruct) {
		switch xyz.ValueOf(v) {
		case inStructs.X:
			_ = inStructs.X.Get(v)
		case inStructs.T:
			_ = inStructs.T.Get(v)
		}
	})
	decodeClientJSON(t, "slice storage", func(v inSlice) {
		switch xyz.ValueOf(v) {
		case inSlices.X:
			_ = inSlices.X.Get(v)
		case inSlices.Y:
			_ = inSlices.Y.Get(v)
		case inSlices.T:
			_ = inSlices.T.Get(v)
		}
	})
	decodeClientJSON(t, "int storage", func(v inInt) {
		switch xyz.ValueOf(v) {
		case inInts.X:
			_ = inInts.X.Get(v)
		case inInts.T:
			_ = inInts.T.Get(v)
		}
	})
	decodeClientJSON(t, "string storage", func(v inString) {
		switch xyz.ValueOf(v) {
		case inStrings.V:
			_ = inStrings.V.Get(v)
		case inStrings.A:
			_ = inStrings.A.Get(v)
		}
	})
	decodeClientJSON(t, "error storage", func(inError) {})
	decodeClientJSON(t, "pointer storage", func(v inPointer) {
		switch xyz.ValueOf(v) {
		case inPointers.A:
			_ = inPointers.A.Get(v)
		case inPointers.I:
			_ = inPointers.I.Get(v)
		}
	})
}

// raced is a new type for every N, each with nothing cached for it.
type raced[N any] xyz.Tagged[any, struct {
	A xyz.Case[raced[N], int] `json:"a"`
	B xyz.Case[raced[N], int] `json:"b"`
}]

// decodeFirstUseConcurrently has a type first used by several requests at once,
// which is up to when clients connect. Each builds the accessors, and the value
// decoded with a losing set fails every comparison with the set kept for Get.
func decodeFirstUseConcurrently[N any](t *testing.T) {
	const decoders = 16
	var ready, finished sync.WaitGroup
	ready.Add(1)
	values := make([]raced[N], decoders)
	for i := range values {
		finished.Go(func() {
			ready.Wait()
			if err := json.Unmarshal([]byte(`{"a":1}`), &values[i]); err != nil {
				t.Error(err)
			}
		})
	}
	ready.Done()
	finished.Wait()
	cases := xyz.AccessorFor(raced[N].Values)
	for i := range values {
		if xyz.ValueOf(values[i]) != cases.A {
			t.Errorf("value %d of %T is the case %v", i, values[i], xyz.ValueOf(values[i]))
			continue
		}
		if got := cases.A.Get(values[i]); got != 1 {
			t.Errorf("value %d of %T is %d", i, values[i], got)
		}
	}
}

func decodeFirstUseOfEight[N any](t *testing.T) {
	decodeFirstUseConcurrently[[0]N](t)
	decodeFirstUseConcurrently[[1]N](t)
	decodeFirstUseConcurrently[[2]N](t)
	decodeFirstUseConcurrently[[3]N](t)
	decodeFirstUseConcurrently[[4]N](t)
	decodeFirstUseConcurrently[[5]N](t)
	decodeFirstUseConcurrently[[6]N](t)
	decodeFirstUseConcurrently[[7]N](t)
}

// The types are the bound on this stress loop: 64 first uses, by 16 decoders.
func TestConcurrentFirstUseAgreesOnAccessors(t *testing.T) {
	decodeFirstUseOfEight[int8](t)
	decodeFirstUseOfEight[int16](t)
	decodeFirstUseOfEight[int32](t)
	decodeFirstUseOfEight[int64](t)
	decodeFirstUseOfEight[uint8](t)
	decodeFirstUseOfEight[uint16](t)
	decodeFirstUseOfEight[uint32](t)
	decodeFirstUseOfEight[uint64](t)
}

// A first use must not replace what an extension has registered meanwhile.
func TestFirstUseOfExternKeepsExtensions(t *testing.T) {
	type open xyz.Extern[open, any]
	type extended xyz.Extend[open, any, struct {
		Text xyz.Case[open, string] `json:",string"`
	}]
	var v open
	if err := json.Unmarshal([]byte(`"before"`), &v); err != nil {
		t.Fatal(err)
	}
	extensions := xyz.AccessorFor(extended.Values)
	if err := json.Unmarshal([]byte(`"after"`), &v); err != nil {
		t.Fatal(err)
	}
	if v.IsZero() || extensions.Text.Get(v) != "after" {
		t.Errorf("the extension was lost: %v", v)
	}
}

func TestNullIntoInterfaceCase(t *testing.T) {
	var v nilable
	if err := json.Unmarshal([]byte(`{"any":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != nilables.Any || nilables.Any.Get(v) != nil {
		t.Errorf("got %v", v)
	}
	if err := json.Unmarshal([]byte(`{"error":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != nilables.Error || nilables.Error.Get(v) != nil {
		t.Errorf("got %v", v)
	}
}

func TestDecodeIntoStructAndSliceStorage(t *testing.T) {
	var s inStruct
	if err := json.Unmarshal([]byte(`{"x":7}`), &s); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(s) != inStructs.X || inStructs.X.Get(s) != 7 {
		t.Errorf("struct storage: %v", s)
	}
	var v inSlice
	if err := json.Unmarshal([]byte(`{"x":[1,2]}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inSlices.X || inSlices.X.Get(v) != [2]uint32{1, 2} {
		t.Errorf("slice storage: %v", v)
	}
	// Smaller than one element of storage, which must still have room for it.
	if err := json.Unmarshal([]byte(`{"y":9}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != inSlices.Y || inSlices.Y.Get(v) != 9 {
		t.Errorf("slice storage: %v", v)
	}
}

// Storage that cannot hold a case's value is a mistake in the type, it cannot
// be allowed to surface as a panic that any client is able to trigger.
func TestDecodeThatStorageCannotHoldIsAnError(t *testing.T) {
	var e inError
	if err := json.Unmarshal([]byte(`{"s":"x"}`), &e); err == nil {
		t.Error("a string does not implement error")
	}
	var a inPointer
	if err := json.Unmarshal([]byte(`{"a":1}`), &a); err == nil {
		t.Error("an interface case cannot be held by a pointer")
	}
	if err := json.Unmarshal([]byte(`{"a":null}`), &a); err == nil {
		t.Error("an interface case cannot be held by a pointer")
	}
}

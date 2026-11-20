package xyz_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

type operator xyz.Tagged[string, struct {
	Add     operator                `txt:"+"`
	Mod     operator                `txt:"%"`
	Percent xyz.Case[operator, int] `txt:"%d%%"`
}]

type discount xyz.Tagged[any, struct {
	None  discount                `txt:"none"`
	Half  discount                `txt:"50%"`
	Verb  discount                `txt:"%d off"`
	Twice discount                `txt:"100%%"`
	Rate  xyz.Case[discount, int] `txt:"%d percent"`
}]

// A case without a value has nothing to format, so a percent sign in its text
// is a percent sign, whatever follows it. Before, such a case was written as
// the empty string or the error text of fmt, which read back as another value.
func TestCaseWithoutAValueKeepsThePercentSignsOfItsText(t *testing.T) {
	Operators := xyz.AccessorFor(operator.Values)
	marshalOperator := func(v operator) ([]byte, error) { return v.MarshalText() }
	if got := Operators.Mod.String(); got != "%" {
		t.Errorf("String() = %q, want %q", got, "%")
	}
	checkJSONRoundTrip(t, Operators.Mod, `"%"`)
	checkJSONRoundTrip(t, Operators.Add, `"+"`)
	checkTextRoundTrip(t, Operators.Mod, marshalOperator, "%")
	pair, err := Operators.Mod.MarshalPair()
	if err != nil || pair.X != "%" {
		t.Errorf("MarshalPair gave %q, %v", pair.X, err)
	}
	var back operator
	if err := back.UnmarshalPair(pair); err != nil || back != Operators.Mod {
		t.Errorf("UnmarshalPair gave the case %q, %v", back.Tag(), err)
	}
	// Beside it, a case with a value still formats that value.
	checkJSONRoundTrip(t, Operators.Percent.As(5), `"5%"`)
	checkTextRoundTrip(t, Operators.Percent.As(5), marshalOperator, "5%")

	Discounts := xyz.AccessorFor(discount.Values)
	marshalDiscount := func(v discount) ([]byte, error) { return v.MarshalText() }
	for text, in := range map[string]discount{
		"none":   Discounts.None,
		"50%":    Discounts.Half,
		"%d off": Discounts.Verb,
		"100%%":  Discounts.Twice,
	} {
		if got := in.String(); got != text {
			t.Errorf("String() = %q, want %q", got, text)
		}
		wire, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		checkJSONRoundTrip(t, in, string(wire))
		checkTextRoundTrip(t, in, marshalDiscount, text)
	}
	checkJSONRoundTrip(t, Discounts.Rate.As(20), `"20 percent"`)
	// Text that no case has leaves the value as it was.
	for _, text := range []string{"100%", "5 off", "%"} {
		var v discount
		if err := v.UnmarshalText([]byte(text)); err != nil || !v.IsZero() {
			t.Errorf("%q, which no case has, decoded to the case %q, %v", text, v.Tag(), err)
		}
	}
}

// An error that says what did not match quotes it, and no longer than 64
// runes, so that one made of a string of any length says one that can be
// read.
func TestUnmatchedCaseIsQuotedAndCutShort(t *testing.T) {
	Operators := xyz.AccessorFor(operator.Values)
	pair, err := Operators.Mod.MarshalPair()
	if err != nil {
		t.Fatal(err)
	}
	var unmatched operator
	err = unmatched.UnmarshalPair(xyz.NewPair("no such case", pair.Y))
	if err == nil || !strings.Contains(err.Error(), `"no such case"`) {
		t.Errorf("an unmatched case gave %v", err)
	}
	unmatched = operator{}
	err = unmatched.UnmarshalPair(xyz.NewPair(strings.Repeat("a", 200), pair.Y))
	if err == nil {
		t.Fatal("a long unmatched case gave no error")
	}
	if len(err.Error()) > 100 {
		t.Errorf("the error of a long case is %d bytes long, want it cut short", len(err.Error()))
	}
}

type share xyz.Tagged[string, struct {
	Half    xyz.Case[share, int] `txt:"50%"`
	Whole   xyz.Case[share, int] `txt:"100%%"`
	Dangles xyz.Case[share, int] `txt:"ends in %"`
	Plain   xyz.Case[share, int] `txt:"plain"`
	Rate    xyz.Case[share, int] `txt:"%d%%"`
}]

type shareOfAny xyz.Tagged[any, struct {
	Half    xyz.Case[shareOfAny, int] `txt:"50%"`
	Whole   xyz.Case[shareOfAny, int] `txt:"100%%"`
	Dangles xyz.Case[shareOfAny, int] `txt:"ends in %"`
	Plain   xyz.Case[shareOfAny, int] `txt:"plain"`
	Rate    xyz.Case[shareOfAny, int] `txt:"%d%%"`
}]

// textOfCase checks that in, a case whose text has no verb, is that text
// wherever the text of a case is written or read.
func textOfCase[V roundTrippable, P interface {
	*V
	UnmarshalText([]byte) error
	UnmarshalPair(xyz.Pair[string, S]) error
}, S any](t *testing.T, in V, text string, marshalText func(V) ([]byte, error), marshalPair func(V) (xyz.Pair[string, S], error)) {
	t.Helper()
	if got := fmt.Sprint(in); got != text {
		t.Errorf("String() = %q, want %q", got, text)
	}
	wire, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(in)
	if err != nil || string(got) != string(wire) {
		t.Errorf("marshalled %s, %v; want %s", got, err, wire)
	}
	var out V
	if err := json.Unmarshal(wire, &out); err != nil || out.Tag() != in.Tag() {
		t.Errorf("%s decoded as the case tagged %q, %v; want %q", wire, out.Tag(), err, in.Tag())
	}
	if got, err := marshalText(in); err != nil || string(got) != text {
		t.Errorf("MarshalText gave %q, %v; want %q", got, err, text)
	}
	var fromText V
	if err := P(&fromText).UnmarshalText([]byte(text)); err != nil || fromText.Tag() != in.Tag() {
		t.Errorf("the text %q decoded as the case tagged %q, %v; want %q", text, fromText.Tag(), err, in.Tag())
	}
	pair, err := marshalPair(in)
	if err != nil || pair.X != text {
		t.Errorf("MarshalPair gave %q, %v; want %q", pair.X, err, text)
	}
	var fromPair V
	if err := P(&fromPair).UnmarshalPair(pair); err != nil || fromPair.Tag() != in.Tag() {
		t.Errorf("the pair of %q decoded as the case tagged %q, %v; want %q", text, fromPair.Tag(), err, in.Tag())
	}
}

// The text of a case with a value is a format only if it has a verb for the
// value. Without one it is text, as "plain" is, whatever percent signs it has:
// formatted, the value came out as the error text of fmt.
func TestCaseWithAValueAndNoVerbKeepsThePercentSignsOfItsText(t *testing.T) {
	Shares := xyz.AccessorFor(share.Values)
	for text, in := range map[string]share{
		"50%":       Shares.Half.As(5),
		"100%%":     Shares.Whole.As(5),
		"ends in %": Shares.Dangles.As(5),
		"plain":     Shares.Plain.As(5),
	} {
		textOfCase(t, in, text, share.MarshalText, share.MarshalPair)
	}
	Any := xyz.AccessorFor(shareOfAny.Values)
	for text, in := range map[string]shareOfAny{
		"50%":       Any.Half.As(5),
		"100%%":     Any.Whole.As(5),
		"ends in %": Any.Dangles.As(5),
		"plain":     Any.Plain.As(5),
	} {
		textOfCase(t, in, text, shareOfAny.MarshalText, shareOfAny.MarshalPair)
	}
	// Storage that can hold the value still does.
	if got := Any.Half.Get(Any.Half.As(5)); got != 5 {
		t.Errorf("the value of a case with plain text is %d, want 5", got)
	}

	// Beside them, a text with a verb still formats its value, and %% in it is
	// one percent sign.
	checkJSONRoundTrip(t, Shares.Rate.As(20), `"20%"`)
	checkJSONRoundTrip(t, Any.Rate.As(20), `"20%"`)

	// What a format would have made of the text is the text of no case.
	for _, text := range []string{"505", "50", "100", "ends in 5", "ends in %!(NOVERB)%!(EXTRA int=5)", "%"} {
		var v share
		if err := v.UnmarshalText([]byte(text)); err != nil || !v.IsZero() {
			t.Errorf("%q, which no case has, decoded to the case %q, %v", text, v.Tag(), err)
		}
		var w shareOfAny
		if err := w.UnmarshalText([]byte(text)); err != nil || !w.IsZero() {
			t.Errorf("%q, which no case has, decoded to the case %q, %v", text, w.Tag(), err)
		}
	}
}

type rawStored xyz.Tagged[json.RawMessage, struct {
	I xyz.Case[rawStored, int64] `json:"i"`
}]

// The zero value of every storage is null, and the bytes of a json.RawMessage
// are only its JSON once it holds data that no case took. Before, the zero
// value of json.RawMessage storage failed to marshal.
func TestZeroValueOfRawMessageStorageIsNull(t *testing.T) {
	Stored := xyz.AccessorFor(rawStored.Values)
	checkJSONRoundTrip(t, Stored.I.As(5), `{"i":5}`)

	var zero rawStored
	if b, err := json.Marshal(zero); err != nil || string(b) != "null" {
		t.Errorf("the zero value marshalled as %q, %v, want null", b, err)
	}
	type holder struct {
		ID    int       `json:"id"`
		Value rawStored `json:"value"`
	}
	if b, err := json.Marshal(holder{ID: 1}); err != nil || string(b) != `{"id":1,"value":null}` {
		t.Errorf("a struct holding the zero value marshalled as %s, %v", b, err)
	}

	var unknown rawStored
	if err := json.Unmarshal([]byte(`{"later":[1,2]}`), &unknown); err != nil {
		t.Fatal(err)
	}
	if b, err := json.Marshal(unknown); err != nil || string(b) != `{"later":[1,2]}` {
		t.Errorf("unknown data came back as %s, %v", b, err)
	}
}

type withWhen xyz.Tagged[any, struct {
	Text xyz.Case[withWhen, string]  `json:"text"`
	Op   xyz.When[withWhen, textual] `json:"op"`
}]

type withInt xyz.Tagged[any, struct {
	Text  xyz.Case[withInt, string] `json:"text"`
	Count int                       `json:"count"`
}]

// firstUse returns what the first use of a type whose declaration is mistaken
// panics with.
func firstUse(t *testing.T, use func()) (message string) {
	t.Helper()
	defer func() {
		message, _ = recover().(string)
	}()
	use()
	t.Error("the first use did not panic")
	return ""
}

// When has no way to make or read a value, so a type that declares one says so
// by name on its first use, like any other mistake in a declaration.
func TestWhenCaseIsRefusedByNameOnFirstUse(t *testing.T) {
	uses := map[string]func(){
		"AccessorFor": func() { _ = xyz.AccessorFor(withWhen.Values) },
		"Unmarshal":   func() { _ = json.Unmarshal([]byte(`{"text":"x"}`), new(withWhen)) },
	}
	for name, use := range uses {
		message := firstUse(t, use)
		for _, want := range []string{"'Op'", "xyz.When", "not implemented", "xyz.Case"} {
			if !strings.Contains(message, want) {
				t.Errorf("%s: the panic doesn't mention %s: %q", name, want, message)
			}
		}
	}
	message := firstUse(t, func() { _ = xyz.AccessorFor(withInt.Values) })
	for _, want := range []string{"'Count'", "int", "neither the type of the variant nor an xyz.Case"} {
		if !strings.Contains(message, want) {
			t.Errorf("a field that is no case: the panic doesn't mention %s: %q", want, message)
		}
	}
	// Before, String dereferenced a nil accessor.
	var unset xyz.When[withInt, textual]
	if got := unset.String(); got != "" {
		t.Errorf("a When that is no case of anything is named %q", got)
	}
}

type retryOptions struct {
	Retries xyz.Maybe[int] `json:"retries"`
}

// A Maybe is a map, which a copy shares, so decoding has to replace it: a
// struct of defaults that each request is decoded over must stay as it is.
func TestDecodingIntoACopyOfAMaybeLeavesTheOriginal(t *testing.T) {
	for _, test := range []struct {
		body     string
		want     int
		included bool
		fails    bool
	}{
		{body: `{"retries":5}`, want: 5, included: true},
		{body: `{"retries":null}`},
		{body: `{}`, want: 3, included: true},
		{body: `{"retries":"many"}`, want: 3, included: true, fails: true},
	} {
		defaults := retryOptions{Retries: xyz.New(3)}
		request := defaults
		if err := json.Unmarshal([]byte(test.body), &request); (err != nil) != test.fails {
			t.Errorf("%s: %v", test.body, err)
		}
		if got, ok := request.Retries.Get(); got != test.want || ok != test.included {
			t.Errorf("%s decoded to %v, %v; want %v, %v", test.body, got, ok, test.want, test.included)
		}
		if got, ok := defaults.Retries.Get(); !ok || got != 3 {
			t.Errorf("decoding %s into a copy changed the original to %v, %v; want 3, true", test.body, got, ok)
		}
	}

	// The other way round: a value decoded into the copy of an omitted Maybe.
	var omitted retryOptions
	request := omitted
	if err := json.Unmarshal([]byte(`{"retries":5}`), &request); err != nil {
		t.Fatal(err)
	}
	if _, ok := omitted.Retries.Get(); ok {
		t.Error("decoding into a copy included the original's value")
	}
}

// null leaves a Maybe that was nil nil, and one that was not empty but not nil,
// as clearing it in place did.
func TestNullKeepsWhetherAMaybeIsNil(t *testing.T) {
	var unset xyz.Maybe[int]
	if err := json.Unmarshal([]byte(`null`), &unset); err != nil || unset != nil {
		t.Errorf("null into a nil Maybe gave %v, %v", unset, err)
	}
	set := xyz.New(3)
	if err := json.Unmarshal([]byte(`null`), &set); err != nil || set == nil || len(set) != 0 {
		t.Errorf("null into a set Maybe gave %v, %v", set, err)
	}
}

// Requests are decoded at once, each over its own copy of the same defaults.
// Run with -race, this fails if any of them writes the map they share.
func TestCopiesOfAMaybeDecodeAtOnce(t *testing.T) {
	defaults := retryOptions{Retries: xyz.New(3)}
	var decoders sync.WaitGroup
	for i := range 8 {
		decoders.Go(func() {
			body := `{"retries":5}`
			if i%2 == 1 {
				body = `{"retries":null}`
			}
			for range 100 {
				request := defaults
				if err := json.Unmarshal([]byte(body), &request); err != nil {
					t.Error(err)
				}
			}
		})
	}
	decoders.Wait()
	if got, ok := defaults.Retries.Get(); !ok || got != 3 {
		t.Errorf("the defaults are now %v, %v", got, ok)
	}
}

// An array with more elements than the tuple has values is not that tuple, and
// decoding it would lose the rest without a word. A shorter one is an error as
// it always was.
func TestTuplesOfMoreElementsAreAnError(t *testing.T) {
	for _, input := range []string{`[1,2,3]`, `[1,2,3,4,5,6]`} {
		p := xyz.NewPair(8, 9)
		if err := json.Unmarshal([]byte(input), &p); err == nil {
			t.Errorf("%s decoded to the pair %v", input, p)
		}
	}
	for _, input := range []string{`[1,2,3,4]`} {
		r := xyz.NewTrio(7, 8, 9)
		if err := json.Unmarshal([]byte(input), &r); err == nil {
			t.Errorf("%s decoded to the trio %v", input, r)
		}
	}
	for _, input := range []string{`[1,2,3,4,5]`} {
		q := xyz.NewQuad(6, 7, 8, 9)
		if err := json.Unmarshal([]byte(input), &q); err == nil {
			t.Errorf("%s decoded to the quad %v", input, q)
		}
	}
	for _, input := range []string{`[]`, `[1]`} {
		p := xyz.NewPair(8, 9)
		if err := json.Unmarshal([]byte(input), &p); err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
			t.Errorf("%s gave %v, want the error it always gave", input, err)
		}
	}
	// A tuple inside a tuple counts as one element of it.
	var nested xyz.Pair[xyz.Pair[int, int], int]
	if err := json.Unmarshal([]byte(`[[1,2,3],4]`), &nested); err == nil {
		t.Errorf("[[1,2,3],4] decoded to %v", nested)
	}
	if err := json.Unmarshal([]byte(`[[1,2],4]`), &nested); err != nil || nested != xyz.NewPair(xyz.NewPair(1, 2), 4) {
		t.Errorf("[[1,2],4] decoded to %v, %v", nested, err)
	}
}

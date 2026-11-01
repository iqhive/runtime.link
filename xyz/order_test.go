package xyz_test

import (
	"encoding/json"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

// Once a ",string" case of a number is reached, the cases after it read their
// value as text too, which is how 840f71d decoded this.
func TestStringHintReadsLaterCasesAsText(t *testing.T) {
	var v stringHint
	if err := json.Unmarshal([]byte(`{"b":"5"}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != stringHints.B || stringHints.B.Get(v) != 5 {
		t.Errorf("got %v", v)
	}
}

type kindNamed xyz.Tagged[any, struct {
	Text   xyz.Case[kindNamed, string]  `json:"text,string"`
	Number xyz.Case[kindNamed, float64] `json:"number,number"`
}]

var kindNameds = xyz.AccessorFor(kindNamed.Values)

// 840f71d kept every object of this type as unknown data. One that a case
// writes, as it is, is that case now, and anything else is still kept whole.
func TestUnknownDataBecomesACaseOnlyWhenNothingIsLost(t *testing.T) {
	var v kindNamed
	if err := json.Unmarshal([]byte(`{"text":"x"}`), &v); err != nil {
		t.Fatal(err)
	}
	if xyz.ValueOf(v) != kindNameds.Text || kindNameds.Text.Get(v) != "x" {
		t.Errorf("got %v", v)
	}
	for _, input := range []string{`{"text":null}`, `{"text":"x","number":5}`, `{"number":5.0}`} {
		var v kindNamed
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			t.Fatal(err)
		}
		if !v.IsZero() {
			t.Errorf("%s decoded to %v", input, xyz.ValueOf(v))
		}
		if b, err := v.MarshalJSON(); err != nil || string(b) != input {
			t.Errorf("%s came back as %s, %v", input, b, err)
		}
	}
}

type namedFirst xyz.Tagged[any, struct {
	Named xyz.Case[namedFirst, int]        `json:"named"`
	Any   xyz.Case[namedFirst, wireObject] `json:",object"`
}]

// The first case that matches decides, and if its value does not decode, that
// is the error, as in 840f71d, rather than a later case that loses the value.
func TestFailedDecodeKeepsTheErrorOfTheCaseThatMatched(t *testing.T) {
	want := json.Unmarshal([]byte(`"x"`), new(int))
	var v namedFirst
	err := json.Unmarshal([]byte(`{"named":"x"}`), &v)
	if err == nil || err.Error() != want.Error() {
		t.Errorf("got %v, want %v", err, want)
	}
	if !v.IsZero() {
		t.Errorf("the failure changed the value to %v", xyz.ValueOf(v))
	}

	data := []byte(`{"type":"int","value":"x"}`)
	want = json.Unmarshal(data, new(int))
	var m merged
	if err := json.Unmarshal(data, &m); err == nil || err.Error() != want.Error() {
		t.Errorf("got %v, want %v", err, want)
	}
}

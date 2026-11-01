package id

import (
	"errors"
	"testing"
)

// The tests in this file pin how identifier errors and JSON look to callers,
// so it must not change.

func TestWrapUnwrapsToTheCause(t *testing.T) {
	cause := errors.New("bad data size")
	err := Wrap("ulid", cause)
	if got := errors.Unwrap(err); got != cause {
		t.Errorf("errors.Unwrap(Wrap) = %v, want the cause", got)
	}
	if err.Error() != "ulid: bad data size" {
		t.Errorf("message = %q", err.Error())
	}
	if Wrap("ulid", cause) == err {
		t.Error("two Wrap calls returned errors that compare equal")
	}
}

func TestScanTextErrorHasNoCause(t *testing.T) {
	_, err := ScanText(42)
	if err == nil || err.Error() != "cannot scan int into an identifier" {
		t.Fatalf("ScanText(42) = %v", err)
	}
	if got := errors.Unwrap(err); got != nil {
		t.Errorf("errors.Unwrap(ScanText(42)) = %v, want nil", got)
	}
}

func TestMarshalJSONEmitsTheTextHeld(t *testing.T) {
	cases := map[string]string{"abc": `"abc"`, "ABC": `"ABC"`, `a"b`: `"a\"b"`}
	for in, want := range cases {
		if b, err := MarshalJSON(in); err != nil || string(b) != want {
			t.Errorf("MarshalJSON(%q) = %s, %v, want %s", in, b, err, want)
		}
	}
}

package may_test

import (
	"encoding/json"
	"testing"

	"runtime.link/may"
)

func TestOmit(t *testing.T) {
	var val may.Omit[string]
	val = may.Include("hello")

	v, ok := val.Get()
	if !ok {
		t.Fatal("unexpected value")
	}
	if v != "hello" {
		t.Fatal("unexpected value")
	}

	clear(val)

	b, err := json.Marshal(struct {
		Field may.Omit[string] `json:"field,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{}" {
		t.Fatal("unexpected value")
	}
}

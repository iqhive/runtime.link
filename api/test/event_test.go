package test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventArgNamesJSON(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Event{Call: "Hello", ArgNames: []string{"who", "n"}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Call != "Hello" || len(decoded.ArgNames) != 2 || decoded.ArgNames[0] != "who" {
		t.Errorf("round-trip = %+v from %s", decoded, data)
	}

	empty, err := json.Marshal(Event{Call: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(empty), "argNames") {
		t.Errorf("omitzero failed: %s", empty)
	}
}

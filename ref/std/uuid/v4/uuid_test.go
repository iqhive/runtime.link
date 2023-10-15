package uuid

import (
	"testing"
)

func TestUUID(t *testing.T) {
	uuid := New[any]()
	if uuid.String() == "" {
		t.Fatal("expected non-empty string")
	}
	var uuid2 For[any]
	if err := uuid2.UnmarshalText([]byte(uuid.String())); err != nil {
		t.Fatal(err)
	}
}

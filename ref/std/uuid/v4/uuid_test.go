package uuid

import (
	"testing"
)

func TestUUID(t *testing.T) {
	uuid := New()
	if uuid.String() == "" {
		t.Fatal("expected non-empty string")
	}

	var uuid2 Ref
	if err := uuid2.UnmarshalText([]byte(uuid.String())); err != nil {
		t.Fatal(err)
	}
}

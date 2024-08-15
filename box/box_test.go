package box

import (
	"bytes"
	"fmt"

	"testing"
)

func TestBox(t *testing.T) {
	type Something struct {
		Number int
		Bool   bool
	}
	var (
		s = Something{Number: 42, Bool: true}
	)
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(s); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.Bytes())
	var decoded Something
	if err := NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != s {
		t.Fatalf("decoded value does not match original: %v != %v", decoded, s)
	}
}

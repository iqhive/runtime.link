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
	enc := NewEncoder(&buf)
	enc.SetPacked(true)
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.Bytes())
	buf = bytes.Buffer{}
	enc = NewEncoder(&buf)
	enc.SetPacked(false)
	if err := enc.Encode(s); err != nil {
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

func TestStrings(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode("hello"); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.Bytes())

	buf = bytes.Buffer{}
	var Strings struct {
		A string
		B string
	}
	Strings.A = "hello"
	enc = NewEncoder(&buf)
	enc.SetPacked(true)
	if err := enc.Encode(Strings); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.Bytes())
}

package grpc_test

import (
	"bytes"
	"testing"

	"github.com/iqhive/runtime.link/api/grpc"
)

type listNode struct {
	Name string
	Next *listNode
}

type selfPointer *selfPointer

// A value that cannot be encoded used to be found out halfway through, with
// the start of the message already written under a length that counted the
// bad field as minus one byte. Nothing told the reader of the stream that the
// message had been abandoned.
func TestEncodeWritesNothingForAValueItRefuses(t *testing.T) {
	type inner struct {
		Before string
		Bad    chan int
	}
	for name, value := range map[string]any{
		"func field": struct {
			Name string
			Hook func()
		}{Name: "hello"},
		"complex field between strings": struct {
			A string
			C complex64
			B string
		}{A: "x", B: "y"},
		"chan in a slice of any":         []any{1, make(chan int)},
		"uintptr in a map of any":        map[string]any{"a": uintptr(1)},
		"complex128 at the top":          complex(1, 2),
		"uintptr in a struct in a slice": []any{"ok", struct{ P uintptr }{1}},
		"nested struct":                  struct{ Outer inner }{inner{Before: "written"}},
		"behind a pointer":               &inner{Before: "written"},
		"array element":                  [2]any{"written", func() {}},
		"map key":                        map[any]string{complex64(1): "v"},
		"slice in a map in a struct": struct {
			ID    int
			Named map[string][]any
		}{ID: 7, Named: map[string][]any{"k": {"written", make(chan bool)}}},
	} {
		var buf bytes.Buffer
		err := grpc.NewEncoder(&buf).Encode(value)
		if err == nil {
			t.Errorf("%s: encoded without an error", name)
		}
		if buf.Len() > 0 {
			t.Errorf("%s: Encode failed (%v) after writing % x", name, err, buf.Bytes())
		}
	}
}

// A func or chan that is never reached is not a reason to refuse a value.
func TestEncodeSkipsUnsupportedTypesItNeverReaches(t *testing.T) {
	var buf bytes.Buffer
	value := struct {
		Name   string
		hook   func()
		Absent *chan int
		Empty  []func()
		Unset  any
	}{Name: "ok"}
	if err := grpc.NewEncoder(&buf).Encode(value); err != nil {
		t.Fatal(err)
	}
	want := []byte{0x0a, 0x04, 0x0a, 0x02, 'o', 'k'}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got % x, want % x", buf.Bytes(), want)
	}
}

// A value that leads back to itself used to recurse until the stack ran out,
// which is a fatal error that ends the process.
func TestEncodeRefusesValuesThatLeadBackToThemselves(t *testing.T) {
	ring := &listNode{Name: "a"}
	ring.Next = &listNode{Name: "b", Next: ring}

	var pointer selfPointer
	pointer = &pointer

	slice := []any{"first", nil}
	slice[1] = slice

	table := map[string]any{"name": "table"}
	table["self"] = table

	for name, value := range map[string]any{
		"ring of structs":       ring,
		"pointer to itself":     pointer,
		"slice holding itself":  slice,
		"map holding itself":    table,
		"ring inside a message": struct{ Head *listNode }{ring},
	} {
		var buf bytes.Buffer
		err := grpc.NewEncoder(&buf).Encode(value)
		if err == nil {
			t.Errorf("%s: encoded without an error", name)
		}
		if buf.Len() > 0 {
			t.Errorf("%s: Encode failed (%v) after writing %d bytes", name, err, buf.Len())
		}
	}
}

// The depth limit is for values without an end. A long list and a value
// reached twice are neither.
func TestEncodeAcceptsDeepAndSharedValues(t *testing.T) {
	var list *listNode
	for range 1000 {
		list = &listNode{Name: "n", Next: list}
	}
	var buf bytes.Buffer
	if err := grpc.NewEncoder(&buf).Encode(list); err != nil {
		t.Fatalf("a list of 1000: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("a list of 1000: nothing written")
	}

	shared := &listNode{Name: "shared"}
	buf.Reset()
	err := grpc.NewEncoder(&buf).Encode(struct{ Left, Right *listNode }{shared, shared})
	if err != nil {
		t.Fatalf("a value reached twice: %v", err)
	}
	once := []byte{0x0a, 0x06, 's', 'h', 'a', 'r', 'e', 'd'}
	want := append([]byte{0x0a, 0x14, 0x0a, 0x08}, once...)
	want = append(want, 0x12, 0x08)
	want = append(want, once...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("a value reached twice: got % x, want % x", buf.Bytes(), want)
	}
}

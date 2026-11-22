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

func refused(t *testing.T, name string, value any) {
	t.Helper()
	var buf bytes.Buffer
	err := grpc.NewEncoder(&buf).Encode(value)
	if err == nil {
		t.Errorf("%s: encoded as % x without an error", name, buf.Bytes())
	}
	if buf.Len() > 0 {
		t.Errorf("%s: Encode failed (%v) after writing % x", name, err, buf.Bytes())
	}
}

// A repeated field is its elements one after another, with nothing to say
// where a list among them ends. Lists of lists and of maps used to be written
// that way, so that [[1 2] [3]] and [[1] [2 3]] were the same bytes, and a
// reader was handed one list with no error on either side.
func TestEncodeRefusesListsOfListsAndOfMaps(t *testing.T) {
	inner := []int{1, 2}
	for name, value := range map[string]any{
		"list of lists":               [][]int{{1, 2}, {3}},
		"list of one list":            [][]int{{1, 2, 3}},
		"list of an empty list":       [][]string{{}},
		"list of a nil list":          [][]string{nil},
		"list of maps":                []map[string]int{{"a": 1}, {"b": 2}},
		"list of an empty map":        []map[string]int{{}},
		"list of any holding lists":   []any{[]int{1}, []int{2}},
		"list of any holding a map":   []any{"a", map[string]int{"b": 2}},
		"list of pointers to lists":   []*[]int{&inner},
		"list of lists of bytes":      [][][]byte{{{1}}, {{2}}},
		"in a field of a message":     struct{ Rows [][]int }{[][]int{{1}, {2}}},
		"in a value of a map":         map[string][][]int{"k": {{1}, {2}}},
		"in an element of an array":   [1][][]int{{{1}, {2}}},
		"in a message that is listed": []struct{ Rows [][]int }{{[][]int{{1}}}},
	} {
		refused(t, name, value)
	}
}

// A nil element used to be left out, so that a list arrived shorter, with
// every element after the nil moved up, and with no error on either side.
func TestEncodeRefusesNilElementsOfAList(t *testing.T) {
	type item struct{ N int32 }
	var (
		one, two = int32(1), int32(2)
		unset    *int32
		behind   = &unset
	)
	for name, value := range map[string]any{
		"nil pointer to a number":      []*int32{nil, &one, &two},
		"nil pointer to a message":     []*item{{1}, nil, {2}},
		"nil any":                      []any{nil, "a"},
		"only a nil":                   []*int32{nil},
		"nil at the end":               []*int32{&one, nil},
		"nil behind a pointer":         []**int32{behind},
		"nil pointer inside an any":    []any{"a", unset},
		"in a field of a message":      struct{ Items []*item }{[]*item{nil}},
		"in a value of a map":          map[string][]*int32{"k": {nil}},
		"in a message that is listed":  []struct{ Items []*item }{{[]*item{nil}}},
		"nil error":                    []error{nil},
		"nil pointer to a list":        []*[]byte{nil},
		"nil pointer to bytes, in any": []any{(*[]byte)(nil)},
	} {
		refused(t, name, value)
	}
}

// What is refused is a list inside a list and a nil inside a list, and no
// more than that: the lists that have a form, and a nil anywhere a field can
// be left out to say so, are written as they were.
func TestEncodeStillWritesListsAndNilsThatHaveAForm(t *testing.T) {
	type item struct{ N int32 }
	var (
		one, two = int32(1), int32(2)
		unset    *int32
	)
	for name, test := range map[string]struct {
		value any
		want  []byte
	}{
		"list of bytes, empty and nil ones too": {[][]byte{{1}, {}, nil, {2}}, []byte{0x0a, 1, 1, 0x0a, 0, 0x0a, 0, 0x0a, 1, 2}},
		"list of arrays":                        {[][2]int32{{1, 2}, {3, 4}}, []byte{0x0a, 4, 0x08, 1, 0x10, 2, 0x0a, 4, 0x08, 3, 0x10, 4}},
		"list of pointers":                      {[]*int32{&one, &two}, []byte{0x08, 1, 0x08, 2}},
		"list of messages":                      {[]*item{{1}, {2}}, []byte{0x0a, 2, 0x08, 1, 0x0a, 2, 0x08, 2}},
		"list of messages that hold lists":      {[]struct{ Row []int32 }{{[]int32{1, 2}}, {[]int32{3}}}, []byte{0x0a, 4, 0x08, 1, 0x08, 2, 0x0a, 2, 0x08, 3}},
		"list of any":                           {[]any{int32(1), "a"}, []byte{0x08, 1, 0x0a, 1, 'a'}},
		"map of lists":                          {map[string][]int32{"k": {1, 2}}, []byte{0x0a, 7, 0x0a, 1, 'k', 0x10, 1, 0x10, 2}},
		"map of maps":                           {map[string]map[string]int32{"k": {"l": 1}}, []byte{0x0a, 10, 0x0a, 1, 'k', 0x12, 5, 0x0a, 1, 'l', 0x10, 1}},
		"empty list of lists":                   {[][]int{}, nil},
		"nil list of pointers":                  {[]*int32(nil), nil},
		"nil field of a message": {struct {
			Unset *int32
			Text  string
		}{nil, "x"}, []byte{0x0a, 3, 0x12, 1, 'x'}},
		"nil element of an array": {[2]*int32{nil, &two}, []byte{0x0a, 2, 0x10, 2}},
		"nil value of a map":      {map[string]*int32{"k": nil}, []byte{0x0a, 3, 0x0a, 1, 'k'}},
		"nil pointer at the top":  {unset, nil},
	} {
		var buf bytes.Buffer
		if err := grpc.NewEncoder(&buf).Encode(test.value); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.want) {
			t.Errorf("%s: got % x, want % x", name, buf.Bytes(), test.want)
		}
	}
}

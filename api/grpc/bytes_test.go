package grpc

import (
	"bytes"
	"testing"
)

func encodeBytes(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBytesAreOneLengthDelimitedField(t *testing.T) {
	type named byte
	for _, value := range []any{[]byte{1, 2, 3}, []named{1, 2, 3}} {
		b := encodeBytes(t, value)
		num, typ, n := consumeTag(b)
		if n < 0 || num != 1 || typ != typeBytes {
			t.Fatalf("%T: tag %d type %d n %d in % x", value, num, typ, n, b)
		}
		body, m := consumeBytes(b[n:])
		if m < 0 || n+m != len(b) {
			t.Fatalf("%T: not one field: % x", value, b)
		}
		if !bytes.Equal(body, []byte{1, 2, 3}) {
			t.Errorf("%T: got % x", value, body)
		}
	}
}

// The length prefix of the enclosing message depends on sizeof agreeing with encode.
func TestBytesInsideMessages(t *testing.T) {
	type Message struct {
		Before int
		Data   []byte
		Empty  []byte
		After  string
	}
	b := encodeBytes(t, map[string]Message{"k": {Before: 1, Data: bytes.Repeat([]byte{7}, 300), After: "x"}})
	num, typ, n := consumeTag(b)
	if n < 0 || num != 1 || typ != typeBytes {
		t.Fatalf("entry: tag %d type %d n %d", num, typ, n)
	}
	entry, m := consumeBytes(b[n:])
	if m < 0 || n+m != len(b) {
		t.Fatalf("the entry length does not match its body")
	}
	key, _, keyLen := consumeField(entry)
	if keyLen < 0 || key != 1 {
		t.Fatalf("key: %d, %d", key, keyLen)
	}
	_, _, n = consumeTag(entry[keyLen:])
	if n < 0 {
		t.Fatal("no value tag")
	}
	message, m := consumeBytes(entry[keyLen+n:])
	if m < 0 || keyLen+n+m != len(entry) {
		t.Fatal("the message length does not match its body")
	}
	var fields []wireNumber
	for rest := message; len(rest) > 0; {
		num, _, n := consumeField(rest)
		if n < 0 {
			t.Fatalf("message: %v", parseError(n))
		}
		fields = append(fields, num)
		rest = rest[n:]
	}
	// An empty []byte is the protobuf default, so it is left out like any empty slice.
	if want := []wireNumber{1, 2, 4}; !bytes.Equal(wireNumbers(fields), wireNumbers(want)) {
		t.Errorf("fields %v, want %v", fields, want)
	}
}

func wireNumbers(numbers []wireNumber) []byte {
	var out []byte
	for _, number := range numbers {
		out = append(out, byte(number))
	}
	return out
}

func TestSlicesOfOtherElementsStayRepeated(t *testing.T) {
	b := encodeBytes(t, []int{1, 2, 3})
	var fields int
	for rest := b; len(rest) > 0; {
		_, typ, n := consumeField(rest)
		if n < 0 || typ != typeVarint {
			t.Fatalf("% x", b)
		}
		fields++
		rest = rest[n:]
	}
	if fields != 3 {
		t.Errorf("got %d fields", fields)
	}
}

func TestEncodeNil(t *testing.T) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("wrote % x", buf.Bytes())
	}
	var ptr *int
	if err := NewEncoder(&buf).Encode(ptr); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("wrote % x for a nil pointer", buf.Bytes())
	}
}

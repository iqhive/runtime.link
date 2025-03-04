package jrpc_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"testing"

	"runtime.link/api/jrpc"
	"runtime.link/bit"
)

func BenchmarkJSON(t *testing.B) {
	enc := json.NewEncoder(io.Discard)
	var structure struct {
		Hello string
	}
	structure.Hello = "world"
	for t.Loop() {
		enc.Encode(structure)
	}
}

type Discard struct{}

func (Discard) WriteBits(array bit.Array) (int, error) {
	return array.Len(), nil
}

func BenchmarkJRPC(t *testing.B) {
	enc := jrpc.NewEncoder(Discard{})
	var structure struct {
		Hello string
	}
	structure.Hello = "world"
	for t.Loop() {
		enc.Encode(structure)
	}
}

type BitWriter struct {
	buf bytes.Buffer
}

func (w *BitWriter) WriteBits(array bit.Array) (int, error) {
	n, err := w.buf.Write(array.Bytes())
	return n * 8, err
}

func TestEncoder(t *testing.T) {
	var buf = new(BitWriter)
	enc := jrpc.NewEncoder(buf)
	if err := enc.Encode(true); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != "true" {
		t.Fatal("unexpected output:", buf.buf.String())
	}
	buf.buf.Reset()
	if err := enc.Encode(math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != "9223372036854775807" {
		t.Fatal("unexpected output:", buf.buf.String())
	}
	buf.buf.Reset()
	if err := enc.Encode("hello world"); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != `"hello world"` {
		t.Fatal("unexpected output:", buf.buf.String())
	}
	buf.buf.Reset()
	arr := []int{1, 2, 3}
	if err := enc.Encode(arr); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != "[1,2,3]" {
		t.Fatal("unexpected output:", buf.buf.String())
	}
	mip := map[string]any{"hello": "world"}
	buf.buf.Reset()
	if err := enc.Encode(mip); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != `{"hello":"world"}` {
		t.Fatal("unexpected output:", buf.buf.String())
	}
	buf.buf.Reset()
	var structure struct {
		Hello string `json:"hello"`
	}
	structure.Hello = "world"
	if err := enc.Encode(structure); err != nil {
		t.Fatal(err)
	}
	if buf.buf.String() != `{"hello":"world"}` {
		t.Fatal("unexpected output:", buf.buf.String())
	}
}

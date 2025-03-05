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

	t.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			enc.Encode(map[string]string{"hello": "world"})
		}
	})
}

type Discard struct{}

func (Discard) WriteBits(array bit.Array) (int, error) {
	return array.Len(), nil
}

func BenchmarkJRPC(t *testing.B) {
	enc := jrpc.NewEncoder(io.Discard)

	t.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := make(map[string]string)
			m["hello"] = "world"
			enc.Encode(m)
		}
	})
}

type BitWriter struct {
	*bytes.Buffer
}

func (w BitWriter) WriteBits(array bit.Array) (int, error) {
	n, err := w.Write(array.Bytes())
	return n * 8, err
}

func TestEncoder(t *testing.T) {
	var buf = new(bytes.Buffer)
	enc := jrpc.NewEncoder(BitWriter{buf})
	if err := enc.Encode(true); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "true" {
		t.Fatal("unexpected output:", buf.String())
	}
	buf.Reset()
	if err := enc.Encode(math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "9223372036854775807" {
		t.Fatal("unexpected output:", buf.String())
	}
	buf.Reset()
	if err := enc.Encode("hello world"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `"hello world"` {
		t.Fatal("unexpected output:", buf.String())
	}
	buf.Reset()
	arr := []int{1, 2, 3}
	if err := enc.Encode(arr); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "[1,2,3]" {
		t.Fatal("unexpected output:", buf.String())
	}
	mip := map[string]any{"hello": "world"}
	buf.Reset()
	if err := enc.Encode(mip); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `{"hello":"world"}` {
		t.Fatal("unexpected output:", buf.String())
	}
	buf.Reset()
	var structure struct {
		Hello string `json:"hello"`
	}
	structure.Hello = "world"
	if err := enc.Encode(structure); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `{"hello":"world"}` {
		t.Fatal("unexpected output:", buf.String())
	}
}

package jrpc_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"testing"

	"github.com/iqhive/runtime.link/api/internal/bit"
	"github.com/iqhive/runtime.link/api/jrpc"
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
	if err := enc.Encode(int64(math.MaxInt64)); err != nil {
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

// A string is written as the standard package writes it by default: '<',
// '>' and '&' are escaped, because they can lead to security holes when
// user-controlled strings are rendered into JSON and served to some
// browsers.
func TestEncoderEscapesHTML(t *testing.T) {
	var buf = new(bytes.Buffer)
	enc := jrpc.NewEncoder(BitWriter{buf})
	if err := enc.Encode("<script>alert(\"1\")</script> &"); err != nil {
		t.Fatal(err)
	}
	const want = `"\u003cscript\u003ealert(\"1\")\u003c/script\u003e \u0026"`
	if buf.String() != want {
		t.Fatalf("unexpected output: %s, want %s", buf.String(), want)
	}
}

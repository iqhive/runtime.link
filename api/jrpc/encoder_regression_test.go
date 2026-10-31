package jrpc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/iqhive/runtime.link/api/jrpc"
)

func TestEncoderStructFields(t *testing.T) {
	type document struct {
		Hidden      string   `json:"-"`
		A           int      `json:"a"`
		Zero        int      `json:"zero,omitzero"`
		B           int      `json:"b,omitzero"`
		Empty       []int    `json:"empty,omitempty"`
		Values      []int    `json:"values,omitempty"`
		EmptyStruct struct{} `json:"emptyStruct,omitempty"`
		EmptyArray  [0]int   `json:"emptyArray,omitempty"`
		Array       [1]int   `json:"array,omitempty"`
		internal    string
	}
	for _, value := range []any{
		struct{ A, B int }{1, 2},
		struct {
			Secret string `json:"-"`
		}{"secret"},
		struct {
			Value int `json:"value,omitzero"`
		}{42},
		document{Hidden: "secret", A: 1, B: 2, Empty: []int{}, Values: []int{3}, internal: "private"},
		document{},
		[2]int{1, 2},
		[2]byte{1, 2},
		[0]int{},
		&[2]int{3, 4},
	} {
		t.Run(fmt.Sprintf("%T/%v", value, value), func(t *testing.T) {
			want, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := jrpc.NewEncoder(&buf).Encode(value); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Fatalf("got %s, want %s", buf.Bytes(), want)
			}
		})
	}
}

type failAfterWriter struct {
	remaining int
	err       error
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, w.err
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestEncoderStringWriterErrors(t *testing.T) {
	failure := errors.New("write failed")
	for _, value := range []string{"", "hello", "a\"b\\c\n\r\t\x00", "a\xffb", "a\u2028b\u2029c"} {
		var complete bytes.Buffer
		if err := jrpc.NewEncoder(&complete).Encode(value); err != nil {
			t.Fatal(err)
		}
		for limit := 0; limit < complete.Len(); limit++ {
			t.Run(fmt.Sprintf("%q/%d", value, limit), func(t *testing.T) {
				writer := &failAfterWriter{remaining: limit, err: failure}
				if err := jrpc.NewEncoder(writer).Encode(value); !errors.Is(err, failure) {
					t.Fatalf("got error %v, want %v", err, failure)
				}
			})
		}
	}
}

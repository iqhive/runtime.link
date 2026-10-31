package jrpc_test

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/iqhive/runtime.link/api/jrpc"
)

type chunkReader struct {
	io.Reader
	size int
}

func (r chunkReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:min(len(p), r.size)]) }

func TestDecoderShortReads(t *testing.T) {
	input := `{"text":"` + strings.Repeat("abcdefghijk", 60) + `","values":[123,456,789],"pointer":42}`
	type document struct {
		Text    string `json:"text"`
		Values  []int  `json:"values"`
		Pointer *int   `json:"pointer"`
	}
	var want document
	if err := json.Unmarshal([]byte(input), &want); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{1, 3, 255, 256, 257} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), size})
			var got document
			if err := dec.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded %#v, want %#v", got, want)
			}
		})
	}
}

func TestDecoderDataWithEOF(t *testing.T) {
	dec := jrpc.NewDecoder(iotest.DataErrReader(strings.NewReader(`"complete"`)))
	var got string
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != "complete" {
		t.Fatalf("got %q", got)
	}
	if err := dec.Decode(&got); err != io.EOF {
		t.Fatalf("next decode error = %v, want EOF", err)
	}
}

func TestDecoderNumberAtBufferBoundary(t *testing.T) {
	for _, input := range []string{
		"[" + strings.Repeat(" ", 253) + "1,2]",
		"[" + strings.Repeat(" ", 253) + "1.0,2.0]",
	} {
		dec := jrpc.NewDecoder(strings.NewReader(input))
		if strings.Contains(input, ".") {
			var got []float64
			if err := dec.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, []float64{1, 2}) {
				t.Fatalf("got %v", got)
			}
		} else {
			var got []int
			if err := dec.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, []int{1, 2}) {
				t.Fatalf("got %v", got)
			}
		}
	}
}

func TestDecoderFloatValues(t *testing.T) {
	for _, input := range []string{"-2.5", "-0.5", "2", "0", "-0", "2.5 ", "1e3", "-1.25e-2", "1.234567890123456789"} {
		t.Run(input, func(t *testing.T) {
			for _, target := range []any{new(float32), new(float64)} {
				want := reflect.New(reflect.TypeOf(target).Elem()).Interface()
				if err := json.Unmarshal([]byte(input), want); err != nil {
					t.Fatal(err)
				}
				dec := jrpc.NewDecoder(strings.NewReader(input))
				if err := dec.Decode(target); err != nil {
					t.Fatal(err)
				}
				gotFloat := reflect.ValueOf(target).Elem().Float()
				wantFloat := reflect.ValueOf(want).Elem().Float()
				if gotFloat != wantFloat || math.Signbit(gotFloat) != math.Signbit(wantFloat) {
					t.Fatalf("%T got %v, want %v", target, gotFloat, wantFloat)
				}
			}
		})
	}
}

func TestDecoderReusedBoolean(t *testing.T) {
	dec := jrpc.NewDecoder(chunkReader{strings.NewReader("true false true false"), 1})
	var got bool
	for _, want := range []bool{true, false, true, false} {
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDecoderPointerAcrossRefills(t *testing.T) {
	dec := jrpc.NewDecoder(chunkReader{strings.NewReader("42 null"), 1})
	var value *int
	if err := dec.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if value == nil || *value != 42 {
		t.Fatalf("got %v, want pointer to 42", value)
	}
	if err := dec.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("got %v, want nil", value)
	}
}

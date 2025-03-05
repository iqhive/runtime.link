package jrpc_test

import (
	"encoding/json"
	"strings"
	"testing"

	"runtime.link/api/jrpc"
)

func BenchmarkStandardDecodeBool(b *testing.B) {
	var dec = json.NewDecoder(strings.NewReader(`true`))
	b.ResetTimer()
	var v bool
	for i := 0; i < b.N; i++ {
		dec.Decode(&v)
	}
}

func BenchmarkStandardDecodeInt(b *testing.B) {
	var dec = json.NewDecoder(strings.NewReader(`22`))
	b.ResetTimer()
	var v int
	for i := 0; i < b.N; i++ {
		dec.Decode(&v)
	}
}

func BenchmarkDecodeBool(b *testing.B) {
	var rdr = strings.NewReader(`true`)
	b.ResetTimer()
	var dec = jrpc.NewDecoder(rdr)
	var v bool
	for i := 0; i < b.N; i++ {
		dec.Decode(&v)
	}
}

func BenchmarkDecodeInt(b *testing.B) {
	var rdr = strings.NewReader(`22`)
	b.ResetTimer()
	var dec = jrpc.NewDecoder(rdr)
	var v int
	for i := 0; i < b.N; i++ {
		dec.Decode(&v)
	}
}

func TestDecoder(t *testing.T) {
	var dec = jrpc.NewDecoder(strings.NewReader(`true`))
	var v bool
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if !v {
		t.Fatal("expected true")
	}
	dec = jrpc.NewDecoder(strings.NewReader(`22`))
	var i int
	if err := dec.Decode(&i); err != nil {
		t.Fatal(err)
	}
	if i != 22 {
		t.Fatal("expected 22, got", i)
	}
	dec = jrpc.NewDecoder(strings.NewReader(`2.2`))
	var f float64
	if err := dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	if f != 2.2 {
		t.Fatal("expected 2.2, got", f)
	}
}

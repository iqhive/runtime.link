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

func TestDecodeString(t *testing.T) {
	var dec = jrpc.NewDecoder(strings.NewReader(`"hello world"`))
	var s string
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s != "hello world" {
		t.Fatal("expected 'hello world', got", s)
	}
	
	// Test with escape sequences
	dec = jrpc.NewDecoder(strings.NewReader(`"hello\nworld"`))
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s != "hello\nworld" {
		t.Fatal("expected 'hello\\nworld', got", s)
	}
	
	// Test with unicode escape
	dec = jrpc.NewDecoder(strings.NewReader(`"hello\u0020world"`))
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s != "hello world" {
		t.Fatal("expected 'hello world', got", s)
	}
}

func TestDecodeObject(t *testing.T) {
	// Test map decoding
	var dec = jrpc.NewDecoder(strings.NewReader(`{"hello":"world"}`))
	var m map[string]string
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["hello"] != "world" {
		t.Fatal("expected m[\"hello\"] = \"world\", got", m["hello"])
	}
	
	// Test struct decoding
	dec = jrpc.NewDecoder(strings.NewReader(`{"hello":"world"}`))
	var s struct {
		Hello string `json:"hello"`
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.Hello != "world" {
		t.Fatal("expected s.Hello = \"world\", got", s.Hello)
	}
	
	// Test nested objects
	dec = jrpc.NewDecoder(strings.NewReader(`{"nested":{"value":42}}`))
	var n struct {
		Nested struct {
			Value int `json:"value"`
		} `json:"nested"`
	}
	if err := dec.Decode(&n); err != nil {
		t.Fatal(err)
	}
	if n.Nested.Value != 42 {
		t.Fatal("expected n.Nested.Value = 42, got", n.Nested.Value)
	}
}

func TestDecodeArray(t *testing.T) {
	// Test slice decoding
	var dec = jrpc.NewDecoder(strings.NewReader(`[1,2,3]`))
	var s []int
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 3 || s[0] != 1 || s[1] != 2 || s[2] != 3 {
		t.Fatal("expected [1,2,3], got", s)
	}
	
	// Test array decoding
	dec = jrpc.NewDecoder(strings.NewReader(`[1,2,3]`))
	var a [3]int
	if err := dec.Decode(&a); err != nil {
		t.Fatal(err)
	}
	if a[0] != 1 || a[1] != 2 || a[2] != 3 {
		t.Fatal("expected [1,2,3], got", a)
	}
	
	// Test nested arrays
	dec = jrpc.NewDecoder(strings.NewReader(`[[1,2],[3,4]]`))
	var n [][]int
	if err := dec.Decode(&n); err != nil {
		t.Fatal(err)
	}
	if len(n) != 2 || len(n[0]) != 2 || len(n[1]) != 2 ||
		n[0][0] != 1 || n[0][1] != 2 || n[1][0] != 3 || n[1][1] != 4 {
		t.Fatal("expected [[1,2],[3,4]], got", n)
	}
}

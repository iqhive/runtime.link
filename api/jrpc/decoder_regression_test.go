package jrpc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
	"testing/iotest"
	"time"

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

func TestDecoderEmptyContainers(t *testing.T) {
	for _, tc := range []struct {
		input  string
		target any
	}{
		{`{}`, new(map[string]int)},
		{`[{}, {"a":1}, {}]`, new([]map[string]int)},
		{`[{}, {}]`, new([]struct{})},
		{`[[], [1], []]`, new([][]int)},
		{`{"items":[],"more":[],"end":[2]}`, new(map[string][]int)},
		{`{"empty":{},"end":{"x":2}}`, new(map[string]map[string]int)},
	} {
		t.Run(tc.input, func(t *testing.T) {
			want := reflect.New(reflect.TypeOf(tc.target).Elem()).Interface()
			if err := json.Unmarshal([]byte(tc.input), want); err != nil {
				t.Fatal(err)
			}
			dec := jrpc.NewDecoder(chunkReader{strings.NewReader(tc.input + ` true`), 1})
			if err := dec.Decode(tc.target); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.target, want) {
				t.Fatalf("got %#v, want %#v", tc.target, want)
			}
			var next bool
			if err := dec.Decode(&next); err != nil || !next {
				t.Fatalf("next decode = %v, %v", next, err)
			}
		})
	}
}

func TestDecoderIntegerBounds(t *testing.T) {
	for _, target := range []any{new(int), new(int8), new(int16), new(int32), new(int64), new(uint), new(uint8), new(uint16), new(uint32), new(uint64)} {
		t.Run(fmt.Sprintf("%T", target), func(t *testing.T) {
			for _, input := range []string{
				"0", "127", "128", "-128", "-129", "255", "256", "32767", "32768", "-32768", "-32769",
				"65535", "65536", "2147483647", "2147483648", "-2147483648", "-2147483649",
				"4294967295", "4294967296", "9223372036854775807", "9223372036854775808",
				"-9223372036854775808", "-9223372036854775809", "18446744073709551615", "18446744073709551616",
				"99999999999999999999999999999999999999999999",
			} {
				got := reflect.New(reflect.TypeOf(target).Elem()).Interface()
				want := reflect.New(reflect.TypeOf(target).Elem()).Interface()
				wantErr := json.Unmarshal([]byte(input), want)
				dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), 1})
				err := dec.Decode(got)
				if (err == nil) != (wantErr == nil) || !reflect.DeepEqual(got, want) {
					t.Fatalf("input %s: got %v (%v), want %v (%v)", input, reflect.ValueOf(got).Elem(), err, reflect.ValueOf(want).Elem(), wantErr)
				}
			}
		})
	}
}

func TestDecoderUnknownFields(t *testing.T) {
	for _, value := range []string{`2`, `1e1000`, `"text"`, `"escaped\\\"{}[]"`, `true`, `false`, `null`, `[]`, `{}`, `[1,{"nested":[false,{},[],1e1000]}]`} {
		input := `{"before":` + value + `,"A":1,"after":` + value + `,"B":2}`
		dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), 1})
		var got struct{ A, B int }
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got.A != 1 || got.B != 2 {
			t.Fatalf("%s: got %+v", input, got)
		}
	}
}

func TestDecoderRejectsInvalidUnknownValues(t *testing.T) {
	for _, value := range []string{`tru`, `1e`, `[1,]`, `{"a":}`, `"\x"`} {
		dec := jrpc.NewDecoder(strings.NewReader(`{"unknown":` + value + `,"A":1}`))
		var got struct{ A int }
		if err := dec.Decode(&got); err == nil {
			t.Fatalf("accepted invalid unknown value %s", value)
		}
	}
}

type nestedArray []nestedArray

type nestedObject map[string]nestedObject

type nestedStruct struct {
	Next *nestedStruct `json:"n"`
}

// The stack limit is lowered so that unbounded recursion fails quickly instead
// of first growing to the 1 GB default.
func lowerStackLimit(t *testing.T) {
	t.Helper()
	limit := debug.SetMaxStack(64 << 20)
	t.Cleanup(func() { debug.SetMaxStack(limit) })
}

func TestDecoderRejectsExcessiveNesting(t *testing.T) {
	lowerStackLimit(t)
	type holder struct {
		Value any `json:"v"`
	}
	for _, tc := range []struct {
		name   string
		open   string
		target func() any
	}{
		{"array", `[`, func() any { return new(nestedArray) }},
		{"object", `{"n":`, func() any { return new(nestedObject) }},
		{"struct", `{"n":`, func() any { return new(nestedStruct) }},
		{"any array", `[`, func() any { return new(any) }},
		{"any object", `{"n":`, func() any { return new(any) }},
		{"unknown field", `{"n":`, func() any { return new(struct{}) }},
		{"array inside map", `[`, func() any { return new(map[string]any) }},
		{"raw", `[`, func() any { return new(json.RawMessage) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Repeat(tc.open, 500000)
			dec := jrpc.NewDecoder(strings.NewReader(input))
			err := dec.Decode(tc.target())
			if err == nil || !strings.Contains(err.Error(), "exceeded max depth") {
				t.Fatalf("got error %v, want exceeded max depth", err)
			}
		})
	}
}

func TestDecoderNestingLimitIsTheOneEncodingJSONUses(t *testing.T) {
	array := func(depth int) string { return strings.Repeat("[", depth) + strings.Repeat("]", depth) }
	object := func(depth int) string { return strings.Repeat(`{"n":`, depth-1) + `{}` + strings.Repeat("}", depth-1) }
	for _, tc := range []struct {
		name   string
		input  func(depth int) string
		target func() any
	}{
		{"array", array, func() any { return new(nestedArray) }},
		{"any", array, func() any { return new(any) }},
		{"object", object, func() any { return new(nestedObject) }},
		{"struct", object, func() any { return new(nestedStruct) }},
		{"unknown field", object, func() any { return new(struct{}) }},
		{"typed then any", array, func() any { return new([][][]any) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, depth := range []int{9999, 10000, 10001} {
				input := tc.input(depth)
				wantErr := json.Unmarshal([]byte(input), tc.target())
				dec := jrpc.NewDecoder(strings.NewReader(input))
				err := dec.Decode(tc.target())
				if (err == nil) != (wantErr == nil) {
					t.Fatalf("depth %d: got error %v, encoding/json %v", depth, err, wantErr)
				}
			}
		})
	}
}

func TestDecoderIgnoresUnexportedFieldNames(t *testing.T) {
	type document struct {
		Name   string `json:"name"`
		secret string
		tagged string `json:"tagged"`
	}
	for _, input := range []string{`{"secret":"x"}`, `{"tagged":"x"}`, `{"Secret":"x","name":"n"}`} {
		var got document
		dec := jrpc.NewDecoder(strings.NewReader(input))
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got.secret != "" || got.tagged != "" {
			t.Fatalf("%s: wrote an unexported field: %+v", input, got)
		}
	}
}

type hiddenTagged struct{ A int }

// A tag makes an embedded struct of an unexported type a member of its own,
// whose value reflect will neither hand out nor let anyone replace.
func TestDecoderTaggedEmbeddedUnexportedStruct(t *testing.T) {
	for _, input := range []string{
		`{"x":{"A":1}}`, `{"x":null}`, `{"x":1}`, `{"x":"s"}`, `{"x":true}`, `{"x":[1]}`, `{"X":{"a":2},"x":{"A":3}}`,
	} {
		t.Run(input, func(t *testing.T) {
			compareDecoding(t, input, func() any {
				return new(struct {
					hiddenTagged `json:"x"`
				})
			})
		})
	}
}

// encoding/json panics on the nil pointer here, so this is not compared with it.
func TestDecoderTaggedEmbeddedUnexportedPointer(t *testing.T) {
	type document struct {
		*hiddenTagged `json:"x"`
		B             int
	}
	for _, input := range []string{`{"x":{"A":1},"B":2}`, `{"x":1,"B":2}`, `{"x":[1],"B":2}`, `{"x":"s","B":2}`} {
		t.Run(input, func(t *testing.T) {
			var got document
			dec := jrpc.NewDecoder(strings.NewReader(input))
			if err := dec.Decode(&got); err == nil {
				t.Fatal("stored into a pointer nothing can set")
			}
			if got.B != 2 {
				t.Fatalf("did not finish the object: %+v", got)
			}
		})
	}
	for _, input := range []string{`{"x":null,"B":2}`, `{"B":2}`} {
		var got document
		dec := jrpc.NewDecoder(strings.NewReader(input))
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
	}
	got := document{hiddenTagged: &hiddenTagged{}}
	dec := jrpc.NewDecoder(strings.NewReader(`{"x":{"A":1}}`))
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.hiddenTagged.A != 1 {
		t.Fatalf("got %+v", got.hiddenTagged)
	}
}

func TestDecoderIntoAny(t *testing.T) {
	for _, input := range []string{
		`1`, `-2.5e3`, `"text"`, `true`, `false`, `null`, `[]`, `{}`,
		`[1,"a",null,[true],{"k":[]}]`, `{"id":1,"nested":{"list":[1.5,"x"]}}`,
		`12345678901234567890`, `"` + highSurrogate + lowSurrogate + `"`,
	} {
		t.Run(input, func(t *testing.T) {
			var want any
			if err := json.Unmarshal([]byte(input), &want); err != nil {
				t.Fatal(err)
			}
			var got any
			dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), 1})
			if err := dec.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestDecoderIntoMapOfAny(t *testing.T) {
	var got map[string]any
	dec := jrpc.NewDecoder(strings.NewReader(`{"id":1}`))
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"id": 1.0}) {
		t.Fatalf("got %#v", got)
	}
}

func TestDecoderIntoNonNilInterface(t *testing.T) {
	type point struct{ X, Y int }
	for _, tc := range []struct {
		name   string
		input  string
		target func() any
	}{
		{"number over number", `2`, func() any { var v any = 1; return &v }},
		{"string over number", `"s"`, func() any { var v any = 1; return &v }},
		{"null over number", `null`, func() any { var v any = 1; return &v }},
		{"object over value struct", `{"X":1}`, func() any { var v any = point{Y: 2}; return &v }},
		{"array over slice", `[1]`, func() any { var v any = []int{5}; return &v }},
		{"through a pointer", `{"X":1}`, func() any { var v any = &point{Y: 2}; return &v }},
		{"through a pointer to scalar", `7`, func() any { var v any = new(int); return &v }},
		{"null keeps pointer target", `null`, func() any { var v any = &point{Y: 2}; return &v }},
		{"struct field", `{"V":{"X":1}}`, func() any { return &struct{ V any }{V: &point{Y: 2}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, got := tc.target(), tc.target()
			wantErr := json.Unmarshal([]byte(tc.input), want)
			dec := jrpc.NewDecoder(strings.NewReader(tc.input))
			err := dec.Decode(got)
			if (err == nil) != (wantErr == nil) || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v (%v), want %#v (%v)", got, err, want, wantErr)
			}
		})
	}
}

type endlessPointer *endlessPointer

func TestDecoderRejectsEndlessPointers(t *testing.T) {
	lowerStackLimit(t)
	for _, tc := range []struct {
		name   string
		target func() any
	}{
		{"interface pointing at itself", func() any { var v any; v = &v; return &v }},
		{"interfaces pointing at each other", func() any { var a, b any; a, b = &b, &a; return &a }},
		{"pointer type of itself", func() any { return new(endlessPointer) }},
		{"pointer type pointing at itself", func() any { var p endlessPointer; p = &p; return &p }},
		{"field", func() any { return new(struct{ P endlessPointer }) }},
	} {
		for _, input := range []string{`1`, `"text"`, `null`, `[1]`, `{"P":1}`} {
			t.Run(tc.name+"/"+input, func(t *testing.T) {
				dec := jrpc.NewDecoder(strings.NewReader(input + ` 2`))
				_ = dec.Decode(tc.target())
				var next int
				if err := dec.Decode(&next); err != nil || next != 2 {
					t.Fatalf("next decode = %d, %v", next, err)
				}
			})
		}
	}
}

func TestDecoderIntoInterfaceWithMethods(t *testing.T) {
	var got fmt.Stringer
	dec := jrpc.NewDecoder(strings.NewReader(`1 2`))
	var mismatch *json.UnmarshalTypeError
	if err := dec.Decode(&got); !errors.As(err, &mismatch) {
		t.Fatalf("got error %v, want a type error", err)
	}
	var next int
	if err := dec.Decode(&next); err != nil || next != 2 {
		t.Fatalf("next decode = %d, %v", next, err)
	}
}

func TestDecoderNull(t *testing.T) {
	type document struct{ A, B int }
	one := 1
	for _, tc := range []struct {
		name   string
		target func() any
	}{
		{"slice", func() any { return &[]int{1} }},
		{"nil slice", func() any { return new([]int) }},
		{"bytes", func() any { return &[]byte{1} }},
		{"map", func() any { return &map[string]int{"a": 1} }},
		{"nil map", func() any { return new(map[string]int) }},
		{"pointer", func() any { p := &one; return &p }},
		{"interface", func() any { var v any = 1; return &v }},
		{"bool keeps its value", func() any { v := true; return &v }},
		{"string keeps its value", func() any { v := "kept"; return &v }},
		{"int keeps its value", func() any { v := 7; return &v }},
		{"float keeps its value", func() any { v := 1.5; return &v }},
		{"struct keeps its value", func() any { return &document{A: 1, B: 2} }},
		{"array keeps its value", func() any { return &[2]int{1, 2} }},
		{"time keeps its value", func() any { return &time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compareDecoding(t, `null`, tc.target)
			compareDecoding(t, `{"V":null}`, func() any {
				return reflect.New(reflect.StructOf([]reflect.StructField{{Name: "V", Type: reflect.TypeOf(tc.target()).Elem()}})).Interface()
			})
			compareDecoding(t, `[null]`, func() any {
				return reflect.New(reflect.SliceOf(reflect.TypeOf(tc.target()).Elem())).Interface()
			})
			compareDecoding(t, `{"k":null}`, func() any {
				return reflect.New(reflect.MapOf(reflect.TypeFor[string](), reflect.TypeOf(tc.target()).Elem())).Interface()
			})
		})
	}
}

// Built from pieces so the inputs keep their ASCII escapes: a tool that decodes
// escapes on the way into this file would otherwise turn them into the emoji.
const (
	highSurrogate      = `\` + `ud83d`
	lowSurrogate       = `\` + `ude00`
	upperHighSurrogate = `\` + `uD83D`
	upperLowSurrogate  = `\` + `uDE00`
	escapedLetter      = `\` + `u0041`
)

func TestDecoderSurrogatePairs(t *testing.T) {
	for _, input := range []string{
		`"` + highSurrogate + lowSurrogate + `"`,
		`"` + upperHighSurrogate + upperLowSurrogate + `"`,
		`"a` + highSurrogate + lowSurrogate + `b"`,
		`"` + highSurrogate + `"`,
		`"` + lowSurrogate + `"`,
		`"` + highSurrogate + `x"`,
		`"` + highSurrogate + `\n"`,
		`"` + highSurrogate + escapedLetter + `"`,
		`"` + highSurrogate + highSurrogate + lowSurrogate + `"`,
		`"` + lowSurrogate + highSurrogate + `"`,
		`"\` + `u0000\` + `u00e9\` + `u20ac"`,
	} {
		t.Run(input, func(t *testing.T) {
			compareDecoding(t, input, func() any { return new(string) })
			compareDecoding(t, `{`+input+`:1}`, func() any { return new(map[string]int) })
		})
	}
}

func TestDecoderSurrogatePairIsOneCharacter(t *testing.T) {
	var got string
	dec := jrpc.NewDecoder(strings.NewReader(`"` + highSurrogate + lowSurrogate + `"`))
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != "\xf0\x9f\x98\x80" {
		t.Fatalf("got %q", got)
	}
}

func TestDecoderStringsMatchEncodingJSON(t *testing.T) {
	for _, input := range []string{
		"\"caf\xc3\xa9\"",
		"\"a\xffb\"",
		"\"\xed\xa0\x80\"",
		"\"line\nbreak\"",
		"\"tab\there\"",
		"\"nul\x00\"",
		`"\/\b\f\n\r\t\"\\"`,
		`"\u00zz"`,
		`"\u12"`,
		`"\q"`,
		`"unterminated`,
		`"\`,
	} {
		t.Run(input, func(t *testing.T) {
			compareDecoding(t, input, func() any { return new(string) })
			compareDecoding(t, input, func() any { return new(any) })
		})
	}
}

func TestDecoderBase64Bytes(t *testing.T) {
	for _, input := range []string{`""`, `"aGk="`, `"aGk"`, `"!!"`, `null`, `[1,2]`, `[]`, `"a\` + `u0047k="`} {
		t.Run(input, func(t *testing.T) {
			compareDecoding(t, input, func() any { return new([]byte) })
			compareDecoding(t, input, func() any { return &[]byte{9, 9, 9, 9} })
			compareDecoding(t, `{"B":`+input+`}`, func() any { return new(struct{ B []byte }) })
		})
	}
}

func TestDecoderMapKeys(t *testing.T) {
	type name string
	for _, tc := range []struct {
		input  string
		target func() any
	}{
		{`{"1":"a","-2":"b"}`, func() any { return new(map[int]string) }},
		{`{"127":1}`, func() any { return new(map[int8]int) }},
		{`{"128":1}`, func() any { return new(map[int8]int) }},
		{`{"255":1}`, func() any { return new(map[uint8]int) }},
		{`{"256":1}`, func() any { return new(map[uint8]int) }},
		{`{"-1":1}`, func() any { return new(map[uint]int) }},
		{`{"x":1}`, func() any { return new(map[int]int) }},
		{`{"1.5":1}`, func() any { return new(map[int]int) }},
		{`{"18446744073709551615":1}`, func() any { return new(map[uint64]int) }},
		{`{"n":1}`, func() any { return new(map[name]int) }},
		{`{"k":1}`, func() any { return new(map[textKey]int) }},
		{`{"k":1,"K":2}`, func() any { return new(map[textKey]int) }},
	} {
		t.Run(fmt.Sprintf("%T/%s", tc.target(), tc.input), func(t *testing.T) {
			compareDecoding(t, tc.input, tc.target)
		})
	}
}

// encoding/json documents these as unsupported, and go1.25 rejects them.
// go1.27 reads some of them, so this is checked without comparing the two.
func TestDecoderRejectsOtherMapKeyKinds(t *testing.T) {
	for _, target := range []any{
		new(map[bool]int), new(map[float64]int), new(map[any]int), new(map[[1]int]int),
		new(map[struct{ A int }]int), new(map[*int]int),
	} {
		t.Run(fmt.Sprintf("%T", target), func(t *testing.T) {
			dec := jrpc.NewDecoder(strings.NewReader(`{"true":1} 2`))
			var mismatch *json.UnmarshalTypeError
			if err := dec.Decode(target); !errors.As(err, &mismatch) {
				t.Fatalf("got error %v", err)
			}
			var next int
			if err := dec.Decode(&next); err != nil || next != 2 {
				t.Fatalf("next decode = %d, %v", next, err)
			}
		})
	}
}

// Tag names follow the documented rules go1.25 applies; go1.27 accepts more.
func TestStructTagNamesWithInvalidCharactersAreIgnored(t *testing.T) {
	type document struct {
		Backslash int `json:"a\\b"`
		Quote     int `json:"c\"d"`
	}
	var got document
	dec := jrpc.NewDecoder(strings.NewReader(`{"Backslash":1,"Quote":2}`))
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != (document{1, 2}) {
		t.Fatalf("got %+v", got)
	}
	var buf bytes.Buffer
	if err := jrpc.NewEncoder(&buf).Encode(got); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `{"Backslash":1,"Quote":2}` {
		t.Fatalf("got %s", buf.Bytes())
	}
}

// A text key decides its own contents, so a client can make it unhashable.
type dynamicKey struct{ value any }

func (k *dynamicKey) UnmarshalText(text []byte) error {
	k.value = []byte(text)
	return nil
}

func TestDecoderUnhashableMapKey(t *testing.T) {
	var got map[dynamicKey]int
	dec := jrpc.NewDecoder(strings.NewReader(`{"k":1} 2`))
	if err := dec.Decode(&got); err == nil {
		t.Fatal("stored an unhashable key")
	}
	var next int
	if err := dec.Decode(&next); err != nil || next != 2 {
		t.Fatalf("next decode = %d, %v", next, err)
	}
}

type textKey struct{ value string }

func (k *textKey) UnmarshalText(text []byte) error {
	k.value = strings.ToUpper(string(text))
	return nil
}

func (k textKey) MarshalText() ([]byte, error) { return []byte(k.value), nil }

type shout string

func (s *shout) UnmarshalJSON(raw []byte) error {
	*s = shout(strings.ToUpper(string(raw)))
	return nil
}

type rejecting struct{}

var errRejected = errors.New("rejected")

func (*rejecting) UnmarshalJSON([]byte) error { return errRejected }

type EmbeddedExported struct {
	Promoted int
	Shadowed string
}

type embeddedHidden struct {
	Visible string
	hidden  string
}

type EmbeddedPointer struct {
	Deep []int `json:"deep"`
}

type embeddedHiddenPointer struct {
	Reached int
}

type embeddedScalar int

type EmbeddedScalar int

type ambiguousLeft struct{ Clash, Left int }

type ambiguousRight struct{ Clash, Right int }

type embeddedDocument struct {
	EmbeddedExported
	embeddedHidden
	*EmbeddedPointer
	Shadowed int
}

func TestDecoderMatchesEncodingJSON(t *testing.T) {
	type inner struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	type tagged struct {
		Name       string `json:"name"`
		Dash       string `json:"-,"`
		Skip       string `json:"-"`
		Options    string `json:",omitempty"`
		Unknown    string `json:"known,nonsense"`
		Punct      string `json:"a.b-c"`
		Other      int    `xml:"other"`
		MixedCase  int
		lowerField int
	}
	type quoted struct {
		Int     int         `json:"int,string"`
		Uint    uint8       `json:"uint,string"`
		Float   float64     `json:"float,string"`
		Bool    bool        `json:"bool,string"`
		Text    string      `json:"text,string"`
		Pointer *int        `json:"pointer,string"`
		Number  json.Number `json:"number,string"`
		Plain   []int       `json:"plain,string"`
	}
	ambiguous := func() any {
		return new(struct {
			ambiguousLeft
			ambiguousRight
		})
	}
	stamp := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, tc := range []struct {
		input  string
		target func() any
	}{
		// Field names.
		{`{"name":"a","-":"b","Skip":"c"}`, func() any { return new(tagged) }},
		{`{"Options":"a","known":"b","nonsense":"c"}`, func() any { return new(tagged) }},
		{`{"a.b-c":"a"}`, func() any { return new(tagged) }},
		{`{"other":1,"Other":2}`, func() any { return new(tagged) }},
		{`{"NAME":"a","mixedcase":1,"MIXEDCASE":2}`, func() any { return new(tagged) }},
		{`{"lowerField":1}`, func() any { return new(tagged) }},
		{"{\"\xe2\x84\xaaey\":1,\"Key\":2}", func() any { return new(struct{ Key int }) }},
		{`{"key":1,"Key":2,"KEY":3}`, func() any {
			return new(struct {
				Lower int `json:"key"`
				Upper int `json:"Key"`
			})
		}},
		{`{"KEY":3}`, func() any {
			return new(struct {
				Lower int `json:"key"`
				Upper int `json:"Key"`
			})
		}},
		// Embedded structs.
		{`{"Promoted":1,"Shadowed":2,"Visible":"v","hidden":"h","deep":[1]}`, func() any { return new(embeddedDocument) }},
		{`{"EmbeddedExported":{"Promoted":1}}`, func() any { return new(embeddedDocument) }},
		{`{"Clash":1,"Left":2,"Right":3}`, ambiguous},
		{`{"Value":1,"EmbeddedScalar":2,"embeddedScalar":3}`, func() any {
			return new(struct {
				EmbeddedScalar
				embeddedScalar
				Value int
			})
		}},
		{`{"Renamed":{"Promoted":1},"Promoted":2}`, func() any {
			return new(struct {
				EmbeddedExported `json:"Renamed"`
			})
		}},
		{`{"Reached":1,"Other":2}`, func() any {
			return new(struct {
				*embeddedHiddenPointer
				Other int
			})
		}},
		// Quoted scalars.
		{`{"int":"1","uint":"2","float":"1.5","bool":"true","text":"\"s\"","pointer":"3","number":"4"}`, func() any { return new(quoted) }},
		{`{"int":1}`, func() any { return new(quoted) }},
		{`{"int":null,"pointer":null}`, func() any { return &quoted{Int: 5} }},
		{`{"int":"null","text":"null","pointer":"null"}`, func() any { return &quoted{Int: 5} }},
		{`{"text":"s"}`, func() any { return new(quoted) }},
		{`{"int":"x"}`, func() any { return new(quoted) }},
		{`{"int":" 1"}`, func() any { return new(quoted) }},
		{`{"plain":[1]}`, func() any { return new(quoted) }},
		// Arrays and slices.
		{`[1,2,3]`, func() any { return new([2]int) }},
		{`[1]`, func() any { return &[3]int{7, 8, 9} }},
		{`[]`, func() any { return &[2]int{7, 8} }},
		{`[1,[2],{"a":3},"x"]`, func() any { return new([1]int) }},
		{`[1,2]`, func() any { return &[]int{7, 8, 9, 10} }},
		{`[]`, func() any { return &[]int{7, 8} }},
		{`[1,2,3,4,5,6,7,8,9]`, func() any { return new([]int) }},
		{`[{"x":1}]`, func() any { return &[]inner{{X: 7, Y: 8}} }},
		{`[1]`, func() any { return new([0]int) }},
		// Existing values.
		{`{"x":1}`, func() any { return &inner{Y: 2} }},
		{`{"a":1}`, func() any { return &map[string]int{"b": 2} }},
		{`{"a":{"x":1}}`, func() any { return &map[string]inner{"a": {Y: 2}} }},
		{`{"x":1,"x":2}`, func() any { return new(inner) }},
		{`{"a":1,"a":2}`, func() any { return new(map[string]int) }},
		{`{"I":{"x":1},"I":{"y":2}}`, func() any { return new(struct{ I inner }) }},
		{`{"P":{"x":1}}`, func() any { return &struct{ P *inner }{P: &inner{Y: 2}} }},
		// Pointers.
		{`1`, func() any { return new(***int) }},
		{`{"x":1}`, func() any { return new(**inner) }},
		{`[1]`, func() any { return new(*[]int) }},
		{`{"a":1}`, func() any { return new(*map[string]int) }},
		// Custom decoding.
		{`"2024-02-03T04:05:06Z"`, func() any { return new(time.Time) }},
		{`{"At":"2024-02-03T04:05:06Z"}`, func() any { return new(struct{ At time.Time }) }},
		{`{"At":"2024-02-03T04:05:06Z"}`, func() any { return &struct{ At *time.Time }{At: &stamp} }},
		{`"not a time"`, func() any { return new(time.Time) }},
		{`{"Kept":{"a":[1, 2]},"Number":12.50}`, func() any {
			return new(struct {
				Kept   json.RawMessage
				Number json.Number
			})
		}},
		{`[{"a":1},"b",3,null,true]`, func() any { return new([]json.RawMessage) }},
		{`"x"`, func() any { return new(shout) }},
		{`[1, 2]`, func() any { return new(shout) }},
		{`{"a": 1}`, func() any { return new(shout) }},
		{`["a",{"b":1}]`, func() any { return new([]shout) }},
		{`"text"`, func() any { return new(textKey) }},
		{`["text"]`, func() any { return new([]textKey) }},
		{`[]`, func() any { return new(textKey) }},
		{`{}`, func() any { return new(textKey) }},
		{`1`, func() any { return new(textKey) }},
		{`{"a":1}`, func() any { return new(rejecting) }},
		// Mismatches.
		{`"s"`, func() any { return new(int) }},
		{`1`, func() any { return new(string) }},
		{`1`, func() any { return new(bool) }},
		{`true`, func() any { return new(int) }},
		{`[]`, func() any { return new(int) }},
		{`{}`, func() any { return new(string) }},
		{`[]`, func() any { return new(map[string]int) }},
		{`{}`, func() any { return new([]int) }},
		{`{}`, func() any { return new([2]int) }},
		{`[]`, func() any { return new(struct{}) }},
		{`1.5`, func() any { return new(int) }},
		{`1e2`, func() any { return new(int) }},
		{`1e400`, func() any { return new(float64) }},
		{`1e400`, func() any { return new(any) }},
		{`1e60`, func() any { return new(float32) }},
		{`12`, func() any { return new(json.Number) }},
		{`"12"`, func() any { return new(json.Number) }},
		{`"x"`, func() any { return new(json.Number) }},
		{`1`, func() any { return new(uintptr) }},
		{`1`, func() any { return new(chan int) }},
		{`"s"`, func() any { return new(func()) }},
		{`[1]`, func() any { return new(complex128) }},
		{`{"C":1}`, func() any { return new(struct{ C chan int }) }},
		// Malformed input.
		{``, func() any { return new(any) }},
		{`   `, func() any { return new(int) }},
		{`nul`, func() any { return new(*int) }},
		{`nulx`, func() any { return new(*int) }},
		{`tru`, func() any { return new(bool) }},
		{`[`, func() any { return new([]int) }},
		{`[1`, func() any { return new([]int) }},
		{`[1,`, func() any { return new([]int) }},
		{`[1,]`, func() any { return new([]int) }},
		{`[,1]`, func() any { return new([]int) }},
		{`[1 2]`, func() any { return new([]int) }},
		{`{`, func() any { return new(map[string]int) }},
		{`{"a"`, func() any { return new(map[string]int) }},
		{`{"a":`, func() any { return new(map[string]int) }},
		{`{"a":1`, func() any { return new(map[string]int) }},
		{`{"a":1,`, func() any { return new(map[string]int) }},
		{`{"a":1,}`, func() any { return new(map[string]int) }},
		{`{a:1}`, func() any { return new(map[string]int) }},
		{`{1:1}`, func() any { return new(map[int]int) }},
		{`{"a" 1}`, func() any { return new(map[string]int) }},
		{`{"a":1 "b":2}`, func() any { return new(struct{ A, B int }) }},
		{`{"a":1,}`, func() any { return new(struct{ A int }) }},
		{`[}`, func() any { return new(any) }},
		{`{]`, func() any { return new(any) }},
		{`]`, func() any { return new(any) }},
		{`}`, func() any { return new(struct{}) }},
		{`,`, func() any { return new(int) }},
		{`:`, func() any { return new(int) }},
		{`+1`, func() any { return new(int) }},
		{`01`, func() any { return new(int) }},
		{`-`, func() any { return new(int) }},
		{`1.`, func() any { return new(float64) }},
		{`.5`, func() any { return new(float64) }},
		{`1e`, func() any { return new(float64) }},
		{`0x10`, func() any { return new(int) }},
		{`NaN`, func() any { return new(float64) }},
		{`Infinity`, func() any { return new(float64) }},
		{"\xef\xbb\xbf1", func() any { return new(int) }},
		{`1 x`, func() any { return new(int) }},
		{`[] x`, func() any { return new([]int) }},
		{`1x`, func() any { return new(int) }},
		{`"a"b`, func() any { return new(string) }},
		{`truefalse`, func() any { return new(bool) }},
	} {
		t.Run(fmt.Sprintf("%T/%s", tc.target(), tc.input), func(t *testing.T) {
			compareDecoding(t, tc.input, tc.target)
		})
	}
}

// decodeAll decodes the single value input must hold, the way json.Unmarshal
// does, so anything after that value is a failure.
func decodeAll(dec *jrpc.Decoder, target any) error {
	if err := dec.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("data after the value: %v, %w", extra, err)
	}
	return nil
}

func compareDecoding(t *testing.T, input string, target func() any) {
	t.Helper()
	want, got := target(), target()
	wantErr := json.Unmarshal([]byte(input), want)
	dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), 1})
	err := decodeAll(&dec, got)
	if (err == nil) != (wantErr == nil) {
		t.Fatalf("%s into %T: got error %v, encoding/json %v", input, got, err, wantErr)
	}
	if err == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("%s into %T: got %s, encoding/json %s", input, got, describe(got), describe(want))
	}
}

func describe(value any) string {
	text, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%#v", value)
	}
	return string(text)
}

// readStream collects the values of a stream and how it ended. The step limit
// is there because a decoder that makes no progress must fail, not hang.
func readStream(decode func(any) error) ([]any, error) {
	var values []any
	for range 1000 {
		var value any
		if err := decode(&value); err != nil {
			return values, err
		}
		values = append(values, value)
	}
	return values, errors.New("stream did not end")
}

func compareStreams(t *testing.T, input []byte) {
	t.Helper()
	reference := json.NewDecoder(bytes.NewReader(input))
	wantValues, wantErr := readStream(reference.Decode)
	dec := jrpc.NewDecoder(chunkReader{bytes.NewReader(input), 3})
	gotValues, gotErr := readStream(dec.Decode)
	if !reflect.DeepEqual(gotValues, wantValues) {
		t.Fatalf("%q: got values %v, encoding/json %v", input, gotValues, wantValues)
	}
	// Not the other way round: a value read whole is only judged at its closing
	// delimiter, so input that is malformed and then cut off reads as cut off.
	if (gotErr == io.EOF) != (wantErr == io.EOF) || wantErr == io.ErrUnexpectedEOF && gotErr != wantErr {
		t.Fatalf("%q: stream ended with %v, encoding/json %v", input, gotErr, wantErr)
	}
}

func TestDecoderStreamsMatchEncodingJSON(t *testing.T) {
	for _, input := range []string{
		``, ` `, `1`, `1 2 3`, "1\n2\n", `{}{}`, `[][]`, `{"a":1}{"b":2}`, `[1][2]`, `"a""b"`, `"a" "b"`, `truefalse`, `nullnull`,
		`true false`, `null 1`, `{}1`, `1{}`, `1[]`, `1"a"`, `[]"a"`, `1 x`, `x`, `{} x`, `{"a":1`, `[1`, `"a`, `tru`, `1 tru`, `{}{`, `1e`, `-`,
		`]`, `}`, `,`, `1,2`, `1]`, `[]]`, `{}}`,
	} {
		t.Run(input, func(t *testing.T) { compareStreams(t, []byte(input)) })
	}
}

// encoding/json's stream reader splits these into two numbers. Here they are
// one malformed number, as they are to json.Unmarshal.
func TestDecoderRejectsNumbersRunTogether(t *testing.T) {
	for _, input := range []string{`1-2`, `00`, `01`, `1+2`, `1e5e5`, `1.5.5`, `1e5.5`, `[1-2]`, `{"A":00}`} {
		for _, target := range []any{new(any), new(int), new(float64), new([]int), new(struct{ A int })} {
			dec := jrpc.NewDecoder(strings.NewReader(input))
			if err := dec.Decode(target); err == nil {
				t.Fatalf("%s into %T: accepted", input, target)
			}
		}
	}
}

func hasNumbersRunTogether(input []byte) bool {
	for _, run := range regexp.MustCompile(`[-0-9][-+0-9.eE]*`).FindAll(input, -1) {
		if !json.Valid(run) {
			return true
		}
	}
	return false
}

func FuzzDecoderStreamsMatchEncodingJSON(f *testing.F) {
	for _, seed := range []string{``, `1 2`, `{}{}`, `[1][2]`, `"a""b"`, `truefalse`, `null 1`, `1"a"`, `{"a":1`, `1 x`, `]`, `{}}`, "1\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if hasNumbersRunTogether(input) {
			t.Skip("rejected on purpose: TestDecoderRejectsNumbersRunTogether")
		}
		compareStreams(t, input)
	})
}

func TestDecoderKeepsDecodingAfterAMismatch(t *testing.T) {
	type document struct {
		A int
		B int
		C []int
		D map[string]int
	}
	input := `{"A":"text","B":2,"C":[1,"x",3],"D":{"a":1,"b":"x","c":3}} {"A":1}`
	for _, size := range []int{1, 7, 1024} {
		var want, got document
		reference := json.NewDecoder(strings.NewReader(input))
		wantErr := reference.Decode(&want)
		dec := jrpc.NewDecoder(chunkReader{strings.NewReader(input), size})
		err := dec.Decode(&got)
		var mismatch *json.UnmarshalTypeError
		if !errors.As(wantErr, &mismatch) {
			t.Fatalf("encoding/json returned %v", wantErr)
		}
		if !errors.As(err, &mismatch) {
			t.Fatalf("got error %v, want a type error", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		if err := dec.Decode(&got); err != nil || got.A != 1 {
			t.Fatalf("next value: %+v, %v", got, err)
		}
	}
}

func TestDecoderReportsFirstMismatch(t *testing.T) {
	var got struct {
		A int
		B bool
	}
	dec := jrpc.NewDecoder(strings.NewReader(`{"A":"text","B":1}`))
	var mismatch *json.UnmarshalTypeError
	if err := dec.Decode(&got); !errors.As(err, &mismatch) || mismatch.Type.Kind() != reflect.Int {
		t.Fatalf("got error %v, want the mismatch on the int field", err)
	}
}

func TestDecoderReturnsUnmarshalerError(t *testing.T) {
	var got struct {
		R rejecting
		A int
	}
	dec := jrpc.NewDecoder(strings.NewReader(`{"R":[1,{"x":2}],"A":1} 2`))
	if err := dec.Decode(&got); !errors.Is(err, errRejected) {
		t.Fatalf("got error %v", err)
	}
	var next int
	if err := dec.Decode(&next); err != nil || next != 2 {
		t.Fatalf("next decode = %d, %v", next, err)
	}
}

func TestDecoderTruncatedInput(t *testing.T) {
	type document struct {
		A []any          `json:"a"`
		B map[string]int `json:"b"`
		C string         `json:"c"`
		D bool           `json:"d"`
		E *int           `json:"e"`
	}
	input := `{"a":[1,"x",{"k":null}],"b":{"k":1},"c":"text","d":true,"e":null}`
	for end := 1; end < len(input); end++ {
		for _, target := range []any{new(any), new(document), new(struct{})} {
			dec := jrpc.NewDecoder(strings.NewReader(input[:end]))
			if err := dec.Decode(target); err != io.ErrUnexpectedEOF {
				t.Fatalf("%s into %T: got error %v, want %v", input[:end], target, err, io.ErrUnexpectedEOF)
			}
		}
	}
}

func TestDecoderCleanEndOfStream(t *testing.T) {
	for _, input := range []string{``, ` `, "\n\t\r "} {
		var got any
		dec := jrpc.NewDecoder(strings.NewReader(input))
		if err := dec.Decode(&got); err != io.EOF {
			t.Fatalf("%q: got error %v, want EOF", input, err)
		}
	}
}

func TestDecoderReadErrorIsReturned(t *testing.T) {
	failure := errors.New("connection reset")
	input := `{"a":[1,"text`
	for end := 1; end <= len(input); end++ {
		reader := io.MultiReader(strings.NewReader(input[:end]), iotest.ErrReader(failure))
		for _, target := range []any{new(any), new(map[string][]any), new(struct{})} {
			dec := jrpc.NewDecoder(reader)
			if err := dec.Decode(target); err == nil {
				t.Fatalf("%s: read failure swallowed", input[:end])
			}
		}
	}
	dec := jrpc.NewDecoder(io.MultiReader(strings.NewReader(`[1,`), iotest.ErrReader(failure)))
	var got []int
	if err := dec.Decode(&got); !errors.Is(err, failure) {
		t.Fatalf("got error %v", err)
	}
}

func TestDecoderSyntaxErrorEndsTheStream(t *testing.T) {
	dec := jrpc.NewDecoder(strings.NewReader(`{"a":x} {"smuggled":1}`))
	var got map[string]int
	first := dec.Decode(&got)
	if first == nil {
		t.Fatal("accepted invalid input")
	}
	if err := dec.Decode(&got); err != first {
		t.Fatalf("got %v and %v after a syntax error", err, got)
	}
}

func TestDecoderRejectsBadTargets(t *testing.T) {
	var nilPointer *int
	var nilMap map[string]int
	for _, target := range []any{nil, nilPointer, 1, "text", nilMap, struct{}{}} {
		dec := jrpc.NewDecoder(strings.NewReader(`1`))
		if err := dec.Decode(target); err == nil {
			t.Fatalf("%#v accepted", target)
		}
	}
}

type differentialEmbedded struct {
	Promoted int
	Shadow   string
}

type DifferentialPointer struct {
	Reached []string `json:"reached"`
}

type differentialInner struct {
	X int     `json:"x"`
	Y *string `json:"y,omitempty"`
}

type (
	namedSlice []int
	namedMap   map[string]int
	namedBytes []byte
	namedText  string
)

type foldedNames struct {
	Key   int `json:"key"`
	KEY   int
	Other int `json:"OTHER"`
	Sna_k int
}

type differentialDocument struct {
	Bool     bool
	Int      int
	Int8     int8
	Uint8    uint8
	Uintptr  uintptr
	Float32  float32
	Float64  float64
	String   string
	Bytes    []byte
	Array    [2]int
	Slice    []int
	Map      map[string]int
	IntMap   map[int]string
	UintMap  map[uint8]bool
	TextMap  map[textKey]int
	Pointer  *int
	Double   **string
	Any      any
	Anys     []any
	AnyMap   map[string]any
	Number   json.Number
	Raw      json.RawMessage
	Time     time.Time
	Custom   shout
	Text     textKey
	Inner    differentialInner
	InnerPtr *differentialInner
	Inners   []differentialInner
	Quoted   int     `json:"quoted,string"`
	QFloat   float64 `json:"qfloat,string"`
	QBool    bool    `json:"qbool,string"`
	QText    string  `json:"qtext,string"`
	QPointer *int    `json:"qpointer,string"`
	Dash     int     `json:"-,"`
	Skipped  int     `json:"-"`
	Omit     int     `json:"omit,omitempty"`
	Zero     int     `json:"zero,omitzero"`
	Shadow   int
	hidden   int
	differentialEmbedded
	*DifferentialPointer
}

func differentialTargets() []func() any {
	return []func() any{
		func() any { return new(any) },
		func() any { return new(bool) },
		func() any { return new(int) },
		func() any { return new(int8) },
		func() any { return new(uint16) },
		func() any { return new(float32) },
		func() any { return new(float64) },
		func() any { return new(string) },
		func() any { return new([]byte) },
		func() any { return new(*int) },
		func() any { return new([]int) },
		func() any { return new([]any) },
		func() any { return new([][]string) },
		func() any { return new([3]int) },
		func() any { return new(map[string]any) },
		func() any { return new(map[int]string) },
		func() any { return new(map[uint8]float64) },
		func() any { return new(map[textKey][]int) },
		func() any { return new(json.Number) },
		func() any { return new(json.RawMessage) },
		func() any { return new(time.Time) },
		func() any { return new(differentialDocument) },
		func() any { return new([]differentialDocument) },
		func() any { return new(map[string]differentialDocument) },
		func() any { return new(embeddedDocument) },
		func() any { return new(foldedNames) },
		func() any { return new([]foldedNames) },
		func() any { return new(namedSlice) },
		func() any { return new(namedMap) },
		func() any { return new(namedBytes) },
		func() any { return new(namedText) },
		func() any { return new(map[namedText]namedSlice) },
		func() any { return new([]*int) },
		func() any { return new(map[string]*differentialInner) },
		func() any { return new(map[string]json.RawMessage) },
		func() any { return new([2][]any) },
		func() any { return new([4]uint8) },
		func() any { return new([]uint8) },
		func() any { return new([]time.Time) },
		func() any { return new([]shout) },
		func() any { return new(*big.Int) },
		func() any { return new(fmt.Stringer) },
		func() any { return new(struct{ S fmt.Stringer }) },
		func() any {
			return new(struct {
				hiddenTagged `json:"x"`
			})
		},
		func() any {
			var v any = &differentialInner{X: 1}
			return &v
		},
		func() any {
			var v any = []int{1}
			return &v
		},
		func() any { return &[]int{1, 2, 3} },
		func() any { return &map[string]any{"preset": 1.0} },
		func() any { return &differentialDocument{Int: 1, Slice: []int{1, 2}, Map: map[string]int{"preset": 1}} },
	}
}

func FuzzDecoderMatchesEncodingJSON(f *testing.F) {
	for _, seed := range []string{
		`null`, `true`, `false`, `0`, `-1.5e3`, `"text"`, `"` + highSurrogate + lowSurrogate + `"`, `"` + highSurrogate + `"`, `[]`, `{}`, `[1,2,3]`,
		`{"a":1,"b":[true,null,"x"],"c":{"d":1.5}}`, `{"1":"a"}`, `{"-1":1,"300":2}`, `"aGVsbG8="`, `[[1],[2,3]]`,
		`{"Bool":true,"INT":1,"Int8":300,"Uint8":-1,"Float32":1e60,"Bytes":"aGk=","Array":[1,2,3],"Slice":[1],"Map":{"a":1}}`,
		`{"IntMap":{"1":"a"},"UintMap":{"255":true},"TextMap":{"k":1},"Pointer":1,"Double":"s","Any":{"a":[1]}}`,
		`{"Anys":[1,"a"],"AnyMap":{"k":null},"Number":12.5,"Raw":{"a": 1},"Time":"2024-02-03T04:05:06Z","Custom":[1]}`,
		`{"Text":"t","Inner":{"x":1,"y":"s"},"InnerPtr":{"x":2},"Inners":[{"x":3},{}]}`,
		`{"quoted":"1","qfloat":"1.5","qbool":"true","qtext":"\"s\"","qpointer":"2"}`,
		`{"-":1,"omit":2,"zero":3,"Shadow":4,"Promoted":5,"reached":["a"],"hidden":6,"differentialEmbedded":{}}`,
		`{"Deep":[1],"EmbeddedExported":{},"Visible":"v","Shadowed":1}`,
		`{"key":1,"KEY":2,"Key":3,"other":4,"sna_k":5,"SNAK":6}`, `{"x":{"A":1}}`, `{"x":1}`, `{"S":1}`, `[null,1]`, `"2024-02-03T04:05:06Z"`,
		"\"a\xffb\"", "\"\x01\"", `[1,]`, `{"a":}`, `{"a":1,}`, `tru`, `1e`, `[`, `{`, `"`, `-`, `01`, `1 2`, `[}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, target := range differentialTargets() {
			want, got := target(), target()
			wantErr := json.Unmarshal(input, want)
			dec := jrpc.NewDecoder(chunkReader{bytes.NewReader(input), 3})
			err := decodeAll(&dec, got)
			if (err == nil) != (wantErr == nil) {
				t.Fatalf("%q into %T: got error %v, encoding/json %v", input, got, err, wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, want) {
				t.Fatalf("%q into %T: got %s, encoding/json %s", input, got, describe(got), describe(want))
			}
		}
	})
}

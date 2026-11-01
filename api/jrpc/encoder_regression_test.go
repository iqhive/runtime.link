package jrpc_test

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"
	"unsafe"

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

// compareEncoding checks that both encoders agree on whether value encodes and
// that their output parses to the same thing. Numbers stay text, so a format
// that only happens to parse back equal still shows up.
func compareEncoding(t *testing.T, value any) {
	t.Helper()
	want, wantErr := json.Marshal(value)
	var buf bytes.Buffer
	err := jrpc.NewEncoder(&buf).Encode(value)
	if (err == nil) != (wantErr == nil) {
		t.Fatalf("%#v: got %s, error %v; encoding/json %s, error %v", value, buf.Bytes(), err, want, wantErr)
	}
	if err != nil {
		if buf.Len() != 0 {
			t.Fatalf("%#v: wrote %s and then failed with %v", value, buf.Bytes(), err)
		}
		return
	}
	if !reflect.DeepEqual(parseKeepingNumbers(t, buf.Bytes()), parseKeepingNumbers(t, want)) {
		t.Fatalf("%#v: got %s, encoding/json %s", value, buf.Bytes(), want)
	}
}

func parseKeepingNumbers(t *testing.T, text []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		t.Fatalf("invalid JSON %s: %v", text, err)
	}
	return unquoteStrings(value)
}

// unquoteStrings reads the JSON text inside a ,string field, whose escapes may
// be spelled differently by encoders that agree on the string they stand for.
func unquoteStrings(value any) any {
	switch value := value.(type) {
	case string:
		var inner string
		if err := json.Unmarshal([]byte(value), &inner); err == nil && strings.HasPrefix(value, `"`) {
			return inner
		}
	case []any:
		for i := range value {
			value[i] = unquoteStrings(value[i])
		}
	case map[string]any:
		for key := range value {
			value[key] = unquoteStrings(value[key])
		}
	}
	return value
}

func TestEncoderNil(t *testing.T) {
	var nilPointer *int
	var nilMarshaler json.Marshaler
	var nilError error
	for _, value := range []any{nil, nilPointer, nilMarshaler, nilError, []any{nil}, map[string]any{"k": nil}} {
		compareEncoding(t, value)
	}
	var buf bytes.Buffer
	if err := jrpc.NewEncoder(&buf).Encode(nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "null" {
		t.Fatalf("got %s", buf.Bytes())
	}
}

func TestEncoderBytes(t *testing.T) {
	type name []byte
	type octet byte
	for _, tc := range []struct {
		value any
		want  string
	}{
		{[]byte(nil), `null`},
		{[]byte{}, `""`},
		{[]byte("h"), `"aA=="`},
		{[]byte("hi"), `"aGk="`},
		{[]byte("hi!"), `"aGkh"`},
		{name("hi"), `"aGk="`},
		{[]octet("hi"), `"aGk="`},
		{struct{ B []byte }{[]byte{}}, `{"B":""}`},
		{struct {
			B []byte `json:"b,omitempty"`
		}{[]byte{}}, `{}`},
	} {
		compareEncoding(t, tc.value)
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(tc.value); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tc.want {
			t.Fatalf("%#v: got %s, want %s", tc.value, buf.Bytes(), tc.want)
		}
	}
}

func TestBytesRoundTrip(t *testing.T) {
	for _, want := range [][]byte{nil, {}, {0}, []byte("hi"), bytes.Repeat([]byte{0xff, 0x00, 0x7f}, 100)} {
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(want); err != nil {
			t.Fatal(err)
		}
		got := []byte("existing")
		dec := jrpc.NewDecoder(&buf)
		if err := dec.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

// byteMarshaler has methods, so encoding/json writes a slice of them as an array.
type byteMarshaler byte

func (b byteMarshaler) MarshalText() ([]byte, error) { return []byte{'b', byte(b)}, nil }

func TestEncoderByteElementsWithMethods(t *testing.T) {
	compareEncoding(t, []byteMarshaler("hi"))
}

type textValue struct{ value string }

func (v textValue) MarshalText() ([]byte, error) { return []byte(v.value), nil }

type textPointer struct{ value string }

func (v *textPointer) MarshalText() ([]byte, error) { return []byte(v.value), nil }

type textFailure struct{}

func (textFailure) MarshalText() ([]byte, error) { return nil, errRejected }

func TestEncoderMapKeys(t *testing.T) {
	type name string
	type number int
	for _, value := range []any{
		map[int]string{1: "a"},
		map[int]string{-1: "a", 2: "b"},
		map[int8]int{-128: 1},
		map[int64]int{math.MaxInt64: 1},
		map[uint]int{1: 1},
		map[uint8]int{255: 1},
		map[uint64]int{math.MaxUint64: 1},
		map[uintptr]int{1: 1},
		map[string]int{"a\"b": 1},
		map[name]int{"n": 1},
		map[number]int{1: 1},
		map[textValue]int{{"t\"x"}: 1},
		map[*textPointer]int{{"t"}: 1, nil: 2},
		map[json.Number]int{"1": 1},
		map[time.Time]int{time.Unix(0, 0).UTC(): 1},
		map[textFailure]int{{}: 1},
		map[int]map[int]int{1: {2: 3}},
		map[int]string(nil),
		map[int]string{},
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			compareEncoding(t, value)
		})
	}
}

// encoding/json documents these as unsupported, and go1.25 rejects them.
// go1.27 writes some of them, so this is checked without comparing the two.
func TestEncoderRejectsOtherMapKeyKinds(t *testing.T) {
	for _, value := range []any{
		map[bool]int{true: 1},
		map[bool]int{},
		map[bool]int(nil),
		map[float64]int{1.5: 1},
		map[any]int{"a": 1},
		map[[1]int]int{{1}: 1},
		map[struct{ A int }]int{{1}: 1},
		map[*int]int{},
		struct{ M map[bool]int }{},
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			var buf bytes.Buffer
			var unsupported *json.UnsupportedTypeError
			if err := jrpc.NewEncoder(&buf).Encode(value); !errors.As(err, &unsupported) {
				t.Fatalf("got %s, error %v", buf.Bytes(), err)
			}
		})
	}
}

// encoding/json panics on both of these, so they are not compared with it.
func TestEncoderNilInterfaceMapKey(t *testing.T) {
	var buf bytes.Buffer
	value := map[encoding.TextMarshaler]int{nil: 1}
	if err := jrpc.NewEncoder(&buf).Encode(value); err == nil {
		t.Fatalf("got %s, want an error", buf.Bytes())
	}
	var nilText *textValue
	buf.Reset()
	if err := jrpc.NewEncoder(&buf).Encode(map[encoding.TextMarshaler]int{nilText: 1}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `{"":1}` {
		t.Fatalf("got %s", buf.Bytes())
	}
}

func TestMapKeysRoundTrip(t *testing.T) {
	want := map[int]map[uint8]string{-1: {255: "a"}, 2: {}}
	var buf bytes.Buffer
	if err := jrpc.NewEncoder(&buf).Encode(want); err != nil {
		t.Fatal(err)
	}
	var got map[int]map[uint8]string
	dec := jrpc.NewDecoder(&buf)
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestEncoderRejectsNonFiniteFloats(t *testing.T) {
	inf := math.Inf(1)
	nan := math.NaN()
	for _, value := range []any{
		nan, inf, math.Inf(-1), float32(nan), float32(inf), &nan,
		[]float64{1, nan},
		map[string]float64{"a": inf},
		struct{ F float64 }{nan},
		struct {
			F float64 `json:"f,string"`
		}{inf},
	} {
		compareEncoding(t, value)
		var unsupported *json.UnsupportedValueError
		if err := jrpc.NewEncoder(io.Discard).Encode(value); !errors.As(err, &unsupported) {
			t.Fatalf("%#v: got error %v", value, err)
		}
	}
}

func TestEncoderRejectsUnsupportedKinds(t *testing.T) {
	var nilFunc func()
	var nilChan chan int
	for _, value := range []any{
		func() {}, nilFunc, make(chan int), nilChan, complex(1, 2), complex64(1), unsafe.Pointer(nil),
		[]func(){nilFunc}, map[string]chan int{"c": nil}, []complex128{1},
		struct {
			F func() `json:"f"`
		}{},
		struct{ C chan int }{},
		struct{ A any }{nilFunc},
		&nilFunc,
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			compareEncoding(t, value)
			var unsupported *json.UnsupportedTypeError
			if err := jrpc.NewEncoder(io.Discard).Encode(value); !errors.As(err, &unsupported) {
				t.Fatalf("got error %v", err)
			}
		})
	}
}

func TestEncoderUnsupportedKindsThatAreNotWritten(t *testing.T) {
	var nilFunc *func()
	for _, value := range []any{
		nilFunc,
		[]func(){},
		[]chan int(nil),
		map[string]func(){},
		struct {
			F func() `json:"-"`
		}{},
		struct {
			f func()
		}{},
		struct {
			F func() `json:"f,omitzero"`
		}{},
		struct {
			F *func() `json:"f,omitempty"`
		}{},
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) { compareEncoding(t, value) })
	}
}

func TestEncoderWritesNothingOnError(t *testing.T) {
	for _, value := range []any{
		[]any{1, func() {}},
		[]float64{1, math.NaN()},
		map[string]any{"a": make(chan int)},
		struct {
			A int
			B func()
		}{A: 1},
		[]any{"prefix", failingMarshaler{}},
		[]any{"prefix", json.RawMessage(`{`)},
	} {
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(value); err == nil {
			t.Fatalf("%#v: no error", value)
		}
		if buf.Len() != 0 {
			t.Fatalf("%#v: wrote %q", value, buf.Bytes())
		}
	}
}

func TestEncoderOneWritePerValue(t *testing.T) {
	var writes countingWriter
	if err := jrpc.NewEncoder(&writes).Encode(map[string]any{"a": []int{1, 2}, "b": []byte("hello"), "c": "text"}); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("got %d writes", writes)
	}
}

type countingWriter int

func (w *countingWriter) Write(p []byte) (int, error) {
	*w++
	return len(p), nil
}

type cyclicNode struct {
	Next *cyclicNode
}

type cyclicValue struct {
	Next any
}

func TestEncoderRejectsCycles(t *testing.T) {
	lowerStackLimit(t)
	self := &cyclicNode{}
	self.Next = self
	two := &cyclicValue{Next: &cyclicValue{}}
	two.Next.(*cyclicValue).Next = two
	loop := map[string]any{}
	loop["self"] = loop
	list := []any{nil}
	list[0] = list
	var pointer any
	pointer = &pointer
	var double *any
	var again any = &double
	double = &again
	for name, value := range map[string]any{
		"pointer":   self,
		"interface": two,
		"map":       loop,
		"slice":     list,
		"indirect":  pointer,
		"mutual":    double,
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := jrpc.NewEncoder(&buf).Encode(value)
			var unsupported *json.UnsupportedValueError
			if !errors.As(err, &unsupported) {
				t.Fatalf("got error %v, want an unsupported value", err)
			}
			if buf.Len() != 0 {
				t.Fatalf("wrote %d bytes", buf.Len())
			}
		})
	}
}

func TestEncoderNestingLimitIsTheOneReadersAccept(t *testing.T) {
	nested := func(depth int) any {
		var value []any
		for range depth {
			value = []any{value}
		}
		return value
	}
	var buf bytes.Buffer
	if err := jrpc.NewEncoder(&buf).Encode(nested(10000)); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatal("encoded more nesting than a reader accepts")
	}
	buf.Reset()
	if err := jrpc.NewEncoder(&buf).Encode(nested(10001)); err == nil {
		t.Fatal("no error beyond the limit")
	}
}

type valueMarshaler struct{ Field int }

func (m valueMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`{ "marshaled" : ` + strconv.Itoa(m.Field) + ` }`), nil
}

type pointerMarshaler struct{ Field int }

func (m *pointerMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"marshaled ` + strconv.Itoa(m.Field) + `"`), nil
}

type failingMarshaler struct{}

func (failingMarshaler) MarshalJSON() ([]byte, error) { return nil, errRejected }

type invalidMarshaler struct{}

func (invalidMarshaler) MarshalJSON() ([]byte, error) { return []byte(`{`), nil }

type emptyMarshaler struct{}

func (emptyMarshaler) MarshalJSON() ([]byte, error) { return nil, nil }

type bothMarshalers struct{}

func (bothMarshalers) MarshalJSON() ([]byte, error) { return []byte(`"json"`), nil }

func (bothMarshalers) MarshalText() ([]byte, error) { return []byte("text"), nil }

type pointerJSONValueText struct{}

func (*pointerJSONValueText) MarshalJSON() ([]byte, error) { return []byte(`"json"`), nil }

func (pointerJSONValueText) MarshalText() ([]byte, error) { return []byte("text"), nil }

type marshaledScalar int

func (m marshaledScalar) MarshalJSON() ([]byte, error) { return []byte(`"scalar"`), nil }

type holdsPointerMarshaler struct {
	Value pointerMarshaler
	Text  textPointer
}

func TestEncoderMarshalers(t *testing.T) {
	var nilValue *valueMarshaler
	var nilPointer *pointerMarshaler
	value := valueMarshaler{1}
	pointer := pointerMarshaler{2}
	text := textPointer{"text"}
	raw := json.RawMessage(`{"a" : [1, 2]}`)
	var nilMarshaler json.Marshaler = nilValue
	for _, value := range []any{
		value, &value, nilValue, []valueMarshaler{value}, map[string]valueMarshaler{"k": value},
		pointer, &pointer, nilPointer, []pointerMarshaler{pointer}, [1]pointerMarshaler{pointer}, &[1]pointerMarshaler{pointer},
		map[string]pointerMarshaler{"k": pointer}, map[string]*pointerMarshaler{"k": &pointer}, map[string]holdsPointerMarshaler{"k": {}},
		[]holdsPointerMarshaler{{}},
		holdsPointerMarshaler{}, &holdsPointerMarshaler{}, []any{holdsPointerMarshaler{}}, []any{&holdsPointerMarshaler{}},
		textValue{"text"}, &textValue{"text"}, textPointer{"text"}, &text, []textPointer{text}, []any{textValue{"a\"b"}},
		bothMarshalers{}, pointerJSONValueText{}, &pointerJSONValueText{},
		failingMarshaler{}, textFailure{}, invalidMarshaler{}, emptyMarshaler{},
		raw, &raw, json.RawMessage(nil), json.RawMessage{}, json.RawMessage(`nonsense`), json.RawMessage(`1 2`),
		[]json.RawMessage{raw, nil},
		struct {
			Raw json.RawMessage `json:"raw"`
		}{},
		struct {
			Raw json.RawMessage `json:"raw"`
		}{json.RawMessage{}},
		struct {
			Raw json.RawMessage `json:"raw,omitempty"`
		}{json.RawMessage{}},
		nilMarshaler,
		struct{ M json.Marshaler }{},
		struct{ M json.Marshaler }{value},
		struct{ T encoding.TextMarshaler }{},
		struct{ T encoding.TextMarshaler }{textValue{"t"}},
		time.Date(2024, 2, 3, 4, 5, 6, 7, time.UTC),
		struct{ At time.Time }{time.Unix(0, 0).UTC()},
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		big.NewInt(12),
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) { compareEncoding(t, value) })
	}
}

// encoding/json panics on this one, so it is checked on its own.
type hiddenMarshaler struct{ A int }

func (h hiddenMarshaler) IsZero() bool { return h.A == 0 }

func (h hiddenMarshaler) MarshalText() ([]byte, error) { return []byte("text"), nil }

// Two embedded marshalers cancel each other's promotion, so the methods stay on
// the unexported fields, whose values reflect refuses to hand out.
func TestEncoderTaggedEmbeddedUnexportedFields(t *testing.T) {
	compareEncoding(t, struct {
		hiddenTagged `json:"x"`
	}{hiddenTagged{1}})
	compareEncoding(t, struct {
		*hiddenTagged `json:"x"`
	}{&hiddenTagged{1}})
	compareEncoding(t, struct {
		*hiddenTagged `json:"x"`
	}{})
	for _, value := range []any{
		struct {
			hiddenMarshaler `json:"x"`
			textValue       `json:"y"`
		}{hiddenMarshaler{1}, textValue{"a"}},
		struct {
			hiddenMarshaler `json:"x,omitzero"`
			textValue       `json:"y"`
		}{hiddenMarshaler{1}, textValue{"a"}},
		struct {
			hiddenMarshaler `json:"x,omitzero"`
			textValue       `json:"y"`
		}{},
	} {
		if err := jrpc.NewEncoder(io.Discard).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEncoderInterfaceAroundNilPointer(t *testing.T) {
	var nilValue *valueMarshaler
	var nilText *textValue
	var nilZero *zeroMethod
	for _, tc := range []struct {
		value any
		want  string
	}{
		{struct{ M json.Marshaler }{nilValue}, `{"M":null}`},
		{struct{ T encoding.TextMarshaler }{nilText}, `{"T":null}`},
		{[]json.Marshaler{nilValue}, `[null]`},
		{struct {
			Z zeroInterface `json:"z,omitzero"`
		}{nilZero}, `{}`},
	} {
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(tc.value); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tc.want {
			t.Fatalf("got %s, want %s", buf.Bytes(), tc.want)
		}
	}
}

func TestEncoderMarshalerErrorIsWrapped(t *testing.T) {
	err := jrpc.NewEncoder(io.Discard).Encode([]any{failingMarshaler{}})
	var wrapped *json.MarshalerError
	if !errors.As(err, &wrapped) || !errors.Is(err, errRejected) {
		t.Fatalf("got error %v", err)
	}
	err = jrpc.NewEncoder(io.Discard).Encode(map[textFailure]int{{}: 1})
	if !errors.Is(err, errRejected) {
		t.Fatalf("got error %v", err)
	}
}

func TestEncoderFloats(t *testing.T) {
	for _, value := range []any{
		0.0, math.Copysign(0, -1), 1.0, -1.5, 0.1, 1e6, 1e20, 1e21, 1.5e21, 1e-6, 9.9e-7, 1e-7, 1e-10, 1e100, 1e-100,
		math.MaxFloat64, math.SmallestNonzeroFloat64, 123456789012345678.0,
		float32(0.1), float32(0.3), float32(1e20), float32(1e21), float32(1e-6), float32(1e-7), float32(16777216),
		float32(math.MaxFloat32), float32(math.SmallestNonzeroFloat32), []float32{0.1, 0.2},
		struct {
			F float32 `json:"f"`
			G float32 `json:"g,string"`
			H float64 `json:"h,string"`
		}{0.1, 0.1, 1e21},
	} {
		compareEncoding(t, value)
	}
}

func TestEncoderIntegers(t *testing.T) {
	for _, value := range []any{
		math.MinInt64, int8(-128), int16(-32768), int32(math.MinInt32), int64(math.MaxInt64),
		uint8(255), uint16(65535), uint32(math.MaxUint32), uint64(math.MaxUint64), uint(math.MaxUint64),
		uintptr(0), uintptr(math.MaxUint64), []uintptr{1},
		struct {
			P uintptr `json:"p,omitempty"`
		}{},
		struct {
			P uintptr `json:"p,string"`
		}{1},
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) { compareEncoding(t, value) })
	}
}

func TestEncoderNumber(t *testing.T) {
	for _, value := range []any{
		json.Number("12"), json.Number("-1.5e3"), json.Number(""), json.Number("x"), json.Number("01"),
		json.Number(" 1"), json.Number("1 "), json.Number("true"), json.Number(`"1"`),
		[]json.Number{"1"},
		struct {
			N json.Number `json:"n,string"`
		}{"1"},
		struct {
			N json.Number `json:"n,string"`
		}{},
		struct {
			N json.Number `json:"n,string"`
		}{"x"},
	} {
		compareEncoding(t, value)
	}
}

func TestEncoderQuotedFields(t *testing.T) {
	type name string
	type quoted struct {
		Int     int             `json:"int,string"`
		Int8    int8            `json:"int8,string"`
		Uint    uint            `json:"uint,string"`
		Float   float64         `json:"float,string"`
		Bool    bool            `json:"bool,string"`
		Text    string          `json:"text,string"`
		Name    name            `json:"name,string"`
		Pointer *int            `json:"pointer,string"`
		Double  **int           `json:"double,string"`
		Words   []int           `json:"words,string"`
		Map     map[string]int  `json:"map,string"`
		Inner   struct{ A int } `json:"inner,string"`
		Any     any             `json:"any,string"`
		Custom  marshaledScalar `json:"custom,string"`
		Empty   int             `json:"empty,string,omitempty"`
	}
	one := 1
	two := &one
	for _, value := range []any{
		quoted{},
		quoted{
			Int: -1, Int8: 2, Uint: 3, Float: 1.5, Bool: true, Text: "a\"b\n", Name: "n", Pointer: &one, Double: &two,
			Words: []int{1}, Map: map[string]int{"a": 1}, Any: 1, Custom: 4,
		},
		&quoted{Text: ""},
	} {
		compareEncoding(t, value)
	}
}

func TestEncoderStructFieldsMatchEncodingJSON(t *testing.T) {
	type tagged struct {
		Dash       string `json:"-,"`
		Punct      string `json:"a.b-c"`
		Blank      string `json:" "`
		Unicode    string `json:"日本語"`
		Options    string `json:",omitempty"`
		Unknown    string `json:"known,nonsense"`
		Other      int    `xml:"other"`
		Second     int    `json:"second" xml:"x"`
		lowerField int
		unused     string `json:"unused"`
	}
	type omitted struct {
		Bool    bool           `json:"bool,omitempty"`
		Int     int            `json:"int,omitempty"`
		Uintptr uintptr        `json:"uintptr,omitempty"`
		Float   float64        `json:"float,omitempty"`
		Text    string         `json:"text,omitempty"`
		Slice   []int          `json:"slice,omitempty"`
		Map     map[string]int `json:"map,omitempty"`
		Pointer *int           `json:"pointer,omitempty"`
		Any     any            `json:"any,omitempty"`
		Struct  struct{}       `json:"struct,omitempty"`
		Array   [1]int         `json:"array,omitempty"`
		Both    int            `json:"both,omitempty,omitzero"`
		Time    time.Time      `json:"time,omitempty"`
	}
	type zeroed struct {
		Time       time.Time       `json:"time,omitzero"`
		Pointer    *time.Time      `json:"pointer,omitzero"`
		Value      zeroMethod      `json:"value,omitzero"`
		Via        zeroPointer     `json:"via,omitzero"`
		Interface  zeroInterface   `json:"interface,omitzero"`
		Slice      []int           `json:"slice,omitzero"`
		EmptySlice []int           `json:"emptySlice,omitzero"`
		Struct     struct{ A int } `json:"struct,omitzero"`
	}
	var nilZero *zeroMethod
	for _, value := range []any{
		tagged{Dash: "a", Punct: "c", Blank: "d", Unicode: "e", Options: "f", Unknown: "g", Other: 1, Second: 2, lowerField: 3, unused: "u"},
		tagged{},
		omitted{},
		omitted{Slice: []int{}, Map: map[string]int{}, Any: 0, Array: [1]int{0}},
		zeroed{},
		&zeroed{},
		zeroed{Time: time.Unix(0, 0).UTC(), EmptySlice: []int{}, Value: zeroMethod{1}, Via: zeroPointer{1}, Interface: zeroMethod{0}},
		zeroed{Time: time.Unix(1, 0).UTC(), Pointer: &time.Time{}, Value: zeroMethod{2}, Via: zeroPointer{2}, Interface: zeroMethod{2}},
		zeroed{Interface: nilZero},
		struct {
			Any any `json:"any,omitzero"`
		}{(*int)(nil)},
		struct {
			Any any `json:"any,omitempty"`
		}{(*int)(nil)},
		embeddedDocument{},
		embeddedDocument{EmbeddedExported: EmbeddedExported{Promoted: 1, Shadowed: "a"}, Shadowed: 2},
		embeddedDocument{embeddedHidden: embeddedHidden{Visible: "v", hidden: "h"}, EmbeddedPointer: &EmbeddedPointer{Deep: []int{1}}},
		struct {
			ambiguousLeft
			ambiguousRight
		}{ambiguousLeft{1, 2}, ambiguousRight{3, 4}},
		struct {
			EmbeddedScalar
			embeddedScalar
			Value int
		}{1, 2, 3},
		struct {
			EmbeddedExported `json:"Renamed"`
		}{EmbeddedExported{Promoted: 1}},
		struct {
			*embeddedHiddenPointer
			Other int
		}{&embeddedHiddenPointer{1}, 2},
		struct {
			*embeddedHiddenPointer
			Other int
		}{nil, 2},
		struct {
			Tagged  int `json:"Name"`
			Name    int
			Another int `json:"Name"`
		}{1, 2, 3},
		struct {
			EmbeddedExported
			Promoted int `json:"Promoted"`
		}{EmbeddedExported{Promoted: 1}, 2},
		struct {
			fmt.Stringer
			Text textValue
		}{},
		recursiveEmbed{Value: 1},
		recursiveEmbed{Value: 1, recursiveEmbed: &recursiveEmbed{Value: 2}},
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) { compareEncoding(t, value) })
	}
}

type recursiveEmbed struct {
	*recursiveEmbed
	Value int
}

type zeroInterface interface{ IsZero() bool }

type zeroMethod struct{ Value int }

func (z zeroMethod) IsZero() bool { return z.Value <= 1 }

type zeroPointer struct{ Value int }

func (z *zeroPointer) IsZero() bool { return z.Value <= 1 }

func TestEncoderValuesMatchEncodingJSON(t *testing.T) {
	type inner struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	for _, value := range []any{
		"", "text", "a\"b\\c\n\r\t\x00\x1f\x7f", "a\xffb", "\xed\xa0\x80", "a b c", "<&>", "日本語😀",
		[]string(nil), []string{}, []int{1}, [][]int{{1}, nil, {}}, [0]int{}, [2]string{"a", "b"},
		map[string]int(nil), map[string]int{}, map[string]any{"a": []any{1, "b", nil, 1.5, true}},
		[]inner{{1, "a"}}, &[]inner{{1, "a"}}, map[string]inner{"k": {1, "a"}}, map[string]*inner{"k": nil},
		struct{}{}, &struct{}{}, []struct{}{{}}, [1]bool{true},
		new(int), new(*int), new([]int), new(map[string]int), new(any),
	} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) { compareEncoding(t, value) })
	}
}

func encoderDifferentialTypes() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[bool](), reflect.TypeFor[int](), reflect.TypeFor[int8](), reflect.TypeFor[uint16](),
		reflect.TypeFor[uint64](), reflect.TypeFor[uintptr](), reflect.TypeFor[float32](), reflect.TypeFor[float64](),
		reflect.TypeFor[string](), reflect.TypeFor[[]byte](), reflect.TypeFor[[]string](), reflect.TypeFor[[3]int8](),
		reflect.TypeFor[[2]byte](), reflect.TypeFor[[][]float32](), reflect.TypeFor[*int](), reflect.TypeFor[**string](),
		reflect.TypeFor[map[string]int](), reflect.TypeFor[map[int]string](), reflect.TypeFor[map[uint8][]byte](),
		reflect.TypeFor[map[int64]map[int16]bool](), reflect.TypeFor[map[string]*float64](),
		reflect.TypeFor[quickDocument](), reflect.TypeFor[*quickDocument](), reflect.TypeFor[[]quickDocument](),
		reflect.TypeFor[map[string]quickDocument](),
	}
}

type quickDocument struct {
	Bool    bool    `json:"bool"`
	Int     int     `json:"int,omitempty"`
	Int8    int8    `json:"int8,string"`
	Uint    uint32  `json:"uint,omitzero"`
	Uintptr uintptr `json:"uintptr"`
	Float   float64 `json:"float"`
	Small   float32 `json:"small,string"`
	Text    string  `json:"text"`
	Quoted  string  `json:"quoted,string"`
	Bytes   []byte  `json:"bytes,omitempty"`
	List    []int16 `json:"list"`
	Array   [2]bool `json:"array"`
	Keys    map[int]string
	Pointer *int `json:"pointer,string"`
	Skip    int  `json:"-"`
	EmbeddedExported
	*EmbeddedPointer
}

func TestEncoderMatchesEncodingJSONOnRandomValues(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	for _, rtype := range encoderDifferentialTypes() {
		t.Run(rtype.String(), func(t *testing.T) {
			for range 100 {
				value, ok := quick.Value(rtype, random)
				if !ok {
					t.Fatalf("cannot generate %v", rtype)
				}
				compareEncoding(t, value.Interface())
			}
		})
	}
}

func TestRoundTripMatchesEncodingJSONOnRandomValues(t *testing.T) {
	random := rand.New(rand.NewSource(2))
	for _, rtype := range encoderDifferentialTypes() {
		t.Run(rtype.String(), func(t *testing.T) {
			for range 100 {
				value, ok := quick.Value(rtype, random)
				if !ok {
					t.Fatalf("cannot generate %v", rtype)
				}
				var buf bytes.Buffer
				if err := jrpc.NewEncoder(&buf).Encode(value.Interface()); err != nil {
					t.Fatal(err)
				}
				text := append([]byte(nil), buf.Bytes()...)
				got := reflect.New(rtype)
				dec := jrpc.NewDecoder(&buf)
				if err := dec.Decode(got.Interface()); err != nil {
					t.Fatalf("%s: %v", text, err)
				}
				want := reflect.New(rtype)
				if err := json.Unmarshal(text, want.Interface()); err != nil {
					t.Fatalf("%s: %v", text, err)
				}
				if !reflect.DeepEqual(got.Interface(), want.Interface()) {
					t.Fatalf("%s: got %#v, want %#v", text, got.Elem(), want.Elem())
				}
			}
		})
	}
}

func FuzzEncoderMatchesEncodingJSON(f *testing.F) {
	for _, seed := range []string{
		`null`, `1`, `1.5e300`, `1e-7`, `"text"`, "\"a\xffb\"", `"aGk="`, `[1,2]`, `{"a":1}`, `{"1":"a"}`, `{"-1":"a"}`,
		`{"Bool":true,"Int":1,"Float32":0.1,"Float64":1e21,"String":"<x>","Bytes":"aGk=","Array":[1,2],"Slice":[],"Map":{}}`,
		`{"IntMap":{"1":"a"},"UintMap":{"1":true},"TextMap":{"k":1},"Pointer":1,"Double":"s","Any":{"a":[1,null]}}`,
		`{"Anys":[1,"a"],"AnyMap":{"k":null},"Number":12.5,"Raw":{"a": 1},"Time":"2024-02-03T04:05:06Z","Custom":"x"}`,
		`{"Inner":{"x":1,"y":"s"},"InnerPtr":{"x":2},"Inners":[{"x":3}],"quoted":"1","qtext":"\"s\"","qpointer":"2"}`,
		`{"-":1,"omit":2,"zero":3,"Shadow":4,"Promoted":5,"reached":["a"]}`,
		`{"Deep":[1],"Promoted":1,"Visible":"v","Shadowed":1}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, target := range differentialTargets() {
			value := target()
			if err := json.Unmarshal(input, value); err != nil {
				continue
			}
			compareEncoding(t, value)
			compareEncoding(t, reflect.ValueOf(value).Elem().Interface())
		}
	})
}

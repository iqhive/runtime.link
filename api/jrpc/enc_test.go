package jrpc_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"strings"
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

type handWrittenLabel struct{ text string }

// MarshalJSON writes its text as a JSON string by hand, as many types do.
func (l handWrittenLabel) MarshalJSON() ([]byte, error) { return []byte(`"` + l.text + `"`), nil }

type handWrittenPointer struct{ text string }

func (l *handWrittenPointer) MarshalJSON() ([]byte, error) { return []byte(`"` + l.text + `"`), nil }

// The JSON that a MarshalJSON method or a [json.RawMessage] hands over is
// escaped like any other string, as the standard package escapes it by
// default. It used to be written as it was, so that text a type wrote for
// itself reached a browser with its '<', '>' and '&' in it.
func TestEncoderEscapesHTMLFromMarshalers(t *testing.T) {
	type page struct {
		Title string          `json:"title"`
		Body  json.RawMessage `json:"body"`
		Label handWrittenLabel
	}
	var (
		pointer    = handWrittenPointer{"<p>"}
		separators = string([]rune{0x2028, 0x2029})
		markup     = "<>&" + separators
	)
	for _, value := range []any{
		json.RawMessage(`"</script><script>alert(1)</script>"`),
		json.RawMessage(`{ "<k>" : [ "a&b" , 1 ] }`),
		json.RawMessage(`"line` + separators + `end"`),
		handWrittenLabel{"</script>"},
		&pointer,
		page{Title: "<b>", Body: json.RawMessage(`{"html":"<img src=x onerror=alert(1)>&"}`), Label: handWrittenLabel{"a>b"}},
		map[string]any{"k": handWrittenLabel{"a&b"}},
		[]any{handWrittenLabel{"<"}, json.RawMessage(`">"`)},
		map[string]json.RawMessage{"k": json.RawMessage(`"&"`)},
	} {
		want, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(value); err != nil {
			t.Fatal(err)
		}
		got := buf.String()
		if got != string(want) {
			t.Errorf("%T: got %s, encoding/json writes %s", value, got, want)
		}
		if strings.ContainsAny(got, markup) {
			t.Errorf("%T: %q has a character of %q in it", value, got, markup)
		}
		// Escaping spells the same characters another way, and no more.
		var plain bytes.Buffer
		unescaped := json.NewEncoder(&plain)
		unescaped.SetEscapeHTML(false)
		if err := unescaped.Encode(value); err != nil {
			t.Fatal(err)
		}
		var back, original any
		if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
			t.Fatalf("%T: %s does not parse: %v", value, got, err)
		}
		if err := json.Unmarshal(plain.Bytes(), &original); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, original) {
			t.Errorf("%T: escaping changed the value to %v, from %v", value, back, original)
		}
	}
}

// What has nothing to escape, or is escaped already, is written as it was
// before marshalers were escaped: compacted, and otherwise untouched.
func TestEncoderLeavesOtherMarshalerOutputAsItWas(t *testing.T) {
	for raw, want := range map[string]string{
		`{"a" : [1, 2.5e3, true, null]}`: `{"a":[1,2.5e3,true,null]}`,
		`"plain \"quoted\" text"`:        `"plain \"quoted\" text"`,
		`"\u003cb"`:                      `"\u003cb"`,
		`"\\u003c"`:                      `"\\u003c"`,
		`"é"`:                            `"é"`,
	} {
		var buf bytes.Buffer
		if err := jrpc.NewEncoder(&buf).Encode(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != want {
			t.Errorf("%s: got %s, want %s", raw, got, want)
		}
	}
}

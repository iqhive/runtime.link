package rest

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"runtime.link/api"
)

func newFn(t *testing.T, typ any, args, outs []string, tag string) api.Function {
	t.Helper()
	return api.Function{
		Name: "Fn",
		Type: reflect.TypeOf(typ),
		Args: args,
		Outs: outs,
		Tags: reflect.StructTag(tag),
	}
}

func TestNamedBodyRules(t *testing.T) {
	t.Parallel()
	fn := newFn(t, func(context.Context, string, string) {}, []string{"left", "right"}, nil, `rest:"POST /pair"`)
	p := newParser(fn)
	if got := namedBodyRules(fn, p); !reflect.DeepEqual(got, []string{"left", "right"}) {
		t.Errorf("two named bodies = %q", got)
	}

	fn = newFn(t, func(context.Context, string) {}, []string{"message"}, nil, `rest:"POST /echo"`)
	p = newParser(fn)
	if got := namedBodyRules(fn, p); got != nil {
		t.Errorf("single body must stay raw, got %q", got)
	}

	fn = newFn(t, func(context.Context, string, string) {}, []string{"left", ""}, nil, `rest:"POST /pair"`)
	p = newParser(fn)
	if got := namedBodyRules(fn, p); got != nil {
		t.Errorf("one unnamed body must not default, got %q", got)
	}

	fn = newFn(t, func(context.Context, string, string, string) {}, []string{"a", "b", "c"}, nil, `rest:"POST /triple"`)
	p = newParser(fn)
	if got := namedBodyRules(fn, p); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("three bodies = %q", got)
	}

	fn = newFn(t, func(context.Context, string, string, string) {}, []string{"id", "left", "right"}, nil, `rest:"POST /x"`)
	p = newParser(fn)
	p.list[0].Location = parameterInPath
	if got := namedBodyRules(fn, p); !reflect.DeepEqual(got, []string{"left", "right"}) {
		t.Errorf("path+two bodies = %q", got)
	}

	fn = newFn(t, func(context.Context, string) {}, nil, nil, `rest:"POST /x"`)
	p = newParser(fn)
	if got := namedBodyRules(fn, p); got != nil {
		t.Errorf("unknown names = %q", got)
	}
}

func TestResultRulesFor(t *testing.T) {
	t.Parallel()
	fn := newFn(t, func() (float64, float64) { return 0, 0 }, nil, []string{"lat", "lon"}, `rest:"GET /coords"`)
	if got := resultRulesFor(fn); !reflect.DeepEqual(got, []string{"lat", "lon"}) {
		t.Errorf("named results = %q", got)
	}

	fn = newFn(t, func() (float64, float64) { return 0, 0 }, nil, []string{"lat", "lon"}, `rest:"GET /coords x,y"`)
	if got := resultRulesFor(fn); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("explicit rules win = %q", got)
	}

	fn = newFn(t, func() float64 { return 0 }, nil, []string{"lat"}, `rest:"GET /lat"`)
	if got := resultRulesFor(fn); got != nil {
		t.Errorf("single result stays raw = %q", got)
	}

	fn = newFn(t, func() (float64, float64) { return 0, 0 }, nil, []string{"lat", ""}, `rest:"GET /coords"`)
	if got := resultRulesFor(fn); got != nil {
		t.Errorf("blank result name = %q", got)
	}

	fn = newFn(t, func() (float64, float64) { return 0, 0 }, nil, nil, `rest:"GET /coords"`)
	if got := resultRulesFor(fn); got != nil {
		t.Errorf("unknown results = %q", got)
	}
}

func TestArgumentRulesForExplicitWins(t *testing.T) {
	t.Parallel()
	fn := newFn(t, func(context.Context, string, string) {}, []string{"left", "right"}, nil, `rest:"POST /pair (x,y)"`)
	p := newParser(fn)
	if got := argumentRulesFor(fn, p); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("explicit = %q", got)
	}
}

func TestPrettyNamedJSON(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`["a","b"]`)
	if got := prettyNamedJSON(nil, raw); got == "" || got[0] != '[' {
		t.Errorf("no names should stay an array: %q", got)
	}
	got := prettyNamedJSON([]string{"left", "right"}, raw)
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["left"] != "a" || obj["right"] != "b" {
		t.Errorf("labeled = %s", got)
	}
	got = prettyNamedJSON([]string{"", ""}, raw)
	if got[0] != '[' {
		t.Errorf("all-blank names should stay an array: %q", got)
	}
	got = prettyNamedJSON([]string{"only"}, json.RawMessage(`["a","b"]`))
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["only"] != "a" || obj["1"] != "b" {
		t.Errorf("short names = %s", got)
	}
	if got := prettyNamedJSON([]string{"x"}, json.RawMessage(`not-json`)); got != "not-json" {
		t.Errorf("invalid json = %q", got)
	}
	if got := prettyNamedJSON([]string{"x"}, json.RawMessage(``)); got != "" {
		t.Errorf("empty raw = %q", got)
	}
	if got := prettyNamedJSON([]string{"x"}, json.RawMessage(`[]`)); !strings.Contains(got, "{") && got != "[]" {
		// empty array has no values to label; stays an array
		if got != "[]" && got != "[\n]" {
			t.Errorf("empty array = %q", got)
		}
	}
	got = prettyNamedJSON([]string{"a", "b", "c"}, json.RawMessage(`[1,2]`))
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["c"]; ok {
		t.Errorf("extra names should not invent values: %s", got)
	}
	if obj["a"] != float64(1) || obj["b"] != float64(2) {
		t.Errorf("partial labels = %s", got)
	}
	if got := prettyNamedJSON([]string{"x"}, json.RawMessage(`{"keep":true}`)); !strings.Contains(got, "keep") {
		t.Errorf("object input should stay an object: %q", got)
	}
}

func TestNamedBodyRulesAfterQuery(t *testing.T) {
	t.Parallel()
	str := reflect.TypeFor[string]()
	fn := newFn(t, func(context.Context, string, string, string) {}, []string{"q", "left", "right"}, nil, `rest:"POST /search?q=%v"`)
	p := newParser(fn)
	if err := p.parseQuery("?q=%v", []reflect.Type{str, str, str}); err != nil {
		t.Fatal(err)
	}
	if got := namedBodyRules(fn, p); !reflect.DeepEqual(got, []string{"left", "right"}) {
		t.Errorf("query leftover bodies = %q", got)
	}
}

func TestNamedBodyRulesAllPath(t *testing.T) {
	t.Parallel()
	str := reflect.TypeFor[string]()
	fn := newFn(t, func(context.Context, string, string) {}, []string{"a", "b"}, nil, `rest:"GET /{a=%v}/{b=%v}"`)
	p := newParser(fn)
	if err := p.parsePath("/{a=%v}/{b=%v}", []reflect.Type{str, str}); err != nil {
		t.Fatal(err)
	}
	if got := namedBodyRules(fn, p); got != nil {
		t.Errorf("all-path should have no body rules, got %q", got)
	}
}

func TestResultRulesForOutsMismatch(t *testing.T) {
	t.Parallel()
	fn := newFn(t, func() (float64, float64) { return 0, 0 }, nil, []string{"lat"}, `rest:"GET /coords"`)
	if got := resultRulesFor(fn); got != nil {
		t.Errorf("short Outs = %q", got)
	}
	fn = newFn(t, func() (float64, float64) { return 0, 0 }, nil, []string{"lat", "lon", "extra"}, `rest:"GET /coords"`)
	if got := resultRulesFor(fn); got != nil {
		t.Errorf("long Outs = %q", got)
	}
}

func TestArgumentRulesForFallsBackToNames(t *testing.T) {
	t.Parallel()
	fn := newFn(t, func(context.Context, string, string) {}, []string{"left", "right"}, nil, `rest:"POST /pair"`)
	p := newParser(fn)
	if got := argumentRulesFor(fn, p); !reflect.DeepEqual(got, []string{"left", "right"}) {
		t.Errorf("fallback = %q", got)
	}
}

package api

import (
	"context"
	"reflect"
	"testing"
)

func TestArgumentScannerPercent(t *testing.T) {
	a := reflect.ValueOf("one")
	b := reflect.ValueOf("two")
	c := reflect.ValueOf("three")
	s := NewArgumentScanner([]reflect.Value{a, b, c})

	v, err := s.Scan("%v")
	if err != nil || v.String() != "one" {
		t.Fatalf("first %%v: %v %v", v, err)
	}
	v, err = s.Scan("%[2]v")
	if err != nil || v.String() != "three" {
		t.Fatalf("%%[2]v is relative to the last %%v, got %v %v", v, err)
	}

	s = NewNamedArgumentScanner([]reflect.Value{a, b, c}, []string{"first", "second", "third"})
	v, err = s.Scan("%v")
	if err != nil || v.String() != "one" {
		t.Fatalf("named %%v: %v %v", v, err)
	}
	v, err = s.Scan("%[1]v")
	if err != nil || v.String() != "two" {
		t.Fatalf("%%[1]v: %v %v", v, err)
	}
}

func TestArgumentScannerErrors(t *testing.T) {
	s := NewNamedArgumentScanner([]reflect.Value{reflect.ValueOf("x")}, []string{"name"})
	if _, err := s.Scan(""); err == nil {
		t.Error("empty format")
	}
	if _, err := s.Scan("missing"); err == nil {
		t.Error("unknown name")
	}
	if _, err := s.Scan("%[0]v"); err == nil {
		t.Error("zero index is invalid")
	}
	if _, err := s.Scan("%[99]v"); err == nil {
		t.Error("out of range index")
	}
	if _, err := s.Scan("%[nope]v"); err == nil {
		t.Error("non-numeric index")
	}
	empty := NewArgumentScanner(nil)
	if _, err := empty.Scan("%v"); err == nil {
		t.Error("percent-v with no args")
	}
}

func TestArgumentScannerJSONTag(t *testing.T) {
	type rec struct {
		Title string `json:"title"`
	}
	s := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf(rec{Title: "hi"})},
		[]string{"rec"},
	)
	v, err := s.Scan("title")
	if err != nil || v.String() != "hi" {
		t.Fatalf("json tag: %v %v", v, err)
	}
	v, err = s.Scan("rec")
	if err != nil || v.Kind() != reflect.Struct {
		t.Fatalf("param name: %v %v", v, err)
	}
}

func TestArgumentScannerEmptyNamesAreIgnored(t *testing.T) {
	s := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf("a"), reflect.ValueOf("b")},
		[]string{"", "b"},
	)
	if _, err := s.Scan(""); err == nil {
		t.Error("empty name must not match the first argument")
	}
	v, err := s.Scan("b")
	if err != nil || v.String() != "b" {
		t.Fatalf("second name: %v %v", v, err)
	}
}

func TestArgumentScannerNameIndexPastArgs(t *testing.T) {
	s := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf("only")},
		[]string{"only", "ghost"},
	)
	if _, err := s.Scan("ghost"); err == nil {
		t.Error("name past args should fail")
	}
}

func TestNewArgumentScannerDoesNotUseNames(t *testing.T) {
	s := NewArgumentScanner([]reflect.Value{reflect.ValueOf("x")})
	if _, err := s.Scan("x"); err == nil {
		t.Error("unnamed scanner should not look up by parameter name")
	}
}

func TestInNameOutNameNamed(t *testing.T) {
	fn := Function{
		Type: reflect.TypeFor[func(int, string) (float64, error)](),
		Args: []string{"n", "s"},
		Outs: []string{"f"},
	}
	if !fn.Named() || fn.InName(0) != "n" || fn.OutName(0) != "f" {
		t.Fatalf("Named=%v In=%q Out=%q", fn.Named(), fn.InName(0), fn.OutName(0))
	}
	fn.Args = []string{"n", ""}
	if fn.Named() {
		t.Error("blank entry is not Named")
	}
	fn.Args = []string{"n"}
	if fn.Named() {
		t.Error("short Args is not Named")
	}
	fn.Args = nil
	if fn.Named() {
		t.Error("nil Args with parameters is not Named")
	}
	zero := Function{Type: reflect.TypeFor[func()]()}
	if !zero.Named() {
		t.Error("zero-arg function is Named")
	}
	if zero.InName(0) != "" || zero.OutName(0) != "" {
		t.Error("out of range on empty")
	}
	unnamed := Function{Type: reflect.TypeFor[func(context.Context, int)]()}
	if unnamed.Named() {
		t.Error("nil Args with a parameter is not Named")
	}
	if unnamed.NumIn() != 1 {
		t.Errorf("NumIn=%d", unnamed.NumIn())
	}
}

func TestArgumentScannerIndexDoesNotAdvance(t *testing.T) {
	a := reflect.ValueOf("one")
	b := reflect.ValueOf("two")
	c := reflect.ValueOf("three")
	s := NewNamedArgumentScanner([]reflect.Value{a, b, c}, []string{"first", "second", "third"})
	v, err := s.Scan("%[2]v")
	if err != nil || v.String() != "two" {
		t.Fatalf("%%[2]v: %v %v", v, err)
	}
	v, err = s.Scan("%v")
	if err != nil || v.String() != "one" {
		t.Fatalf("%%v after index should still start at 0, got %v %v", v, err)
	}
	v, err = s.Scan("third")
	if err != nil || v.String() != "three" {
		t.Fatalf("name lookup after %%v: %v %v", v, err)
	}
}

func TestArgumentScannerJSONOmitempty(t *testing.T) {
	type rec struct {
		Title string `json:"title,omitempty"`
	}
	s := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf(rec{Title: "hi"})},
		[]string{"rec"},
	)
	v, err := s.Scan("title")
	if err != nil || v.String() != "hi" {
		t.Fatalf("json tag with omitempty: %v %v", v, err)
	}
}

func TestArgumentScannerTwoStructs(t *testing.T) {
	type a struct{ Name string }
	type b struct{ Name string }
	s := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf(a{Name: "first"}), reflect.ValueOf(b{Name: "second"})},
		[]string{"left", "right"},
	)
	v, err := s.Scan("Name")
	if err != nil || v.String() != "first" {
		t.Fatalf("first matching field: %v %v", v, err)
	}
	v, err = s.Scan("right")
	if err != nil || v.Kind() != reflect.Struct {
		t.Fatalf("param name: %v %v", v, err)
	}
}

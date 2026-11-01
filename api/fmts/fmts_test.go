package fmts_test

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/fmts"
)

func TestFormats(t *testing.T) {
	var FMT = api.Import[struct {
		api.Specification

		Parser func(string) any

		Default   func(string) string                         `fmts:"%v"`
		ColonPair func(string, string) string                 `fmts:"%v:%v"`
		SlashQuad func(string, string, string, string) string `fmts:"%v/%v/%v/%v"`

		ParseDefault   func(string) (string, error)                         `fmts:"%v"`
		ParseColonPair func(string) (string, string, error)                 `fmts:"%v:%v"`
		ParseSlashQuad func(string) (string, string, string, string, error) `fmts:"%v/%v/%v/%v"`
	}](fmts.API, fmt.Sprintf, func(value, format string, into ...any) (int, error) {
		pattern := regexp.MustCompile(strings.ReplaceAll(regexp.QuoteMeta(format), "%v", "(.*)"))
		matches := pattern.FindStringSubmatch(value)
		if len(matches) == 0 {
			return 0, fmt.Errorf("no match")
		}
		var done int
		for i, match := range matches[1:] {
			c, err := fmt.Sscan(match, into[i])
			if err != nil {
				return done, err
			}
			done += c
		}
		return done, nil
	})

	if FMT.ColonPair("a", "b") != "a:b" {
		t.Error("FMT.ColonPair failed")
	}
	if FMT.SlashQuad("a", "b", "c", "d") != "a/b/c/d" {
		t.Error("FMT.SlashQuad failed")
	}

	switch parser := FMT.Parser("a:b").(type) {
	case func() (string, string, error):
		a, b, err := parser()
		if err != nil {
			t.Error("FMT.Parse failed", err)
		}
		if a != "a" || b != "b" {
			t.Error("FMT.Parse failed", a, b)
		}
	default:
		t.Error("FMT.Parse failed", reflect.TypeOf(parser))
	}

	switch parser := FMT.Parser("a/b/c/d").(type) {
	case func() (string, string, string, string, error):
		a, b, c, d, err := parser()
		if err != nil {
			t.Error("FMT.Parse failed", err)
		}
		if a != "a" || b != "b" || c != "c" || d != "d" {
			t.Error("FMT.Parse failed", a, b, c, d)
		}
	default:
		t.Error("FMT.Parse failed", reflect.TypeOf(parser))
	}
}

func scan(value, format string, into ...any) (int, error) {
	pattern := regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(format), "%v", "(.*)") + "$")
	matches := pattern.FindStringSubmatch(value)
	if len(matches) == 0 {
		return 0, errors.New("no match")
	}
	var done int
	for i, match := range matches[1:] {
		c, err := fmt.Sscan(match, into[i])
		if err != nil {
			return done, err
		}
		done += c
	}
	return done, nil
}

func TestParserRejectsValueThatMatchesNoFormat(t *testing.T) {
	F := api.Import[struct {
		api.Specification
		Parser func(string) any
		Colon  func(string) (string, string, error) `fmts:"%v:%v"`
	}](fmts.API, fmt.Sprintf, scan)

	if got := F.Parser("no-separator-here"); got != nil {
		t.Errorf("selected a parser: %T", got)
	}
	if _, ok := F.Parser("a:b").(func() (string, string, error)); !ok {
		t.Errorf("a:b selected no parser")
	}
}

func TestParserFallsBackToCatchAll(t *testing.T) {
	F := api.Import[struct {
		api.Specification
		Parser  func(string) any
		Default func(string) (string, error)         `fmts:"%v"`
		Colon   func(string) (string, string, error) `fmts:"%v:%v"`
	}](fmts.API, fmt.Sprintf, scan)

	parser, ok := F.Parser("plain").(func() (string, error))
	if !ok {
		t.Fatal("the catch-all was not selected")
	}
	if got, err := parser(); got != "plain" || err != nil {
		t.Errorf("got %q, %v", got, err)
	}
	if _, ok := F.Parser("a:b").(func() (string, string, error)); !ok {
		t.Error("a catch-all must lose to a format that matches more of the value")
	}
}

func TestParserAmbiguousCatchAlls(t *testing.T) {
	F := api.Import[struct {
		api.Specification
		Parser func(string) any
		First  func(string) (string, error) `fmts:"%v"`
		Second func(string) (int, error)    `fmts:"%v"`
	}](fmts.API, fmt.Sprintf, scan)

	if got := F.Parser("1"); got != nil {
		t.Errorf("selected one of two equal parsers: %T", got)
	}
}

func TestParserInNamespace(t *testing.T) {
	var F struct {
		api.Specification
		Sub struct {
			Parser func(string) any
			Colon  func(string) (string, string, error) `fmts:"%v:%v"`
			Format func(string, string) string          `fmts:"%v:%v"`
		}
	}
	if err := fmts.API.Link(api.StructureOf(&F), fmt.Sprintf, scan); err != nil {
		t.Fatal(err)
	}
	if _, ok := F.Sub.Parser("a:b").(func() (string, string, error)); !ok {
		t.Error("a parser in a namespace found no candidates")
	}
	if got := F.Sub.Format("a", "b"); got != "a:b" {
		t.Errorf("got %q", got)
	}
}

func TestLinkReportsErrorInNamespace(t *testing.T) {
	var Nested struct {
		api.Specification
		Sub struct {
			Deeper struct {
				Bad func(string) int `fmts:"%v"`
			}
		}
	}
	if err := fmts.API.Link(api.StructureOf(&Nested), fmt.Sprintf, scan); err == nil {
		t.Error("the function was left nil with no error")
	}
}

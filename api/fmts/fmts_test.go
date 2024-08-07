package fmts_test

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/fmts"
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

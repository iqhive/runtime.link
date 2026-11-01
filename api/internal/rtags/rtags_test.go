package rtags_test

import (
	"testing"

	"github.com/iqhive/runtime.link/api/internal/rtags"
)

func TestRegex(t *testing.T) {
	const example = "POST /path/to/endpoint/{ID}"
	if rtags.InStructRegex.FindStringSubmatch(example)[1] != "ID" {
		t.Error("InBodyRegex failed")
	}
	if len(rtags.InPathRegex.FindStringSubmatch(example)) > 0 {
		t.Error("InPathRegex failed")
	}
}

// TestRulesTolerateOddTags reads tags that used to index past their own text.
func TestRulesTolerateOddTags(t *testing.T) {
	for _, tag := range []string{"GET /x  y", "GET /x ", "POST /x (", "POST /x ( ", "POST /x () y"} {
		rtags.ResultRulesOf(tag)
		rtags.ArgumentRulesOf(tag)
	}
	if got := rtags.ArgumentRulesOf("POST /x (a,b) c"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("argument rules: %q", got)
	}
	if got := rtags.ResultRulesOf("POST /x (a,b) c"); len(got) != 1 || got[0] != "c" {
		t.Errorf("result rules: %q", got)
	}
}

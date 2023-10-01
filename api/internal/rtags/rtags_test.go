package rtags_test

import (
	"testing"

	"runtime.link/api/internal/rtags"
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

package call

import (
	"testing"

	"runtime.link/api/call/internal/ffi"
)

func TestGoArgName(t *testing.T) {
	t.Parallel()
	names := []string{"buf", "n"}
	if got := goArgName(names, ffi.Type{Maps: 2}, 0); got != "n" {
		t.Errorf("Maps=2 → %q", got)
	}
	if got := goArgName(names, ffi.Type{Maps: 1}, 99); got != "buf" {
		t.Errorf("Maps=1 → %q", got)
	}
	if got := goArgName(names, ffi.Type{}, 1); got != "n" {
		t.Errorf("index fallback → %q", got)
	}
	if got := goArgName(names, ffi.Type{Maps: 9}, -1); got != "" {
		t.Errorf("out of range → %q", got)
	}
	if got := goArgName(nil, ffi.Type{Maps: 1}, 0); got != "" {
		t.Errorf("nil names → %q", got)
	}
	if got := goArgName([]string{""}, ffi.Type{Maps: 1}, 0); got != "" {
		t.Errorf("blank name → %q", got)
	}
	if got := goArgName([]string{"only"}, ffi.Type{}, -1); got != "" {
		t.Errorf("negative index → %q", got)
	}
	if got := goArgName([]string{"a", "b"}, ffi.Type{Maps: 0}, 0); got != "a" {
		t.Errorf("Maps=0 index fallback → %q", got)
	}
}

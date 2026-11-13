//go:build cgo

package call

import (
	"strings"
	"testing"
	"unsafe"
)

// wantFailure checks that err is the failure C reported, with the given text.
func wantFailure(t *testing.T, name string, err error, text string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: the Go error is nil", name)
		return
	}
	if !strings.Contains(err.Error(), text) {
		t.Errorf("%s: error %q, want it to say %q", name, err, text)
	}
}

func TestErrorResultFollowsAComparisonOnTheResult(t *testing.T) {
	positive := makeFn[func(int32, int32) error](t, helper(t, "first_of"), "func(int,int)int>0")
	if err := positive(5, 0); err != nil {
		t.Errorf("5 meets >0, but got %v", err)
	}
	if err := positive(0, 0); err == nil {
		t.Errorf("0 fails >0, but the error is nil")
	}
	isNonZero := makeFn[func(int32) error](t, helper(t, "is_nonzero"), "func(int)bool=0")
	if err := isNonZero(0); err != nil {
		t.Errorf("false meets =0, but got %v", err)
	}
	if err := isNonZero(1); err == nil {
		t.Errorf("true fails =0, but the error is nil")
	}
	null := makeFn[func(unsafe.Pointer) error](t, helper(t, "is_null"), "func(&void)int=1")
	if err := null(nil); err != nil {
		t.Errorf("1 meets =1, but got %v", err)
	}
}

func TestResultWithoutAnErrorKeepsItsValue(t *testing.T) {
	first := makeFn[func(int32, int32) int32](t, helper(t, "first_of"), "func(int,int)int")
	if got := first(5, 0); got != 5 {
		t.Errorf("first_of(5, 0) = %d", got)
	}
	discarded := makeFn[func(int32, int32)](t, helper(t, "first_of"), "func(int,int)int")
	if r := recovered(func() { discarded(5, 0) }); r != nil {
		t.Errorf("a discarded result panicked: %v", r)
	}
	null := makeFn[func() string](t, helper(t, "null_str"), "func()&char")
	if got := null(); got != "" {
		t.Errorf("null_str() = %q", got)
	}
}

func TestArgumentAssertionsStillComeFirst(t *testing.T) {
	memset := makeFn[func([]byte, int32, uint64) error](t, libc(t, "memset"), "func(&void[=@3],int,size_t)&void")
	buf := make([]byte, 4)
	wantFailure(t, "memset past the end", memset(buf, 0x41, 12), "capacity")
	if buf[0] != 0 {
		t.Errorf("C was called: %q", buf)
	}
	if err := memset(buf, 0x41, 4); err != nil {
		t.Errorf("memset returned its non-NULL destination, but got %v", err)
	}
}

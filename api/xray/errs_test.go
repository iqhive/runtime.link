package xray_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api/xray"
)

func TestErrors(t *testing.T) {
	var err = xray.New(errors.New("hello world"))
	fmt.Println(err, err.Error())
}

func TestErrorVerbsKeepFilePathsOutOfMessages(t *testing.T) {
	inner := xray.New(errors.New("boom"))
	tests := []struct{ format, want string }{
		{"%s", "boom"},
		{"%v", "boom"},
		{"%q", `"boom"`},
		{"%10s", "      boom"},
	}
	for _, tt := range tests {
		if got := fmt.Sprintf(tt.format, inner); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.format, got, tt.want)
		}
	}
	if got := fmt.Errorf("saving user: %w", inner).Error(); got != "saving user: boom" {
		t.Errorf("%%w: got %q", got)
	}
	if got := fmt.Sprint(xray.New(inner)); got != "boom" {
		t.Errorf("nested: got %q", got)
	}
	if !errors.Is(fmt.Errorf("saving user: %w", inner), inner) {
		t.Error("%w no longer wraps")
	}
}

// %+v is how a trace is asked for, in the format it has always had.
func TestErrorTraceForPlusV(t *testing.T) {
	inner := xray.New(errors.New("boom"))
	wrapped := xray.New(fmt.Errorf("saving user: %w", inner))

	got := fmt.Sprintf("%+v", wrapped)
	if !strings.HasPrefix(got, "Error: saving user: boom\n\n") {
		t.Errorf("no heading in %q", got)
	}
	if strings.Count(got, "errs_test.go:") != 2 {
		t.Errorf("want the file and line of both errors: %q", got)
	}
	if !strings.Contains(got, "TestErrorTraceForPlusV()\n\t") {
		t.Errorf("no function in %q", got)
	}
}

package xray_test

import (
	"errors"
	"fmt"
	"io"
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

type problems []string

func (p problems) Error() string { return strings.Join(p, ", ") }

type problemTable map[string]string

func (p problemTable) Error() string { return fmt.Sprint(map[string]string(p)) }

// because holds an error in a struct, which makes it a type that can be
// compared whatever the error inside is.
type because struct{ cause error }

func (b because) Error() string { return "because " + b.cause.Error() }

// tracedProblem is not inlined, so that every error it returns has one trace.
//
//go:noinline
func tracedProblem(err error) error { return xray.New(err) }

// errors.Is compares with == when the type of its target can be compared,
// which what New returned always claimed, being a struct. Two of them that
// held errors of one slice or map type then panicked, in errors.Is and in ==,
// where the same errors without a trace are simply not equal.
func TestErrorsThatCannotBeComparedDoNotPanic(t *testing.T) {
	for name, pair := range map[string][2]error{
		"slices":            {problems{"a"}, problems{"b"}},
		"equal slices":      {problems{"a"}, problems{"a"}},
		"maps":              {problemTable{"a": "b"}, problemTable{"a": "b"}},
		"a slice in struct": {because{problems{"a"}}, because{problems{"a"}}},
		"through Error":     {xray.Error(problems{"a"}, 0), xray.Error(problems{"a"}, 0)},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic: %v", r)
				}
			}()
			first, second := tracedProblem(pair[0]), tracedProblem(pair[1])
			if errors.Is(first, second) {
				t.Error("errors.Is matches two errors that do not match without a trace")
			}
			if first == second {
				t.Error("== holds for two errors that cannot be compared")
			}
			if same := first; !errors.Is(first, same) || first != same {
				t.Error("an error is not itself")
			}
			if errors.Is(first, io.EOF) || errors.Is(io.EOF, first) || errors.Is(fmt.Errorf("wrapped: %w", first), second) {
				t.Error("matches an unrelated error")
			}
			if !errors.Is(fmt.Errorf("wrapped: %w", first), first) {
				t.Error("not found behind %w")
			}
			if got := errors.Unwrap(first); got == nil || got.Error() != pair[0].Error() {
				t.Errorf("Unwrap: %v", got)
			}
			if got := fmt.Sprint(first); got != pair[0].Error() {
				t.Errorf("%%v: %q", got)
			}
			if got := fmt.Sprintf("%+v", first); !strings.Contains(got, "errs_test.go:") || !strings.Contains(got, "tracedProblem()") {
				t.Errorf("%%+v has no trace: %q", got)
			}
		})
	}
	if errors.Is(problems{"a"}, problems{"a"}) {
		t.Error("two slices match without a trace")
	}
	var list problems
	if err := xray.New(problems{"a", "b"}); !errors.As(err, &list) || len(list) != 2 {
		t.Errorf("errors.As found %v", list)
	}
}

// An error that can be compared is held as it always was: two with the same
// trace and the same error inside are equal, as a caller that compares what a
// function returned twice, or keeps one as a sentinel, relies on.
func TestErrorsThatCanBeComparedStillAre(t *testing.T) {
	var (
		sentinel = errors.New("not found")
		other    = errors.New("not found")
		first    = tracedProblem(sentinel)
		second   = tracedProblem(sentinel)
	)
	if first != second || !errors.Is(first, second) {
		t.Error("two errors with one trace and one error inside are not equal")
	}
	if !errors.Is(first, sentinel) || errors.Is(first, other) {
		t.Error("the error inside is not what errors.Is finds")
	}
	if elsewhere := xray.New(sentinel); first == elsewhere || !errors.Is(elsewhere, sentinel) {
		t.Error("a trace from another line compares equal, or lost its error")
	}
	if first == tracedProblem(other) {
		t.Error("equal with another error inside")
	}
	if pointer := tracedProblem(&because{sentinel}); pointer == tracedProblem(&because{sentinel}) {
		t.Error("two pointers compare equal")
	}
	if value := tracedProblem(because{sentinel}); value != tracedProblem(because{sentinel}) {
		t.Error("a struct that can be compared is no longer equal to itself")
	}
}

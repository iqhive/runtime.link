package api

import (
	"context"
	"iter"
	"testing"

	"runtime.link/api/test"
)

// envSuite is a minimal test suite that records which environment instance ran
// so the Environments dispatch can be asserted.
type envSuite struct {
	TestingFramework

	name string
	ran  *string
}

// TestRecord is a runnable test method; running it records the suite's name.
func (s *envSuite) TestRecord(ctx context.Context) error {
	*s.ran = s.name
	return nil
}

func TestEnvironmentsDispatch(t *testing.T) {
	var ran string
	history := stubHistory{}
	envs := Environments{
		"":   &envSuite{name: "default", ran: &ran, TestingFramework: TestingFramework{History: history}},
		"st": &envSuite{name: "st", ran: &ran, TestingFramework: TestingFramework{History: history}},
		"dev": &envSuite{name: "dev", ran: &ran, TestingFramework: TestingFramework{History: history}},
	}
	doc := Documentation(func(ctx context.Context) (Examples, error) { return envs, nil })

	// A bare name selects the "" environment (backwards compatible).
	ran = ""
	if _, ok := doc.Test(t.Context(), "TestRecord"); !ok {
		t.Fatal("bare TestRecord did not run")
	}
	if ran != "default" {
		t.Fatalf("bare name ran %q, want default environment", ran)
	}

	// An env-prefixed name selects that environment's suite.
	ran = ""
	if _, ok := doc.Test(t.Context(), "st:TestRecord"); !ok {
		t.Fatal("st:TestRecord did not run")
	}
	if ran != "st" {
		t.Fatalf("st:TestRecord ran %q, want st", ran)
	}

	// An unknown environment is not found rather than silently defaulting.
	if _, ok := doc.Test(t.Context(), "nope:TestRecord"); ok {
		t.Fatal("unknown environment should not run")
	}

	// Environments lists the sorted keys for the UI selector.
	got := doc.Environments(t.Context())
	want := []string{"", "dev", "st"}
	if len(got) != len(want) {
		t.Fatalf("Environments() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Environments() = %v, want %v", got, want)
		}
	}

	// Tests enumerates a single representative with unprefixed names.
	tests, err := doc.Tests(t.Context())
	if err != nil {
		t.Fatalf("Tests: %v", err)
	}
	var names []string
	for _, group := range tests {
		names = append(names, group...)
	}
	if len(names) != 1 || names[0] != "TestRecord" {
		t.Fatalf("Tests() = %v, want [TestRecord]", names)
	}

	// History is discoverable through the Environments map.
	if doc.History(t.Context()) == nil {
		t.Fatal("History() = nil, want the assigned history")
	}
}

// stubHistory is a no-op test.History used to confirm Environments surfaces an
// assigned history.
type stubHistory struct{}

func (stubHistory) Summary(context.Context) ([]test.Summary, error) { return nil, nil }
func (stubHistory) Inspect(context.Context, string) (iter.Seq[test.Execution], error) {
	return func(func(test.Execution) bool) {}, nil
}
func (stubHistory) Capture(context.Context, test.Execution) error { return nil }

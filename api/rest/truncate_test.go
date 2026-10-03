package rest

import "testing"

func TestTruncateBody(t *testing.T) {
	if got := string(truncateBody([]byte("short"), 10)); got != "short" {
		t.Fatalf("got %q", got)
	}
}

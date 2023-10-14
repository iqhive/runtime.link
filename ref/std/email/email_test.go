package email_test

import (
	"testing"

	"runtime.link/ref/std/email"
)

func TestEmailAddress(t *testing.T) {
	var addr = email.Address("user@example.com")

	if err := addr.Validate(); err != nil {
		t.Fatal("unexpected value!", err)
	}

	var quoted = email.Address(`"user spaced"@example.com`)
	if err := quoted.Validate(); err != nil {
		t.Fatal("unexpected value!", err)
	}
}

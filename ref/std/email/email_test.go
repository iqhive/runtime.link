package email_test

import (
	"testing"

	"runtime.link/ref/std/email"
)

func TestEmailAddress(t *testing.T) {
	var addr = email.Address("user@example.com")

	s, err := addr.Get()
	if err != nil || s != "user@example.com" {
		t.Fatal("unexpected value!", err)
	}

	var quoted = email.Address(`"user spaced"@example.com`)
	s, err = quoted.Get()
	if err != nil || s != `"user spaced"@example.com` {
		t.Fatal("unexpected value!", err)
	}
}

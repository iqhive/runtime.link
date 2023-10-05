package txt_test

import (
	"net/url"
	"testing"

	"runtime.link/txt"
	"runtime.link/txt/std/email"
)

func TestEmailAddress(t *testing.T) {
	var addr = txt.Is[email.Address]("user@example.com")

	s, err := addr.Get()
	if err != nil || s != "user@example.com" {
		t.Fatal("unexpected value!", err)
	}

	var quoted = txt.Is[email.Address](`"user spaced"@example.com`)
	s, err = quoted.Get()
	if err != nil || s != `"user spaced"@example.com` {
		t.Fatal("unexpected value!", err)
	}
}

func TestURL(t *testing.T) {
	var url = txt.Is[url.URL]("https://example.com")

	s, err := url.Get()
	if err != nil || s != "https://example.com" {
		t.Fatal("unexpected value!", err)
	}
}

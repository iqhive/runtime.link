package txt_test

import (
	"net/url"
	"testing"

	"runtime.link/txt"
)

func TestURL(t *testing.T) {
	var url = txt.Is[url.URL]("https://example.com")

	s, err := url.Get()
	if err != nil || s != "https://example.com" {
		t.Fatal("unexpected value!", err)
	}
}

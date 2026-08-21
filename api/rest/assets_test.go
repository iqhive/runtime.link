package rest_test

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"runtime.link/api/rest"
)

// TestEmbeddedAssets confirms the documentation assets are served from the
// binary, gzipped when the client supports it and decompressed otherwise.
func TestEmbeddedAssets(t *testing.T) {
	impl := streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			ch := make(chan string)
			close(ch)
			return ch, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/assets/marked.min.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("gzip request: got status %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", resp.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	gzipped, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}

	resp, err = http.Get(srv.URL + "/assets/marked.min.js") // transparent (decompressed) via net/http
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	plain, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("unexpected content type %q", resp.Header.Get("Content-Type"))
	}
	if string(plain) != string(gzipped) {
		t.Fatalf("gzip and identity responses differ: %d vs %d bytes", len(gzipped), len(plain))
	}
	if len(plain) == 0 || !strings.Contains(string(plain[:200]), "marked") {
		t.Fatalf("unexpected asset content: %q", string(plain[:min(200, len(plain))]))
	}

	resp, err = http.Get(srv.URL + "/assets/purify.min.js")
	if err != nil {
		t.Fatal(err)
	}
	purify, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(purify[:min(200, len(purify))]), "DOMPurify") {
		t.Fatalf("unexpected DOMPurify asset response: status %d, content %q", resp.StatusCode, string(purify[:min(200, len(purify))]))
	}

	if resp, err := http.Get(srv.URL + "/assets/nope.js"); err != nil {
		t.Fatal(err)
	} else if resp.Body.Close(); resp.StatusCode != 404 {
		t.Fatalf("missing asset: got status %d", resp.StatusCode)
	}
}

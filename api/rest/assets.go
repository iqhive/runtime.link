package rest

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// Embedded (gzipped) copies of the third-party scripts and stylesheets used
// by the documentation, examples and testruns pages, so that these pages
// work without reaching out to a CDN.
//
// Versions: DOMPurify 3.2.6, marked 18.0.9, mermaid 11.9.0,
// swagger-ui-dist 5.32.8.
//
//go:embed assets/*.gz
var assetsFS embed.FS

// docs_head references assets with page-relative "assets/..." URLs, which
// only resolve from pages one level deep (/documentation, /testruns). Pages
// two levels deep (/examples/{name}, /testruns/{name}) use this variant.
var docs_head_nested = bytes.ReplaceAll(docs_head, []byte(`"assets/`), []byte(`"../assets/`))

// assetsHandler serves the embedded assets, gzipped whenever the client
// accepts that, decompressed otherwise.
func assetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.PathValue("file"))
		data, err := assetsFS.ReadFile("assets/" + name + ".gz")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch path.Ext(name) {
		case ".js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data)
			return
		}
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		io.Copy(w, zr)
	})
}

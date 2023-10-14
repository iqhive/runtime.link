package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	_ "embed"
)

//go:embed oas.html
var html []byte

func ListenAndServe(addr string, auth Auth[*http.Request], implementation any) error {
	spec, err := DocumentationOf(StructureOf(implementation))
	if err != nil {
		panic(err)
	}
	spec.Details.Name = "Pet Store"

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(spec)

	return http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(buf.Bytes())
			return
		case "/openapi.html":
			w.Header().Set("Content-Type", "text/html")
			w.Write(html)
			return
		}
	}))
}

package rest

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/oas"
)

//go:embed form.js
var script string

type formHandler struct {
	res resource
}

func (h formHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept"), "application/schema+json") {
		var schema = new(oas.Schema)
		params := h.res.Operations[http_api.Method("POST")].Parameters
		for _, param := range params {
			if param.Location == parameterInBody {
				schema = schemaFor(schema, param.Type)
				if err := json.NewEncoder(w).Encode(schema); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		}
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html")

		docs := h.res.Operations[http_api.Method("POST")].Docs
		if docs == "" {
			docs = h.res.Operations[http_api.Method("GET")].Docs
		}

		fmt.Fprintln(w, `<html style="display: flex; justify-content: center">`)
		fmt.Fprintln(w, `<head>`)
		fmt.Fprint(w, `<title>`)
		fmt.Fprint(w, h.res.Name)
		fmt.Fprintln(w, `</title>`)
		fmt.Fprintln(w, `<link href="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/deps/opt/bootstrap.min.css" rel="stylesheet">`)
		fmt.Fprintln(w, `</head>`)
		fmt.Fprintln(w, `<body style="width: 60rem"><h1>`)
		fmt.Fprint(w, h.res.Name)
		fmt.Fprint(w, `</h1>`)
		fmt.Fprint(w, `<p>`)
		fmt.Fprint(w, docs)
		fmt.Fprint(w, `</p>`)
		fmt.Fprint(w, `<form></form><pre style="display: none;"></pre>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/deps/jquery.min.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/deps/underscore.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/deps/opt/jsv.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/lib/jsonform.js"></script>`)
		fmt.Fprintln(w, `<script>`)
		fmt.Fprintln(w, script)
		fmt.Fprintln(w, `</script>`)
		fmt.Fprintln(w, `</body>`)
		fmt.Fprintln(w, `</html>`)
		return
	}
	http.NotFound(w, r)
}

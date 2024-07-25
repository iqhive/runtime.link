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
		op := h.res.Operations[http_api.Method("POST")]
		params := op.Parameters
		if op.argumentsNeedsMapping {
			for _, param := range params {
				if param.Location == parameterInBody {
					schema.Properties[oas.PropertyName(param.Name)] = schemaFor(schema, param.Type)
				}
			}
		} else {
			for _, param := range params {
				if param.Location == parameterInBody {
					schema = schemaFor(nil, param.Type)
					break
				}
			}
		}
		if err := json.NewEncoder(w).Encode(schema); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		fmt.Fprintln(w, `<link type="text/css" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" rel="stylesheet"/>`)
		fmt.Fprintln(w, `<link type="text/css" href="//cdn.jsdelivr.net/npm/alpaca@1.5.27/dist/alpaca/bootstrap/alpaca.min.css" rel="stylesheet"/>`)
		fmt.Fprintln(w, `</head>`)
		fmt.Fprintln(w, `<body style="width: 60rem"><h1>`)
		fmt.Fprint(w, h.res.Name)
		fmt.Fprint(w, `</h1>`)
		fmt.Fprint(w, `<p>`)
		fmt.Fprint(w, docs)
		fmt.Fprint(w, `</p>`)
		fmt.Fprint(w, `<form></form><pre style="display: none;"></pre>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/jsonform@2.2.5/deps/jquery.min.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="//cdnjs.cloudflare.com/ajax/libs/handlebars.js/4.0.5/handlebars.min.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/js/bootstrap.min.js"></script>`)
		fmt.Fprintln(w, `<script type="text/javascript" src="//cdn.jsdelivr.net/npm/alpaca@1.5.27/dist/alpaca/bootstrap/alpaca.min.js"></script>`)
		fmt.Fprintln(w, `<script>`)
		fmt.Fprintln(w, script)
		fmt.Fprintln(w, `</script>`)
		fmt.Fprintln(w, `</body>`)
		fmt.Fprintln(w, `</html>`)
		return
	}
	http.NotFound(w, r)
}

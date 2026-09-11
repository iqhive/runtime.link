package rest

import (
	"net/http"
	"net/url"
	"strings"
)

type mux struct {
	routes map[string]http.Handler
	values []string
	for404 *http.Handler
}

func (m *mux) SetNotFoundHandler(h http.Handler) { (*m.for404) = h }

func (m *mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.Handle(pattern, handler)
}

func (m *mux) Handle(pattern string, handler http.Handler) {
	if m.routes == nil {
		m.routes = make(map[string]http.Handler)
	}
	if m.for404 == nil {
		m.for404 = new(http.Handler)
	}
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		path = method
		method = ""
	}
	path = strings.TrimPrefix(path, "/")
	if path != "" {
		this, _, ok := strings.Cut(path, "/")
		name := ""
		if len(this) > 2 && this[0] == '{' && this[len(this)-1] == '}' {
			name = this[1 : len(this)-1]
			this = ""
		}
		router, ok := m.routes[this].(*mux)
		if !ok {
			router = new(mux)
			router.for404 = m.for404
			m.routes[this] = router
		}
		split := strings.Split(path, "/")
		router.Handle(method+" "+strings.Join(split[1:], "/"), handler)
		if this == "" {
			router.values = append(router.values, name)
		}
		return
	}
	m.routes[method] = handler
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	this, _, ok := strings.Cut(path, "/")
	if this == "" {
		this = r.Method
		if this == "POST" {
			if override := r.Header.Get("X-HTTP-Method-Override"); override == "QUERY" {
				this = override
			}
		}
	}
	if ok {
		path = path[strings.Index(path, "/"):]
	} else {
		path = "/"
	}
	if h, ok := m.routes[this]; ok {
		h.ServeHTTP(w, withRoutedPath(r, path))
		return
	}
	if h, ok := m.routes[""]; ok {
		v, ok := h.(*mux)
		if ok {
			for _, name := range v.values {
				r.SetPathValue(name, this)
			}
		}
		h.ServeHTTP(w, withRoutedPath(r, path))
		return
	}
	// No route matched: the 404 handler sees the request unchanged, with the
	// full path still intact (we never mutated it).
	(*m.for404).ServeHTTP(w, r)
}

// withRoutedPath returns a shallow copy of r whose URL carries the routed
// (segment-trimmed) path so nested muxes and the matched handler route on the
// remaining path. The caller's request and its *url.URL are left untouched, so
// upstream middleware (Sentry, access logging, tracing) continues to observe
// the URL the client actually requested rather than a trimmed remainder.
func withRoutedPath(r *http.Request, path string) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	u := new(url.URL)
	*u = *r.URL
	u.RawPath = r.URL.Path
	u.Path = path
	r2.URL = u
	return r2
}

package rest

import (
	"net/http"
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
	}
	if ok {
		path = path[strings.Index(path, "/"):]
	} else {
		path = "/"
	}
	if h, ok := m.routes[this]; ok {
		r.URL.RawPath = r.URL.Path
		r.URL.Path = path
		h.ServeHTTP(w, r)
		return
	}
	if h, ok := m.routes[""]; ok {
		v, ok := h.(*mux)
		if ok {
			for _, name := range v.values {
				r.SetPathValue(name, this)
			}
		}
		r.URL.RawPath = r.URL.Path
		r.URL.Path = path
		h.ServeHTTP(w, r)
		return
	}
	r.URL.Path = r.URL.RawPath
	(*m.for404).ServeHTTP(w, r)
}

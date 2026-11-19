package rest

import (
	"net/http"
	"net/url"
	"strings"
)

// mux is one node of a route tree. Methods live in their own map, apart from
// the path segments below the node, so that a client can never reach a
// method's handler by spelling the method's name in the path.
type mux struct {
	methods  map[string]http.Handler
	literals map[string]*mux
	wildcard *mux
	values   []string // names of the wildcard this node is.
	for404   *http.Handler
}

func (m *mux) SetNotFoundHandler(h http.Handler) { (*m.for404) = h }

func (m *mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.Handle(pattern, handler)
}

func (m *mux) Handle(pattern string, handler http.Handler) {
	if m.for404 == nil {
		m.for404 = new(http.Handler)
	}
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		path = method
		method = ""
	}
	node := m
	if path = strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/"); path != "" {
		for _, segment := range strings.Split(path, "/") {
			node = node.child(segment)
		}
	}
	if node.methods == nil {
		node.methods = make(map[string]http.Handler)
	}
	node.methods[method] = handler
}

// child returns the node for a segment of a pattern, creating it when it is new.
func (m *mux) child(segment string) *mux {
	if len(segment) > 2 && segment[0] == '{' && segment[len(segment)-1] == '}' {
		if m.wildcard == nil {
			m.wildcard = &mux{for404: m.for404}
		}
		m.wildcard.values = append(m.wildcard.values, segment[1:len(segment)-1])
		return m.wildcard
	}
	if m.literals == nil {
		m.literals = make(map[string]*mux)
	}
	if m.literals[segment] == nil {
		m.literals[segment] = &mux{for404: m.for404}
	}
	return m.literals[segment]
}

// pathValue is the value of a wildcard, held back until a route has matched,
// so that a branch abandoned while backtracking leaves nothing behind.
type pathValue struct {
	names []string
	value string
}

// match walks the tree depth first, literals before wildcards, so the most
// specific route wins and a literal without the method falls back to a
// wildcard. Each node is visited at most once, whatever the client sends.
func (m *mux) match(segments []string, method string, values *[]pathValue) http.Handler {
	if len(segments) == 0 || len(segments) == 1 && segments[0] == "" { // tolerate one trailing slash.
		if handler, ok := m.methods[method]; ok {
			return handler
		}
		return m.methods[""]
	}
	segment := segments[0]
	if segment == "" {
		return nil // an empty segment never names a literal, nor fills a wildcard.
	}
	if child, ok := m.literals[segment]; ok {
		if handler := child.match(segments[1:], method, values); handler != nil {
			return handler
		}
	}
	if m.wildcard != nil {
		held := len(*values)
		*values = append(*values, pathValue{m.wildcard.values, segment})
		if handler := m.wildcard.match(segments[1:], method, values); handler != nil {
			return handler
		}
		*values = (*values)[:held]
	}
	return nil
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		values  []pathValue
		handler http.Handler
	)
	if segments, ok := pathSegments(r.URL); ok {
		handler = m.match(segments, routedMethod(r), &values)
	}
	if handler == nil {
		// No route matched: the 404 handler sees the request unchanged.
		(*m.for404).ServeHTTP(w, r)
		return
	}
	for _, value := range values {
		for _, name := range value.names {
			r.SetPathValue(name, value.value)
		}
	}
	handler.ServeHTTP(w, withRoutedPath(r))
}

// pathSegments splits the escaped path and only then decodes each segment, so
// that an escaped slash stays inside the value it belongs to.
func pathSegments(u *url.URL) ([]string, bool) {
	segments := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	for i, segment := range segments {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return nil, false
		}
		segments[i] = decoded
	}
	return segments, true
}

// routedMethod lets clients behind infrastructure without the QUERY method
// reach those routes, as the documentation pages advertise.
func routedMethod(r *http.Request) string {
	if r.Method == http.MethodPost && r.Header.Get("X-HTTP-Method-Override") == "QUERY" {
		return "QUERY"
	}
	return r.Method
}

// withRoutedPath returns a shallow copy of r whose URL carries the path left
// once routing has used it up, which is "/", as handlers have always seen. The
// caller's request and its *url.URL are left untouched, so upstream middleware
// (Sentry, access logging, tracing) continues to observe the URL the client
// actually requested.
func withRoutedPath(r *http.Request) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	u := new(url.URL)
	*u = *r.URL
	u.Path, u.RawPath = "/", ""
	r2.URL = u
	return r2
}

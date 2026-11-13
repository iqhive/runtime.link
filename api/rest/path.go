package rest

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// mux is one node of a route tree. Methods live in their own map, apart from
// path segments, so that a client can never reach a method's handler by
// spelling its name in the path.
type mux struct {
	methods   map[string]http.Handler
	literals  map[string]*mux
	wildcard  *mux
	remainder *mux
	names     []string // of the wildcard or remainder this node is, aliases included.
	for404    *http.Handler
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
		segments := strings.Split(path, "/")
		for i, segment := range segments {
			node = node.child(segment, i == len(segments)-1)
		}
	}
	if node.methods == nil {
		node.methods = make(map[string]http.Handler)
	}
	node.methods[method] = handler
}

// child returns the node for a pattern segment, creating it when it is new.
func (m *mux) child(segment string, last bool) *mux {
	if len(segment) <= 2 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		if m.literals == nil {
			m.literals = make(map[string]*mux)
		}
		if m.literals[segment] == nil {
			m.literals[segment] = new(mux)
		}
		return m.literals[segment]
	}
	spelled := segment[1 : len(segment)-1]
	name, isRemainder := strings.CutSuffix(spelled, "*")
	slot := &m.wildcard
	if isRemainder && last {
		slot = &m.remainder
	}
	if *slot == nil {
		*slot = new(mux)
	}
	// The value goes by the name as spelled too, asterisk and all, as that
	// is the name code reading r.PathValue has always had to use.
	for _, alias := range []string{name, spelled} {
		if !slices.Contains((*slot).names, alias) {
			(*slot).names = append((*slot).names, alias)
		}
	}
	return *slot
}

// pathValue is a wildcard's value, held back from the request until a route has
// matched so that a branch abandoned while backtracking leaves nothing behind.
type pathValue struct {
	names []string
	value string

	// escaped is the value as it was sent. It still tells a comma that
	// separates the elements of a list from one inside an element.
	escaped string
}

// escapedName is the name a wildcard's value goes by while it is still escaped,
// next to the decoded one under the wildcard's own name. No parameter can have
// this name, as a slash ends a segment of the pattern it would be written in.
func escapedName(name string) string {
	return name + "/escaped"
}

// requestPath is the segments of a path, decoded for matching and as they were
// sent.
type requestPath struct {
	segments []string
	escaped  []string
}

func (p requestPath) from(i int) requestPath {
	return requestPath{p.segments[i:], p.escaped[i:]}
}

// match walks the tree depth first, literals before wildcards, so the most
// specific route for the method wins and a literal that has no such method
// falls back to a wildcard. Each node is visited at most once, so the cost is
// bounded by the size of the tree, whatever the client sends.
func (m *mux) match(path requestPath, method string, values *[]pathValue) http.Handler {
	if pathIsConsumed(path.segments) {
		return m.handlerFor(method)
	}
	segment := path.segments[0]
	if segment == "" {
		return nil
	}
	if child, ok := m.literals[segment]; ok {
		if handler := child.match(path.from(1), method, values); handler != nil {
			return handler
		}
	}
	if m.wildcard != nil {
		held := len(*values)
		*values = append(*values, pathValue{m.wildcard.names, segment, path.escaped[0]})
		if handler := m.wildcard.match(path.from(1), method, values); handler != nil {
			return handler
		}
		*values = (*values)[:held]
	}
	if m.remainder != nil {
		if handler := m.remainder.handlerFor(method); handler != nil {
			*values = append(*values, pathValue{m.remainder.names, strings.Join(path.segments, "/"), strings.Join(path.escaped, "/")})
			return handler
		}
	}
	return nil
}

func (m *mux) handlerFor(method string) http.Handler {
	if handler, ok := m.methods[method]; ok {
		return handler
	}
	return m.methods[""]
}

// pathIsConsumed tolerates a single trailing slash: /things/ is /things.
func pathIsConsumed(segments []string) bool {
	return len(segments) == 0 || len(segments) == 1 && segments[0] == ""
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var values []pathValue
	var handler http.Handler
	if path, ok := pathSegments(r.URL); ok {
		handler = m.match(path, routedMethod(r), &values)
	}
	if handler == nil {
		(*m.for404).ServeHTTP(w, r)
		return
	}
	for _, value := range values {
		for _, name := range value.names {
			r.SetPathValue(name, value.value)
			r.SetPathValue(escapedName(name), value.escaped)
		}
	}
	handler.ServeHTTP(w, routedRequest(r))
}

// routedFrom is the context key of the URL a request had before it was routed.
type routedFrom struct{}

// routedRequest returns a copy of r for the handler of the route it matched,
// whose URL holds the path left once routing has used it up, which is "/". That
// is what a handler, and the Auth it calls, has always been shown of the path.
// The caller's request and URL are left as they are.
func routedRequest(r *http.Request) *http.Request {
	routed := r.WithContext(context.WithValue(r.Context(), routedFrom{}, r.URL))
	u := *r.URL
	u.Path, u.RawPath = "/", "/"
	routed.URL = &u
	return routed
}

// originalPath is the path a client asked for, before it was routed.
func originalPath(r *http.Request) string {
	if original, ok := r.Context().Value(routedFrom{}).(*url.URL); ok {
		return original.Path
	}
	return r.URL.Path
}

// pathSegments splits the escaped path and only then decodes each segment, so
// that an escaped slash stays inside the value it belongs to.
func pathSegments(u *url.URL) (requestPath, bool) {
	escaped := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	segments := make([]string, len(escaped))
	for i, segment := range escaped {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return requestPath{}, false
		}
		segments[i] = decoded
	}
	return requestPath{segments, escaped}, true
}

// routedMethod lets clients behind infrastructure without the QUERY method
// reach those routes, as the documentation pages advertise.
func routedMethod(r *http.Request) string {
	if r.Method == http.MethodPost && r.Header.Get("X-HTTP-Method-Override") == "QUERY" {
		return "QUERY"
	}
	return r.Method
}

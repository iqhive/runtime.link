package http

import (
	"net/http"
	"strings"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// WithHeaders returns a copy of req with the headers set, unless req is a
// redirect that left the host it was first sent to, which must not be handed
// the credentials the headers carry. The request itself is not modified, as a
// RoundTripper must not.
func WithHeaders(req *http.Request, headers map[string]string) *http.Request {
	if redirectedAway(req) {
		return req
	}
	req = req.Clone(req.Context())
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return req
}

// redirectedAway reports whether req is a redirect that left the scheme and host
// of the first request in its chain. The scheme counts, as a redirect from https
// to http would send a credential in the clear.
func redirectedAway(req *http.Request) bool {
	first := req
	for first.Response != nil && first.Response.Request != nil {
		first = first.Response.Request
	}
	return first.URL.Scheme != req.URL.Scheme || !strings.EqualFold(first.URL.Host, req.URL.Host)
}

func ClientWithHeaders(headers map[string]string) *http.Client {
	var DefaultTransport = http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return DefaultTransport.RoundTrip(WithHeaders(r, headers))
		}),
	}
}

func ClientWithHeader(name, auth string) *http.Client {
	return ClientWithHeaders(map[string]string{name: auth})
}

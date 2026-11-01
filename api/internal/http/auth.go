package http

import (
	"net/http"
	"strings"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// WithHeaders returns req with the given headers, unless a redirect has taken it
// away from where it started: these are credentials, meant for that server. It
// returns a copy, as a [http.RoundTripper] must not change the request it is given.
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
// of the first request in its chain. The scheme counts, a redirect from https to
// http would send a credential in the clear.
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

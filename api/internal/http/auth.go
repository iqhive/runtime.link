package http

import (
	"net/http"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func ClientWithHeaders(headers map[string]string) *http.Client {
	var DefaultTransport = http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			for k, v := range headers {
				r.Header.Set(k, v)
				defer r.Header.Del(k)
			}
			return DefaultTransport.RoundTrip(r)
		}),
	}
}

func ClientWithHeader(name, auth string) *http.Client {
	var DefaultTransport = http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r.Header.Set(name, auth)
			defer r.Header.Del(name)
			return DefaultTransport.RoundTrip(r)
		}),
	}
}

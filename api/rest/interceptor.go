package rest

import (
	"context"
	"errors"
	"net/http"
)

type interceptorKey struct{}

// Interceptor returns a context in which every request that a linked REST API
// sends goes through fn. fn can inspect or change the request, send it (or a
// different one) with send, or not send it at all, and inspect, change or make
// up the response. A response fn returns is handled exactly as one from the
// network would be: decoded into the function's results, or into its error.
//
// Interceptors nest: the outermost sees the request first and the response
// last. WebSocket connections are not intercepted.
func Interceptor(ctx context.Context, fn func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error)) context.Context {
	outer, _ := ctx.Value(interceptorKey{}).([]interceptor)
	return context.WithValue(ctx, interceptorKey{}, append(outer[:len(outer):len(outer)], fn))
}

type interceptor = func(*http.Request, func(*http.Request) (*http.Response, error)) (*http.Response, error)

var errNoResponse = errors.New("rest interceptor returned no response")

// send req with client, through the interceptors in ctx.
func send(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	chain, _ := ctx.Value(interceptorKey{}).([]interceptor)
	var through func(i int, req *http.Request) (*http.Response, error)
	through = func(i int, req *http.Request) (*http.Response, error) {
		if i == len(chain) {
			return client.Do(req)
		}
		resp, err := chain[i](req, func(req *http.Request) (*http.Response, error) {
			return through(i+1, req)
		})
		if err == nil && resp == nil {
			return nil, errNoResponse
		}
		if resp != nil && resp.Body == nil {
			resp.Body = http.NoBody
		}
		return resp, err
	}
	return through(0, req)
}

//go:build !js

package rest

import (
	_ "embed"
	"net/http"

	"runtime.link/api"
	"runtime.link/api/xray"
)

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
func ListenAndServe(addr string, auth api.Auth[*http.Request], impl any) error {
	handler, err := Handler(auth, impl)
	if err != nil {
		return xray.New(err)
	}
	return http.ListenAndServe(addr, handler)
}

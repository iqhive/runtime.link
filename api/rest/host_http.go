//go:build !js

package rest

import (
	_ "embed"
	"net/http"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/xray"
)

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication, and an error the app returns is answered with
// its status text alone, as there is no Redact to declare what of it
// the caller may see.
//
// The server reads a request's headers within 10 seconds and drops a
// connection that has sat idle for 2 minutes, but never times out a
// request that is in progress, so a stream or a websocket is served for
// as long as it lasts.
func ListenAndServe(addr string, auth api.Auth[*http.Request], impl any) error {
	handler, err := Handler(auth, impl)
	if err != nil {
		return xray.New(err)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	return server.ListenAndServe()
}

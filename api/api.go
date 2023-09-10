/*
Package api provides an API layer for runtime.link.

Four tags are recognised by this package, they all represent underlying
transport protocols:

  - http
  - rest (http + json)
  - soap (http + xml)
  - grpc (http + protobuf)

Each tag has its own format that describes how to link against the function
using the protocol.
*/
package api

import (
	"errors"
	"net/http"
	"reflect"

	api_http "runtime.link/api/internal/http"
	"runtime.link/std"
)

var (
	ErrNotImplemented = api_http.ErrNotImplemented
)

// Transport function should return a HTTP handler that serves the given
// runtime.link structure. If the [AccessController] is nil, no authentication steps
// will be performed. If link is true, overwrite all of the structure's
// functions so that they call the API using this transport. The handler
// returned this way will serve as a proxy.
type Transport func(link bool, access AccessController, spec std.Structure, hosts ...string) (http.Handler, error)

// Specification can be embedded into a runtime.link structure to indicate that
// it supports the API link layer.
type Specification interface {
	std.Host
}

// Import the given runtime.link structure as an API of the given
// type reachable at the given URL. If the [Authentication] is nil
// [http.DefaultClient] will be used and no authentication will be
// performed.
func Import[API any](T Transport, url string, auth AccessController) API {
	var (
		api       API
		structure = std.StructureOf(&api)
	)
	T(true, auth, structure, url)
	return api
}

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
func ListenAndServe(addr string, auth AccessController, impl any) error {
	return errors.New("not implemented")
}

// Handler returns a [http.Handler] that serves supported API types.
// If the [Authenticator] is nil, requests will not require any
// authentication.
func Handler(auth AccessController, impl any) http.Handler {
	return nil
}

// AccessController returns an error if the given request is not
// allowed to access the given function. Used to implement
// authentication and authorisation for API calls.
type AccessController interface {
	// AssertHeader is called before the request is processed it
	// should confirm the identify of the caller.
	AssertHeader(*http.Request, std.Function) error

	// AssertAccess is called after arguments have been passed
	// and before the function is called. It should assert that
	// the identified caller is allowed to access the function.
	AssertAccess(*http.Request, std.Function, []reflect.Value) error
}

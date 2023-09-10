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
	"context"
	"errors"
	"fmt"
	"net/http"

	"runtime.link/std"
)

// Type of API.
type Type int

// Types of APIs recognised by this package.
const (
	HTTP Type = iota
	REST
	SOAP
	GRPC
)

type Specification interface {
	std.Host
}

// Authentication returns a HTTP client that will be used to make
// requests to the given URL. Can additionally be used to control
// properties of the [http.Client] making API calls.
type Authentication func(context.Context) http.Client

// Import the given runtime.link structure as an API of the given
// type reachable at the given URL. If the [Authentication] is nil
// [http.DefaultClient] will be used and no authentication will be
// performed.
func Import[API any](t Type, url string, auth Authentication) API {
	var (
		api       API
		structure = std.StructureOf(&api)
	)
	switch t {
	case HTTP:
		structure.MakeError(fmt.Errorf("function imported using currently unsupported API type HTTP"))
	case REST:
		structure.MakeError(fmt.Errorf("function imported using currently unsupported API type REST"))
	case SOAP:
		structure.MakeError(fmt.Errorf("function imported using currently unsupported API type SOAP"))
	case GRPC:
		structure.MakeError(fmt.Errorf("function imported using currently unsupported API type GRPC"))
	default:
		structure.MakeError(fmt.Errorf("unknown API type: %d", t))
	}
	return api
}

// Authenticator returns true if the given request is authenticated,
// false otherwise.
type Authenticator func(*http.Request, std.Function) bool

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
func ListenAndServe(addr string, auth Authenticator, impl any) error {
	return errors.New("not implemented")
}

// Handler returns a [http.Handler] that serves supported API types.
// If the [Authenticator] is nil, requests will not require any
// authentication.
func Handler(auth Authenticator, impl any) http.Handler {
	return nil
}

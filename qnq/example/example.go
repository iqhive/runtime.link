// Package example provide an example on how to create a runtime.link structures for supported link layers.
package example

import (
	"runtime.link/api"
	"runtime.link/cmd"
	"runtime.link/lib"
)

// API specification structure, typically named API for general structures, may
// be more suitably named Functions, Library or Command when the API is
// restricted to a specific runtime.link layer. Any Go comments in the source
// are intended to document design notes and ideas. This leaves Go struct tags
// for recording developer-facing documentation.
type API struct {
	cmd.Line `cmd:"example"
        [usage] example hello_world`
	api.Specification `api:"Example"
        is an example of a runtime.link API structure.` // this tag contains the API's introductory documentation.
	lib.Documentation `lib:"libexample"
        exposes a single function for returning the string "Hello World"` // this tag contains the API's introductory documentation.

	// HelloWorld includes runtime.link tags that specify how the function is called
	// across different link-layers. Typically, a context.Context argument and error
	// return value should be included here, they are omitted here for brevity.
	HelloWorld func() string `cmd:"hello_world" ffi:"example_helloworld func()$char" rest:"GET /hello_world"
        returns the string "Hello World"` // documentation for the function.
}

// New returns an implementation of the API. This doesn't have to be defined in the
// same package and may not even be implemented in Go. This will often be the case when
// representing an external API controlled by a third-party.
func New() API {
	return API{
		HelloWorld: func() string {
			return "Hello World"
		},
	}
}

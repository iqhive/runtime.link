package api_test

import (
	"log"
	"os"

	"runtime.link/api"
	"runtime.link/api/args"
	"runtime.link/api/rest"
)

// API specification structure, typically named API for general structures, may
// be more suitably named Functions, Library or Command when the API is
// restricted to a specific runtime.link layer. Any Go comments in the source
// are intended to document design notes and ideas. This leaves Go struct tags
// for recording developer-facing documentation.
type API struct {
	api.Specification `api:"Example" link:"libexample" exec:"example"
        is an example of a runtime.link API structure.` // this section of the tag contains documentation.

	// HelloWorld includes runtime.link tags that specify how the function is called
	// across different link-layers. Typically, a context.Context argument and error
	// return value should be included here, they are omitted here for brevity.
	HelloWorld func() string `args:"hello_world" link:"example_helloworld func()$char" rest:"GET /hello_world"
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

func Example() {
	example := New()
	if port := os.Getenv("PORT"); port != "" {
		if err := rest.ListenAndServe(port, nil, example); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := args.Main(os.Args, os.Environ(), example); err != nil {
		log.Fatal(err)
	}
}

package api_test

import (
	"testing"

	ffi "runtime.link"
	"runtime.link/api"
)

type Example struct {
	_ ffi.Documentation `
		API used for testing.`

	HelloWorld func() string `rest:"GET /hello-world"
		returns "Hello World"`
}

var ExampleClient = api.Import[Example](api.REST, "http://localhost:8080", nil)

func TestExample(t *testing.T) {
	api.ListenAndServe(":8080", nil, Example{
		HelloWorld: func() string {
			return "Hello World"
		},
	})

}

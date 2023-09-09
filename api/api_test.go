package api_test

import (
	"testing"

	"runtime.link/api"
	"runtime.link/ffi"
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

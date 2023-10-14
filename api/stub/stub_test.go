package stub_test

import (
	"errors"
	"testing"

	"runtime.link/api"
	"runtime.link/api/stub"
)

func TestStub(t *testing.T) {
	type Example struct {
		api.Specification

		HelloWorld          func() string
		HelloWorldWithError func() (string, error)
	}
	var example = api.Import[Example](stub.API, stub.Testing, nil)
	if example.HelloWorld() != "" {
		t.Fatal("got", example.HelloWorld(), "want", "")
	}

	example = api.Import[Example](stub.API, stub.Testing, errors.New("not implemented"))
	_, err := example.HelloWorldWithError()
	if err == nil {
		t.Fatal("got", err, "want", "not implemented")
	}
	if err.Error() != "not implemented" {
		t.Fatal("got", err, "want", "not implemented")
	}
}

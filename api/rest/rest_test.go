package rest_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
	"runtime.link/xyz"
)

func TestErrors(t *testing.T) {
	type Error api.Error[struct {
		Internal xyz.Case[Error, error] `http:"500"
			internal server error`
		AccessDenied Error `http:"403"
			access denied`
	}]
	var Errors = xyz.AccessorFor(Error.Values)
	var API struct {
		api.Specification

		api.Register[error, Error]

		DoSomething func(context.Context) error `rest:"POST /"`
	}
	API.DoSomething = func(ctx context.Context) error {
		return Errors.AccessDenied
	}

	handler, err := rest.Handler(nil, &API)
	if err != nil {
		t.Fatal(err)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("POST", "/", nil))

	if resp.Code != 403 {
		t.Errorf("got %v, want %v", resp.Code, 403)
	}
	if resp.Body.String() != "access denied\n" {
		t.Errorf("got %q, want %q", resp.Body.String(), "access denied\n")
	}
}

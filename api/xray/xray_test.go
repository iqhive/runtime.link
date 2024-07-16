package xray_test

import (
	"context"
	"net/http"
	"testing"

	"runtime.link/api"
	"runtime.link/api/example/petstore"
	"runtime.link/api/rest"
	"runtime.link/api/xray"
)

func TestRay(t *testing.T) {
	var ctx = xray.NewContext(context.Background())
	var Petstore = api.Import[petstore.API](rest.API, "http://localhost:9999", nil)

	Petstore.AddPet(ctx, petstore.Pet{})

	for xray.ContextHas[*http.Request](ctx) {
		req := xray.ContextGet[*http.Request](ctx)
		t.Logf("Request: %s %s", req.Method, req.URL)
		t.Logf("Body: %s", req.Body)
	}
}

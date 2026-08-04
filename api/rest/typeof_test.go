package rest_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
	"runtime.link/xyz"
)

type Shape xyz.Tagged[any, struct {
	Circle   xyz.Case[Shape, struct{ Radius int }] `json:"circle"`
	Square   xyz.Case[Shape, struct{ Side int }]   `json:"square"`
	Triangle xyz.Case[Shape, struct{ Base int }]   `json:"triangle"`
}]

type ShapeFilter struct {
	Owner string          `json:"owner"`
	Kind  xyz.TypeOf[Shape] `json:"kind,omitzero"`
}

// TestTypeOfParam ensures a tagged-union type selector (xyz.TypeOf[T]) can be
// decoded from a textual query parameter. Such fields are interfaces, so they
// need the union-resolution path rather than fmt.Sscanf, which previously
// failed with "can't scan type: *xyz.TypeOf[...]".
func TestTypeOfParam(t *testing.T) {
	type API struct {
		api.Specification

		Fetch func(context.Context, ShapeFilter) (string, error) `rest:"GET /shapes?%v shapes"`
	}
	var impl = API{
		Fetch: func(ctx context.Context, filter ShapeFilter) (string, error) {
			if filter.Kind == nil {
				return "none", nil
			}
			key, _ := filter.Kind.Key()
			return key, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}

	do := func(query string) (int, string) {
		req := httptest.NewRequest("GET", "/shapes?"+query, nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	want := func(key string) string { return "{\n\t\"shapes\": \"" + key + "\"\n}" }
	if code, body := do("owner=amy&kind=square"); code != 200 || body != want("square") {
		t.Fatalf("square: code=%d body=%q", code, body)
	}
	if code, body := do("owner=amy&kind=circle"); code != 200 || body != want("circle") {
		t.Fatalf("circle: code=%d body=%q", code, body)
	}
	if code, body := do("owner=amy"); code != 200 || body != want("none") {
		t.Fatalf("omitted: code=%d body=%q", code, body)
	}
	if code, _ := do("owner=amy&kind=bogus"); code == 200 {
		t.Fatalf("bogus kind should not succeed, got code=%d", code)
	}
}

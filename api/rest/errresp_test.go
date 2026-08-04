package rest

import (
	"strconv"
	"testing"

	"runtime.link/api"
	"runtime.link/api/internal/oas"
	"runtime.link/xyz"
)

// loginError mimics an error type registered against an API via
// api.Register[error, loginError]: a raw xyz.Switch whose cases each carry a
// json tag. Such a type has no per-case api.Scenario metadata (only
// api.Error-based types expose Reflection), so its cases are surfaced through
// its schema instead.
type loginError xyz.Switch[int, struct {
	ExpiredPassword    loginError `json:"expired_password"`
	InvalidCredentials loginError `json:"invalid_credentials"`
}]

var loginErrors = xyz.AccessorFor(loginError.Values)

func (e loginError) Error() string { return e.String() }

type errResponseAPI struct {
	api.Specification

	_ api.Register[error, loginError]

	Login func() error `rest:"POST /login"`
}

// TestErrorResponsesGenerated verifies that error types registered against an
// API are documented as response entries keyed by their HTTP status, with a
// concrete example payload drawn from the type's enumerated values.
func TestErrorResponsesGenerated(t *testing.T) {
	var doc oas.Document
	if err := addFunctionTo(&doc, api.StructureOf(errResponseAPI{}).Functions[0], "default"); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/login"].Post
	if op == nil {
		t.Fatal("no POST operation generated")
	}
	// A raw registered error defaults to a 500 response (matching how the host
	// handles an un-annotated error).
	resp := op.Responses[xyz.Raw[oas.ResponseKey](strconv.Itoa(500))]
	if resp == nil {
		t.Fatal("no 500 error response generated")
	}
	media := resp.Content["application/json"]
	if media.Schema == nil {
		t.Fatal("error response has no schema")
	}
	if got := string(media.Example); got != `"expired_password"` {
		t.Errorf("expected example %q, got %q", `"expired_password"`, got)
	}
	// The schema is registered as a reusable component; its enum lists the
	// possible error values.
	comp := doc.Components.Schemas["rest"].Defs["loginError"]
	if comp == nil {
		t.Fatal("error schema was not registered as a component")
	}
	if len(comp.Enum) != 2 {
		t.Fatalf("expected 2 enum values, got %v", comp.Enum)
	}
}

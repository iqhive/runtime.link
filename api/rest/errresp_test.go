package rest

import (
	"strconv"
	"strings"
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
	if err := addFunctionTo(&doc, api.StructureOf(errResponseAPI{}).Functions[0], "default", nil); err != nil {
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

// envelope is a stand-in for a host's wire error representation: a struct that
// wraps the bare error and carries its own HTTP status.
type envelope struct {
	Code  string `json:"error"`
	Guide string `json:"guide"`
}

func (envelope) Error() string   { return "envelope" }
func (envelope) StatusHTTP() int { return 422 }

// docWithEnvelope implements the rest package's errorDocumenter hook, turning
// any error into an envelope.
type docWithEnvelope struct{}

func (docWithEnvelope) DocumentError(err error) error {
	return envelope{Code: err.Error(), Guide: "how to fix " + err.Error()}
}

// TestErrorResponsesUseDocumenter verifies that when a host provides an
// errorDocumenter, error responses are documented using the wire representation
// it returns (schema, status and example) rather than the bare error type.
func TestErrorResponsesUseDocumenter(t *testing.T) {
	var doc oas.Document
	if err := addFunctionTo(&doc, api.StructureOf(errResponseAPI{}).Functions[0], "default", docWithEnvelope{}); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/login"].Post
	if op == nil {
		t.Fatal("no POST operation generated")
	}
	// The envelope reports a 422 status, so the response is keyed there rather
	// than the bare error's default 500.
	resp := op.Responses[xyz.Raw[oas.ResponseKey](strconv.Itoa(422))]
	if resp == nil {
		t.Fatalf("no 422 error response generated; responses: %v", op.Responses)
	}
	media := resp.Content["application/json"]
	if media.Schema == nil {
		t.Fatal("error response has no schema")
	}
	// The example is the marshaled envelope, drawn from the documenter.
	if got := string(media.Example); !strings.Contains(got, `"guide"`) || !strings.Contains(got, `"error"`) {
		t.Errorf("expected envelope example with guide/error fields, got %q", got)
	}
}

// errAlpha and errBeta are two distinct bare error types that share a 400
// status, used to verify same-status errors are unioned rather than clobbered.
type errAlpha struct{}

func (errAlpha) Error() string   { return "alpha" }
func (errAlpha) StatusHTTP() int { return 400 }

type errBeta struct{}

func (errBeta) Error() string   { return "beta" }
func (errBeta) StatusHTTP() int { return 400 }

type multiErrorAPI struct {
	api.Specification

	_ api.Register[error, errAlpha]
	_ api.Register[error, errBeta]

	Do func() error `rest:"POST /do"`
}

// TestErrorResponsesOneOf verifies that multiple distinct error types sharing a
// status are documented as a single response whose schema is the oneOf union of
// their schemas, rather than the last-registered error overwriting the rest.
func TestErrorResponsesOneOf(t *testing.T) {
	var doc oas.Document
	if err := addFunctionTo(&doc, api.StructureOf(multiErrorAPI{}).Functions[0], "default", nil); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/do"].Post
	if op == nil {
		t.Fatal("no POST operation generated")
	}
	resp := op.Responses[xyz.Raw[oas.ResponseKey](strconv.Itoa(400))]
	if resp == nil {
		t.Fatalf("no 400 error response generated; responses: %v", op.Responses)
	}
	media := resp.Content["application/json"]
	if media.Schema == nil {
		t.Fatal("error response has no schema")
	}
	if len(media.Schema.OneOf) != 2 {
		t.Fatalf("expected oneOf of 2 schemas, got %d: %+v", len(media.Schema.OneOf), media.Schema)
	}
}

// TestErrorResponsesMergeExamples verifies that when several errors share both a
// status and (via the documenter) a wire schema, the schema is documented once
// and each error contributes a distinct named example.
func TestErrorResponsesMergeExamples(t *testing.T) {
	var doc oas.Document
	if err := addFunctionTo(&doc, api.StructureOf(multiErrorAPI{}).Functions[0], "default", docWithEnvelope{}); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/do"].Post
	if op == nil {
		t.Fatal("no POST operation generated")
	}
	// Both errors transform to the same envelope type (422), so the schema
	// dedupes to a single (non-oneOf) schema.
	resp := op.Responses[xyz.Raw[oas.ResponseKey](strconv.Itoa(422))]
	if resp == nil {
		t.Fatalf("no 422 error response generated; responses: %v", op.Responses)
	}
	media := resp.Content["application/json"]
	if media.Schema == nil || len(media.Schema.OneOf) != 0 {
		t.Fatalf("expected a single schema, got %+v", media.Schema)
	}
	// Each error contributes a named example keyed by its slug.
	if len(media.Examples) != 2 {
		t.Fatalf("expected 2 named examples, got %d: %v", len(media.Examples), media.Examples)
	}
	for _, name := range []string{"alpha", "beta"} {
		ex, ok := media.Examples[name]
		if !ok {
			t.Errorf("missing example %q", name)
			continue
		}
		if !strings.Contains(string(ex.Value), `"`+name+`"`) {
			t.Errorf("example %q value does not carry its slug: %s", name, ex.Value)
		}
	}
}

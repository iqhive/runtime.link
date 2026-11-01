package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/api/stub"
	"github.com/iqhive/runtime.link/xyz"
)

// ForLogin is a named error interface that arbitrary errors do not implement.
type ForLogin interface {
	error
	LoginError()
}

// loginFailure is an implementation with a case able to hold any error.
type loginFailure api.Error[struct {
	Rejected xyz.Case[loginFailure, error] `http:"401"
		those credentials were rejected`
	Unavailable xyz.Case[loginFailure, error] `http:"503"
		try again later`
}]

func (loginFailure) LoginError() {}

var loginFailures = xyz.AccessorFor(loginFailure.Values)

// rejection is an implementation without a case for a failure on the server.
type rejection api.Error[struct {
	Rejected rejection `http:"401"`
	Denied   xyz.Case[rejection, error]
}]

func (rejection) LoginError() {}

// plainLoginError implements the interface without being registered.
type plainLoginError struct{ text string }

func (e plainLoginError) Error() string { return e.text }
func (plainLoginError) LoginError()     {}

type pointerLoginError struct{}

func (*pointerLoginError) Error() string { return "pointer login error" }
func (*pointerLoginError) LoginError()   {}

type bareLoginAPI struct {
	api.Specification
	Login func(context.Context) (int, ForLogin) `rest:"GET /login"`
}

type registeredLoginAPI struct {
	api.Specification
	Error api.Register[ForLogin, loginFailure]
	Login func(context.Context) (int, ForLogin) `rest:"GET /login"`
}

type registeredOnErrorLoginAPI struct {
	api.Specification
	Error api.Register[error, loginFailure]
	Login func(context.Context) (int, ForLogin)
}

type rejectionLoginAPI struct {
	api.Specification
	Error api.Register[ForLogin, rejection]
	Login func(context.Context) (int, ForLogin)
}

type pointerLoginAPI struct {
	api.Specification
	Error api.Register[ForLogin, *pointerLoginError]
	Login func(context.Context) (int, ForLogin)
}

func TestErrorThatCanBeReturnedAsIs(t *testing.T) {
	var a bareLoginAPI
	want := plainLoginError{"bad password"}
	api.StructureOf(&a).MakeError(want)
	if _, err := a.Login(context.Background()); err != ForLogin(want) {
		t.Errorf("got %v", err)
	}

	wrapped := fmt.Errorf("while calling: %w", want)
	api.StructureOf(&a).MakeError(wrapped)
	if _, err := a.Login(context.Background()); err != ForLogin(want) {
		t.Errorf("an error in the chain was not found: %v", err)
	}
}

func TestArbitraryErrorIsWrappedInServerFailureCase(t *testing.T) {
	var a registeredLoginAPI
	cause := errors.New("connection refused")
	api.StructureOf(&a).MakeError(cause)
	n, err := a.Login(context.Background())
	if n != 0 || err == nil || err.Error() != "connection refused" {
		t.Fatalf("got %d, %v", n, err)
	}
	failure, ok := err.(loginFailure)
	if !ok {
		t.Fatalf("got %T", err)
	}
	// A refused connection says nothing about the credentials.
	if failure.StatusHTTP() != 503 {
		t.Errorf("reported as a %d", failure.StatusHTTP())
	}
	if loginFailures.Unavailable.Get(failure) != cause {
		t.Errorf("wrapped %v", loginFailures.Unavailable.Get(failure))
	}
}

func TestImplementationRegisteredOnErrorIsUsed(t *testing.T) {
	var a registeredOnErrorLoginAPI
	api.StructureOf(&a).MakeError(errors.New("connection refused"))
	if _, err := a.Login(context.Background()); err == nil || err.Error() != "connection refused" {
		t.Errorf("got %v", err)
	}
}

// recovered returns what fn panics with, or nil.
func recovered(fn func()) (value any) {
	defer func() { value = recover() }()
	fn()
	return nil
}

// checkPanicNames fails unless a panic names everything needed to fix it.
func checkPanicNames(t *testing.T, value any, cause error, function string) {
	t.Helper()
	failure, ok := value.(error)
	if !ok {
		t.Fatalf("want a panic with an error, got %v", value)
	}
	for _, part := range []string{function, "api_test.ForLogin", cause.Error(), "api.Register"} {
		if !strings.Contains(failure.Error(), part) {
			t.Errorf("%q does not name %q", failure, part)
		}
	}
	if strings.Contains(failure.Error(), "reflect:") {
		t.Errorf("still the raw reflect panic: %q", failure)
	}
	if !errors.Is(failure, cause) {
		t.Error("the cause cannot be reached with errors.Is")
	}
}

// A nil result would read as a success, and Go cannot make up a value of an
// arbitrary interface, so the only honest outcome is to stop.
func TestErrorThatNothingCanHoldPanicsClearly(t *testing.T) {
	var a bareLoginAPI
	cause := errors.New("transport failed")
	api.StructureOf(&a).MakeError(cause)
	value := recovered(func() { a.Login(context.Background()) })
	if value == nil {
		t.Fatal("the failed call read as a success")
	}
	checkPanicNames(t, value, cause, "Login")
}

func TestPanicNamesTheNestedFunction(t *testing.T) {
	var a struct {
		api.Specification
		Auth struct {
			Login func(context.Context) (int, ForLogin)
		}
	}
	cause := errors.New("transport failed")
	api.StructureOf(&a).MakeError(cause)
	checkPanicNames(t, recovered(func() { a.Auth.Login(context.Background()) }), cause, "Auth.Login")
}

// A registered type with no case for this failure cannot carry the cause, so a
// blank value of it would hide the reason behind the text "empty error".
func TestRegisteredImplementationThatCannotHoldTheErrorPanics(t *testing.T) {
	cause := errors.New("connection refused")

	var a rejectionLoginAPI
	api.StructureOf(&a).MakeError(cause)
	checkPanicNames(t, recovered(func() { a.Login(context.Background()) }), cause, "Login")

	var p pointerLoginAPI
	api.StructureOf(&p).MakeError(cause)
	checkPanicNames(t, recovered(func() { p.Login(context.Background()) }), cause, "Login")
}

// Linking is not when a function fails: an API with one function that can't
// report the error must still import, and its other functions must still work.
func TestUnrepresentableErrorWaitsForTheCall(t *testing.T) {
	type mixed struct {
		api.Specification
		Login func(context.Context) (int, ForLogin)
		Other func(context.Context) error
	}
	cause := errors.New("stubbed")
	var a mixed
	if value := recovered(func() { a = api.Import[mixed](stub.API, stub.Testing, cause) }); value != nil {
		t.Fatalf("Import panicked: %v", value)
	}
	if err := a.Other(context.Background()); err != cause {
		t.Errorf("got %v", err)
	}
	checkPanicNames(t, recovered(func() { a.Login(context.Background()) }), cause, "Login")

	var b mixed
	if value := recovered(func() { api.StructureOf(&b).MakeError(cause) }); value != nil {
		t.Fatalf("MakeError panicked: %v", value)
	}
	if err := b.Other(context.Background()); err != cause {
		t.Errorf("got %v", err)
	}
}

// The result is worked out on each call now, which must not share any state.
func TestFailingStubIsSafeForConcurrentCalls(t *testing.T) {
	a := api.Import[registeredLoginAPI](stub.API, stub.Testing, errors.New("stubbed"))
	var b registeredLoginAPI
	api.StructureOf(&b).MakeError(errors.New("stubbed"))
	var callers sync.WaitGroup
	for range 8 {
		callers.Go(func() {
			if _, err := a.Login(context.Background()); err == nil {
				t.Error("stub: the failure read as a success")
			}
			if _, err := b.Login(context.Background()); err == nil {
				t.Error("MakeError: the failure read as a success")
			}
		})
	}
	callers.Wait()
}

func TestMakeReturnsNamedErrorInterface(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("transport failed")

	var a registeredLoginAPI
	fn := api.StructureOf(&a).Functions[0]

	fn.Make(func(context.Context, []reflect.Value) ([]reflect.Value, error) { return nil, cause })
	if _, err := a.Login(ctx); err == nil || err.Error() != cause.Error() {
		t.Errorf("[]reflect.Value: got %v", err)
	}
	fn.Make(func(context.Context, []any) ([]any, error) { return nil, cause })
	if _, err := a.Login(ctx); err == nil || err.Error() != cause.Error() {
		t.Errorf("[]any: got %v", err)
	}
	got := fn.Return(nil, cause)
	if err := got[1].Interface().(ForLogin); err.Error() != cause.Error() {
		t.Errorf("Return: got %v", err)
	}
}

func TestStubReturnsNamedErrorInterface(t *testing.T) {
	a := api.Import[registeredLoginAPI](stub.API, stub.Testing, errors.New("stubbed"))
	if _, err := a.Login(context.Background()); err == nil || err.Error() != "stubbed" {
		t.Errorf("got %v", err)
	}
	cause := errors.New("stubbed")
	b := api.Import[bareLoginAPI](stub.API, stub.Testing, cause)
	checkPanicNames(t, recovered(func() { b.Login(context.Background()) }), cause, "Login")
}

func TestTransportFailureOnNamedErrorInterface(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing is listening on url from here on.

	a := api.Import[registeredLoginAPI](rest.API, url, http.DefaultClient)
	_, err := a.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("got %v", err)
	}
}

type concreteError struct{ text string }

func (e concreteError) Error() string { return e.text }

func TestConcreteErrorResult(t *testing.T) {
	var a struct {
		api.Specification
		F func() (int, concreteError)
	}
	fn := api.StructureOf(&a).Functions[0]
	fn.MakeError(concreteError{"held"})
	if _, err := a.F(); err != (concreteError{"held"}) {
		t.Errorf("got %v", err)
	}
	// The zero value of a concrete error type means no error, so it is no
	// more able to report this failure than a nil interface is.
	fn.MakeError(errors.New("cannot be held"))
	value := recovered(func() { a.F() })
	failure, ok := value.(error)
	if !ok || !strings.Contains(failure.Error(), "api_test.concreteError") || !strings.Contains(failure.Error(), "cannot be held") {
		t.Errorf("got %v", value)
	}
}

func TestCallDoesNotReportZeroErrorResults(t *testing.T) {
	var a struct {
		api.Specification
		Value   func() (int, concreteError)
		Pointer func() (int, *pointerLoginError)
		Named   func() (int, ForLogin)
		Plain   func() (int, error)
	}
	functions := api.StructureOf(&a).Functions
	call := func(fn api.Function) ([]reflect.Value, error) {
		t.Helper()
		return fn.Call(context.Background(), nil)
	}
	check := func(fn api.Function) {
		t.Helper()
		results, err := call(fn)
		if err != nil {
			t.Errorf("%s: a zero error result was reported: %v", fn, err)
			return
		}
		if len(results) != 1 || results[0].Int() != 1 {
			t.Errorf("%s: got %v", fn, results)
		}
	}
	a.Value = func() (int, concreteError) { return 1, concreteError{} }
	a.Pointer = func() (int, *pointerLoginError) { return 1, nil }
	a.Named = func() (int, ForLogin) { return 1, nil }
	a.Plain = func() (int, error) { return 1, nil }
	check(functions[0])
	check(functions[1])
	check(functions[2])
	check(functions[3])

	// The mistake Go is known for: a nil pointer returned as an error.
	a.Plain = func() (int, error) {
		var none *pointerLoginError
		return 1, none
	}
	check(functions[3])
	a.Named = func() (int, ForLogin) {
		var none *pointerLoginError
		return 1, none
	}
	check(functions[2])
}

func TestCallStillReportsErrorResults(t *testing.T) {
	var a struct {
		api.Specification
		Value   func() (int, concreteError)
		Pointer func() (int, *pointerLoginError)
		Plain   func() (int, error)
	}
	functions := api.StructureOf(&a).Functions
	check := func(fn api.Function, want error) {
		t.Helper()
		results, err := fn.Call(context.Background(), nil)
		if err == nil || results != nil {
			t.Fatalf("%s: got %v, %v", fn, results, err)
		}
		if err.Error() != want.Error() {
			t.Errorf("%s: got %v, want %v", fn, err, want)
		}
	}
	a.Value = func() (int, concreteError) { return 1, concreteError{"failed"} }
	check(functions[0], concreteError{"failed"})
	a.Pointer = func() (int, *pointerLoginError) { return 1, &pointerLoginError{} }
	check(functions[1], &pointerLoginError{})

	// Only the declared type decides what zero means. Behind an error
	// interface a value without fields is an error like any other, which is
	// how api.ErrAccessDenied has to be read.
	a.Plain = func() (int, error) { return 1, api.ErrAccessDenied }
	check(functions[2], api.ErrAccessDenied)
	a.Plain = func() (int, error) { return 1, concreteError{} }
	check(functions[2], concreteError{})
}

func TestMakeErrorWithNilError(t *testing.T) {
	var a struct {
		api.Specification
		F func() (int, error)
	}
	api.StructureOf(&a).Functions[0].MakeError(nil)
	if n, err := a.F(); n != 0 || err != nil {
		t.Errorf("got %d, %v", n, err)
	}
}

func TestMakeWithAnyResults(t *testing.T) {
	var a struct {
		api.Specification
		One  func(context.Context) (int, error)
		Pair func(context.Context) (int, string, error)
		Ptr  func(context.Context) (*int, error)
		Bare func(context.Context) int
	}
	structure := api.StructureOf(&a)
	ctx := context.Background()
	boom := errors.New("boom")
	var one, pair, ptr, bare = structure.Functions[0], structure.Functions[1], structure.Functions[2], structure.Functions[3]

	one.Make(func(context.Context, []any) ([]any, error) { return nil, boom })
	if n, err := a.One(ctx); n != 0 || err != boom {
		t.Errorf("nil, err: got %d, %v", n, err)
	}
	pair.Make(func(context.Context, []any) ([]any, error) { return nil, boom })
	if n, s, err := a.Pair(ctx); n != 0 || s != "" || err != boom {
		t.Errorf("nil, err with two results: got %d, %q, %v", n, s, err)
	}
	one.Make(func(context.Context, []any) ([]any, error) { return nil, nil })
	if n, err := a.One(ctx); n != 0 || err != nil {
		t.Errorf("nil, nil: got %d, %v", n, err)
	}
	ptr.Make(func(context.Context, []any) ([]any, error) { return []any{nil}, nil })
	if p, err := a.Ptr(ctx); p != nil || err != nil {
		t.Errorf("a nil element: got %v, %v", p, err)
	}
	bare.Make(func(context.Context, []any) ([]any, error) { return nil, nil })
	if n := a.Bare(ctx); n != 0 {
		t.Errorf("nil, nil without an error result: got %d", n)
	}
	one.Make(func(context.Context, []any) ([]any, error) { return []any{7}, nil })
	if n, err := a.One(ctx); n != 7 || err != nil {
		t.Errorf("a value: got %d, %v", n, err)
	}
}

func TestReturnPadsWithEachResultsOwnType(t *testing.T) {
	var a struct {
		api.Specification
		F func() (int, string, error)
	}
	fn := api.StructureOf(&a).Functions[0]
	got := fn.Return([]reflect.Value{reflect.ValueOf(1)}, nil)
	if len(got) != 3 {
		t.Fatalf("got %d results", len(got))
	}
	for i, value := range got {
		if value.Type() != fn.Type.Out(i) {
			t.Errorf("result %d has type %v, want %v", i, value.Type(), fn.Type.Out(i))
		}
	}
}

func TestTransportHintForBool(t *testing.T) {
	_, invalid := strconv.ParseBool("maybe")
	if got := (&api.TransportError{Name: "active", Err: invalid}).Error(); got != "please provide a valid active (must be true or false)" {
		t.Errorf("got %q", got)
	}
	_, invalid = strconv.Atoi("maybe")
	if got := (&api.TransportError{Name: "count", Err: invalid}).Error(); got != "please provide a valid count (must be a number)" {
		t.Errorf("got %q", got)
	}
}

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// TransportError reports an argument sent by the caller that the transport
// (such as [runtime.link/api/rest]) could not decode into the parameter of the
// function being called, so the call was rejected before it was made. It is always the caller's to fix, so it maps to
// a 400.
//
// Its Error is safe to show the caller: it is written only from the API's own
// description, being the parameter's name and (where the underlying failure is
// one runtime.link recognises) what a valid value looks like. The underlying
// failure is written for a Go developer, may quote internal type names or the
// value sent, and is only reachable via Unwrap, for logs.
type TransportError struct {
	// Name is the parameter as the caller addresses it (the path, query or body
	// field name), or empty when the request body as a whole was rejected.
	Name string
	Err  error
}

func (e *TransportError) Error() string {
	subject := e.Name
	if subject == "" {
		subject = "request body"
	}
	if hint := transportHint(e.Err); hint != "" {
		return fmt.Sprintf("please provide a valid %s (%s)", subject, hint)
	}
	return fmt.Sprintf("please provide a valid %s", subject)
}

func (e *TransportError) StatusHTTP() int { return 400 }

func (e *TransportError) Unwrap() error { return e.Err }

// transportHint describes what a valid value looks like for the decoding
// failures runtime.link recognises, else "". Every hint here is fixed text or
// is drawn from the wire representation (JSON field names and kinds), never
// from the failure's own message or the Go types involved, as that is what
// makes a [TransportError] safe to show the caller. Add to it with that in mind.
func transportHint(err error) string {
	var (
		timeErr   *time.ParseError
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		numErr    *strconv.NumError
	)
	switch {
	case errors.As(err, &timeErr):
		switch timeErr.Layout {
		case time.RFC3339, time.RFC3339Nano, `"` + time.RFC3339 + `"`:
			return "times must be RFC 3339 with a zone offset, such as 2006-01-02T15:04:05Z"
		}
		return "times must be formatted like " + timeErr.Layout
	case errors.As(err, &syntaxErr):
		return "the body is not valid JSON"
	case errors.As(err, &typeErr):
		if typeErr.Field != "" {
			return fmt.Sprintf("%s cannot be a JSON %s", typeErr.Field, typeErr.Value)
		}
		return fmt.Sprintf("cannot be a JSON %s", typeErr.Value)
	case errors.As(err, &numErr):
		return "must be a number"
	}
	return ""
}

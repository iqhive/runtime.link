// Package http provides an extendable shell API based on http.
package http

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"runtime.link/api/xray"
)

var (
	ErrNotImplemented = errorString("not implemented")
	ErrNotFound       = errorString("not found")
)

type errorString string

func (e errorString) Error() string { return string(e) }

type Method string

type Header = http.Header

type HeaderWriter interface {
	WriteHeadersHTTP(http.Header)
}

type HeaderReader interface {
	ReadHeadersHTTP(http.Header)
}

// Error that can be returned to HTTP clients.
type Error interface {
	error

	WithStatus
}

type WithStatus interface {
	//StatusHTTP should return the HTTP status code
	//relating to this error.
	StatusHTTP() int
}

type responseError struct {
	Internal error

	Code    int
	Subject string
	Message string
}

func (e *responseError) StatusHTTP() int {
	if e.Code == 0 {
		e.Code = 500
	}
	return e.Code
}

func (e *responseError) Error() string {
	if e.Message == "" {
		return http.StatusText(e.Code)
	}
	return e.Message
}

func (e *responseError) Unwrap() error {
	return e.Internal
}

// ResponseError converts a http.Response into an error.
func ResponseError(resp *http.Response) error {
	var subject string

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		//ok

	//400s
	case http.StatusBadRequest:
		subject = "request"
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusProxyAuthRequired,
		http.StatusNetworkAuthenticationRequired:
		subject = "access denied"
	case http.StatusPaymentRequired:
		subject = "wallet"
	case http.StatusNotFound, http.StatusGone:
		subject = "missing"
	case http.StatusMethodNotAllowed:
		subject = "method"
	case http.StatusNotAcceptable:
		subject = "user-agent"
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		subject = "timeout"
	case http.StatusConflict:
		subject = "conflict"
	case http.StatusLengthRequired:
		return &responseError{
			Code:    resp.StatusCode,
			Subject: "content-length",
			Message: "please provide a Content-Length header",
		}
	case http.StatusPreconditionFailed, http.StatusPreconditionRequired:
		subject = "precondition"
	case http.StatusRequestEntityTooLarge:
		return &responseError{
			Code:    resp.StatusCode,
			Subject: "content-length",
			Message: "please provide a smaller payload",
		}
	case http.StatusRequestURITooLong:
		return &responseError{
			Code:    resp.StatusCode,
			Subject: "uri",
			Message: "please provide a smaller uri",
		}
	case http.StatusUnsupportedMediaType:
		subject = "mediatype"
	case http.StatusRequestedRangeNotSatisfiable:
		subject = "range"
	case http.StatusExpectationFailed:
		subject = "expectation"
	case http.StatusTeapot:
		subject = "teapot"
	case http.StatusMisdirectedRequest:
		subject = "misdirection"
	case http.StatusUnprocessableEntity:
		subject = "unprocessable"
	case http.StatusLocked:
		subject = "lock"
	case http.StatusFailedDependency:
		subject = "dependency"
	case http.StatusTooEarly:
		subject = "early"
	case http.StatusUpgradeRequired:
		subject = "upgrade"
	case http.StatusTooManyRequests:
		return &responseError{
			Code:    resp.StatusCode,
			Subject: "ratelimit",
			Message: "please slow down",
		}
	case http.StatusRequestHeaderFieldsTooLarge:
		return &responseError{
			Code:    resp.StatusCode,
			Subject: "header",
			Message: "please reduce the size of your header fields",
		}
	case http.StatusUnavailableForLegalReasons:
		subject = "legal"

		//500s
	case http.StatusInternalServerError:
		subject = ""

	case http.StatusNotImplemented:
		subject = "todo"

	case http.StatusBadGateway:
		subject = "gateway"

	case http.StatusServiceUnavailable:
		subject = "unavailable"

	case http.StatusHTTPVersionNotSupported, http.StatusVariantAlsoNegotiates:
		subject = "http"
	case http.StatusInsufficientStorage:
		subject = "storage"
	case http.StatusLoopDetected:
		subject = "infinite loop"
	case http.StatusNotExtended:
		subject = "request"

	default:
		subject = "unexpected"
	}

	if subject != "" {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return xray.New(errors.New("unexpected status (failed read): " + resp.Status))
		}
		message := strings.TrimSpace(string(b))
		return &responseError{
			Internal: errors.New(message),
			Code:     resp.StatusCode,
			Subject:  subject,
			Message:  message,
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return xray.New(errors.New("unexpected status (failed read): " + resp.Status))

		}
		return xray.New(errors.New("unexpected status : " + resp.Status + " " + string(b)))
	}

	return nil
}

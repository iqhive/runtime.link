// Package http provides an extendable shell API based on http.
package http

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iqhive/runtime.link/api/xray"
)

var (
	ErrNotImplemented = &responseError{Code: 501, Message: "not implemented"}
	ErrNotFound       = &responseError{Code: 404, Message: "not found"}
)

type Method string

type Header = http.Header

// HeaderWriter and HeaderReader carry arbitrary headers on values and
// errors. A header only earns a dedicated interface of its own (such as
// [WithRetryAfter]) when this package does something with it beyond
// passing it through: normalising a non-trivial wire format, documenting
// it in OpenAPI, or changing behaviour. Everything else belongs here.
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

// WithRetryAfter is implemented by errors that tell the client how long
// to wait before retrying, typically on 429 or 503 (and sometimes 413).
// A non-positive duration means the Retry-After header should be omitted.
type WithRetryAfter interface {
	RetryAfterHTTP() time.Duration
}

type responseError struct {
	Internal error

	Code       int
	Subject    string
	Message    string
	RetryAfter time.Duration
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

func (e *responseError) RetryAfterHTTP() time.Duration {
	return e.RetryAfter
}

func (e *responseError) WriteHeadersHTTP(h http.Header) {
	if v := FormatRetryAfter(e.RetryAfter); v != "" {
		h.Set("Retry-After", v)
	}
}

func (e *responseError) ReadHeadersHTTP(h http.Header) {
	e.RetryAfter = ParseRetryAfter(h.Get("Retry-After"))
}

// FormatRetryAfter returns a Retry-After header value as delay-seconds,
// or "" if d is not positive.
func FormatRetryAfter(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	secs := d / time.Second
	if d%time.Second != 0 {
		secs++
	}
	return strconv.FormatInt(int64(secs), 10)
}

// ParseRetryAfter parses a Retry-After header (delay-seconds or HTTP-date).
// Invalid or missing values, and HTTP-dates in the past, return 0.
func ParseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if secs, err := strconv.ParseUint(value, 10, 64); err == nil {
		const maxSecs = uint64(time.Duration(^uint64(0)>>1) / time.Second)
		if secs > maxSecs {
			return time.Duration(^uint64(0) >> 1)
		}
		return time.Duration(secs) * time.Second
	}
	t, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func fallbackMessage(code int) string {
	switch code {
	case http.StatusTooManyRequests:
		return "please slow down"
	case http.StatusRequestEntityTooLarge:
		return "please provide a smaller payload"
	case http.StatusRequestURITooLong:
		return "please provide a smaller uri"
	case http.StatusRequestHeaderFieldsTooLarge:
		return "please reduce the size of your header fields"
	default:
		return ""
	}
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
		subject = "content-length"
	case http.StatusRequestURITooLong:
		subject = "uri"
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
		subject = "ratelimit"
	case http.StatusRequestHeaderFieldsTooLarge:
		subject = "header"
	case http.StatusUnavailableForLegalReasons:
		subject = "legal"

		//500s
	case http.StatusInternalServerError:
		subject = "internal"

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

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}

	return responseFrom(resp, subject)
}

func responseFrom(resp *http.Response, subject string) error {
	var (
		message  string
		internal error
	)
	if resp.Body != nil {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			internal = xray.New(errors.New("unexpected status (failed read): " + resp.Status))
		} else {
			message = strings.TrimSpace(string(b))
		}
	}
	if message == "" {
		message = fallbackMessage(resp.StatusCode)
	}
	if internal == nil {
		internal = errors.New(message)
	} else if message == "" {
		message = "unexpected status (failed read): " + resp.Status
	}
	response := &responseError{
		Internal: internal,
		Code:     resp.StatusCode,
		Subject:  subject,
		Message:  message,
	}
	response.ReadHeadersHTTP(resp.Header)
	return response
}

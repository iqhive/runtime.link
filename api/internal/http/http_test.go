package http_test

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	apihttp "github.com/iqhive/runtime.link/api/internal/http"
)

// failingBody stands in for a response whose body cannot be read, so the
// read-failure branch of ResponseError can be exercised.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset") }
func (failingBody) Close() error             { return nil }

func response(code int, body string, header http.Header) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     strings.TrimSpace(http.StatusText(code)),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestResponseErrorFallbacks checks that the statuses carrying a canned
// message use it only when the payload is empty, and that every decoded
// error keeps a recoverable status.
func TestResponseErrorFallbacks(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
	}{
		{name: "429 empty", code: http.StatusTooManyRequests, body: "", want: "please slow down"},
		{name: "429 whitespace", code: http.StatusTooManyRequests, body: "  \n\t", want: "please slow down"},
		{name: "429 payload", code: http.StatusTooManyRequests, body: "rate limit exceeded, try again later", want: "rate limit exceeded, try again later"},
		{name: "429 padded payload", code: http.StatusTooManyRequests, body: "  too many logins  \n", want: "too many logins"},
		{name: "413 empty", code: http.StatusRequestEntityTooLarge, body: "", want: "please provide a smaller payload"},
		{name: "413 whitespace", code: http.StatusRequestEntityTooLarge, body: " \n", want: "please provide a smaller payload"},
		{name: "413 payload", code: http.StatusRequestEntityTooLarge, body: "max 1MB", want: "max 1MB"},
		{name: "414 empty", code: http.StatusRequestURITooLong, body: "", want: "please provide a smaller uri"},
		{name: "414 whitespace", code: http.StatusRequestURITooLong, body: "   ", want: "please provide a smaller uri"},
		{name: "414 payload", code: http.StatusRequestURITooLong, body: "uri exceeds 8KiB", want: "uri exceeds 8KiB"},
		{name: "431 empty", code: http.StatusRequestHeaderFieldsTooLarge, body: "", want: "please reduce the size of your header fields"},
		{name: "431 whitespace", code: http.StatusRequestHeaderFieldsTooLarge, body: "\t\n", want: "please reduce the size of your header fields"},
		{name: "431 payload", code: http.StatusRequestHeaderFieldsTooLarge, body: "cookie too large", want: "cookie too large"},
		{name: "500 empty", code: http.StatusInternalServerError, body: "", want: "Internal Server Error"},
		{name: "500 payload", code: http.StatusInternalServerError, body: "database down", want: "database down"},
		{name: "500 padded payload", code: http.StatusInternalServerError, body: "  boom  \n", want: "boom"},
		{name: "503 empty", code: http.StatusServiceUnavailable, body: "", want: "Service Unavailable"},
		{name: "400 payload", code: http.StatusBadRequest, body: "bad field", want: "bad field"},
		{name: "unknown status", code: 599, body: "who knows", want: "who knows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apihttp.ResponseError(response(tt.code, tt.body, nil))
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
			var status apihttp.WithStatus
			if !errors.As(err, &status) {
				t.Fatalf("StatusHTTP() not recoverable from %T", err)
			}
			if got := status.StatusHTTP(); got != tt.code {
				t.Errorf("StatusHTTP() = %d, want %d", got, tt.code)
			}
		})
	}
}

// TestResponseErrorLengthRequired documents that 411 keeps its canned
// message even when the server sends a payload, since the message is about
// the shape of the request rather than anything the server can explain.
func TestResponseErrorLengthRequired(t *testing.T) {
	err := apihttp.ResponseError(response(http.StatusLengthRequired, "please set Content-Length", nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "please provide a Content-Length header"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	var status apihttp.WithStatus
	if !errors.As(err, &status) {
		t.Fatalf("StatusHTTP() not recoverable from %T", err)
	}
	if got := status.StatusHTTP(); got != http.StatusLengthRequired {
		t.Errorf("StatusHTTP() = %d, want %d", got, http.StatusLengthRequired)
	}
}

func TestResponseErrorSuccessStatus(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent, 299} {
		if err := apihttp.ResponseError(response(code, "", nil)); err != nil {
			t.Errorf("ResponseError(%d) = %v, want nil", code, err)
		}
	}
}

func TestResponseErrorNilBody(t *testing.T) {
	err := apihttp.ResponseError(&http.Response{
		StatusCode: http.StatusTooManyRequests,
		Status:     "429 Too Many Requests",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "please slow down"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestResponseErrorUnreadableBody checks that a body that fails to read
// still yields a status-bearing error: the canned message when the status
// has one, and the raw status otherwise.
func TestResponseErrorUnreadableBody(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "with fallback", code: http.StatusTooManyRequests, want: "please slow down"},
		{name: "without fallback", code: http.StatusInternalServerError, want: "unexpected status (failed read): Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apihttp.ResponseError(&http.Response{
				StatusCode: tt.code,
				Status:     http.StatusText(tt.code),
				Body:       failingBody{},
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
			var status apihttp.WithStatus
			if !errors.As(err, &status) {
				t.Fatalf("StatusHTTP() not recoverable from %T", err)
			}
			if got := status.StatusHTTP(); got != tt.code {
				t.Errorf("StatusHTTP() = %d, want %d", got, tt.code)
			}
		})
	}
}

// TestResponseErrorRetryAfter checks that the decoder lifts Retry-After off
// the response onto the error, for every status that may carry one.
func TestResponseErrorRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		header string
		want   time.Duration
	}{
		{name: "429", code: http.StatusTooManyRequests, header: "120", want: 120 * time.Second},
		{name: "503", code: http.StatusServiceUnavailable, header: "30", want: 30 * time.Second},
		{name: "413", code: http.StatusRequestEntityTooLarge, header: "5", want: 5 * time.Second},
		{name: "absent", code: http.StatusTooManyRequests, header: "", want: 0},
		{name: "invalid", code: http.StatusTooManyRequests, header: "soon", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := make(http.Header)
			if tt.header != "" {
				header.Set("Retry-After", tt.header)
			}
			err := apihttp.ResponseError(response(tt.code, "", header))
			var retry apihttp.WithRetryAfter
			if !errors.As(err, &retry) {
				t.Fatalf("WithRetryAfter not implemented by %T", err)
			}
			if got := retry.RetryAfterHTTP(); got != tt.want {
				t.Errorf("RetryAfterHTTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestResponseErrorRetryAfterRoundTrip checks that a decoded error can put
// the delay back on the wire, so a proxy can forward it unchanged.
func TestResponseErrorRetryAfterRoundTrip(t *testing.T) {
	header := make(http.Header)
	header.Set("Retry-After", "90")
	err := apihttp.ResponseError(response(http.StatusTooManyRequests, "", header))

	var writer apihttp.HeaderWriter
	if !errors.As(err, &writer) {
		t.Fatalf("HeaderWriter not implemented by %T", err)
	}
	out := make(http.Header)
	writer.WriteHeadersHTTP(out)
	if got := out.Get("Retry-After"); got != "90" {
		t.Errorf("Retry-After = %q, want 90", got)
	}

	// Without a delay the header is omitted rather than written as zero.
	err = apihttp.ResponseError(response(http.StatusTooManyRequests, "", nil))
	if !errors.As(err, &writer) {
		t.Fatalf("HeaderWriter not implemented by %T", err)
	}
	out = make(http.Header)
	writer.WriteHeadersHTTP(out)
	if _, ok := out["Retry-After"]; ok {
		t.Errorf("Retry-After written for a zero delay: %q", out.Get("Retry-After"))
	}
}

// TestResponseErrorReadHeaders checks the HeaderReader side directly: a
// decoded error re-reads Retry-After from whatever headers it is given.
func TestResponseErrorReadHeaders(t *testing.T) {
	err := apihttp.ResponseError(response(http.StatusTooManyRequests, "", nil))
	var reader apihttp.HeaderReader
	if !errors.As(err, &reader) {
		t.Fatalf("HeaderReader not implemented by %T", err)
	}
	header := make(http.Header)
	header.Set("Retry-After", "45")
	reader.ReadHeadersHTTP(header)

	var retry apihttp.WithRetryAfter
	if !errors.As(err, &retry) {
		t.Fatalf("WithRetryAfter not implemented by %T", err)
	}
	if got := retry.RetryAfterHTTP(); got != 45*time.Second {
		t.Errorf("RetryAfterHTTP() = %v, want 45s", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "seconds", value: "120", want: 120 * time.Second},
		{name: "zero", value: "0", want: 0},
		{name: "padded", value: "  15  ", want: 15 * time.Second},
		{name: "empty", value: "", want: 0},
		{name: "invalid", value: "n/a", want: 0},
		{name: "negative", value: "-5", want: 0},
		{name: "fractional", value: "1.5", want: 0},
		{name: "overflow", value: "10000000000", want: time.Duration(math.MaxInt64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apihttp.ParseRetryAfter(tt.value); got != tt.want {
				t.Errorf("ParseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}

	past := time.Now().UTC().Add(-time.Hour).Format(http.TimeFormat)
	if got := apihttp.ParseRetryAfter(past); got != 0 {
		t.Errorf("ParseRetryAfter(past date) = %v, want 0", got)
	}
	future := time.Now().UTC().Add(2 * time.Hour).Format(http.TimeFormat)
	if got := apihttp.ParseRetryAfter(future); got < time.Hour || got > 3*time.Hour {
		t.Errorf("ParseRetryAfter(future date) = %v, want ~2h", got)
	}
}

func TestFormatRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "zero", in: 0, want: ""},
		{name: "negative", in: -time.Second, want: ""},
		{name: "whole seconds", in: 120 * time.Second, want: "120"},
		{name: "rounds up", in: 1500 * time.Millisecond, want: "2"},
		{name: "sub-second rounds up", in: time.Millisecond, want: "1"},
		{name: "minutes", in: 2 * time.Minute, want: "120"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apihttp.FormatRetryAfter(tt.in); got != tt.want {
				t.Errorf("FormatRetryAfter(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRetryAfterRoundTrip checks that formatting and parsing agree, so a
// delay survives a hop through the header.
func TestRetryAfterRoundTrip(t *testing.T) {
	for _, d := range []time.Duration{time.Second, 30 * time.Second, 2 * time.Minute, time.Hour} {
		if got := apihttp.ParseRetryAfter(apihttp.FormatRetryAfter(d)); got != d {
			t.Errorf("round trip of %v = %v", d, got)
		}
	}
}

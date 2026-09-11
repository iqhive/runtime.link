package http_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	apihttp "github.com/iqhive/runtime.link/api/internal/http"
)

func TestResponseErrorTooManyRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty", body: "", want: "please slow down"},
		{name: "whitespace", body: "  \n\t", want: "please slow down"},
		{name: "payload", body: "rate limit exceeded, try again later", want: "rate limit exceeded, try again later"},
		{name: "padded payload", body: "  too many logins  \n", want: "too many logins"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Status:     "429 Too Many Requests",
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			err := apihttp.ResponseError(resp)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
			status, ok := err.(apihttp.WithStatus)
			if !ok {
				t.Fatalf("error %T does not implement WithStatus", err)
			}
			if got := status.StatusHTTP(); got != http.StatusTooManyRequests {
				t.Errorf("StatusHTTP() = %d, want %d", got, http.StatusTooManyRequests)
			}
		})
	}
}

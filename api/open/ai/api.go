package ai

import (
	"net/http"

	http_internal "runtime.link/api/internal/http"
	"runtime.link/xyz"
)

type Error struct {
	Code ErrorCode `json:"code"`
}

func (e Error) Error() string {
	return e.Code.String()
}

type ErrorCode xyz.Switch[string, struct {
	ContextLengthExceeded ErrorCode `json:"context_length_exceeded"`
	ModelNotFound         ErrorCode `json:"model_not_found"`
}]

var ErrorCodes = xyz.AccessorFor(ErrorCode.Values)

// Client authentication.
func Client(key string) *http.Client {
	return http_internal.ClientWithHeader("Authorization", "Bearer "+key)
}

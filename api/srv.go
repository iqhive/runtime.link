package api

import (
	"net/http"

	_ "embed"
)

func ListenAndServe(addr string, auth Auth[*http.Request], implementation any) error {
	return ErrNotImplemented
}

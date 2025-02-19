package cors

import (
	"net/http"

	"runtime.link/api"
)

type AccessControl struct {
	AllowOrigin      string
	ExposeHeaders    string
	MaxAge           int
	AllowCredentials bool
	AllowMethods     string
	AllowHeaders     string
}

type Authenticator interface {
	api.Auth[*http.Request]

	CrossOriginResourceSharing(*http.Request, api.Function) AccessControl
}

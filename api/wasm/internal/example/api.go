package example

import (
	"embed"
	"io/fs"

	"github.com/iqhive/runtime.link/api"
)

//go:embed *.go
var source embed.FS

type API struct {
	api.Specification

	HelloWorld func()

	HostArch func() string

	Add func(a, b int) int
}

func (API) Source() fs.FS { return source }

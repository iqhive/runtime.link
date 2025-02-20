package example

import "runtime.link/api"

type API struct {
	api.Specification

	HelloWorld func()

	HostArch func() string
}

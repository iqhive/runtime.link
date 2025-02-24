package example

import "runtime.link/api"

type API struct {
	api.Specification

	HelloWorld func()

	HostArch func() string

	Add func(a, b int) int

	// AddWithCallback demonstrates function parameter support
	AddWithCallback func(a int, callback func(int) int) int

	// GetFormatter demonstrates function return value support
	GetFormatter func() func(string) string
}

package std

import (
	"errors"
	"sync"
)

var links []func(Structure)
var hosts []func(Structure)
var mutex sync.Mutex

// RegisterHost registers a function to be called when Main is called.
func RegisterHost(fn func(Structure)) {
	mutex.Lock()
	defer mutex.Unlock()
	hosts = append(hosts, fn)
}

// RegisterLinker registers a linker function to be called when Link is called.
func RegisterLinker(tag string, fn func(Structure)) {
	mutex.Lock()
	defer mutex.Unlock()
	links = append(links, fn)
}

// Link any unimplemented APIs, commands and libraries within
// the given structure. Returns an error if any functions
// failed to link.
func Link(structure any) error {
	// TODO, need to walk through the structure and only import
	// things if they are not implemented.
	return errors.New("not implemented")
}

// Main is a convienience function that can be used to expose default api, cmd, and lib
// implementations for a given runtime.link structure. It will also look for supported
// environment variables to determine which bindings to generate. For larger projects,
// initialise the runtime.link structure using the desired link layer.
func Main(functions any) {
	var (
		local     []func(Structure)
		structure = StructureOf(functions)
	)
	func() {
		mutex.Lock()
		defer mutex.Unlock()
		local = make([]func(Structure), len(hosts))
		copy(local, hosts)
	}()
	for _, fn := range local {
		fn(structure)
	}
}

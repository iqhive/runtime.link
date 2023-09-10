// Package sdk provides tooling for generating bindings for runtime.link structures.
package sdk

import (
	"os"

	"runtime.link/api"
	"runtime.link/cmd"
	"runtime.link/lib"
)

// Link any unimplemented APIs, commands and libraries within
// the given structure. Returns an error if any of the APIs,
// commands or libraries could not be linked.
func Link(structure any) error {
	// TODO, need to walk through the structure and only import
	// things if they are not implemented.
	return nil
}

// Main is a convienience function that can be used to expose default api, cmd, and lib
// implementations for a given runtime.link structure. It will also look for supported
// environment variables to determine which bindings to generate.
func Main(functions any) {
	if len(os.Args) > 1 {
		cmd.Main(functions)
	}
	if dir := os.Getenv("SDK_LIB"); dir != "" {
		lib.Make(dir, functions)
	}
	if port := os.Getenv("PORT"); port != "" {
		api.ListenAndServe(":"+port, nil, functions)
	}
}

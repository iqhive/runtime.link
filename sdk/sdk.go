// Package sdk provides tooling for generating bindings for runtime.link structures.
package sdk

import (
	"os"

	"runtime.link/api"
	"runtime.link/cmd"
	"runtime.link/lib"
)

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

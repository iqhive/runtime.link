# runtime.link &nbsp;[![Go Reference](https://pkg.go.dev/badge/runtime.link.svg)](https://pkg.go.dev/runtime.link)

The runtime.link project provides a dictionary for representing software interfaces
via Go source. It also provides several Go linkers that enable you to link to
these interfaces at runtime. They can be connected via network protocols (ie. HTTP),
through command line interfaces, or through supported platform-native ABIs.

The project is still in development and although we don't plan to make any major changes
to established components before the first stable release, there may be continue to be
minor breaking changes here and there as we work to refine the exported interfaces.

Example:
```go
// Package example provides the specification for the runtime.link example API.
package example

import (
	"log"
	"os"

	"runtime.link/api"
)

// API specification structure, typically named API for general structures, may
// be more suitably named Functions, Library or Command when the API is
// restricted to a specific runtime.link layer. Any Go comments in the source
// are intended to document design notes and ideas. This leaves Go struct tags
// for recording developer-facing documentation.
type API struct {
	api.Specification `api:"Example" cmd:"example" lib:"libexample"
		is an example of a runtime.link API structure.` // this section of the tag contains documentation.

	// HelloWorld includes runtime.link tags that specify how the function is called
	// across different link-layers. Typically, a context.Context argument and error
	// return value should be included here, they are omitted here for brevity.
	HelloWorld func() string `cmdl:"hello_world" link:"example_helloworld func()$char" rest:"GET /hello_world"
		returns the string "Hello World"` // documentation for the function.
}

// New returns an implementation of the API. This doesn't have to be defined in the
// same package and may not even be implemented in Go. This will often be the case when
// representing an external API controlled by a third-party.
func New() API {
	return API{
		HelloWorld: func() string {
			return "Hello World"
		},
	}
}
```

## More Practical Examples

* [Quickly use REST API endpoints in Go without the need for a Go 'client library'](api/example/Link.md)

## Runtime Linkers.
Each linker lives under the `api` package and enables an API to be linked against a host
implementation via a standard communication protocol. A linker can also serve a host
implementation written in Go.

Currently available runtime.linkers include:

    * cmdl - parse command line arguments or execute command line programs.
    * link - generate c-shared export directives or dynamicaly link to shared libraries (via ABI).
    * rest - link to, or host a REST API server over the network.
    * stub - create a stub implementation of an API, that returns empty values or errors.
    * xray - debug linkers with API call introspection.

## Standard Interfaces and Open Source Software

The runtime.link project includes a selection of builtin representations for well-known software
standards and interfaces. These are intended to act as a reference on how the package can
be utilised and also as readily available interfaces that can be imported into your Go
projects. We aim to keep a consistent level of quality for these packages.

Common command line programs and shared libraries that are readily available on many
systems can be discovered under the 'com', 'std' and 'oss' subdirectories under each
link layer.

## Proprietary Software Interfaces

If you would like to include a runtime.link API structure for proprietary software (so that
it can be made available to all runtime.link users), we can help create this representation
for you, please contact us for a quote. Our only requirement is that any resulting runtime.link
API structure packages must be released under the same license as runtime.link (BSD0).

## Our Design Values

1. Full readable words for exported identifiers rather than abbreviations ie. `PutString` over `puts`.
2. Acronyms as package names and/or as a suffix, rather than mixed use ie. `TheExampleAPI` over `TheAPIExample`.
3. Explicitly tagged types that define data relationships rather than implicit use of primitives. `Customer CustomerID` over `Customer string`.
4. Don't stutter exported identifiers. `customer.Account` over `customer.Customer`.

## Contribution Guidance

This project is open for contributions that help update or define clear, compatible
runtime.link structures for software standards and interfaces. We will consider pull
requests and/or ideas for additional interfaces and/or standards that have well-known
and widely available implementations under an Open Source Initiative approved license.

Apart from what's on the Roadmap, we cannot accept any pull requests for new top level
packages at this time, although you are welcome to start a GitHub Discussion for any
ideas you may have, our current goal for runtime.link is to stick to a well-defined
and cohesive design space.

runtime.link aims to be dependency free, we will not accept any pull requests that add
any additional Go dependencies to the project.

**NOTE**: we adopt a different convention for Go struct tags, which are permitted to be
multi-line and include inline-documentation on subsequent lines of the tag. This can
raise a warning with Go linters, so we recommend using the following configurations:

govet
`go vet -structtag=false ./...`

VS Code + gopls
```
"go.vetFlags": [
    "-structtag=false"
],
"gopls": {
    "analyses": {
        "structtag": false
    },
},
```

golangci-lint.yml
```
linters-settings:
  govet:
    disable:
      - structtag # support runtime.link convention.
```

## Roadmap

* Support for additional linkers, such as `mock`, `grpc`, `soap`, `jrpc`, `xrpc`, and `sock`.

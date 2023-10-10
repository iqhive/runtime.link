# runtime.link &nbsp;[![Go Reference](https://pkg.go.dev/badge/runtime.link.svg)](https://pkg.go.dev/runtime.link)

The runtime.link project provides a dictionary for representing software interfaces 
via Go source. It also provides builtin Go linkers that enable you to link to 
these interfaces at runtime. They can be connected via network protocols (ie. HTTP), 
through command line interfaces, or through a supported platform-native ABI.

This repo also serves as a home for Go source representations written using the 
runtime.link dictionary. Our primary aim is to promote the practise of representing
software interfaces and specifications as source code, rather than a PDF document.

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
	HelloWorld func() string `args:"hello_world" link:"example_helloworld func()$char" rest:"GET /hello_world"
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

* [Quickly use REST API endpoints in Go without the need for a Go 'client library'](api/rest/example/Link.md)

## Runtime Linkers.
Each linker lives under the `api` package and enables an API to be linked against a host
implementation via a standard communication protocol. A linker can also serve a host 
implementation written in Go.

Currently available runtime.linkers include:
   
    * args - parse command line arguments or execute command line programs.
    * link - generate c-shared export directives or dynamicaly link to shared libraries (via ABI).
    * rest - link to, or host a REST API server over the network.
    * stub - create a stub implementation of an API, that returns empty values or errors.
    * xray - debug linkers with API call introspection.

## Data Dictionary
In addition to the link layers the runtime.link project defines additional packages to
help represent well-defined, variable data types, strings and structures. These are:

    * api - provides reflection and functions for working with runtime.link API structures.
    * qty - quantity types, representing specific units of measure.
    * ref - create reciever functions for foreign keys, pointer-like values for API values.
    * txt - text tags, syntax structures, human readable tag for textual field names.
    * xyz - sequence tags, switch types (enums/unions/variants), tuples and optional values.

## Resource Dictionary
Most software requires access to external resources, so runtime.link provides a few packages
to help clearly represent these resources. These are:

    * eon - represent asyncronous scheduling, sleep periods and timers.
    * kvs - represent key-value stores.
    * pub - represent asyncronous fan out message queues with Pub/Sub semantics.
    * sql - represent SQL database maps and construct type-safe queries.

## Runtime Optimisation
The runtime.link project provides packages useful for optimising Go functions with runtime 
specified behaviour. These packages are still in an exploratory state:

    * bin - binary bit-level encoding representations (including CPU binary formats).
    * jit - compile safe yet dynamic functions at runtime.

## Standard Interfaces and Open Source Software

The runtime.link project includes a selection of builtin representations for well-known software
standards and interfaces. These are intended to act as a reference on how the package can 
be utilised and also as readily available interfaces that can be imported into your Go
projects. We aim to keep a consistent level of quality for these packages. Currently
we are aiming to include a useful representation of:

* C
* POSIX
* OpenGL
* GNU
* SODIUM

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
raise a warning with Go linters, so we recommend using the following configuration:

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

## Roadmap

* Support for additional linkers, such as `mock`, `grpc`, `soap`, `jrpc`, `xrpc`, and `sock`.
# runtime.link

The runtime.link project defines a standard format for representing software interfaces 
using Go source code. It provides tools that enable you to build software that can link 
to these interfaces at runtime. They can be connected via network protocols (ie. HTTP), 
through command line interfaces, or through a supported platform-native ABI.

As a side-effect to how these interfaces are defined, Go software has first-class support
to link to these interfaces directly. Any required functions can be defined using the 
runtime.link conventions and conveniently imported into the Go program for execution.

Example:
```go
// Package example provides the specification for the runtime.link example API.
package example

import "runtime.link/std"

// API specification structure, typically named API for general structures, may
// be more suitably named Functions, Library or Command when the API is 
// restricted to a specific runtime.link layer. Any Go comments in the source
// are intended to document design notes and ideas. This leaves Go struct tags 
// for recording developer-facing documentation.
type API struct {
    std.Documentation `cmd:"example"
        Example API is an example of a runtime.link API structure.` // this tag contains the API's introductory documentation.

    // HelloWorld includes runtime.link tags that specify how the function is called 
    // across different link-layers. Typically, a context.Context argument and error 
    // return value should be included here, they are omitted here for brevity.
    HelloWorld func() string `cmd:"hello_world" lib:"example_helloworld func()$char" rest:"GET /hello_world"
        returns the string "Hello World"` // documentation for the function.
}

// New returns an implementation of the API. This doesn't have to be defined in the
// same package and may not even be implemented in Go. This will often be the case when 
// representing an external API controlled by a third-party.
func New() API {
    return API{
        HelloWorld: func() string{
            return "Hello World"
        },
    }
}
```

This example API implementation can be boostrapped on all runtime.link layers.

```go
package main

import "./example"
import "runtime.link/sdk"

func main() {
    sdk.Main(example.New())
}
```

This will start a server listening on PORT if it is specified, it will generate a 
c-shared package in 'dir' and then exit when SDK_LIB=dir, otherwise by 
default it will present the API's command line interface if there are any arguments.

## More Practical Examples

* [Quickly use REST API endpoints in Go without the need for a Go 'client library'](api/internal/rest/Link.md)

## Link Layers.
Each layer enables the API to be linked against using a different communication protocol. The 
runtime.link project also provides a builtin Go package for each link level that can be used as 
the linker for that particular link layer. Each linker can act either as an implementation host
or as the client that connects to a remote implementation.

The three available runtime.link layers are:

    * api - the API represents a network interface with a selection of endpoints ie. a REST API.
    * cmd - parse command line arguments or execute command line programs.
    * lib - generate c-shared export directives or dynamicaly link to shared libraries (abi layer).

## Data structures
In addition to standard Go types, the runtime.link project defines an additional package
for representing standard runtime.link structures and types that cross link layer boundaries.

    * std - reflection, variants (enums/unions) and safe pointer-types.

## Included Software Standards and Interfaces

The runtime.link project includes a selection of builtin representations for well-known software
standards and interfaces. These are intended to act as a reference on how the package can 
be utilised and also as readily available interfaces that can be imported into your Go
projects. We aim to keep a consistent level of quality for these packages. Currently
we are aiming to include the latest complete standards for:

* C
* POSIX
* OpenGL
* GNU

Common command line programs and shared libraries that are readily available on many
systems can be discovered in the packages under 'cmd' and 'lib'.

This project is open for contributions that help update or define clear, compilable 
runtime.link structures for these standards and interfaces. We will consider pull 
requests and/or ideas for additional interfaces and/or standards that have well-known 
and widely available implementations under an Open Source license.

If you would like to include runtime.link structure for a proprietary API (so that it 
is available to all runtime.link users), please contact us for a quote. We can help create 
the Go representation for you. The only strict requirement is that the interface itself 
(but not necessarily the implementation) must be included in the project under the same 
license as runtime.link.

## Our Design Values

1. Full readable words for exported identifiers rather than abbreviations ie. "PutString" over "puts".
2. Acronyms as package names and/or as a suffix, rather than mixed in ie. "TheExampleAPI" over "TheAPIExample".
3. Explicit types that define relationships rather than implicit use of primitives.

## Roadmap

1. Docs/Code generation for the runtime.link project will be provided by the `runtime.link/sdk` tooling
    
   

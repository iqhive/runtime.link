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

// API specification structure, typically named API for general APIs, may
// be more suitably named Functions, Library or Command when the API is 
// restricted to a specific runtime.link layer. Any Go comments in the source
// are intended to document design notes and ideas. This leaves Go struct tags 
// for recording developer-facing documentation.
type API struct {
    doc.Tag `
        Example API is an example of a runtime.link API structure` // this tag contains the API's introductory documentation.

    // HelloWorld includes runtime.link tags that specify how the function is called 
    // across different link-layers. Typically, a context.Context argument and error 
    // return value should be included here, they are omitted here for brevity.
    HelloWorld func() string `exe:"hello_world" abi:"example_helloworld func()$char" rpc:"GET /hello_world"
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

```
    package main

    import "./example"

    func main() {
        sdk.Main(example.New())
    }
```

This will start a server listening on PORT if it is specified, it will generate a 
c-shared package in 'dir' and then exit when LINK_LIB=dir, otherwise by 
default it will present the API's command line interface.

## Link Layers.
Each layer enables the API to be linked against using a different communication protocol.

The four available runtime.link layers are:

    * abi - the API represents a shared library that can be called using the platform-native ABI.
    * exe - the API represents program that is executed in scripts and/or over the command line.
    * rpc - the API represents a network interface with a selection of endpoints ie. a REST API.


## Builtin Linkers 
The runtime.link project also provides a selection of builtin Go packages that can be used as 
the linker for a particular link layer. Each linker can act either as an implementation host
or as the client that connects to a remote implementation.

    * api - serve HTTP endpoints, or connect as a client to HTTP endpoints (rpc layer).
    * cmd - parse command line arguments or execute command line programs (exe layer).
    * lib - generate c-shared export directives or dynamicaly link to shared libraries (abi layer).

## Data structures
In addition to standard Go types, the runtime.link project defines an additional package
for representing more complex data-structures that cross language boundaries.

    * ffi - pointer-types.
    * doc - documentation tags for API specifications and documented enums/unions

## Roadmap

1. Docs generation for the runtime.link project will be provided by the `runtime.link/doc` tooling
2. Code generation for the runtime.link project will be provided by the `runtime.link/sdk` tooling
    
   

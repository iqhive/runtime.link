# Quickly use REST API endpoints in Go without the need for a Go 'client library'

In this example, we'll show you how JSON-based REST APIs can be represented
using a Go runtime.link structure so that you can link your software to be
able to call the endpoints you need to use. For this example, we will be
representing the Swagger Petstore API.

## Scoping the Documentation

The first thing to do, is find the API documention for the API you need to
use. Often this will be available from the organisation who is hosting the
software implementation. In this case, the swagger for the Petstore API
can be found at https://petstore.swagger.io

Have a look through the API, get a quick feeling for it and then move onto
the first step, deciding where your representation is going to live. We 
recommend that you create a new package for each API that you want to
represent. This will help keep any APIs you are using appropriately
namespaced. In this case, we'll create a new package called `petstore`
and add an initial runtime.link API specification.

```go
package petstore

import "runtime.link/api"

// API specification, named this way, as it is the runtime.link convention.
// Typically this will be placed in a file called api.go and will be at the
// top of the file, so that it can act as a table of contents for the API.
type API struct {
	api.Specification `www:"https://petstore.swagger.io/v2" // default host name, can be overriden on import.
        is an example petstore API designed by swagger project.`
}
```

The endpoint we want to use in this example, is the `POST /pet` so that we
can add a new pet to the store. Take a look at the [endpoint](https://petstore.swagger.io/#/pet/addPet)
and try to imagine what the Go function signature would look like, if the
endpoint was a Go function. You will need to think about what Go types
can be used to best represent the endpoint's query parameters, request 
and response so that the call has a good developer experience for the 
user of the representation.

## Representing Endpoints

Here's an example of what this could look like, although keep in mind that
you can change the names of functions, types and struct fields to fit
your own design style. The naming conventions in this example follow the
runtime.link guidelines, as outlined in the readme. These are important 
to follow if you want your API to be included in the runtime.link project.

Note: Use https://mholt.github.io/json-to-go/ to quickly create struct types.

```go
package petstore

import (
	"runtime.link/api"
	"runtime.link/xyz"
)

// API specification, named this way, as it is the runtime.link convention.
// Typically this will be placed in a file called api.go and will be at the
// top of the file, so that it can act as a table of contents for the API.
type API struct {
	api.Specification `www:"https://petstore.swagger.io/v2" // default host name, can be overriden on import.
		is an example petstore API designed by swagger project.`

	// AddPet will POST its argument to the /pet endpoint.
	AddPet func(context.Context, Pet) (Pet, error) `rest:"POST /pet"
		adds a new pet to the store.`
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Pet struct {
	ID        int64              `json:"id,omitempty"`
	Name      string             `json:"name"`
	PhotoURLs []string           `json:"photoUrls"`
	Tags      xyz.Maybe[[]Tag]    `json:"tags,omitempty"` 	 // optional
	Category  xyz.Maybe[Category] `json:"category,omitempty"` // optional
}

type Status xyz.Switch[xyz.Enum, struct {
	Available Status `text:"available"`
	Pending   Status `text:"pending"`
	Sold      Status `text:"sold"`
}]

var StatusValues = xyz.AccessorFor(Status.Values)

```

## Importing the API

Now that we have represented the API and placed it inside of a package, 
it can be linked through the API runtime.link layer. This will allow
the REST endpoints of the API to be callable as if they were Go functions.

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"runtime.link/api"
	"runtime.link/api/rest"
	"runtime.link/api/rest/example/petstore" // use your own package import path here.
)

func main() {
	var (
		ctx = context.Background()
	)
	var API struct { // API dependencies for this program.
		petstore petstore.API
	}
	API.petstore = api.Import[petstore.API](rest.API, "", http.DefaultClient)
	pet, err := API.petstore.AddPet(ctx, petstore.Pet{
		Name: "Doggie",
		PhotoURLs: []string{
			"https://example.com/doggie.jpg",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(pet)
}
```
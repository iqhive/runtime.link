// Package petstore serves as an example for how to represent a REST API specification.
package petstore

import (
	"context"

	"runtime.link/api"
	"runtime.link/xyz"
)

// API specification, named this way, as it is the runtime.link convention.
// Typically this will be placed in a file called api.go and will be at the
// top of the file, so that it can act as a table of contents for the API.
type API struct {
	api.Documentation `www:"https://petstore.swagger.io/v2" // default host name, can be overriden on import.
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
	ID        int64               `json:"id,omitempty"`
	Name      string              `json:"name"`
	PhotoURLs []string            `json:"photoUrls"`
	Tags      xyz.Maybe[[]Tag]    `json:"tags,omitempty"`     // optional
	Category  xyz.Maybe[Category] `json:"category,omitempty"` // optional
}

type Status xyz.Switch[xyz.Enum, struct {
	Available Status `text:"available"`
	Pending   Status `text:"pending"`
	Sold      Status `text:"sold"`
}]

var StatusValues = xyz.AccessorFor(Status.Values)

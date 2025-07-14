// Package petstore serves as an example for how to represent a REST API specification.
package petstore

import (
	"context"
	"io/fs"

	"runtime.link/api"
	"runtime.link/api/stub"
	"runtime.link/xyz"
)

// API specification, named this way, as it is the runtime.link convention.
// Typically this will be placed in a file called api.go and will be at the
// top of the file, so that it can act as a table of contents for the API.
type API struct {
	api.Specification `api:"Petstore" www:"https://petstore.swagger.io/v2" // default host name, can be overriden on import.
        is an example petstore API designed by swagger project.`

	UploadImageForPet func(context.Context, PetID, string, fs.File) error `rest:"POST(multipart/form-data) /pet/{petId=%v}/uploadImage (additionalMetadata,file)"
		uploads an image.`
	AddPet func(context.Context, Pet) error `rest:"POST /pet"
        adds a new pet to the store.`
	SetPet func(context.Context, Pet) error `rest:"PUT /pet"
		update an existing pet.`
	FindByStatus func(context.Context, ...Status) ([]Pet, error) `rest:"GET /pet/findByStatus?status=%v"
		(multiple status values can be provided with comma separated strings)`
	FindByTags func(context.Context, ...string) ([]Pet, error) `rest:"GET /pet/findByTags?tags=%v" deprecated:"true"
		(multiple tags can be provided with comma separated strings. Use tag1, tag2, tag3 for testing)`
	GetPet func(context.Context, PetID) (Pet, error) `rest:"GET /pet/{petId=%v}"
		returns a single pet.`
	ModPet func(context.Context, Pet) error `rest:"POST(multipart/form-data) /pet/{petId}"
		updates a pet in the store with form data.`
	DeletePet func(context.Context, PetID) error `rest:"DELETE /pet/{petId=%v}"`
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PetID int64

type Pet struct {
	ID   PetID  `json:"id,omitempty" rest:"petId"`
	Name string `json:"name"
		of the pet.`
	PhotoURLs []string            `json:"photoUrls"`
	Tags      xyz.Maybe[[]Tag]    `json:"tags,omitempty"`     // optional
	Category  xyz.Maybe[Category] `json:"category,omitempty"` // optional
}

type Status xyz.Switch[string, struct {
	Available Status `json:"available"`
	Pending   Status `json:"pending"`
	Sold      Status `json:"sold"`
}]

var StatusValues = xyz.AccessorFor(Status.Values)

func (a API) Documentation() api.Documentation {
	return func(ctx context.Context) (api.Examples, error) {
		tracedAPI := api.Import[API](stub.API, stub.Testing, nil)
		return &ExampleFramework{
			API: tracedAPI,
		}, nil
	}
}

func (a API) Examples(ctx context.Context) (map[string][]string, error) {
	doc := a.Documentation()
	return doc.Examples(ctx)
}

func (a API) Example(ctx context.Context, name string) (api.Example, bool) {
	doc := a.Documentation()
	return doc.Example(ctx, name)
}

type ExampleFramework struct {
	api.TestingFramework
	API API
}

func (e *ExampleFramework) AddPetExample(ctx context.Context) error {
	e.Story("This example demonstrates adding a new pet to the store")
	e.Tests("Validates that pets can be successfully added with required fields")
	
	pet := Pet{
		Name: "Fluffy",
		PhotoURLs: []string{"https://example.com/fluffy.jpg"},
	}
	
	return e.API.AddPet(ctx, pet)
}

func (e *ExampleFramework) GetPetExample(ctx context.Context) error {
	e.Story("This example shows how to retrieve a pet by ID")
	e.Tests("Validates pet retrieval and error handling for non-existent pets")
	
	_, err := e.API.GetPet(ctx, PetID(1))
	return err
}

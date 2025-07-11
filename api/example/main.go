package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"runtime.link/api"
	"runtime.link/api/example/petstore" // use your own package import path here.
	"runtime.link/api/rest"
)

func main() {
	var (
		ctx = context.Background()
	)
	var API struct { // API dependencies for this program.
		petstore petstore.API
	}
	port := os.Getenv("PORT")
	if port == "" {
		API.petstore = api.Import[petstore.API](rest.API, "", http.DefaultClient)
		err := API.petstore.AddPet(ctx, petstore.Pet{
			Name: "Doggie",
			PhotoURLs: []string{
				"https://example.com/doggie.jpg",
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	impl := petstore.API{
		AddPet: func(ctx context.Context, pet petstore.Pet) error {
			log.Printf("Adding pet %q", pet.Name)
			return nil
		},
	}
	if err := rest.ListenAndServe(":"+port, nil, impl); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"runtime.link/api/internal/rest/example/petstore" // use your own package import path here.
	"runtime.link/qnq"
)

func main() {
	var (
		ctx = context.Background()
	)
	var API struct { // API dependencies for this program.
		petstore petstore.API
	}
	if err := qnq.Link(&API); err != nil {
		log.Fatal(err)
	}
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

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"runtime.link/api"
	"runtime.link/api/internal/rest/petstore"
)

func main() {
	var ctx = context.Background()
	var API = api.Import[struct {
		petstore petstore.API // enable reuse of the package name.
	}](api.REST, "", nil)
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

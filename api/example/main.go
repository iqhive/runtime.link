package main

import (
	"context"
	"encoding/json"
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
		return
	}
	api.ListenAndServe(":"+port, nil, petstore.API{})
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"runtime.link/api"
	"runtime.link/api/example/weather"
	"runtime.link/api/rest"
)

func main() {
	ctx := context.Background()
	
	weatherAPI := weather.New()
	
	port := os.Getenv("PORT")
	if port == "" {
		usingAPI(ctx)
		return
	}
	
	fmt.Printf("Starting weather API server on port %s...\n", port)
	if err := rest.ListenAndServe(":"+port, nil, weatherAPI); err != nil {
		log.Fatal(err)
	}
}

func usingAPI(ctx context.Context) {
	var API struct {
		Weather weather.API
	}
	
	API.Weather = api.Import[weather.API](rest.API, "", http.DefaultClient)
	
	station := weather.WeatherStation{
		Name: "Test Station",
		Location: weather.Location{
			City:    "San Francisco",
			Country: "USA",
			Lat:     37.7749,
			Long:    -122.4194,
		},
		Owner: "Jane Doe",
		Email: "jane@example.com",
	}
	
	stationID, err := API.Weather.AddWeatherStation(ctx, station)
	if err != nil {
		log.Fatalf("Failed to add weather station: %v", err)
	}
	fmt.Printf("Added station with ID: %s\n", stationID)
	
	currentWeather, err := API.Weather.GetCurrentWeather(ctx, station.Location)
	if err != nil {
		log.Fatalf("Failed to get current weather: %v", err)
	}
	
	prettyPrint("Current Weather", currentWeather)
	
	forecast, err := API.Weather.GetForecast(ctx, station.Location, 3)
	if err != nil {
		log.Fatalf("Failed to get forecast: %v", err)
	}
	
	prettyPrint("3-Day Forecast", forecast)
	
	updateData := weather.WeatherData{
		Temperature: 25.5,
		Humidity:    70.0,
		WindSpeed:   12.0,
		Conditions:  weather.ConditionValues.Cloudy,
	}
	
	err = API.Weather.UpdateWeatherData(ctx, stationID, updateData)
	if err != nil {
		log.Fatalf("Failed to update weather data: %v", err)
	}
	fmt.Println("Weather data updated successfully")
	
	err = API.Weather.DeleteWeatherStation(ctx, stationID)
	if err != nil {
		log.Fatalf("Failed to delete weather station: %v", err)
	}
	fmt.Println("Weather station deleted successfully")
}

func prettyPrint(title string, data interface{}) {
	fmt.Printf("\n--- %s ---\n", title)
	bytes, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(bytes))
}

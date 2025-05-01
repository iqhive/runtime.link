package weather

import (
	"context"
	"time"

	"runtime.link/api"
	"runtime.link/xyz"
)

type API struct {
	api.Specification `api:"Weather" cmd:"weather" lib:"libweather" www:"https://api.weather.example.com/v1"
		is an example weather service API that demonstrates various runtime.link patterns.`
		
	GetCurrentWeather func(context.Context, Location) (WeatherData, error) `rest:"GET /current?location=%v"
		returns the current weather data for the specified location.`
		
	GetForecast func(context.Context, Location, int) ([]ForecastData, error) `rest:"GET /forecast/{location=%v}?days=%v"
		returns the weather forecast for the specified location and number of days.`
		
	AddWeatherStation func(context.Context, WeatherStation) (StationID, error) `rest:"POST /stations"
		adds a new weather station and returns its ID.`
		
	UpdateWeatherData func(context.Context, StationID, WeatherData) error `rest:"PUT /stations/{stationId=%v}/data"
		updates the current weather data for the specified station.`
		
	DeleteWeatherStation func(context.Context, StationID) error `rest:"DELETE /stations/{stationId=%v}"
		deletes the specified weather station.`
		
	Advanced struct {
		StreamWeatherUpdates func(context.Context, StationID) (<-chan WeatherData, error) `rest:"GET /stations/{stationId=%v}/stream"
			streams real-time weather updates from the specified station.`
			
		BatchUpdateStations func(context.Context, []StationUpdate) (BatchResult, error) `rest:"POST /stations/batch"
			performs batch updates on multiple stations.`
	}
}

type Location struct {
	City    string  `json:"city"`
	Country string  `json:"country"`
	Lat     float64 `json:"latitude,omitempty"`
	Long    float64 `json:"longitude,omitempty"`
}

type WeatherData struct {
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	WindSpeed   float64   `json:"wind_speed"`
	Conditions  Condition `json:"conditions"`
	Timestamp   time.Time `json:"timestamp"`
}

type ForecastData struct {
	Date        time.Time `json:"date"`
	High        float64   `json:"high_temperature"`
	Low         float64   `json:"low_temperature"`
	Humidity    float64   `json:"humidity"`
	WindSpeed   float64   `json:"wind_speed"`
	Conditions  Condition `json:"conditions"`
	Probability float64   `json:"precipitation_probability"`
}

type StationID string

type WeatherStation struct {
	Name     string   `json:"name"`
	Location Location `json:"location"`
	Owner    string   `json:"owner"`
	Email    string   `json:"email"`
}

type Condition xyz.Switch[string, struct {
	Sunny     Condition `json:"sunny"`
	Cloudy    Condition `json:"cloudy"`
	Rainy     Condition `json:"rainy"`
	Snowy     Condition `json:"snowy"`
	Stormy    Condition `json:"stormy"`
	Foggy     Condition `json:"foggy"`
	Windy     Condition `json:"windy"`
	Hurricane Condition `json:"hurricane"`
}]

type StationUpdate struct {
	ID   StationID   `json:"station_id"`
	Data WeatherData `json:"data"`
}

type BatchResult struct {
	Successful int      `json:"successful_updates"`
	Failed     int      `json:"failed_updates"`
	Errors     []string `json:"errors,omitempty"`
}

var ConditionValues = xyz.AccessorFor(Condition.Values)

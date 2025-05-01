package weather

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

func New() API {
	impl := &implementation{
		stations: make(map[StationID]WeatherStation),
		data:     make(map[StationID]WeatherData),
		nextID:   1,
	}
	
	return API{
		GetCurrentWeather: impl.getCurrentWeather,
		GetForecast:       impl.getForecast,
		AddWeatherStation: impl.addWeatherStation,
		UpdateWeatherData: impl.updateWeatherData,
		DeleteWeatherStation: impl.deleteWeatherStation,
		Advanced: struct {
			StreamWeatherUpdates func(context.Context, StationID) (<-chan WeatherData, error)
			BatchUpdateStations  func(context.Context, []StationUpdate) (BatchResult, error)
		}{
			StreamWeatherUpdates: impl.streamWeatherUpdates,
			BatchUpdateStations:  impl.batchUpdateStations,
		},
	}
}

type implementation struct {
	mu       sync.RWMutex
	stations map[StationID]WeatherStation
	data     map[StationID]WeatherData
	nextID   int
}

func (i *implementation) getCurrentWeather(ctx context.Context, loc Location) (WeatherData, error) {
	return WeatherData{
		Temperature: 22.5,
		Humidity:    65.0,
		WindSpeed:   10.0,
		Conditions:  ConditionValues.Sunny,
		Timestamp:   time.Now(),
	}, nil
}

func (i *implementation) getForecast(ctx context.Context, loc Location, days int) ([]ForecastData, error) {
	if days <= 0 || days > 14 {
		return nil, errors.New("days must be between 1 and 14")
	}
	
	forecast := make([]ForecastData, days)
	for j := 0; j < days; j++ {
		forecast[j] = ForecastData{
			Date:        time.Now().AddDate(0, 0, j),
			High:        25.0 - float64(j)*0.5,
			Low:         15.0 - float64(j)*0.3,
			Humidity:    60.0 + float64(j),
			WindSpeed:   8.0 + float64(j)*0.2,
			Conditions:  ConditionValues.Sunny,
			Probability: float64(j) * 5.0,
		}
	}
	
	return forecast, nil
}

func (i *implementation) addWeatherStation(ctx context.Context, station WeatherStation) (StationID, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	id := StationID(fmt.Sprintf("STATION-%03d", i.nextID))
	i.nextID++
	
	i.stations[id] = station
	return id, nil
}

func (i *implementation) updateWeatherData(ctx context.Context, id StationID, data WeatherData) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if _, exists := i.stations[id]; !exists {
		return errors.New("station not found")
	}
	
	i.data[id] = data
	return nil
}

func (i *implementation) deleteWeatherStation(ctx context.Context, id StationID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if _, exists := i.stations[id]; !exists {
		return errors.New("station not found")
	}
	
	delete(i.stations, id)
	delete(i.data, id)
	return nil
}

func (i *implementation) streamWeatherUpdates(ctx context.Context, id StationID) (<-chan WeatherData, error) {
	i.mu.RLock()
	if _, exists := i.stations[id]; !exists {
		i.mu.RUnlock()
		return nil, errors.New("station not found")
	}
	i.mu.RUnlock()
	
	ch := make(chan WeatherData)
	go func() {
		defer close(ch)
		
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				data := WeatherData{
					Temperature: 20.0 + 5.0*float64(time.Now().Second()%10)/10.0,
					Humidity:    60.0 + 10.0*float64(time.Now().Second()%10)/10.0,
					WindSpeed:   8.0 + 2.0*float64(time.Now().Second()%10)/10.0,
					Conditions:  ConditionValues.Sunny,
					Timestamp:   time.Now(),
				}
				
				select {
				case ch <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return ch, nil
}

func (i *implementation) batchUpdateStations(ctx context.Context, updates []StationUpdate) (BatchResult, error) {
	result := BatchResult{}
	
	for _, update := range updates {
		err := i.updateWeatherData(ctx, update.ID, update.Data)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Station %s: %s", update.ID, err.Error()))
		} else {
			result.Successful++
		}
	}
	
	return result, nil
}

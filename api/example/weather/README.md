# Weather API Example

This is an experimental API implementation that demonstrates various patterns and features of the runtime.link framework. The API simulates a weather service with various endpoints for retrieving weather data, managing weather stations, and streaming updates.

## API Design

The Weather API demonstrates:

1. Different HTTP methods (GET, POST, PUT, DELETE)
2. Path parameters, query parameters, and request bodies
3. Different response types including collections and channels
4. Error handling
5. Nested API structure
6. Tagged union types using `xyz.Switch`

## Running the Example

To run the client demonstration:

```bash
go run api/example/weather/main/main.go
```

To start the API server:

```bash
PORT=8080 go run api/example/weather/main/main.go
```

Then you can interact with the API using curl:

```bash
# Get current weather
curl "http://localhost:8080/current?city=San%20Francisco&country=USA"

# Get weather forecast
curl "http://localhost:8080/forecast/San%20Francisco?days=3"

# Add a weather station
curl -X POST http://localhost:8080/stations -d '{"name":"Test Station","location":{"city":"San Francisco","country":"USA"},"owner":"Jane Doe","email":"jane@example.com"}'
```

## Learning Points

This example demonstrates:

1. How to structure a runtime.link API definition
2. How to define function fields with appropriate tags
3. How to organize related functionality using nested structs
4. How to use tagged union types for enumerations
5. How to handle streaming data
6. How to implement batch operations

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func NewWeatherClient(apiKey string) *WeatherClient {
	return &WeatherClient{
		ApiKey:  apiKey,
		BaseURL: "https://api.openweathermap.org/",
	}
}

func (c *WeatherClient) GetCoordinates(city string) (*City, error) {
	url := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=1&appid=%s", c.BaseURL, city, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var cities []City
	if err := json.NewDecoder(resp.Body).Decode(&cities); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	if len(cities) == 0 {
		return nil, fmt.Errorf("no coordinates found for %s", city)
	}

	return &cities[0], nil
}

func (c *WeatherClient) GetWeather(lat, lon float64) (*OneCallResponse, error) {
	url := fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s", c.BaseURL, lat, lon, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result OneCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	return &result, nil
}

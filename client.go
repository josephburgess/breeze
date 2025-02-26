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

func (c *WeatherClient) GetWeather(lat, lon float64) (*Weather, error) {
	url := fmt.Sprintf("%sdata/2.5/weather?lat=%f&lon=%f&appid=%s", c.BaseURL, lat, lon, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Weather []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"weather"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	if len(result.Weather) == 0 {
		return nil, fmt.Errorf("no weather data available")
	}

	first := result.Weather[0]
	w := &Weather{
		ID:          first.ID,
		Icon:        first.Icon,
		Temp:        result.Main.Temp,
		Description: first.Description,
	}
	return w, nil
}

func (c *WeatherClient) GetForecast(lat, lon float64) ([]ForecastItem, error) {
	url := fmt.Sprintf("%sdata/2.5/forecast?lat=%f&lon=%f&appid=%s", c.BaseURL, lat, lon, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("forecast request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		List []struct {
			DtTxt string `json:"dt_txt"`
			Main  struct {
				Temp float64 `json:"temp"`
			} `json:"main"`
			Weather []struct {
				Description string `json:"description"`
			} `json:"weather"`
		} `json:"list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unmarshaling forecast JSON: %w", err)
	}

	var forecast []ForecastItem
	for _, item := range result.List {
		if len(item.Weather) == 0 {
			continue
		}
		forecast = append(forecast, ForecastItem{
			DateTime:    item.DtTxt,
			Temp:        item.Main.Temp,
			Description: item.Weather[0].Description,
		})
	}

	return forecast, nil
}

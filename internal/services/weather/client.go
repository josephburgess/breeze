package weather

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/josephburgess/breeze/internal/models"
)

type Client struct {
	ApiKey  string
	BaseURL string
}

func NewClient(apiKey string) *Client {
	return &Client{
		ApiKey:  apiKey,
		BaseURL: "https://api.openweathermap.org/",
	}
}

func (c *Client) GetCoordinates(city string) (*models.City, error) {
	url := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=1&appid=%s", c.BaseURL, city, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var cities []models.City
	if err := json.NewDecoder(resp.Body).Decode(&cities); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	if len(cities) == 0 {
		return nil, fmt.Errorf("no coordinates found for %s", city)
	}

	return &cities[0], nil
}

func (c *Client) GetWeather(lat, lon float64) (*models.OneCallResponse, error) {
	url := fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s", c.BaseURL, lat, lon, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result models.OneCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	return &result, nil
}

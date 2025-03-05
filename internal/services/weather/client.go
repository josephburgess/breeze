package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

type Client struct {
	ApiKey  string
	BaseURL string
}

func NewClient(apiKey string) *Client {
	logging.Info("Initializing Weather Client")
	return &Client{
		ApiKey:  apiKey,
		BaseURL: "https://api.openweathermap.org/",
	}
}

func (c *Client) GetCoordinates(city string) (*models.City, error) {
	url := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=1&appid=%s", c.BaseURL, city, c.ApiKey)
	logging.Info("Fetching coordinates for city: %s", city)

	resp, err := http.Get(url)
	if err != nil {
		logging.Error("HTTP request failed", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var cities []models.City
	if err := json.NewDecoder(resp.Body).Decode(&cities); err != nil {
		logging.Error("Failed to decode JSON response", err)
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	if len(cities) == 0 {
		logging.Warn("No coordinates found for city: %s", city)
		return nil, fmt.Errorf("no coordinates found for %s", city)
	}

	logging.Info("Coordinates found for city: %s (lat: %f, lon: %f)", city, cities[0].Lat, cities[0].Lon)
	return &cities[0], nil
}

func (c *Client) GetWeather(lat, lon float64, units string) (*models.OneCallResponse, error) {
	var url string

	if units != "" {
		url = fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s&units=%s",
			c.BaseURL, lat, lon, c.ApiKey, units)
		logging.Info("Fetching weather data with units=%s", units)
	} else {
		url = fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s",
			c.BaseURL, lat, lon, c.ApiKey)
		logging.Info("Fetching weather data with default units (Kelvin)")
	}

	logging.Info("Fetching weather data for lat: %f, lon: %f", lat, lon)

	resp, err := http.Get(url)
	if err != nil {
		logging.Error("HTTP request failed", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Warn("API returned non-200 status: %d", resp.StatusCode)
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result models.OneCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Error("Failed to decode JSON response", err)
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	logging.Info("Successfully fetched weather data for lat: %f, lon: %f", lat, lon)
	return &result, nil
}

func (c *Client) SearchCities(query string, limit int) ([]models.City, error) {
	escapedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=%d&appid=%s", c.BaseURL, escapedQuery, limit, c.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		logging.Error("HTTP request failed", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		body := string(data)
		return nil, fmt.Errorf("API returned status %d %s", resp.StatusCode, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Error("Failed to read response body", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var cities []models.City
	if err := json.Unmarshal(data, &cities); err != nil {
		logging.Error("Failed to decode JSON response", err)
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	if len(cities) == 0 {
		logging.Warn("No cities found for query: %s", query)
		return []models.City{}, nil
	}

	logging.Info("Found %d cities for query: %s", len(cities), query)
	return cities, nil
}

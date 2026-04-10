package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

const (
	weatherCacheTTL = 10 * time.Minute
	coordsCacheTTL  = 1 * time.Hour
)

type weatherEntry struct {
	data      *models.OneCallResponse
	expiresAt time.Time
}

type coordsEntry struct {
	data      *models.City
	expiresAt time.Time
}

type Client struct {
	ApiKey       string
	BaseURL      string
	weatherCache sync.Map
	coordsCache  sync.Map
}

func NewClient(apiKey string) *Client {
	logging.Info("Initializing Weather Client")
	return &Client{
		ApiKey:  apiKey,
		BaseURL: "https://api.openweathermap.org/",
	}
}

func (c *Client) GetCoordinates(city string, customApiKey string) (*models.City, error) {
	apiKey := c.ApiKey
	if customApiKey != "" {
		apiKey = customApiKey
	}

	cacheKey := fmt.Sprintf("%s:%s", city, apiKey)
	if v, ok := c.coordsCache.Load(cacheKey); ok {
		entry := v.(coordsEntry)
		if time.Now().Before(entry.expiresAt) {
			logging.Info("Cache hit for city coordinates: %s", city)
			return entry.data, nil
		}
		c.coordsCache.Delete(cacheKey)
	}

	reqURL := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=1&appid=%s", c.BaseURL, url.QueryEscape(city), apiKey)
	logging.Info("Fetching coordinates for city: %s", city)

	resp, err := http.Get(reqURL)
	if err != nil {
		logging.Error("HTTP request failed", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		logging.Error("Invalid API key", nil)
		return nil, fmt.Errorf("invalid_api_key: custom api key is not valid - please run setup again or set with flag -K")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logging.Error("API error", fmt.Errorf("status %d: %s", resp.StatusCode, string(body)))
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

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

	c.coordsCache.Store(cacheKey, coordsEntry{
		data:      &cities[0],
		expiresAt: time.Now().Add(coordsCacheTTL),
	})

	return &cities[0], nil
}

func (c *Client) GetWeather(lat, lon float64, units string, customApiKey string) (*models.OneCallResponse, error) {
	apiKey := c.ApiKey
	if customApiKey != "" {
		apiKey = customApiKey
	}

	cacheKey := fmt.Sprintf("%.4f,%.4f,%s", lat, lon, units)
	if v, ok := c.weatherCache.Load(cacheKey); ok {
		entry := v.(weatherEntry)
		if time.Now().Before(entry.expiresAt) {
			logging.Info("Cache hit for weather: %s", cacheKey)
			return entry.data, nil
		}
		c.weatherCache.Delete(cacheKey)
	}

	var reqURL string
	if units != "" {
		reqURL = fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s&units=%s",
			c.BaseURL, lat, lon, apiKey, units)
		logging.Info("Fetching weather data with units=%s", units)
	} else {
		reqURL = fmt.Sprintf("%sdata/3.0/onecall?lat=%f&lon=%f&appid=%s",
			c.BaseURL, lat, lon, apiKey)
		logging.Info("Fetching weather data with default units (Kelvin)")
	}

	logging.Info("Fetching weather data for lat: %f, lon: %f", lat, lon)

	resp, err := http.Get(reqURL)
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

	c.weatherCache.Store(cacheKey, weatherEntry{
		data:      &result,
		expiresAt: time.Now().Add(weatherCacheTTL),
	})

	return &result, nil
}

func (c *Client) SearchCities(query string, limit int) ([]models.City, error) {
	reqURL := fmt.Sprintf("%sgeo/1.0/direct?q=%s&limit=%d&appid=%s",
		c.BaseURL, url.QueryEscape(query), limit, c.ApiKey)

	resp, err := http.Get(reqURL)
	if err != nil {
		logging.Error("HTTP request failed", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var cities []models.City
	if err := json.NewDecoder(resp.Body).Decode(&cities); err != nil {
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

package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/josephburgess/breeze/internal/models"
)

const (
	DefaultBaseURL = "https://api.openweathermap.org"
	requestTimeout = 10 * time.Second
	searchLimit    = 5
	// places do not move; forecasts refresh upstream about every 10 minutes
	geocodeTTL  = 24 * time.Hour
	forecastTTL = 10 * time.Minute
)

var (
	ErrNotFound   = errors.New("place not found")
	ErrInvalidKey = errors.New("openweather rejected the api key")
)

type UpstreamError struct {
	Status int
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("openweather returned status %d", e.Status)
}

// Client is the only way breeze talks to OpenWeather.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client

	places    *ttlCache[models.City]
	forecasts *ttlCache[*models.OneCallResponse]
}

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: requestTimeout},

		places:    newTTLCache[models.City](geocodeTTL),
		forecasts: newTTLCache[*models.OneCallResponse](forecastTTL),
	}
}

// Forecast geocodes place and fetches its weather. customKey, when set,
// replaces the server key on every upstream call. Results are cached per key,
// so a bad custom key never reads what a good key fetched.
func (c *Client) Forecast(ctx context.Context, place, units, customKey string) (*models.WeatherResponse, error) {
	placeKey := customKey + "|" + strings.ToLower(strings.TrimSpace(place))
	city, ok := c.places.get(placeKey)
	if !ok {
		var cities []models.City
		if err := c.get(ctx, "/geo/1.0/direct", url.Values{"q": {place}, "limit": {"1"}}, customKey, &cities); err != nil {
			return nil, err
		}
		if len(cities) == 0 {
			return nil, ErrNotFound
		}
		city = cities[0]
		c.places.put(placeKey, city)
	}

	weather, err := c.ForecastAt(ctx, city.Lat, city.Lon, units, customKey)
	if err != nil {
		return nil, err
	}
	return &models.WeatherResponse{City: &city, Weather: weather}, nil
}

func (c *Client) ForecastAt(ctx context.Context, lat, lon float64, units, customKey string) (*models.OneCallResponse, error) {
	cacheKey := fmt.Sprintf("%s|%.4f|%.4f|%s", customKey, lat, lon, units)
	if weather, ok := c.forecasts.get(cacheKey); ok {
		return weather, nil
	}

	params := url.Values{
		"lat": {strconv.FormatFloat(lat, 'f', -1, 64)},
		"lon": {strconv.FormatFloat(lon, 'f', -1, 64)},
	}
	if units != "" {
		params.Set("units", units)
	}

	var weather models.OneCallResponse
	if err := c.get(ctx, "/data/3.0/onecall", params, customKey, &weather); err != nil {
		return nil, err
	}
	c.forecasts.put(cacheKey, &weather)
	return &weather, nil
}

func (c *Client) Search(ctx context.Context, query string) ([]models.City, error) {
	cities := []models.City{}
	params := url.Values{"q": {query}, "limit": {strconv.Itoa(searchLimit)}}
	if err := c.get(ctx, "/geo/1.0/direct", params, "", &cities); err != nil {
		return nil, err
	}
	return cities, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, customKey string, out any) error {
	params.Set("appid", c.apiKey)
	if customKey != "" {
		params.Set("appid", customKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build openweather request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// the url holds the key, so drop it from the error
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("call openweather %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrInvalidKey
	case resp.StatusCode != http.StatusOK:
		return &UpstreamError{Status: resp.StatusCode}
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode openweather %s: %w", path, err)
	}
	return nil
}

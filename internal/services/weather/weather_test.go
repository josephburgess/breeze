package weather_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/josephburgess/breeze/internal/services/weather"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const londonGeo = `[{"name":"London","lat":51.5074,"lon":-0.1278,"country":"GB","state":"England"}]`
const forecast = `{"lat":51.5074,"lon":-0.1278,"timezone":"Europe/London","current":{"temp":15.2}}`

func TestForecast_GeocodesThenFetchesWeather(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "server-key", r.URL.Query().Get("appid"))
		switch r.URL.Path {
		case "/geo/1.0/direct":
			assert.Equal(t, "New York", r.URL.Query().Get("q"))
			assert.Equal(t, "1", r.URL.Query().Get("limit"))
			w.Write([]byte(londonGeo))
		case "/data/3.0/onecall":
			assert.Equal(t, "51.5074", r.URL.Query().Get("lat"))
			assert.Equal(t, "metric", r.URL.Query().Get("units"))
			w.Write([]byte(forecast))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	resp, err := client.Forecast(context.Background(), "New York", "metric", "")

	require.NoError(t, err)
	assert.Equal(t, "London", resp.City.Name)
	assert.Equal(t, 15.2, resp.Weather.Current.Temp)
}

func TestForecast_UsesCustomKeyWhenUnitsEmpty(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.URL.Query().Get("appid"))
		assert.False(t, r.URL.Query().Has("units"))
		if r.URL.Path == "/geo/1.0/direct" {
			w.Write([]byte(londonGeo))
			return
		}
		w.Write([]byte(forecast))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "London", "", "custom-key")

	require.NoError(t, err)
	assert.Equal(t, []string{"custom-key", "custom-key"}, keys)
}

func TestForecast_UnknownPlaceIsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "Nowhere", "metric", "")

	assert.ErrorIs(t, err, weather.ErrNotFound)
}

func TestForecast_RejectedKeyIsInvalidKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "London", "metric", "bad-key")

	assert.ErrorIs(t, err, weather.ErrInvalidKey)
}

func TestForecast_UpstreamFailureIsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "London", "metric", "")

	var upstream *weather.UpstreamError
	require.ErrorAs(t, err, &upstream)
	assert.Equal(t, http.StatusBadGateway, upstream.Status)
}

func TestSearch_EscapesQueryAndUsesServerKey(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "São Paulo & co", r.URL.Query().Get("q"))
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		assert.Equal(t, "server-key", r.URL.Query().Get("appid"))
		w.Write([]byte(londonGeo))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	cities, err := client.Search(context.Background(), "São Paulo & co")

	require.NoError(t, err)
	assert.Len(t, cities, 1)
	assert.Equal(t, int32(1), calls.Load())
}

func TestSearch_NoMatchesIsEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	cities, err := client.Search(context.Background(), "zzz")

	require.NoError(t, err)
	assert.Empty(t, cities)
}

func TestForecast_RepeatCallIsServedFromCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/geo/1.0/direct" {
			w.Write([]byte(londonGeo))
			return
		}
		w.Write([]byte(forecast))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "London", "metric", "")
	require.NoError(t, err)
	resp, err := client.Forecast(context.Background(), "london", "metric", "")
	require.NoError(t, err)

	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "London", resp.City.Name)
}

func TestForecast_CacheIsNotSharedAcrossKeys(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("appid") == "bad-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/geo/1.0/direct" {
			w.Write([]byte(londonGeo))
			return
		}
		w.Write([]byte(forecast))
	}))
	defer server.Close()

	client := weather.NewClient("server-key", server.URL)
	_, err := client.Forecast(context.Background(), "London", "metric", "")
	require.NoError(t, err)
	_, err = client.Forecast(context.Background(), "London", "metric", "bad-key")

	assert.ErrorIs(t, err, weather.ErrInvalidKey)
	assert.Equal(t, int32(3), calls.Load())
}

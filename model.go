package main

type City struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type Weather struct {
	ID          int     `json:"id"`
	Icon        string  `json:"icon"`
	Temp        float64 `json:"temp"`
	Description string  `json:"description"`
}

type ForecastItem struct {
	DateTime    string  `json:"dateTime"`
	Temp        float64 `json:"temp"`
	Description string  `json:"description"`
}

type WeatherClient struct {
	ApiKey  string
	BaseURL string
}

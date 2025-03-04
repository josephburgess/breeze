package models

type City struct {
	Name    string  `json:"name"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Country string  `json:"country"`
	State   string  `json:"state"`
}

type WeatherCondition struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type RainData struct {
	OneHour float64 `json:"1h"`
}

type SnowData struct {
	OneHour float64 `json:"1h"`
}

type CurrentWeather struct {
	Dt         int64              `json:"dt"`
	Sunrise    int64              `json:"sunrise"`
	Sunset     int64              `json:"sunset"`
	Temp       float64            `json:"temp"`
	FeelsLike  float64            `json:"feels_like"`
	Pressure   int                `json:"pressure"`
	Humidity   int                `json:"humidity"`
	DewPoint   float64            `json:"dew_point"`
	UVI        float64            `json:"uvi"`
	Clouds     int                `json:"clouds"`
	Visibility int                `json:"visibility"`
	WindSpeed  float64            `json:"wind_speed"`
	WindGust   float64            `json:"wind_gust"`
	WindDeg    int                `json:"wind_deg"`
	Rain       *RainData          `json:"rain,omitempty"`
	Snow       *SnowData          `json:"snow,omitempty"`
	Weather    []WeatherCondition `json:"weather"`
}

type MinuteData struct {
	Dt            int64   `json:"dt"`
	Precipitation float64 `json:"precipitation"`
}

type HourData struct {
	Dt         int64              `json:"dt"`
	Temp       float64            `json:"temp"`
	FeelsLike  float64            `json:"feels_like"`
	Pressure   int                `json:"pressure"`
	Humidity   int                `json:"humidity"`
	DewPoint   float64            `json:"dew_point"`
	UVI        float64            `json:"uvi"`
	Clouds     int                `json:"clouds"`
	Visibility int                `json:"visibility"`
	WindSpeed  float64            `json:"wind_speed"`
	WindGust   float64            `json:"wind_gust"`
	WindDeg    int                `json:"wind_deg"`
	Pop        float64            `json:"pop"`
	Rain       *RainData          `json:"rain,omitempty"`
	Snow       *SnowData          `json:"snow,omitempty"`
	Weather    []WeatherCondition `json:"weather"`
}

type TempData struct {
	Day   float64 `json:"day"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Night float64 `json:"night"`
	Eve   float64 `json:"eve"`
	Morn  float64 `json:"morn"`
}

type FeelsLikeData struct {
	Day   float64 `json:"day"`
	Night float64 `json:"night"`
	Eve   float64 `json:"eve"`
	Morn  float64 `json:"morn"`
}

type DayData struct {
	Dt        int64              `json:"dt"`
	Sunrise   int64              `json:"sunrise"`
	Sunset    int64              `json:"sunset"`
	Moonrise  int64              `json:"moonrise"`
	Moonset   int64              `json:"moonset"`
	MoonPhase float64            `json:"moon_phase"`
	Summary   string             `json:"summary"`
	Temp      TempData           `json:"temp"`
	FeelsLike FeelsLikeData      `json:"feels_like"`
	Pressure  int                `json:"pressure"`
	Humidity  int                `json:"humidity"`
	DewPoint  float64            `json:"dew_point"`
	WindSpeed float64            `json:"wind_speed"`
	WindGust  float64            `json:"wind_gust"`
	WindDeg   int                `json:"wind_deg"`
	Clouds    int                `json:"clouds"`
	UVI       float64            `json:"uvi"`
	Pop       float64            `json:"pop"`
	Rain      float64            `json:"rain,omitempty"`
	Snow      float64            `json:"snow,omitempty"`
	Weather   []WeatherCondition `json:"weather"`
}

type Alert struct {
	SenderName  string   `json:"sender_name"`
	Event       string   `json:"event"`
	Start       int64    `json:"start"`
	End         int64    `json:"end"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type OneCallResponse struct {
	Lat            float64        `json:"lat"`
	Lon            float64        `json:"lon"`
	Timezone       string         `json:"timezone"`
	TimezoneOffset int            `json:"timezone_offset"`
	Current        CurrentWeather `json:"current"`
	Minutely       []MinuteData   `json:"minutely"`
	Hourly         []HourData     `json:"hourly"`
	Daily          []DayData      `json:"daily"`
	Alerts         []Alert        `json:"alerts"`
}

type WeatherResponse struct {
	City    *City            `json:"city"`
	Weather *OneCallResponse `json:"weather"`
}

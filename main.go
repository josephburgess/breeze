package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENWEATHER_API_KEY not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client := NewWeatherClient(apiKey)

	r := mux.NewRouter()

	r.HandleFunc("/api/weather/{city}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cityName := vars["city"]

		city, err := client.GetCoordinates(cityName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error finding city: %v", err), http.StatusNotFound)
			return
		}

		weather, err := client.GetWeather(city.Lat, city.Lon)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting weather: %v", err), http.StatusInternalServerError)
			return
		}

		response := struct {
			City    *City    `json:"city"`
			Weather *Weather `json:"weather"`
		}{
			City:    city,
			Weather: weather,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

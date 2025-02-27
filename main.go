package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/josephburgess/gust-api/templates"
)

var (
	weatherClient *WeatherClient
	userStore     *UserStore
	githubOAuth   *GitHubOAuth
	jwtSecret     string
)

type contextKey string

const (
	userContextKey contextKey = "user"
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
	weatherClient = NewWeatherClient(apiKey)

	dbPath := os.Getenv("DB_PATH")
	userStore, err = NewUserStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}
	defer userStore.Close()

	githubOAuth = NewGitHubOAuth()

	jwtSecret = os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET not set")
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/auth/request", authRequestHandler).Methods("GET")
	r.HandleFunc("/api/auth/callback", authCallbackHandler).Methods("GET")
	r.HandleFunc("/api/auth/exchange", authExchangeHandler).Methods("POST")

	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(apiKeyMiddleware)
	apiRouter.HandleFunc("/weather/{city}", getWeatherHandler).Methods("GET")
	apiRouter.HandleFunc("/user", getUserHandler).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("api_key")

		if apiKey == "" {
			http.Error(w, "API key is required", http.StatusUnauthorized)
			return
		}

		user, err := userStore.ValidateAPIKey(apiKey)
		if err != nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"github_id": user.ID,
		"login":     user.Login,
		"name":      user.Name,
		"email":     user.Email,
		"avatar":    user.AvatarURL,
	})
}

func getWeatherHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cityName := vars["city"]

	city, err := weatherClient.GetCoordinates(cityName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error finding city: %v", err), http.StatusNotFound)
		return
	}

	weather, err := weatherClient.GetWeather(city.Lat, city.Lon)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting weather: %v", err), http.StatusInternalServerError)
		return
	}

	// Add the city name to the response
	response := struct {
		City    *City            `json:"city"`
		Weather *OneCallResponse `json:"weather"`
	}{
		City:    city,
		Weather: weather,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func authRequestHandler(w http.ResponseWriter, r *http.Request) {
	callbackPort := r.URL.Query().Get("callback_port")
	if callbackPort == "" {
		callbackPort = "9876" // Default port
	}

	callbackURL := fmt.Sprintf("http://localhost:%s/callback", callbackPort)
	githubOAuth.RedirectURI = callbackURL

	authURL, state := githubOAuth.GetAuthURL()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":   authURL,
		"state": state,
	})
}

func authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if strings.HasPrefix(githubOAuth.RedirectURI, "http://localhost:") {
		http.Redirect(w, r, fmt.Sprintf("%s?code=%s&state=%s",
			githubOAuth.RedirectURI, code, state), http.StatusFound)
		return
	}

	handleGitHubCallback(w, code, state)
}

func handleGitHubCallback(w http.ResponseWriter, code, state string) {
	user, apiKey, err := handleGitHubAuthCode(code, state)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	if err := templates.RenderSuccessTemplate(w, user.Login, apiKey); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
	}
}

func handleGitHubAuthCode(code, state string) (*User, string, error) {
	token, err := githubOAuth.ExchangeCodeForToken(code, state)
	if err != nil {
		return nil, "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	user, err := githubOAuth.GetUserInfo(token)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user info: %w", err)
	}

	if err := userStore.SaveUser(user); err != nil {
		return nil, "", fmt.Errorf("failed to save user: %w", err)
	}

	credential, err := userStore.CreateAPICredential(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API credential: %w", err)
	}

	return user, credential.ApiKey, nil
}

func authExchangeHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code         string `json:"code"`
		CallbackPort int    `json:"callback_port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	githubOAuth.RedirectURI = fmt.Sprintf("http://localhost:%d/callback", request.CallbackPort)

	user, apiKey, err := handleGitHubAuthCode(request.Code, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key":     apiKey,
		"github_user": user.Login,
	})
}

type GitHubOAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	States       map[string]bool
}

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Token     string `json:"-"`
}

func NewGitHubOAuth() *GitHubOAuth {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	redirectURI := os.Getenv("GITHUB_REDIRECT_URI")

	if clientID == "" || clientSecret == "" {
		log.Fatal("GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET must be set")
	}

	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/callback"
	}

	return &GitHubOAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		States:       make(map[string]bool),
	}
}

func (g *GitHubOAuth) GetAuthURL() (string, string) {
	state := uuid.New().String()
	g.States[state] = true

	return fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email,public_repo",
		g.ClientID,
		url.QueryEscape(g.RedirectURI),
		state,
	), state
}

func (g *GitHubOAuth) ExchangeCodeForToken(code, state string) (string, error) {
	if state != "" && !g.States[state] {
		return "", fmt.Errorf("invalid state parameter")
	}

	if state != "" {
		delete(g.States, state)
	}

	tokenURL := "https://github.com/login/oauth/access_token"
	resp, err := http.PostForm(tokenURL, url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.RedirectURI},
	})
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token := values.Get("access_token")
	if token == "" {
		errorMsg := values.Get("error_description")
		if errorMsg == "" {
			errorMsg = "No access token received"
		}
		return "", fmt.Errorf("github oauth error: %s", errorMsg)
	}

	return token, nil
}

func (g *GitHubOAuth) GetUserInfo(token string) (*User, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	user.Token = token
	return &user, nil
}

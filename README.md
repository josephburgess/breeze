# breeze

Breeze is a Go-based api/middleware service that provides a secure, authenticated proxy to the OpenWeatherMap API. It serves as the backend for my [gust](https://github.com/josephburgess/gust) weather tui app, and also serves a silly widget on my personal site [http://joeburgess.dev](http://joeburgess.dev).

If you need a weather API you're probably better off going straight to the source, this is quite a specific solution, but its been a fun project and works well for my use cases!

## Features

- **GitHub OAuth authentication**: secure login via GitHub to issue api key (no credentials or PII saved by breeze)
- **Rate limiting**: per day request quota with reset tracking
- **Custom OpenWeather key support**: users can bring their own key to avoid rate limits
- **Weather data proxy**: fetches and transforms data from OpenWeatherMap
- **In memory caching**: coordinates cached for 1 hour, weather data for 10 minutes

## API Endpoints

### Public

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/auth/request` | Start GitHub OAuth flow, returns auth URL + state |
| `GET` | `/api/auth/callback` | OAuth callback: redirects to gust's local listener |
| `POST` | `/api/auth/exchange` | Exchange OAuth code for API key |
| `GET` | `/api/cities/search?q=<query>` | City autocomplete (top 5 matches) |

### Authenticated (requires `?api_key=<key>`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/user` | Current user info |
| `GET` | `/api/user/quota?api_key=<key>` | Daily quota usage and reset time |
| `GET` | `/api/weather/{city}?units=metric` | Weather for a city (`units`: metric / imperial / standard) |

Pass your API key as a query parameter: `?api_key=gust_...`

For custom OpenWeather keys (not `gust_` prefixed), the key is passed through directly to OpenWeatherMap with no rate limiting applied by breeze.

## Getting Started

### Prerequisites

- Go 1.24+
- A GitHub OAuth application ([create one here](https://github.com/settings/apps))
- An OpenWeatherMap api key

### Environment Variables

Create a `.env` file (or export these in your environment):

```env
PORT=8080
DB_PATH=./data/gust.db
OPENWEATHER_API_KEY=your_openweather_api_key
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GITHUB_REDIRECT_URI=http://localhost:8080/api/auth/callback
```

### Running Locally

```bash
git clone https://github.com/josephburgess/breeze.git
cd breeze
go mod download
go run cmd/server/main.go
```

The server starts on port 8080 by default. [Air](https://github.com/air-verse/air) works well for live reloading during development.

### Running with Docker

```bash
docker build -t breeze .
docker run -p 8080:8080 --env-file .env breeze
```

## Build & Test

```bash
go build -o breeze ./cmd/server
go test ./...
go vet ./...
```

## Authentication Flow

This is the desktop OAuth flow used by gust:

1. Client calls `GET /api/auth/request?callback_port=9876`: breeze returns a GitHub OAuth URL with a state token
2. Client opens the URL in a browser; user authorises on GitHub
3. GitHub redirects to `GET /api/auth/callback`: breeze redirects to `localhost:9876/callback`
4. gust's local listener receives the code; calls `POST /api/auth/exchange` with the code
5. breeze exchanges the code for a GitHub access token, fetches user info, and issues a `gust_` API key
6. The API key is returned to the client and stored locally for future requests

## License

[MIT License](LICENSE)

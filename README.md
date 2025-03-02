# breeze

Breeze is a Go-based api/middleware service that provides a secure, authenticated proxy to the OpenWeatherMap API. It serves as the backend for my [gust](https://github.com/josephburgess/gust) weather tui app, and also serves a silly widget on my personal site [http://joeburgess.dev](http://joeburgess.dev).

## Features

- **GitHub OAuth Authentication**: Secure user authentication via GitHub
- **API Key Management**: Generates and validates API keys for `gust`
- **Weather Data Proxy**: Fetches/transforms data from OpenWeatherMap

## API Endpoints

### Public Endpoints

- `GET /api/auth/request` - initiates GitHub OAuth flow
- `GET /api/auth/callback` - OAuth callback handler
- `POST /api/auth/exchange` - exchange OAuth code for API key

### Authenticated Endpoints

API key required for these:

- `GET /api/user` - Get current user information
- `GET /api/weather/{city}` - Get weather data for a specific city
  - Optional query parameter: `units` (metric, imperial)

## Getting Started

### Prerequisites

- Go 1.24 or higher
- GitHub OAuth application credentials
- OpenWeather API key

### Environment Variables

Create a `.env` file with the following variables:

```
PORT=8080
DB_PATH=./data/gust.db
OPENWEATHER_API_KEY=your_openweather_api_key

// GH variables - requires setting up a Github application on your account - https://github.com/settings/apps
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GITHUB_REDIRECT_URI=http://localhost:8080/api/auth/callback
JWT_SECRET=secret_string_for_jwts
```

### Running Locally

1. Clone the repository:

   ```
   git clone https://github.com/josephburgess/breeze.git
   cd breeze
   ```

2. Install dependencies:

   ```
   go mod download
   ```

3. Run the application:
   ```
   go run cmd/server/main.go
   ```

The server will start on port 8080 (or the port specified in your `.env` file).

You can also use [Air](https://github.com/air-verse/air) for live reloading.

## Using with Gust CLI

breeze was designed to serve [gust](https://github.com/josephburgess/gust). When running `gust` for the first time it will:

1. Request authentication from Breeze
2. Open a browser for GitHub authentication
3. Receive and store the API key for future requests

## Authentication Flow

1. The client requests an auth URL from `/api/auth/request`
2. The server returns a GitHub OAuth URL
3. The user authenticates with GitHub
4. GitHub redirects to the callback URL with an auth code
5. The server exchanges the auth code for a GitHub access token
6. The server creates or updates the user record and generates an API key
7. The API key is returned to the client for future requests

## License

[MIT License](LICENSE)

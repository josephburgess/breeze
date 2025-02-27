package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/josephburgess/gust-api/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(dbPath string) (*UserStore, error) {
	if dbPath == "" {
		dbPath = "gust.db"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := initDB(db); err != nil {
		db.Close()
		return nil, err
	}

	return &UserStore{db: db}, nil
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		github_id INTEGER UNIQUE NOT NULL,
		login TEXT NOT NULL,
		name TEXT,
		email TEXT,
		avatar_url TEXT,
		token TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_login TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		starred_repo BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS api_credentials (
		id TEXT PRIMARY KEY,
		github_user_id INTEGER NOT NULL,
		api_key TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_used TIMESTAMP,
		request_count INTEGER DEFAULT 0,
		FOREIGN KEY (github_user_id) REFERENCES users(github_id)
	);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func (s *UserStore) Close() error {
	return s.db.Close()
}

func (s *UserStore) SaveUser(user *models.User) error {
	_, err := s.db.Exec(`
	INSERT INTO users (github_id, login, name, email, avatar_url, token, last_login)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(github_id) DO UPDATE SET
		login = excluded.login,
		name = excluded.name,
		email = excluded.email,
		avatar_url = excluded.avatar_url,
		token = excluded.token,
		last_login = CURRENT_TIMESTAMP
	`, user.ID, user.Login, user.Name, user.Email, user.AvatarURL, user.Token)

	return err
}

func (s *UserStore) SetUserStarredRepo(githubID int64) error {
	_, err := s.db.Exec(`
	UPDATE users SET starred_repo = TRUE
	WHERE github_id = ?
	`, githubID)

	return err
}

func (s *UserStore) GetUser(githubID int64) (*models.User, error) {
	var user models.User

	err := s.db.QueryRow(`
	SELECT github_id, login, name, email, avatar_url, token
	FROM users
	WHERE github_id = ?
	`, githubID).Scan(&user.ID, &user.Login, &user.Name, &user.Email, &user.AvatarURL, &user.Token)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &user, err
}

func (s *UserStore) CreateAPICredential(githubUserID int64) (*models.ApiCredential, error) {
	var exists bool
	err := s.db.QueryRow("SELECT 1 FROM users WHERE github_id = ?", githubUserID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user with GitHub ID %d not found", githubUserID)
	}

	id := uuid.New().String()
	apiKey := generateAPIKey()

	_, err = s.db.Exec(`
	INSERT INTO api_credentials (id, github_user_id, api_key, created_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, id, githubUserID, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API credential: %w", err)
	}

	return &models.ApiCredential{
		ID:           id,
		GithubUserID: githubUserID,
		ApiKey:       apiKey,
		CreatedAt:    time.Now(),
	}, nil
}

func (s *UserStore) ValidateAPIKey(apiKey string) (*models.User, error) {
	_, err := s.db.Exec(`
	UPDATE api_credentials
	SET last_used = CURRENT_TIMESTAMP, request_count = request_count + 1
	WHERE api_key = ?
	`, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key usage: %w", err)
	}

	var githubUserID int64
	err = s.db.QueryRow(`
	SELECT github_user_id FROM api_credentials
	WHERE api_key = ?
	`, apiKey).Scan(&githubUserID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid API key")
	}

	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return s.GetUser(githubUserID)
}

func generateAPIKey() string {
	return fmt.Sprintf("gust_%s", uuid.New().String())
}

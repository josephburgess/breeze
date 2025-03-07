package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

type UserStore struct {
	db *gorm.DB
}

func NewUserStore(dbPath string) (*UserStore, error) {
	if dbPath == "" {
		dbPath = "gust.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		logging.Error("error creating db directory", err)
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		logging.Error("Failed to open database", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.ApiCredential{}); err != nil {
		logging.Error("Failed to migrate db", err)
		return nil, err
	}

	logging.Info("UserStore initialized with DB path: %s", dbPath)
	return &UserStore{db: db}, nil
}

func (s *UserStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		logging.Warn("Failed closing db", err)
	}
	return sqlDB.Close()
}

func (s *UserStore) SaveUser(user *models.User) error {
	logging.Info("Saving user: %d (%s)", user.GithubID, user.Login)

	var existingUser models.User
	result := s.db.Where("github_id = ?", user.GithubID).First(&existingUser)

	if result.Error == nil {
		existingUser.Login = user.Login
		existingUser.Name = user.Name
		existingUser.Email = user.Email
		existingUser.Token = user.Token
		existingUser.LastLogin = time.Now()

		return s.db.Save(&existingUser).Error
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return s.db.Create(user).Error
	} else {
		return result.Error
	}
}

func (s *UserStore) GetUser(githubID int64) (*models.User, error) {
	var user models.User
	if err := s.db.Where("github_id = ?", githubID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logging.Warn("user not found: %d", githubID)
			return nil, nil
		}
		logging.Error("error fetching user", err)
		return nil, err
	}

	return &user, nil
}

func (s *UserStore) GetOrCreateAPICredential(githubUserID int64) (*models.ApiCredential, error) {
	var count int64
	if err := s.db.Model(&models.User{}).Where("github_id = ?", githubUserID).Count(&count).Error; err != nil {
		logging.Error("db error while checking for user", err)
		return nil, err
	}
	if count == 0 {
		logging.Warn("user with ID %d not found", githubUserID)
		return nil, fmt.Errorf("user with ID %d not found", githubUserID)
	}

	var credential models.ApiCredential
	err := s.db.Where("github_user_id = ?", githubUserID).First(&credential).Error

	if err == nil {
		logging.Info("Found existing API credential for user: %d", githubUserID)

		if err := s.db.Model(&credential).Updates(map[string]any{
			"last_used": gorm.Expr("CURRENT_TIMESTAMP"),
		}).Error; err != nil {
			logging.Error("Failed to update API key last used timestamp", err)
		}

		return &credential, nil
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.CreateAPICredential(githubUserID)
	} else {
		logging.Error("Database error while checking for credentials", err)
		return nil, err
	}
}

func (s *UserStore) CreateAPICredential(githubUserID int64) (*models.ApiCredential, error) {
	var count int64
	if err := s.db.Model(&models.User{}).Where("github_id = ?", githubUserID).Count(&count).Error; err != nil {
		logging.Error("db error while checking for user", err)
		return nil, err
	}
	if count == 0 {
		logging.Warn("user with ID %d not found", githubUserID)
		return nil, fmt.Errorf("user with ID %d not found", githubUserID)
	}

	apiKey := generateAPIKey()
	apiCredential := &models.ApiCredential{
		ID:              apiKey,
		GithubUserID:    githubUserID,
		ApiKey:          apiKey,
		DailyResetAt:    time.Now().UTC(),
		RateLimitPerDay: 50,
	}

	if err := s.db.Create(apiCredential).Error; err != nil {
		logging.Error("Failed to create API credential", err)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("user already has an API key")
		}
		return nil, err
	}

	logging.Info("API credential created for user: %d", githubUserID)
	return apiCredential, nil
}

func (s *UserStore) ValidateAPIKey(apiKey string) (*models.User, error) {
	logging.Info("Validating API key")

	var credential models.ApiCredential
	if err := s.db.Where("api_key = ?", apiKey).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logging.Warn("Invalid API key")
			return nil, fmt.Errorf("invalid API key")
		}
		logging.Error("Database error", err)
		return nil, err
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	updates := map[string]any{
		"last_used":     now,
		"request_count": gorm.Expr("request_count + 1"),
	}

	if credential.DailyResetAt.Before(today) {
		updates["daily_request_count"] = 1
		updates["daily_reset_at"] = today
	} else {
		if credential.DailyRequestCount >= credential.RateLimitPerDay {
			return nil, &RateLimitError{
				Message:   fmt.Sprintf("rate limit exceeded: maximum of %d requests per day", credential.RateLimitPerDay),
				ResetTime: credential.DailyResetAt.Add(24 * time.Hour),
				RateLimit: credential.RateLimitPerDay,
				Remaining: 0,
			}
		}
		updates["daily_request_count"] = gorm.Expr("daily_request_count + 1")
	}

	if err := s.db.Model(&credential).Updates(updates).Error; err != nil {
		logging.Error("Failed to update API key usage", err)
		return nil, fmt.Errorf("failed to update API key usage: %w", err)
	}

	return s.GetUser(credential.GithubUserID)
}

type RateLimitError struct {
	Message   string
	ResetTime time.Time
	RateLimit int
	Remaining int
}

func (e *RateLimitError) Error() string {
	return e.Message
}

func generateAPIKey() string {
	apiKey := fmt.Sprintf("gust_%s", uuid.New().String())
	logging.Info("Generated API key: %s", apiKey)
	return apiKey
}

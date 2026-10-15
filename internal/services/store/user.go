package store

import (
	"context"
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
		logging.Error("Failed to migrate models", err)
		return nil, fmt.Errorf("failed to migrate models: %w", err)
	}

	if err := initializeExistingRecords(db); err != nil {
		logging.Error("Failed to initialize existing records", err)
		return nil, fmt.Errorf("failed to initialize existing records: %w", err)
	}

	// sqlite has one writer; one connection avoids SQLITE_BUSY under load
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	return &UserStore{db: db}, nil
}

func initializeExistingRecords(db *gorm.DB) error {
	var count int64
	db.Model(&models.ApiCredential{}).Where("daily_reset_at IS NULL").Count(&count)

	if count > 0 {
		logging.Info("Updating daily_reset_at for existing records")
		if err := db.Model(&models.ApiCredential{}).
			Where("daily_reset_at IS NULL").
			Update("daily_reset_at", time.Now().UTC()).Error; err != nil {
			return err
		}
	}

	return nil
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
		RateLimitPerDay: DefaultRateLimitPerDay,
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

// DefaultRateLimitPerDay is the daily request limit for a new gust key.
const DefaultRateLimitPerDay = 50

var (
	ErrInvalidKey  = errors.New("invalid API key")
	ErrRateLimited = errors.New("daily rate limit exceeded")
)

type Usage struct {
	User      *models.User
	Limit     int
	Remaining int
	Reset     time.Time
}

// Spend counts one request against apiKey. It returns ErrInvalidKey for an
// unknown key and ErrRateLimited, with the usage, when the day is spent.
func (s *UserStore) Spend(ctx context.Context, apiKey string) (Usage, error) {
	db := s.db.WithContext(ctx)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if err := db.Model(&models.ApiCredential{}).
		Where("api_key = ? AND daily_reset_at < ?", apiKey, today).
		Updates(map[string]any{"daily_request_count": 0, "daily_reset_at": today}).Error; err != nil {
		return Usage{}, fmt.Errorf("reset daily count: %w", err)
	}

	// one conditional write, so parallel requests cannot pass the limit
	spent := db.Model(&models.ApiCredential{}).
		Where("api_key = ? AND daily_request_count < rate_limit_per_day", apiKey).
		Updates(map[string]any{
			"daily_request_count": gorm.Expr("daily_request_count + 1"),
			"request_count":       gorm.Expr("request_count + 1"),
			"last_used":           now,
		})
	if spent.Error != nil {
		return Usage{}, fmt.Errorf("spend request: %w", spent.Error)
	}

	var credential models.ApiCredential
	if err := db.Preload("User").Where("api_key = ?", apiKey).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Usage{}, ErrInvalidKey
		}
		return Usage{}, fmt.Errorf("read credential: %w", err)
	}

	usage := Usage{
		User:      &credential.User,
		Limit:     credential.RateLimitPerDay,
		Remaining: max(credential.RateLimitPerDay-credential.DailyRequestCount, 0),
		Reset:     credential.DailyResetAt.Add(24 * time.Hour),
	}
	if spent.RowsAffected == 0 {
		return usage, ErrRateLimited
	}
	return usage, nil
}

func generateAPIKey() string {
	apiKey := fmt.Sprintf("gust_%s", uuid.New().String())
	logging.Info("Generated API key: %s", apiKey)
	return apiKey
}

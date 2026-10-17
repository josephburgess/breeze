package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

	if err := hashPlaintextKeys(db); err != nil {
		return nil, fmt.Errorf("failed to hash api keys: %w", err)
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

// IssueAPIKey gives the user a new gust key and returns it. Only the hash is
// stored, so a second sign in replaces the old key.
func (s *UserStore) IssueAPIKey(ctx context.Context, githubUserID int64) (string, error) {
	db := s.db.WithContext(ctx)
	user, err := s.GetUser(githubUserID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("user with ID %d not found", githubUserID)
	}

	key := "gust_" + uuid.NewString()
	credential := &models.ApiCredential{
		ID:              uuid.NewString(),
		GithubUserID:    githubUserID,
		KeyHash:         hashKey(key),
		DailyResetAt:    time.Now().UTC(),
		RateLimitPerDay: DefaultRateLimitPerDay,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "github_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"key_hash"}),
	}).Create(credential).Error; err != nil {
		return "", fmt.Errorf("save api key: %w", err)
	}
	return key, nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// hashPlaintextKeys moves keys from the old api_key column to key_hash.
// The old rows also used the key as the id, so they get a new one.
func hashPlaintextKeys(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.ApiCredential{}, "api_key") {
		return nil
	}

	var rows []struct {
		ID     string
		ApiKey string
	}
	if err := db.Table("api_credentials").Select("id, api_key").Scan(&rows).Error; err != nil {
		return err
	}
	logging.Info("Hashing %d plaintext API keys", len(rows))

	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			if err := tx.Table("api_credentials").Where("id = ?", row.ID).
				Updates(map[string]any{"id": uuid.NewString(), "key_hash": hashKey(row.ApiKey)}).Error; err != nil {
				return err
			}
		}
		if err := tx.Migrator().DropConstraint(&models.ApiCredential{}, "uni_api_credentials_api_key"); err != nil {
			return err
		}
		if err := tx.Migrator().DropColumn(&models.ApiCredential{}, "api_key"); err != nil {
			return err
		}
		// the drop rebuilds the table without its indexes
		return tx.AutoMigrate(&models.ApiCredential{})
	})
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
	hash := hashKey(apiKey)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if err := db.Model(&models.ApiCredential{}).
		Where("key_hash = ? AND daily_reset_at < ?", hash, today).
		Updates(map[string]any{"daily_request_count": 0, "daily_reset_at": today}).Error; err != nil {
		return Usage{}, fmt.Errorf("reset daily count: %w", err)
	}

	// one conditional write, so parallel requests cannot pass the limit
	spent := db.Model(&models.ApiCredential{}).
		Where("key_hash = ? AND daily_request_count < rate_limit_per_day", hash).
		Updates(map[string]any{
			"daily_request_count": gorm.Expr("daily_request_count + 1"),
			"request_count":       gorm.Expr("request_count + 1"),
			"last_used":           now,
		})
	if spent.Error != nil {
		return Usage{}, fmt.Errorf("spend request: %w", spent.Error)
	}

	var credential models.ApiCredential
	if err := db.Preload("User").Where("key_hash = ?", hash).First(&credential).Error; err != nil {
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

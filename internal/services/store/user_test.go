package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *UserStore {
	store, err := NewUserStore("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NotNil(t, store)

	err = store.db.AutoMigrate(&models.User{}, &models.ApiCredential{})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := store.Close()
		if err != nil {
			t.Logf("Error closing test database: %v", err)
		}
	})

	return store
}

func TestUserStore_SaveUser(t *testing.T) {
	store := setupTestDB(t)

	tests := []struct {
		name    string
		user    *models.User
		wantErr bool
	}{
		{
			name: "save new user",
			user: &models.User{
				GithubID: 123,
				Login:    "testuser",
				Token:    "token123",
			},
			wantErr: false,
		},
		{
			name: "update existing user",
			user: &models.User{
				GithubID: 123,
				Login:    "testuser_updated",
				Token:    "token456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SaveUser(tt.user)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			savedUser, err := store.GetUser(tt.user.GithubID)
			require.NoError(t, err)
			assert.Equal(t, tt.user.Login, savedUser.Login)
			assert.Equal(t, tt.user.Token, savedUser.Token)
		})
	}
}

func TestUserStore_IssueAPIKey(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{GithubID: 123, Login: "testuser", Token: "token123"}
	require.NoError(t, store.SaveUser(user))

	t.Run("new key works and only its hash is stored", func(t *testing.T) {
		key, err := store.IssueAPIKey(context.Background(), user.GithubID)
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(key, "gust_"))
		var cred models.ApiCredential
		require.NoError(t, store.db.Where("github_user_id = ?", user.GithubID).First(&cred).Error)
		assert.NotContains(t, cred.KeyHash+cred.ID, key)
		_, err = store.Spend(context.Background(), key)
		assert.NoError(t, err)
	})

	t.Run("second sign in replaces the key", func(t *testing.T) {
		first, err := store.IssueAPIKey(context.Background(), user.GithubID)
		require.NoError(t, err)
		second, err := store.IssueAPIKey(context.Background(), user.GithubID)
		require.NoError(t, err)

		_, err = store.Spend(context.Background(), first)
		assert.ErrorIs(t, err, ErrInvalidKey)
		_, err = store.Spend(context.Background(), second)
		assert.NoError(t, err)
	})

	t.Run("unknown user has no key", func(t *testing.T) {
		_, err := store.IssueAPIKey(context.Background(), 999)
		assert.Error(t, err)
	})
}

func TestNewUserStore_HashesPlaintextKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	// the production schema before key hashes
	require.NoError(t, old.Exec("CREATE TABLE `users` (`id` integer PRIMARY KEY AUTOINCREMENT,`github_id` integer NOT NULL,`login` text NOT NULL,`name` text,`email` text,`token` text NOT NULL,`created_at` datetime,`last_login` datetime,CONSTRAINT `uni_users_github_id` UNIQUE (`github_id`))").Error)
	require.NoError(t, old.Exec("CREATE TABLE `api_credentials` (`id` text,`github_user_id` integer NOT NULL,`api_key` text NOT NULL,`last_used` datetime,`created_at` datetime,`request_count` integer DEFAULT 0,`daily_request_count` integer DEFAULT 0,`rate_limit_per_day` integer DEFAULT 40,`daily_reset_at` datetime NOT NULL,PRIMARY KEY (`id`),CONSTRAINT `fk_api_credentials_user` FOREIGN KEY (`github_user_id`) REFERENCES `users`(`github_id`),CONSTRAINT `uni_api_credentials_api_key` UNIQUE (`api_key`),CONSTRAINT `uni_api_credentials_github_user_id` UNIQUE (`github_user_id`))").Error)
	require.NoError(t, old.Exec("CREATE INDEX `idx_api_credentials_github_user_id` ON `api_credentials`(`github_user_id`)").Error)
	require.NoError(t, old.Exec(`INSERT INTO users (github_id, login, token) VALUES (7, 'old', 't')`).Error)
	require.NoError(t, old.Exec(`INSERT INTO api_credentials (id, github_user_id, api_key, rate_limit_per_day, daily_reset_at) VALUES ('gust_old', 7, 'gust_old', 50, ?)`, time.Now().UTC()).Error)
	sqlDB, _ := old.DB()
	require.NoError(t, sqlDB.Close())

	store, err := NewUserStore(path)
	require.NoError(t, err)
	defer store.Close()

	usage, err := store.Spend(context.Background(), "gust_old")
	require.NoError(t, err)
	assert.Equal(t, "old", usage.User.Login)
	var cred models.ApiCredential
	require.NoError(t, store.db.First(&cred).Error)
	assert.NotEqual(t, "gust_old", cred.ID)
	assert.False(t, store.db.Migrator().HasColumn("api_credentials", "api_key"))
	assert.True(t, store.db.Migrator().HasIndex(&models.ApiCredential{}, "KeyHash"))
}

func TestUserStore_Spend(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{GithubID: 123, Login: "testuser", Token: "token123"}
	require.NoError(t, store.SaveUser(user))
	key, err := store.IssueAPIKey(context.Background(), user.GithubID)
	require.NoError(t, err)

	t.Run("valid key spends one request", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("github_user_id = ?", user.GithubID).
			Updates(map[string]any{"daily_request_count": 0, "daily_reset_at": time.Now().UTC()}).Error)

		usage, err := store.Spend(context.Background(), key)

		require.NoError(t, err)
		assert.Equal(t, "testuser", usage.User.Login)
		assert.Equal(t, DefaultRateLimitPerDay, usage.Limit)
		assert.Equal(t, DefaultRateLimitPerDay-1, usage.Remaining)
		assert.True(t, usage.Reset.After(time.Now()))
	})

	t.Run("unknown key is invalid", func(t *testing.T) {
		_, err := store.Spend(context.Background(), "gust_nope")

		assert.ErrorIs(t, err, ErrInvalidKey)
	})

	t.Run("spent key is rate limited", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("github_user_id = ?", user.GithubID).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay, "daily_reset_at": time.Now().UTC()}).Error)

		usage, err := store.Spend(context.Background(), key)

		assert.ErrorIs(t, err, ErrRateLimited)
		assert.Equal(t, 0, usage.Remaining)
	})

	t.Run("count resets on a new day", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("github_user_id = ?", user.GithubID).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay, "daily_reset_at": time.Now().UTC().Add(-48 * time.Hour)}).Error)

		usage, err := store.Spend(context.Background(), key)

		require.NoError(t, err)
		assert.Equal(t, DefaultRateLimitPerDay-1, usage.Remaining)
	})

	t.Run("parallel requests never pass the limit", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("github_user_id = ?", user.GithubID).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay - 5, "daily_reset_at": time.Now().UTC()}).Error)

		var wg sync.WaitGroup
		var ok atomic.Int32
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := store.Spend(context.Background(), key); err == nil {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(5), ok.Load())
	})
}

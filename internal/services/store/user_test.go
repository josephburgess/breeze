package store

import (
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestUserStore_GetOrCreateAPICredential(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{
		GithubID: 123,
		Login:    "testuser",
		Token:    "token123",
	}
	require.NoError(t, store.SaveUser(user))

	t.Run("create new credential", func(t *testing.T) {
		cred, err := store.GetOrCreateAPICredential(user.GithubID)
		require.NoError(t, err)
		assert.NotEmpty(t, cred.ApiKey)
		assert.Equal(t, user.GithubID, cred.GithubUserID)
	})

	t.Run("get existing credential", func(t *testing.T) {
		cred1, err := store.GetOrCreateAPICredential(user.GithubID)
		require.NoError(t, err)

		cred2, err := store.GetOrCreateAPICredential(user.GithubID)
		require.NoError(t, err)

		assert.Equal(t, cred1.ApiKey, cred2.ApiKey)
	})

	t.Run("non-existent user", func(t *testing.T) {
		_, err := store.GetOrCreateAPICredential(999)
		assert.Error(t, err)
	})
}

func TestUserStore_ValidateAPIKey(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{
		GithubID: 123,
		Login:    "testuser",
		Token:    "token123",
	}
	require.NoError(t, store.SaveUser(user))

	cred, err := store.GetOrCreateAPICredential(user.GithubID)
	require.NoError(t, err)

	t.Run("valid api key", func(t *testing.T) {
		validatedUser, limit, used, resetTime, err := store.ValidateAPIKey(cred.ApiKey)
		require.NoError(t, err)
		assert.Equal(t, user.GithubID, validatedUser.GithubID)
		assert.Greater(t, limit, 0)
		assert.GreaterOrEqual(t, used, 1)
		assert.False(t, resetTime.IsZero())
	})

	t.Run("invalid api key", func(t *testing.T) {
		_, _, _, _, err := store.ValidateAPIKey("invalid_key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		err := store.db.Model(&models.ApiCredential{}).
			Where("api_key = ?", cred.ApiKey).
			Updates(map[string]interface{}{
				"daily_request_count": 50,
				"daily_reset_at":      time.Now().UTC(),
			}).Error
		require.NoError(t, err)

		_, _, _, _, err = store.ValidateAPIKey(cred.ApiKey)
		assert.Error(t, err)
		var rateLimitErr *RateLimitError
		assert.ErrorAs(t, err, &rateLimitErr)
	})

	t.Run("daily reset", func(t *testing.T) {
		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		err := store.db.Model(&models.ApiCredential{}).
			Where("api_key = ?", cred.ApiKey).
			Updates(map[string]interface{}{
				"daily_reset_at":      yesterday,
				"daily_request_count": 50,
			}).Error
		require.NoError(t, err)

		validatedUser, limit, used, resetTime, err := store.ValidateAPIKey(cred.ApiKey)
		require.NoError(t, err)
		assert.Equal(t, user.GithubID, validatedUser.GithubID)
		assert.Greater(t, limit, 0)
		assert.Equal(t, 1, used)
		assert.False(t, resetTime.IsZero())

		var updatedCred models.ApiCredential
		err = store.db.Where("api_key = ?", cred.ApiKey).First(&updatedCred).Error
		require.NoError(t, err)
		assert.Equal(t, 1, updatedCred.DailyRequestCount)
	})

	t.Run("request counting", func(t *testing.T) {
		err := store.db.Model(&models.ApiCredential{}).
			Where("api_key = ?", cred.ApiKey).
			Updates(map[string]interface{}{
				"daily_request_count": 0,
				"daily_reset_at":      time.Now().UTC(),
			}).Error
		require.NoError(t, err)

		numRequests := 3
		for i := 0; i < numRequests; i++ {
			_, _, _, _, err := store.ValidateAPIKey(cred.ApiKey)
			require.NoError(t, err)
		}

		var updatedCred models.ApiCredential
		err = store.db.Where("api_key = ?", cred.ApiKey).First(&updatedCred).Error
		require.NoError(t, err)
		assert.Equal(t, numRequests, updatedCred.DailyRequestCount)
	})
}

func TestUserStore_CreateAPICredential(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{
		GithubID: 123,
		Login:    "testuser",
		Token:    "token123",
	}
	require.NoError(t, store.SaveUser(user))

	t.Run("create credential for existing user", func(t *testing.T) {
		cred, err := store.CreateAPICredential(user.GithubID)
		require.NoError(t, err)
		assert.NotEmpty(t, cred.ApiKey)
		assert.Equal(t, user.GithubID, cred.GithubUserID)
	})

	t.Run("create credential for non-existent user", func(t *testing.T) {
		_, err := store.CreateAPICredential(999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user with ID 999 not found")
	})

	t.Run("prevent duplicate credentials", func(t *testing.T) {
		_, err := store.CreateAPICredential(user.GithubID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user already has an API key")
	})
}

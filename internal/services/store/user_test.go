package store

import (
	"context"
	"sync"
	"sync/atomic"
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

func TestUserStore_Spend(t *testing.T) {
	store := setupTestDB(t)

	user := &models.User{GithubID: 123, Login: "testuser", Token: "token123"}
	require.NoError(t, store.SaveUser(user))
	cred, err := store.GetOrCreateAPICredential(user.GithubID)
	require.NoError(t, err)

	t.Run("valid key spends one request", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("api_key = ?", cred.ApiKey).
			Updates(map[string]any{"daily_request_count": 0, "daily_reset_at": time.Now().UTC()}).Error)

		usage, err := store.Spend(context.Background(), cred.ApiKey)

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
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("api_key = ?", cred.ApiKey).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay, "daily_reset_at": time.Now().UTC()}).Error)

		usage, err := store.Spend(context.Background(), cred.ApiKey)

		assert.ErrorIs(t, err, ErrRateLimited)
		assert.Equal(t, 0, usage.Remaining)
	})

	t.Run("count resets on a new day", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("api_key = ?", cred.ApiKey).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay, "daily_reset_at": time.Now().UTC().Add(-48 * time.Hour)}).Error)

		usage, err := store.Spend(context.Background(), cred.ApiKey)

		require.NoError(t, err)
		assert.Equal(t, DefaultRateLimitPerDay-1, usage.Remaining)
	})

	t.Run("parallel requests never pass the limit", func(t *testing.T) {
		require.NoError(t, store.db.Model(&models.ApiCredential{}).Where("api_key = ?", cred.ApiKey).
			Updates(map[string]any{"daily_request_count": DefaultRateLimitPerDay - 5, "daily_reset_at": time.Now().UTC()}).Error)

		var wg sync.WaitGroup
		var ok atomic.Int32
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := store.Spend(context.Background(), cred.ApiKey); err == nil {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(5), ok.Load())
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

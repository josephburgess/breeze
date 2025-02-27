package models

import "time"

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Token     string `json:"-"`
}

type ApiCredential struct {
	ID           string    `json:"id"`
	GithubUserID int64     `json:"github_user_id"`
	ApiKey       string    `json:"api_key"`
	LastUsed     time.Time `json:"last_used"`
	CreatedAt    time.Time `json:"created_at"`
	RequestCount int       `json:"request_count"`
}

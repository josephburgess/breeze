package models

import (
	"time"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GithubID  int64     `gorm:"unique;not null" json:"github_id"`
	Login     string    `gorm:"not null" json:"login"`
	Name      *string   `json:"name,omitempty"`
	Email     *string   `json:"email,omitempty"`
	Token     string    `gorm:"not null" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	LastLogin time.Time `gorm:"autoUpdateTime" json:"last_login"`
}

type ApiCredential struct {
	ID                string     `gorm:"primaryKey" json:"id"`
	GithubUserID      int64      `gorm:"not null;unique;index" json:"github_user_id"`
	ApiKey            string     `gorm:"unique;not null" json:"api_key"`
	LastUsed          *time.Time `json:"last_used,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	RequestCount      int        `gorm:"default:0" json:"request_count"`
	User              User       `gorm:"foreignKey:GithubUserID;references:GithubID" json:"-"`
	DailyRequestCount int        `gorm:"default:0"`
	RateLimitPerDay   int        `gorm:"default:40"`
	DailyResetAt      time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

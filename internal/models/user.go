package models

import (
	"time"
)

type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GithubID    int64     `gorm:"unique;not null" json:"github_id"`
	Login       string    `gorm:"not null" json:"login"`
	Name        *string   `json:"name,omitempty"`
	Email       *string   `json:"email,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Token       string    `gorm:"not null" json:"-"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	LastLogin   time.Time `gorm:"autoUpdateTime" json:"last_login"`
	StarredRepo bool      `gorm:"default:false" json:"starred_repo"`
}

type ApiCredential struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	GithubUserID int64      `gorm:"not null;index" json:"github_user_id"`
	ApiKey       string     `gorm:"unique;not null" json:"api_key"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	RequestCount int        `gorm:"default:0" json:"request_count"`

	User User `gorm:"foreignKey:GithubUserID;references:GithubID" json:"-"`
}

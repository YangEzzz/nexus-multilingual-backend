package models

import (
	"time"
)

// User represents a user synchronized from the external management system
type User struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ExternalID string     `gorm:"uniqueIndex;not null;size:100" json:"external_id"`
	Nickname   string     `gorm:"size:255" json:"nickname"`
	AvatarURL  string     `gorm:"size:512" json:"avatar_url"`
	Role       string     `gorm:"size:50" json:"role"` // Cached role in this project
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

package models

import "time"

// AICredential is intentionally scoped to one user and one fixed relay.
// A separate table avoids coupling this simplified relay configuration to any
// historical multi-provider credential schema.
type AICredential struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_ai_relay_credential_user" json:"-"`
	EncryptedKey string    `gorm:"type:text;not null" json:"-"`
	KeyLastFour  string    `gorm:"size:4;not null" json:"-"`
	Model        string    `gorm:"size:255;not null" json:"model"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AICredential) TableName() string {
	return "ai_relay_credentials"
}

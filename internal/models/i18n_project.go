package models

import "time"

// I18nProject represents an internal translation project within Nexus.
// This is distinct from the external management system's project concept.
type I18nProject struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Code        string    `gorm:"uniqueIndex;size:100;not null" json:"code"` // unique project identifier e.g. "nexus-frontend"
	Description string    `gorm:"type:text" json:"description"`
	AiPrompt    string    `gorm:"type:text" json:"ai_prompt"`
	CreatedByID uint      `gorm:"index" json:"created_by_id"` // FK -> users.id
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Associations
	Languages []Language `gorm:"foreignKey:ProjectID" json:"languages"`
}

package models

import "time"

// AvailableLanguage is a language that can be selected when configuring a project.
// Project-specific language selections remain stored in Language.
type AvailableLanguage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"uniqueIndex;size:20;not null" json:"code"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	SortOrder int       `gorm:"default:0;index" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

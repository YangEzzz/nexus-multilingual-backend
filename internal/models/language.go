package models

import "time"

// Language represents a language enabled for a specific i18n project.
type Language struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index;not null" json:"project_id"` // FK -> i18n_projects.id
	Code      string    `gorm:"size:20;not null" json:"code"`     // e.g. "en", "cn", "cht"
	Name      string    `gorm:"size:100;not null" json:"name"`    // e.g. "英文", "简体中文"
	IsSource  bool      `gorm:"default:false" json:"is_source"`   // true = primary/source language
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

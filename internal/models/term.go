package models

import "time"

// TermStatus mirrors frontend status values
type TermStatus string

const (
	TermStatusDraft     TermStatus = "draft"
	TermStatusPending   TermStatus = "pending"
	TermStatusReview    TermStatus = "review"
	TermStatusPublished TermStatus = "published"
)

// Term is the core i18n entry (a key in a specific module of a project).
// Composite unique index: (project_id, module, key)
type Term struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ProjectID   uint       `gorm:"not null;uniqueIndex:idx_project_module_key" json:"project_id"` // FK -> i18n_projects.id
	Module      string     `gorm:"size:100;uniqueIndex:idx_project_module_key" json:"module"` // e.g. "common", "auth". Empty means global.
	Key         string     `gorm:"size:255;not null;uniqueIndex:idx_project_module_key" json:"key"` // e.g. "confirm", "username_placeholder"
	Description string     `gorm:"type:text" json:"description"`
	Status      TermStatus `gorm:"size:20;default:'draft'" json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Associations
	Translations []Translation `gorm:"foreignKey:TermID" json:"translations,omitempty"`
	HistoryLogs  []HistoryLog  `gorm:"foreignKey:TermID" json:"history_logs,omitempty"`
}

// Ensure composite unique index at DB level
func (Term) TableName() string { return "terms" }

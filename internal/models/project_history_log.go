package models

import "time"

// ProjectHistoryLog records one project-level operation that can affect many terms.
type ProjectHistoryLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index;not null" json:"project_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Action    string    `gorm:"size:500;not null" json:"action"`
	ItemCount int       `gorm:"not null;default:1" json:"item_count"`
	TermKeys  []string  `gorm:"serializer:json;type:jsonb" json:"term_keys,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	Project I18nProject `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE" json:"-"`
	User    User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

package models

import "time"

// LocaleData keeps the published values independently of the output format.
// language -> module -> key -> translation; an empty module is the global scope.
type LocaleData map[string]map[string]map[string]string

type LocaleRelease struct {
	ID          uint                `gorm:"primaryKey" json:"id"`
	ProjectID   uint                `gorm:"not null;uniqueIndex:idx_locale_release_version" json:"project_id"`
	Version     int                 `gorm:"not null;uniqueIndex:idx_locale_release_version" json:"version"`
	Data        LocaleData          `gorm:"type:jsonb;serializer:json;not null" json:"-"`
	PublishedBy uint                `gorm:"not null" json:"published_by"`
	CreatedAt   time.Time           `json:"created_at"`
	Files       []LocaleReleaseFile `gorm:"foreignKey:ReleaseID" json:"files"`
}

type LocaleReleaseFile struct {
	ID           uint   `gorm:"primaryKey" json:"-"`
	ReleaseID    uint   `gorm:"not null;uniqueIndex:idx_locale_release_file" json:"-"`
	LanguageCode string `gorm:"size:20;not null;uniqueIndex:idx_locale_release_file" json:"language_code"`
	Content      string `gorm:"type:text;not null" json:"-"`
	SHA256       string `gorm:"size:64;not null" json:"-"`
}

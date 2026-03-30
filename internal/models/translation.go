package models


// Translation stores the actual translated text for a specific term in a specific language.
// Composite unique index: (term_id, language_code)
type Translation struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	TermID       uint   `gorm:"uniqueIndex:idx_term_lang;not null" json:"term_id"`          // FK -> terms.id
	LanguageCode string `gorm:"size:20;uniqueIndex:idx_term_lang;not null" json:"language_code"` // e.g. "en", "cn", "cht"
	Content      string `gorm:"type:text" json:"content"`
}

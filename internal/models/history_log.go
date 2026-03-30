package models

import "time"

// HistoryLog records each change event on a term (mirrors frontend term.history array).
type HistoryLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TermID    uint      `gorm:"index;not null" json:"term_id"` // FK -> terms.id
	UserID    uint      `gorm:"index" json:"user_id"`          // FK -> users.id (0 = system/AI)
	Action    string    `gorm:"size:500;not null" json:"action"` // e.g. "更新了 en 翻译", "将状态变更为 已发布"
	CreatedAt time.Time `json:"created_at"`

	Term Term `gorm:"foreignKey:TermID" json:"term,omitempty"`
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

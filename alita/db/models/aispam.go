package models

import "time"

// AISpamSettings is the per-chat opt-in switch for the AI spam filter.
// A missing row means the filter is disabled for that chat.
type AISpamSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled   bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (AISpamSettings) TableName() string {
	return "ai_spam_settings"
}

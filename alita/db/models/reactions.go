package models

import "time"

// Reactions stores per-chat keyword-to-emoji reaction mappings.
type Reactions struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_reactions_chat_keyword" json:"chat_id,omitempty"`
	Keyword   string    `gorm:"column:keyword;not null;uniqueIndex:idx_reactions_chat_keyword" json:"keyword,omitempty"`
	Emoji     string    `gorm:"column:emoji;not null" json:"emoji,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Reactions) TableName() string {
	return "reactions"
}

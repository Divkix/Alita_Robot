package models

import "time"

// RulesSettings represents rules settings for a chat
type RulesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Rules     string    `gorm:"column:rules;type:text" json:"rules,omitempty"`
	RulesBtn  string    `gorm:"column:rules_btn" json:"rules_btn,omitempty"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (RulesSettings) TableName() string {
	return "rules"
}

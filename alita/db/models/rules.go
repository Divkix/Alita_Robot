package models

import "time"

// RulesSettings represents rules settings for a chat
type RulesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	Rules     string    `gorm:"column:rules;type:text" json:"rules,omitempty" bson:"rules"`
	RulesBtn  string    `gorm:"column:rules_btn" json:"rules_btn,omitempty" bson:"rules_btn"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty" bson:"private"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (RulesSettings) TableName() string {
	return "rules"
}

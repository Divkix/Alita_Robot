package models

import "time"

// SpamDBSettings holds a chat's CAS join check: whether it runs and what it
// does to a listed user. A missing row means disabled with the ban action.
type SpamDBSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled   bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty"`
	Action    string    `gorm:"column:action;default:'ban';check:chk_spam_db_action,action IN ('ban','mute','log')" json:"action,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (SpamDBSettings) TableName() string {
	return "spam_db_settings"
}

package models

import "time"

// LockSettings represents lock settings for a chat
type LockSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_lock_chat_type" json:"chat_id,omitempty"`
	LockType  string    `gorm:"column:lock_type;not null;uniqueIndex:idx_lock_chat_type" json:"lock_type,omitempty"`
	Locked    bool      `gorm:"column:locked;default:false" json:"locked,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (LockSettings) TableName() string {
	return "locks"
}

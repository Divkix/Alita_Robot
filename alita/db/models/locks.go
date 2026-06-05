package models

import "time"

// LockSettings represents lock settings for a chat
type LockSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_lock_chat_type" json:"chat_id,omitempty" bson:"chat_id"`
	LockType  string    `gorm:"column:lock_type;not null;uniqueIndex:idx_lock_chat_type" json:"lock_type,omitempty" bson:"lock_type"`
	Locked    bool      `gorm:"column:locked;default:false" json:"locked,omitempty" bson:"locked"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (LockSettings) TableName() string {
	return "locks"
}

package models

import "time"

// ChannelSettings represents channel settings including channel metadata
type ChannelSettings struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	ChannelId   int64     `gorm:"column:channel_id" json:"channel_id,omitempty"`
	ChannelName string    `gorm:"column:channel_name" json:"channel_name,omitempty"`
	Username    string    `gorm:"column:username" json:"username,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ChannelSettings) TableName() string {
	return "channels"
}

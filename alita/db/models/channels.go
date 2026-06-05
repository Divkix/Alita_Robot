package models

import "time"

// ChannelSettings represents channel settings including channel metadata
type ChannelSettings struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId      int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	ChannelId   int64     `gorm:"column:channel_id" json:"channel_id,omitempty" bson:"channel_id"`
	ChannelName string    `gorm:"column:channel_name" json:"channel_name,omitempty" bson:"channel_name"`
	Username    string    `gorm:"column:username" json:"username,omitempty" bson:"username"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ChannelSettings) TableName() string {
	return "channels"
}

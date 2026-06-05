package models

import "time"

// ConnectionSettings represents connection settings
type ConnectionSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserId    int64     `gorm:"column:user_id;not null;index:idx_connection_user_chat" json:"user_id,omitempty" bson:"user_id"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_connection_user_chat" json:"chat_id,omitempty" bson:"chat_id"`
	Connected bool      `gorm:"column:connected;default:false" json:"connected,omitempty" bson:"connected"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ConnectionSettings) TableName() string {
	return "connection"
}

// ConnectionChatSettings represents connection chat settings for a chat.
// AllowConnect determines whether users can connect to this chat remotely.
type ConnectionChatSettings struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	AllowConnect bool      `gorm:"column:allow_connect;default:true" json:"allow_connect,omitempty" bson:"allow_connect"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ConnectionChatSettings) TableName() string {
	return "connection_settings"
}

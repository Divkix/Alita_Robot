package models

import "time"

// ConnectionSettings represents connection settings
type ConnectionSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;not null;index:idx_connection_user_chat" json:"user_id,omitempty"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_connection_user_chat" json:"chat_id,omitempty"`
	Connected bool      `gorm:"column:connected;default:false" json:"connected,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ConnectionSettings) TableName() string {
	return "connection"
}

// ConnectionChatSettings represents connection chat settings for a chat.
// AllowConnect determines whether users can connect to this chat remotely.
type ConnectionChatSettings struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	AllowConnect bool      `gorm:"column:allow_connect;default:true" json:"allow_connect,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ConnectionChatSettings) TableName() string {
	return "connection_settings"
}

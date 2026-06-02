package models

import "time"

// User represents a user in the system
type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId       int64     `gorm:"column:user_id;uniqueIndex;not null" json:"_id,omitempty"`
	UserName     string    `gorm:"column:username;index" json:"username" default:"nil"`
	Name         string    `gorm:"column:name" json:"name" default:"nil"`
	Language     string    `gorm:"column:language;default:'en'" json:"language" default:"en"`
	LastActivity time.Time `gorm:"column:last_activity" json:"last_activity,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`

	// Note: Chat membership is managed via JSONB users field in chats table
}

func (User) TableName() string {
	return "users"
}

// Chat represents a chat/group in the system
type Chat struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId       int64      `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	ChatName     string     `gorm:"column:chat_name" json:"chat_name" default:"nil"`
	Language     string     `gorm:"column:language" json:"language" default:"nil"`
	Users        Int64Array `gorm:"column:users;type:jsonb" json:"users" default:"nil"`
	IsInactive   bool       `gorm:"column:is_inactive;default:false" json:"is_inactive" default:"false"`
	LastActivity time.Time  `gorm:"column:last_activity" json:"last_activity,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`

	// Note: User membership is managed via JSONB users field
}

func (Chat) TableName() string {
	return "chats"
}

// ChatUser represents the many-to-many relationship between chats and users
type ChatUser struct {
	ChatID int64 `gorm:"column:chat_id;primaryKey" json:"chat_id"`
	UserID int64 `gorm:"column:user_id;primaryKey" json:"user_id"`
}

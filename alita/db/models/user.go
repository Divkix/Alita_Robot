package models

import "time"

// User represents a user in the system
type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserId       int64     `gorm:"column:user_id;uniqueIndex;not null" json:"_id,omitempty" bson:"user_id"`
	UserName     string    `gorm:"column:username;index" json:"username" default:"nil" bson:"username"`
	Name         string    `gorm:"column:name" json:"name" default:"nil" bson:"name"`
	Language     string    `gorm:"column:language;default:'en'" json:"language" default:"en" bson:"language"`
	LastActivity time.Time `gorm:"column:last_activity" json:"last_activity,omitempty" bson:"last_activity"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`

	// Note: Chat membership is managed via JSONB users field in chats table
}

func (User) TableName() string {
	return "users"
}

// Chat represents a chat/group in the system
type Chat struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId       int64      `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty" bson:"chat_id"`
	ChatName     string     `gorm:"column:chat_name" json:"chat_name" default:"nil" bson:"chat_name"`
	Language     string     `gorm:"column:language" json:"language" default:"nil" bson:"language"`
	Users        Int64Array `gorm:"column:users;type:jsonb" json:"users" default:"nil" bson:"users"`
	IsInactive   bool       `gorm:"column:is_inactive;default:false" json:"is_inactive" default:"false" bson:"is_inactive"`
	LastActivity time.Time  `gorm:"column:last_activity" json:"last_activity,omitempty" bson:"last_activity"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`

	// Note: User membership is managed via JSONB users field
}

func (Chat) TableName() string {
	return "chats"
}

// ChatUser represents the many-to-many relationship between chats and users
type ChatUser struct {
	ChatID int64 `gorm:"column:chat_id;primaryKey" json:"chat_id" bson:"chat_id"`
	UserID int64 `gorm:"column:user_id;primaryKey" json:"user_id" bson:"user_id"`
}

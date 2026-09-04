package models

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId       int64     `gorm:"column:user_id;uniqueIndex;not null" json:"_id,omitempty"`
	UserName     string    `gorm:"column:username;index" json:"username" default:"nil"`
	Name         string    `gorm:"column:name" json:"name" default:"nil"`
	Language     string    `gorm:"column:language;default:'en'" json:"language" default:"en"`
	LastActivity time.Time `gorm:"column:last_activity" json:"last_activity,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}

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
}

func (Chat) TableName() string {
	return "chats"
}

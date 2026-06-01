package models

import "time"

// AdminSettings represents admin settings for a chat
type AdminSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	AnonAdmin bool      `gorm:"column:anon_admin;default:false" json:"anon_admin"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (AdminSettings) TableName() string {
	return "admin"
}

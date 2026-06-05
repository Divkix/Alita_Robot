package models

import "time"

// AdminSettings represents admin settings for a chat
type AdminSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty" bson:"chat_id"`
	AnonAdmin bool      `gorm:"column:anon_admin;default:false" json:"anon_admin" bson:"anon_admin"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (AdminSettings) TableName() string {
	return "admin"
}

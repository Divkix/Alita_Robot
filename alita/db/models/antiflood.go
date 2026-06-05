package models

import "time"

// AntifloodSettings represents antiflood settings for a chat
type AntifloodSettings struct {
	ID                     uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId                 int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	Limit                  int       `gorm:"column:flood_limit;default:5;check:chk_antiflood_limit,flood_limit >= 0" json:"limit,omitempty" bson:"flood_limit"`
	Action                 string    `gorm:"column:action;default:'mute';check:chk_antiflood_action,action IN ('mute','ban','kick','warn','tban','tmute')" json:"action,omitempty" bson:"action"`
	DeleteAntifloodMessage bool      `gorm:"column:delete_antiflood_message;default:false" json:"delete_antiflood_message,omitempty" bson:"delete_antiflood_message"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (AntifloodSettings) TableName() string {
	return "antiflood_settings"
}

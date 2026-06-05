package models

import "time"

// PinSettings represents pin settings for a chat
type PinSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId         int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	MsgId          int64     `gorm:"column:msg_id" json:"msg_id,omitempty" bson:"msg_id"`
	CleanLinked    bool      `gorm:"column:clean_linked;default:false" json:"clean_linked,omitempty" bson:"clean_linked"`
	AntiChannelPin bool      `gorm:"column:anti_channel_pin;default:false" json:"anti_channel_pin,omitempty" bson:"anti_channel_pin"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (PinSettings) TableName() string {
	return "pins"
}

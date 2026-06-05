package models

import "time"

// WarnSettings represents warning settings for a chat
type WarnSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty" bson:"chat_id"`
	WarnLimit int       `gorm:"column:warn_limit;default:3;check:chk_warn_limit,warn_limit > 0" json:"warn_limit" default:"3" bson:"warn_limit"`
	WarnMode  string    `gorm:"column:warn_mode;check:chk_warn_mode,warn_mode = '' OR warn_mode IN ('ban','kick','mute','tban','tmute')" json:"warn_mode,omitempty" bson:"warn_mode"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (WarnSettings) TableName() string {
	return "warns_settings"
}

// Warns represents user warnings in a chat
type Warns struct {
	ID        uint        `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserId    int64       `gorm:"column:user_id;not null;index:idx_warns_user_chat" json:"user_id,omitempty" bson:"user_id"`
	ChatId    int64       `gorm:"column:chat_id;not null;index:idx_warns_user_chat" json:"chat_id,omitempty" bson:"chat_id"`
	NumWarns  int         `gorm:"column:num_warns;default:0;check:chk_warns_num_warns,num_warns >= 0" json:"num_warns,omitempty" bson:"num_warns"`
	Reasons   StringArray `gorm:"column:warns;type:jsonb" json:"warns" default:"[]" bson:"warns"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (Warns) TableName() string {
	return "warns_users"
}

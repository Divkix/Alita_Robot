package models

import "time"

type WarnSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	WarnLimit int       `gorm:"column:warn_limit;default:3;check:chk_warn_limit,warn_limit > 0" json:"warn_limit" default:"3"`
	WarnMode  string    `gorm:"column:warn_mode;check:chk_warn_mode,warn_mode = '' OR warn_mode IN ('ban','kick','mute','tban','tmute')" json:"warn_mode,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (WarnSettings) TableName() string {
	return "warns_settings"
}

type Warns struct {
	ID        uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64       `gorm:"column:user_id;not null;uniqueIndex:uk_warns_users_user_chat" json:"user_id,omitempty"`
	ChatId    int64       `gorm:"column:chat_id;not null;uniqueIndex:uk_warns_users_user_chat;index:idx_warns_users_chat_id" json:"chat_id,omitempty"`
	NumWarns  int         `gorm:"column:num_warns;default:0;check:chk_warns_num_warns,num_warns >= 0" json:"num_warns,omitempty"`
	Reasons   StringArray `gorm:"column:warns;type:jsonb" json:"warns" default:"[]"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Warns) TableName() string {
	return "warns_users"
}

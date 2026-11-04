package models

import "time"

type AntifloodSettings struct {
	ID     uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId int64  `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Limit  int    `gorm:"column:flood_limit;default:5;check:chk_antiflood_limit,flood_limit >= 0" json:"limit,omitempty"`
	Action string `gorm:"column:action;default:'mute';check:chk_antiflood_action,action IN ('mute','ban','kick','warn','tban','tmute')" json:"action,omitempty"`
	// FloodActionTime is the tban/tmute duration in seconds; 0 means not set.
	FloodActionTime        int       `gorm:"column:flood_action_time;not null;default:0;check:chk_flood_action_time,flood_action_time >= 0" json:"flood_action_time,omitempty"`
	DeleteAntifloodMessage bool      `gorm:"column:delete_antiflood_message;default:false" json:"delete_antiflood_message,omitempty"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (AntifloodSettings) TableName() string {
	return "antiflood_settings"
}

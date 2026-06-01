package models

import "time"

// DisableSettings represents disable settings for commands
type DisableSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId         int64     `gorm:"column:chat_id;not null;index:idx_disable_chat_command" json:"chat_id,omitempty"`
	Command        string    `gorm:"column:command;not null;index:idx_disable_chat_command" json:"command,omitempty"`
	Disabled       bool      `gorm:"column:disabled;default:true" json:"disabled,omitempty"`
	DeleteCommands bool      `gorm:"column:delete_commands;default:false" json:"delete_commands,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (DisableSettings) TableName() string {
	return "disable"
}

// DisableChatSettings represents chat-level disable settings
type DisableChatSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId         int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	DeleteCommands bool      `gorm:"column:delete_commands;default:false" json:"delete_commands,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (DisableChatSettings) TableName() string {
	return "disable_chat_settings"
}

package models

import "time"

// EphemeralSettings is a chat's /ephemeral switch. A chat without a row is off.
type EphemeralSettings struct {
	ChatID    int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled   bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (EphemeralSettings) TableName() string {
	return "ephemeral_settings"
}

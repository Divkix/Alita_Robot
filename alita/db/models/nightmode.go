package models

import (
	"strings"
	"time"
)

// NightModeSettings is a chat's night mode: lock types enforced every day
// from StartMinute to EndMinute (minutes after local midnight in the chat's
// timezone). A chat without a row, or with Enabled false, has none.
type NightModeSettings struct {
	ChatID      int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled     bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	StartMinute int       `gorm:"column:start_minute;not null;default:0" json:"start_minute"`
	EndMinute   int       `gorm:"column:end_minute;not null;default:0" json:"end_minute"`
	LockTypes   string    `gorm:"column:lock_types;not null;default:''" json:"lock_types"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (NightModeSettings) TableName() string {
	return "night_mode_settings"
}

// Types returns the lock types night mode applies, as /lock names them.
func (s NightModeSettings) Types() []string {
	if s.LockTypes == "" {
		return nil
	}
	return strings.Split(s.LockTypes, ",")
}

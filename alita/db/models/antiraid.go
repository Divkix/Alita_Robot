package models

import "time"

type AntiRaidSettings struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID                int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	RaidTime              int       `gorm:"column:raid_time;default:21600" json:"raid_time,omitempty"`
	RaidActionTime        int       `gorm:"column:raid_action_time;default:3600" json:"raid_action_time,omitempty"`
	AutoAntiRaidThreshold int       `gorm:"column:auto_antiraid_threshold;default:0" json:"auto_antiraid_threshold,omitempty"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (AntiRaidSettings) TableName() string {
	return "antiraid_settings"
}

// DefaultAntiRaidSettings is the row a chat without one reads as: raid
// window 6 hours, action window 1 hour, auto anti-raid off. Repository
// loaders and settings declarations consume it.
func DefaultAntiRaidSettings(chatID int64) *AntiRaidSettings {
	return &AntiRaidSettings{
		ChatID:                chatID,
		RaidTime:              21600,
		RaidActionTime:        3600,
		AutoAntiRaidThreshold: 0,
	}
}

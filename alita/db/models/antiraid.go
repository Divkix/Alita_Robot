package models

import "time"

// AntiRaidSettings stores per-chat anti-raid configuration.
type AntiRaidSettings struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatID                int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	RaidTime              int       `gorm:"column:raid_time;default:21600" json:"raid_time,omitempty" bson:"raid_time"`
	RaidActionTime        int       `gorm:"column:raid_action_time;default:3600" json:"raid_action_time,omitempty" bson:"raid_action_time"`
	AutoAntiRaidThreshold int       `gorm:"column:auto_antiraid_threshold;default:0" json:"auto_antiraid_threshold,omitempty" bson:"auto_antiraid_threshold"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (AntiRaidSettings) TableName() string {
	return "antiraid_settings"
}

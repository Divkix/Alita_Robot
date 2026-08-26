package models

import "time"

// LogChannel binds a group chat to a Telegram channel that receives admin logs.
type LogChannel struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id"`
	LogChannelID int64     `gorm:"column:log_channel_id;not null" json:"log_channel_id"`
	CatSettings  bool      `gorm:"column:cat_settings;default:true" json:"cat_settings"`
	CatAdmin     bool      `gorm:"column:cat_admin;default:true" json:"cat_admin"`
	CatUser      bool      `gorm:"column:cat_user;default:true" json:"cat_user"`
	CatAutomated bool      `gorm:"column:cat_automated;default:true" json:"cat_automated"`
	CatReports   bool      `gorm:"column:cat_reports;default:true" json:"cat_reports"`
	CatOther     bool      `gorm:"column:cat_other;default:true" json:"cat_other"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (LogChannel) TableName() string {
	return "log_channels"
}

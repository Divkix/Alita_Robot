package models

import "time"

// ChatTimezone is a chat's IANA timezone name, such as "Asia/Kolkata". A chat
// without a row is on UTC.
type ChatTimezone struct {
	ChatID    int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Timezone  string    `gorm:"column:timezone;not null;default:'UTC'" json:"timezone"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ChatTimezone) TableName() string {
	return "chat_timezones"
}

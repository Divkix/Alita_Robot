package models

import "time"

// ReportChatSettings represents report settings for a chat
type ReportChatSettings struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId      int64      `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	Enabled     bool       `gorm:"column:enabled;default:true" json:"enabled,omitempty" bson:"enabled"`
	Status      bool       `gorm:"column:status;default:true" json:"status,omitempty" bson:"status"`             // Alias for Enabled for compatibility
	BlockedList Int64Array `gorm:"column:blocked_list;type:jsonb" json:"blocked_list,omitempty" bson:"blocked_list"` // List of blocked user IDs
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ReportChatSettings) TableName() string {
	return "report_chat_settings"
}

// ReportUserSettings represents report settings for a user
type ReportUserSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserId    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id,omitempty" bson:"user_id"`
	Enabled   bool      `gorm:"column:enabled;default:true" json:"enabled,omitempty" bson:"enabled"`
	Status    bool      `gorm:"column:status;default:true" json:"status,omitempty" bson:"status"` // Alias for Enabled for compatibility
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ReportUserSettings) TableName() string {
	return "report_user_settings"
}

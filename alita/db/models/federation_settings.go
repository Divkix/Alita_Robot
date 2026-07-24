package models

import "time"

// FederationSettings represents configurable federation-wide settings.
type FederationSettings struct {
	ID                   uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FederationID         uint      `gorm:"column:federation_id;uniqueIndex;not null" json:"federation_id,omitempty"`
	RequireReason        bool      `gorm:"column:require_reason;default:false" json:"require_reason,omitempty"`
	NotificationsEnabled bool      `gorm:"column:notifications_enabled;default:true" json:"notifications_enabled,omitempty"`
	LogChatID            int64     `gorm:"column:log_chat_id" json:"log_chat_id,omitempty"`
	CreatedAt            time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt            time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (FederationSettings) TableName() string {
	return "federation_settings"
}

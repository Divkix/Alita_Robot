package models

import "time"

// FederationChat represents a chat that has joined a federation.
type FederationChat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FederationID uint      `gorm:"column:federation_id;not null" json:"federation_id,omitempty"`
	ChatID       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	JoinedBy     int64     `gorm:"column:joined_by;not null" json:"joined_by,omitempty"`
	JoinedAt     time.Time `gorm:"column:joined_at" json:"joined_at,omitempty"`
	Quiet        bool      `gorm:"column:quiet;default:false" json:"quiet,omitempty"`
}

func (FederationChat) TableName() string {
	return "federation_chats"
}

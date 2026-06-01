package models

import "time"

// ApprovedUsers represents approved users per chat who are immune to anti-spam measures
type ApprovedUsers struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID     int64     `gorm:"column:user_id;not null;uniqueIndex:idx_approved_users_chat_user" json:"user_id,omitempty"`
	ChatID     int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_approved_users_chat_user" json:"chat_id,omitempty"`
	Reason     string    `gorm:"column:reason;default:''" json:"reason,omitempty"`
	ApprovedBy int64     `gorm:"column:approved_by;not null;default:0" json:"approved_by,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ApprovedUsers) TableName() string {
	return "approved_users"
}

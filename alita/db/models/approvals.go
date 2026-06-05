package models

import "time"

// ApprovedUsers represents approved users per chat who are immune to anti-spam measures
type ApprovedUsers struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserID     int64     `gorm:"column:user_id;not null;uniqueIndex:idx_approved_users_chat_user" json:"user_id,omitempty" bson:"user_id"`
	ChatID     int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_approved_users_chat_user" json:"chat_id,omitempty" bson:"chat_id"`
	Reason     string    `gorm:"column:reason;default:''" json:"reason,omitempty" bson:"reason"`
	ApprovedBy int64     `gorm:"column:approved_by;not null;default:0" json:"approved_by,omitempty" bson:"approved_by"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ApprovedUsers) TableName() string {
	return "approved_users"
}

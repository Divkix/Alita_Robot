package models

import "time"

// DigestSettings is one chat's weekly digest opt-in and destination.
//
// Destination is "" (follow the default: the log channel when the chat has
// one, otherwise the group), "log" or "group". LastSentAt is the UTC instant
// a replica claimed and completed the last send; nil means the chat has never
// had one. The conditional update on it is what makes a week's digest go out
// once across replicas.
type DigestSettings struct {
	ChatID      int64      `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled     bool       `gorm:"column:enabled;not null;default:false" json:"enabled"`
	Destination string     `gorm:"column:destination;not null;default:'';check:chk_digest_settings_destination,destination IN ('','log','group')" json:"destination,omitempty"`
	LastSentAt  *time.Time `gorm:"column:last_sent_at" json:"last_sent_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (DigestSettings) TableName() string {
	return "digest_settings"
}

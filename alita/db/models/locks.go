package models

import "time"

type LockSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_lock_chat_type" json:"chat_id,omitempty"`
	LockType  string    `gorm:"column:lock_type;not null;uniqueIndex:idx_lock_chat_type" json:"lock_type,omitempty"`
	Locked    bool      `gorm:"column:locked;default:false" json:"locked,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (LockSettings) TableName() string {
	return "locks"
}

// LockAllowlist is one exception a chat's /allowlist holds: kind "host"
// exempts an exact host (and its subdomains) from the url lock, kind
// "invite" exempts one stable invite identity from the invitelink lock.
type LockAllowlist struct {
	ChatId    int64     `gorm:"column:chat_id;primaryKey" json:"chat_id,omitempty"`
	Kind      string    `gorm:"column:kind;primaryKey" json:"kind,omitempty"`
	Value     string    `gorm:"column:value;primaryKey" json:"value,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (LockAllowlist) TableName() string {
	return "lock_allowlist"
}

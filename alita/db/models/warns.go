package models

import "time"

type WarnSettings struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64  `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	WarnLimit int    `gorm:"column:warn_limit;default:3;check:chk_warn_limit,warn_limit > 0" json:"warn_limit" default:"3"`
	WarnMode  string `gorm:"column:warn_mode;check:chk_warn_mode,warn_mode = '' OR warn_mode IN ('ban','kick','mute','tban','tmute')" json:"warn_mode,omitempty"`
	// WarnActionTime is the tban/tmute duration in seconds; 0 means not set.
	WarnActionTime int       `gorm:"column:warn_action_time;not null;default:0;check:chk_warn_action_time,warn_action_time >= 0" json:"warn_action_time,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// Bounds for WarnActionTime, in seconds. Telegram reads an until_date less
// than 30 s or more than 366 days away as forever; the 60 s floor absorbs
// second rounding and request latency.
const (
	MinWarnActionTime = 60
	MaxWarnActionTime = 366 * 24 * 60 * 60
)

// ValidWarnActionTime reports whether seconds is a usable tban/tmute duration.
func ValidWarnActionTime(seconds int) bool {
	return seconds >= MinWarnActionTime && seconds <= MaxWarnActionTime
}

func (WarnSettings) TableName() string {
	return "warns_settings"
}

type Warns struct {
	ID        uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64       `gorm:"column:user_id;not null;uniqueIndex:uk_warns_users_user_chat" json:"user_id,omitempty"`
	ChatId    int64       `gorm:"column:chat_id;not null;uniqueIndex:uk_warns_users_user_chat;index:idx_warns_users_chat_id" json:"chat_id,omitempty"`
	NumWarns  int         `gorm:"column:num_warns;default:0;check:chk_warns_num_warns,num_warns >= 0" json:"num_warns,omitempty"`
	Reasons   StringArray `gorm:"column:warns;type:jsonb" json:"warns" default:"[]"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Warns) TableName() string {
	return "warns_users"
}

// WarnEntry is one warn: its reason, who issued it and when. WarnedBy 0 with a
// nil WarnedAt is a warn from before entries existed. Anonymous hides the
// issuer, who was an anonymous admin; WarnedBy still records the real account,
// which a backup export drops.
type WarnEntry struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64      `gorm:"column:user_id;not null" json:"user_id,omitempty"`
	ChatId    int64      `gorm:"column:chat_id;not null" json:"chat_id,omitempty"`
	Reason    string     `gorm:"column:reason;not null;default:''" json:"reason"`
	WarnedBy  int64      `gorm:"column:warned_by;not null;default:0" json:"warned_by,omitempty"`
	Anonymous bool       `gorm:"column:anonymous;not null;default:false" json:"anonymous,omitempty"`
	WarnedAt  *time.Time `gorm:"column:warned_at" json:"warned_at,omitempty"`
}

func (WarnEntry) TableName() string {
	return "warn_entries"
}

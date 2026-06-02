package models

import "time"

// CaptchaSettings represents captcha settings for a chat
type CaptchaSettings struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID        int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled       bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty"`
	CaptchaMode   string    `gorm:"column:captcha_mode;default:'math';check:chk_captcha_mode,captcha_mode IN ('math','text')" json:"captcha_mode,omitempty"` // math or text
	Timeout       int       `gorm:"column:timeout;default:2;check:chk_captcha_timeout_range,timeout BETWEEN 1 AND 10" json:"timeout,omitempty"`              // minutes
	FailureAction string    `gorm:"column:failure_action;default:'kick';check:chk_captcha_failure_action,failure_action IN ('kick','ban','mute')" json:"failure_action,omitempty"`
	MaxAttempts   int       `gorm:"column:max_attempts;default:3;check:chk_captcha_max_attempts_range,max_attempts BETWEEN 1 AND 10" json:"max_attempts,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (CaptchaSettings) TableName() string {
	return "captcha_settings"
}

// CaptchaAttempts represents active captcha attempts for users
type CaptchaAttempts struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID       int64     `gorm:"column:user_id;not null;index:idx_captcha_user_chat" json:"user_id,omitempty"`
	ChatID       int64     `gorm:"column:chat_id;not null;index:idx_captcha_user_chat" json:"chat_id,omitempty"`
	Answer       string    `gorm:"column:answer;not null" json:"answer,omitempty"`
	Attempts     int       `gorm:"column:attempts;default:0" json:"attempts,omitempty"`
	MessageID    int64     `gorm:"column:message_id" json:"message_id,omitempty"`
	RefreshCount int       `gorm:"column:refresh_count;default:0" json:"refresh_count,omitempty"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null;check:chk_captcha_expires_at,expires_at > created_at" json:"expires_at,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (CaptchaAttempts) TableName() string {
	return "captcha_attempts"
}

// StoredMessages represents messages sent by users before completing captcha verification
type StoredMessages struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID      int64     `gorm:"column:user_id;not null;index:idx_stored_user_chat" json:"user_id,omitempty"`
	ChatID      int64     `gorm:"column:chat_id;not null;index:idx_stored_user_chat" json:"chat_id,omitempty"`
	MessageType int       `gorm:"column:message_type;not null;default:1" json:"message_type,omitempty"` // TEXT, STICKER, etc.
	Content     string    `gorm:"column:content;type:text" json:"content,omitempty"`
	FileID      string    `gorm:"column:file_id" json:"file_id,omitempty"`                // For media messages
	Caption     string    `gorm:"column:caption;type:text" json:"caption,omitempty"`      // For media captions
	AttemptID   uint      `gorm:"column:attempt_id;not null" json:"attempt_id,omitempty"` // Foreign key to CaptchaAttempts
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (StoredMessages) TableName() string {
	return "stored_messages"
}

// CaptchaMutedUsers tracks users who failed captcha with mute action
// They will be automatically unmuted after UnmuteAt time
type CaptchaMutedUsers struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_captcha_muted_user_chat" json:"user_id,omitempty"`
	ChatID    int64     `gorm:"column:chat_id;not null;index:idx_captcha_muted_user_chat" json:"chat_id,omitempty"`
	UnmuteAt  time.Time `gorm:"column:unmute_at;not null;index:idx_captcha_unmute_at" json:"unmute_at,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at,omitempty"`
}

func (CaptchaMutedUsers) TableName() string {
	return "captcha_muted_users"
}

package models

import "time"

type CaptchaSettings struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID        int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled       bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty"`
	CaptchaMode   string    `gorm:"column:captcha_mode;default:'math';check:chk_captcha_mode,captcha_mode IN ('math','text')" json:"captcha_mode,omitempty"`
	Timeout       int       `gorm:"column:timeout;default:2;check:chk_captcha_timeout_range,timeout BETWEEN 1 AND 10" json:"timeout,omitempty"`
	FailureAction string    `gorm:"column:failure_action;default:'kick';check:chk_captcha_failure_action,failure_action IN ('kick','ban','mute')" json:"failure_action,omitempty"`
	MaxAttempts   int       `gorm:"column:max_attempts;default:3;check:chk_captcha_max_attempts_range,max_attempts BETWEEN 1 AND 10" json:"max_attempts,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (CaptchaSettings) TableName() string {
	return "captcha_settings"
}

type CaptchaAttempts struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID       int64     `gorm:"column:user_id;not null;uniqueIndex:uk_captcha_user_chat" json:"user_id,omitempty"`
	ChatID       int64     `gorm:"column:chat_id;not null;uniqueIndex:uk_captcha_user_chat;index:idx_captcha_attempts_chat_id" json:"chat_id,omitempty"`
	Answer       string    `gorm:"column:answer;not null" json:"answer,omitempty"`
	Attempts     int       `gorm:"column:attempts;default:0" json:"attempts,omitempty"`
	MessageID    int64     `gorm:"column:message_id" json:"message_id,omitempty"`
	RefreshCount int       `gorm:"column:refresh_count;default:0" json:"refresh_count,omitempty"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null;check:chk_captcha_expires_at,expires_at > created_at" json:"expires_at,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`

	PreviousMessageID int64 `gorm:"-" json:"-"`
}

func (CaptchaAttempts) TableName() string {
	return "captcha_attempts"
}

type StoredMessages struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID      int64     `gorm:"column:user_id;not null;index:idx_stored_user_chat" json:"user_id,omitempty"`
	ChatID      int64     `gorm:"column:chat_id;not null;index:idx_stored_user_chat" json:"chat_id,omitempty"`
	MessageType int       `gorm:"column:message_type;not null;default:1" json:"message_type,omitempty"`
	Content     string    `gorm:"column:content;type:text" json:"content,omitempty"`
	FileID      string    `gorm:"column:file_id" json:"file_id,omitempty"`
	Caption     string    `gorm:"column:caption;type:text" json:"caption,omitempty"`
	AttemptID   uint      `gorm:"column:attempt_id;not null;index:idx_stored_attempt" json:"attempt_id,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (StoredMessages) TableName() string {
	return "stored_messages"
}

type CaptchaMutedUsers struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:uk_captcha_muted_user_chat" json:"user_id,omitempty"`
	ChatID    int64     `gorm:"column:chat_id;not null;uniqueIndex:uk_captcha_muted_user_chat;index:idx_captcha_muted_users_chat_id" json:"chat_id,omitempty"`
	UnmuteAt  time.Time `gorm:"column:unmute_at;not null;index:idx_captcha_unmute_at" json:"unmute_at,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at,omitempty"`
}

func (CaptchaMutedUsers) TableName() string {
	return "captcha_muted_users"
}

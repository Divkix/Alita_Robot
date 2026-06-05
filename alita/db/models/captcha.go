package models

import "time"

// CaptchaSettings represents captcha settings for a chat
type CaptchaSettings struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatID        int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	Enabled       bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty" bson:"enabled"`
	CaptchaMode   string    `gorm:"column:captcha_mode;default:'math';check:chk_captcha_mode,captcha_mode IN ('math','text')" json:"captcha_mode,omitempty" bson:"captcha_mode"` // math or text
	Timeout       int       `gorm:"column:timeout;default:2;check:chk_captcha_timeout_range,timeout BETWEEN 1 AND 10" json:"timeout,omitempty" bson:"timeout"`              // minutes
	FailureAction string    `gorm:"column:failure_action;default:'kick';check:chk_captcha_failure_action,failure_action IN ('kick','ban','mute')" json:"failure_action,omitempty" bson:"failure_action"`
	MaxAttempts   int       `gorm:"column:max_attempts;default:3;check:chk_captcha_max_attempts_range,max_attempts BETWEEN 1 AND 10" json:"max_attempts,omitempty" bson:"max_attempts"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (CaptchaSettings) TableName() string {
	return "captcha_settings"
}

// CaptchaAttempts represents active captcha attempts for users
type CaptchaAttempts struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserID       int64     `gorm:"column:user_id;not null;index:idx_captcha_user_chat" json:"user_id,omitempty" bson:"user_id"`
	ChatID       int64     `gorm:"column:chat_id;not null;index:idx_captcha_user_chat" json:"chat_id,omitempty" bson:"chat_id"`
	Answer       string    `gorm:"column:answer;not null" json:"answer,omitempty" bson:"answer"`
	Attempts     int       `gorm:"column:attempts;default:0" json:"attempts,omitempty" bson:"attempts"`
	MessageID    int64     `gorm:"column:message_id" json:"message_id,omitempty" bson:"message_id"`
	RefreshCount int       `gorm:"column:refresh_count;default:0" json:"refresh_count,omitempty" bson:"refresh_count"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null;check:chk_captcha_expires_at,expires_at > created_at" json:"expires_at,omitempty" bson:"expires_at"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (CaptchaAttempts) TableName() string {
	return "captcha_attempts"
}

// StoredMessages represents messages sent by users before completing captcha verification
type StoredMessages struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserID      int64     `gorm:"column:user_id;not null;index:idx_stored_user_chat" json:"user_id,omitempty" bson:"user_id"`
	ChatID      int64     `gorm:"column:chat_id;not null;index:idx_stored_user_chat" json:"chat_id,omitempty" bson:"chat_id"`
	MessageType int       `gorm:"column:message_type;not null;default:1" json:"message_type,omitempty" bson:"message_type"` // TEXT, STICKER, etc.
	Content     string    `gorm:"column:content;type:text" json:"content,omitempty" bson:"content"`
	FileID      string    `gorm:"column:file_id" json:"file_id,omitempty" bson:"file_id"`                // For media messages
	Caption     string    `gorm:"column:caption;type:text" json:"caption,omitempty" bson:"caption"`      // For media captions
	AttemptID   uint      `gorm:"column:attempt_id;not null" json:"attempt_id,omitempty" bson:"attempt_id"` // Foreign key to CaptchaAttempts
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
}

func (StoredMessages) TableName() string {
	return "stored_messages"
}

// CaptchaMutedUsers tracks users who failed captcha with mute action
// They will be automatically unmuted after UnmuteAt time
type CaptchaMutedUsers struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_captcha_muted_user_chat" json:"user_id,omitempty" bson:"user_id"`
	ChatID    int64     `gorm:"column:chat_id;not null;index:idx_captcha_muted_user_chat" json:"chat_id,omitempty" bson:"chat_id"`
	UnmuteAt  time.Time `gorm:"column:unmute_at;not null;index:idx_captcha_unmute_at" json:"unmute_at,omitempty" bson:"unmute_at"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at,omitempty" bson:"created_at"`
}

func (CaptchaMutedUsers) TableName() string {
	return "captcha_muted_users"
}

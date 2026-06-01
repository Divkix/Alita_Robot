package models

import "time"

// WelcomeSettings represents welcome message settings
type WelcomeSettings struct {
	CleanWelcome  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty"`
	ShouldWelcome bool        `gorm:"column:enabled;default:true" json:"welcome_enabled" default:"true"`
	WelcomeText   string      `gorm:"column:text" json:"welcome_text,omitempty"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty"`
	WelcomeType   int         `gorm:"column:type;default:1" json:"welcome_type,omitempty"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty"`
}

// GoodbyeSettings represents goodbye message settings
type GoodbyeSettings struct {
	CleanGoodbye  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty"`
	ShouldGoodbye bool        `gorm:"column:enabled;default:true" json:"enabled" default:"true"`
	GoodbyeText   string      `gorm:"column:text" json:"text,omitempty"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty"`
	GoodbyeType   int         `gorm:"column:type;default:1" json:"type,omitempty"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty"`
}

// GreetingSettings represents greeting settings for a chat
type GreetingSettings struct {
	ID                 uint             `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID             int64            `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	ShouldCleanService bool             `gorm:"column:clean_service_settings;default:false" json:"clean_service_settings" default:"false"`
	WelcomeSettings    *WelcomeSettings `gorm:"embedded;embeddedPrefix:welcome_" json:"welcome_settings" default:"false"`
	GoodbyeSettings    *GoodbyeSettings `gorm:"embedded;embeddedPrefix:goodbye_" json:"goodbye_settings" default:"false"`
	ShouldAutoApprove  bool             `gorm:"column:auto_approve;default:false" json:"auto_approve" default:"false"`
	CreatedAt          time.Time        `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt          time.Time        `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (GreetingSettings) TableName() string {
	return "greetings"
}

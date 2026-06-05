package models

import "time"

// WelcomeSettings represents welcome message settings
type WelcomeSettings struct {
	CleanWelcome  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false" bson:"clean_old"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty" bson:"last_msg_id"`
	ShouldWelcome bool        `gorm:"column:enabled;default:true" json:"welcome_enabled" default:"true" bson:"enabled"`
	WelcomeText   string      `gorm:"column:text" json:"welcome_text,omitempty" bson:"text"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty" bson:"file_id"`
	WelcomeType   int         `gorm:"column:type;default:1" json:"welcome_type,omitempty" bson:"type"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty" bson:"btns"`
}

// GoodbyeSettings represents goodbye message settings
type GoodbyeSettings struct {
	CleanGoodbye  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false" bson:"clean_old"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty" bson:"last_msg_id"`
	ShouldGoodbye bool        `gorm:"column:enabled;default:true" json:"enabled" default:"true" bson:"enabled"`
	GoodbyeText   string      `gorm:"column:text" json:"text,omitempty" bson:"text"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty" bson:"file_id"`
	GoodbyeType   int         `gorm:"column:type;default:1" json:"type,omitempty" bson:"type"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty" bson:"btns"`
}

// GreetingSettings represents greeting settings for a chat
type GreetingSettings struct {
	ID                 uint             `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatID             int64            `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty" bson:"chat_id"`
	ShouldCleanService bool             `gorm:"column:clean_service_settings;default:false" json:"clean_service_settings" default:"false" bson:"clean_service_settings"`
	WelcomeSettings    *WelcomeSettings `gorm:"embedded;embeddedPrefix:welcome_" json:"welcome_settings" default:"false" bson:"welcome_settings"`
	GoodbyeSettings    *GoodbyeSettings `gorm:"embedded;embeddedPrefix:goodbye_" json:"goodbye_settings" default:"false" bson:"goodbye_settings"`
	ShouldAutoApprove  bool             `gorm:"column:auto_approve;default:false" json:"auto_approve" default:"false" bson:"auto_approve"`
	CreatedAt          time.Time        `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt          time.Time        `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (GreetingSettings) TableName() string {
	return "greetings"
}

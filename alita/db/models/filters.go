package models

import "time"

// ChatFilters represents chat filters
type ChatFilters struct {
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_filters_chat_keyword" json:"chat_id,omitempty"`
	KeyWord     string      `gorm:"column:keyword;not null;index:idx_filters_chat_keyword" json:"keyword,omitempty"`
	FilterReply string      `gorm:"column:filter_reply" json:"filter_reply,omitempty"`
	MsgType     int         `gorm:"column:msgtype" json:"msgtype,omitempty"`
	FileID      string      `gorm:"column:fileid" json:"fileid,omitempty"`
	NoNotif     bool        `gorm:"column:nonotif;default:false" json:"nonotif,omitempty"`
	Buttons     ButtonArray `gorm:"column:filter_buttons;type:jsonb" json:"filter_buttons,omitempty"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ChatFilters) TableName() string {
	return "filters"
}

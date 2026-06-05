package models

import "time"

// ChatFilters represents chat filters
type ChatFilters struct {
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_filters_chat_keyword" json:"chat_id,omitempty" bson:"chat_id"`
	KeyWord     string      `gorm:"column:keyword;not null;index:idx_filters_chat_keyword" json:"keyword,omitempty" bson:"keyword"`
	FilterReply string      `gorm:"column:filter_reply" json:"filter_reply,omitempty" bson:"filter_reply"`
	MsgType     int         `gorm:"column:msgtype" json:"msgtype,omitempty" bson:"msgtype"`
	FileID      string      `gorm:"column:fileid" json:"fileid,omitempty" bson:"fileid"`
	NoNotif     bool        `gorm:"column:nonotif;default:false" json:"nonotif,omitempty" bson:"nonotif"`
	Buttons     ButtonArray `gorm:"column:filter_buttons;type:jsonb" json:"filter_buttons,omitempty" bson:"filter_buttons"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (ChatFilters) TableName() string {
	return "filters"
}

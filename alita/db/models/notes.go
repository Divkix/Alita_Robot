package models

import "time"

// NotesSettings represents notes settings for a chat
type NotesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty" bson:"chat_id"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty" bson:"private"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

// PrivateNotesEnabled returns whether private notes are enabled for the chat.
func (ns *NotesSettings) PrivateNotesEnabled() bool {
	return ns.Private
}

func (NotesSettings) TableName() string {
	return "notes_settings"
}

// Notes represents notes in a chat
type Notes struct {
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_notes_chat_name" json:"chat_id,omitempty" bson:"chat_id"`
	NoteName    string      `gorm:"column:note_name;not null;index:idx_notes_chat_name" json:"note_name,omitempty" bson:"note_name"`
	NoteContent string      `gorm:"column:note_content;type:text" json:"note_content,omitempty" bson:"note_content"`
	FileID      string      `gorm:"column:file_id" json:"file_id,omitempty" bson:"file_id"`
	MsgType     int         `gorm:"column:msg_type" json:"msg_type,omitempty" bson:"msg_type"`
	Buttons     ButtonArray `gorm:"column:buttons;type:jsonb" json:"buttons,omitempty" bson:"buttons"`
	AdminOnly   bool        `gorm:"column:admin_only;default:false" json:"admin_only,omitempty" bson:"admin_only"`
	PrivateOnly bool        `gorm:"column:private_only;default:false" json:"private_only,omitempty" bson:"private_only"`
	GroupOnly   bool        `gorm:"column:group_only;default:false" json:"group_only,omitempty" bson:"group_only"`
	WebPreview  bool        `gorm:"column:web_preview;default:true" json:"web_preview,omitempty" bson:"web_preview"`
	IsProtected bool        `gorm:"column:is_protected;default:false" json:"is_protected,omitempty" bson:"is_protected"`
	NoNotif     bool        `gorm:"column:no_notif;default:false" json:"no_notif,omitempty" bson:"no_notif"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (Notes) TableName() string {
	return "notes"
}

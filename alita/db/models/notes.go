package models

import "time"

// NotesSettings represents notes settings for a chat
type NotesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
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
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_notes_chat_name" json:"chat_id,omitempty"`
	NoteName    string      `gorm:"column:note_name;not null;index:idx_notes_chat_name" json:"note_name,omitempty"`
	NoteContent string      `gorm:"column:note_content;type:text" json:"note_content,omitempty"`
	FileID      string      `gorm:"column:file_id" json:"file_id,omitempty"`
	MsgType     int         `gorm:"column:msg_type" json:"msg_type,omitempty"`
	Buttons     ButtonArray `gorm:"column:buttons;type:jsonb" json:"buttons,omitempty"`
	AdminOnly   bool        `gorm:"column:admin_only;default:false" json:"admin_only,omitempty"`
	PrivateOnly bool        `gorm:"column:private_only;default:false" json:"private_only,omitempty"`
	GroupOnly   bool        `gorm:"column:group_only;default:false" json:"group_only,omitempty"`
	WebPreview  bool        `gorm:"column:web_preview;default:true" json:"web_preview,omitempty"`
	IsProtected bool        `gorm:"column:is_protected;default:false" json:"is_protected,omitempty"`
	NoNotif     bool        `gorm:"column:no_notif;default:false" json:"no_notif,omitempty"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Notes) TableName() string {
	return "notes"
}

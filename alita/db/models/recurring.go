package models

import "time"

// RecurringMessage is a message an admin set to be posted in a chat every
// Interval seconds.
//
// The content is stored the way notes store theirs (Text with the notes
// directives, FileID, MsgType, Buttons) and sent through the same media
// sender. NextRunAt is the exactly-once claim: a replica posts an occurrence
// only when its compare-and-set moving next_run_at forward wins. EndsAt, when
// set, is the instant after which no occurrence is posted. LastMessageID is
// the message the last occurrence posted, which DeletePrevious removes once
// the next one is out. Enabled turns false when the message ended or when a
// send failed in a way retrying cannot fix.
type RecurringMessage struct {
	ID             int64       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChatID         int64       `gorm:"column:chat_id;not null;index:idx_recurring_messages_chat_id" json:"chat_id"`
	ThreadID       int64       `gorm:"column:thread_id;not null;default:0" json:"thread_id,omitempty"`
	Text           string      `gorm:"column:text;type:text;not null;default:''" json:"text,omitempty"`
	FileID         string      `gorm:"column:file_id;not null;default:''" json:"file_id,omitempty"`
	MsgType        int         `gorm:"column:msg_type;not null;default:1" json:"msg_type,omitempty"`
	Buttons        ButtonArray `gorm:"column:buttons;type:jsonb" json:"buttons,omitempty"`
	IntervalSecs   int64       `gorm:"column:interval_seconds;not null" json:"interval_seconds"`
	NextRunAt      time.Time   `gorm:"column:next_run_at;not null;index:idx_recurring_messages_next_run_at" json:"next_run_at"`
	EndsAt         *time.Time  `gorm:"column:ends_at" json:"ends_at,omitempty"`
	DeletePrevious bool        `gorm:"column:delete_previous;not null;default:false" json:"delete_previous,omitempty"`
	LastMessageID  int64       `gorm:"column:last_message_id;not null;default:0" json:"last_message_id,omitempty"`
	Enabled        bool        `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedBy      int64       `gorm:"column:created_by;not null;default:0" json:"created_by,omitempty"`
	CreatedAt      time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (RecurringMessage) TableName() string {
	return "recurring_messages"
}

// Interval is the time between two occurrences.
func (m RecurringMessage) Interval() time.Duration {
	return time.Duration(m.IntervalSecs) * time.Second
}

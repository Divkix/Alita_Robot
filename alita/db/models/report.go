package models

import "time"

// Report statuses. A report starts open and is closed exactly once by the
// first admin decision; every other status is a terminal decision.
const (
	ReportStatusOpen      = "open"
	ReportStatusBanned    = "banned"
	ReportStatusKicked    = "kicked"
	ReportStatusMuted     = "muted"
	ReportStatusDeleted   = "deleted"
	ReportStatusDismissed = "dismissed"
)

// Report is one member's report of one message. The unique index keeps a
// repeated /report on the same message from the same member to one row.
type Report struct {
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatId            int64      `gorm:"column:chat_id;not null;uniqueIndex:uq_reports_chat_message_reporter,priority:1;index:idx_reports_chat_target,priority:1" json:"chat_id"`
	ReportedUserId    int64      `gorm:"column:reported_user_id;not null;index:idx_reports_chat_target,priority:2" json:"reported_user_id"`
	ReportedMessageId int64      `gorm:"column:reported_message_id;not null;uniqueIndex:uq_reports_chat_message_reporter,priority:2" json:"reported_message_id"`
	ReporterId        int64      `gorm:"column:reporter_id;not null;uniqueIndex:uq_reports_chat_message_reporter,priority:3" json:"reporter_id"`
	Reason            string     `gorm:"column:reason;not null;default:''" json:"reason,omitempty"`
	Status            string     `gorm:"column:status;size:16;not null;default:open" json:"status"`
	ResolvedBy        *int64     `gorm:"column:resolved_by" json:"resolved_by,omitempty"`
	ResolvedAt        *time.Time `gorm:"column:resolved_at" json:"resolved_at,omitempty"`
	AdminChatId       *int64     `gorm:"column:admin_chat_id" json:"admin_chat_id,omitempty"`
	AdminMessageId    *int64     `gorm:"column:admin_message_id" json:"admin_message_id,omitempty"`
	CreatedAt         time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt         time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Report) TableName() string {
	return "reports"
}

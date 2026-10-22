package models

import "time"

// Moderation actions recorded in moderation_events. Command variants fold into
// these four: tban, dban and sban are ModerationActionBan; dkick and skick are
// ModerationActionKick; tmute, dmute and smute are ModerationActionMute;
// unmute and the self-target commands (kickme, banme) record nothing.
const (
	ModerationActionBan   = "ban"
	ModerationActionKick  = "kick"
	ModerationActionMute  = "mute"
	ModerationActionUnban = "unban"
)

// MaxModerationReasonRunes caps ModerationEvent.Reason. It matches the limit
// warnings put on a stored reason, holds a moderator's typed text whole in the
// common case, and keeps a row small; no message content is ever stored.
const MaxModerationReasonRunes = 3000

// ModerationEvent is one recorded action: who did what to whom, in which chat,
// and when. ActorId is the account that ran the command; for an anonymous
// admin it is the group anonymous bot, like the action log names. TargetUserId
// is a user id, or the sender-chat id when a channel post was banned. CreatedAt
// is UTC.
type ModerationEvent struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatId       int64     `gorm:"column:chat_id;not null;index:idx_moderation_events_chat_created_at,priority:1" json:"chat_id"`
	ActorId      int64     `gorm:"column:actor_id;not null" json:"actor_id"`
	TargetUserId int64     `gorm:"column:target_user_id;not null;index:idx_moderation_events_target_created_at,priority:1" json:"target_user_id"`
	Action       string    `gorm:"column:action;not null;check:chk_moderation_events_action,action IN ('ban','kick','mute','unban')" json:"action"`
	Reason       string    `gorm:"column:reason;not null;default:''" json:"reason,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime;index:idx_moderation_events_created_at;index:idx_moderation_events_chat_created_at,priority:2;index:idx_moderation_events_target_created_at,priority:2" json:"created_at"`
}

func (ModerationEvent) TableName() string {
	return "moderation_events"
}

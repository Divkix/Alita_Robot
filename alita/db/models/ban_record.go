package models

import "time"

// Ban record sources: the path that issued the ban.
const (
	BanSourceManual      = "manual"
	BanSourceWarn        = "warn"
	BanSourceAntiflood   = "antiflood"
	BanSourceBlacklist   = "blacklist"
	BanSourceAntiraid    = "antiraid"
	BanSourceCaptcha     = "captcha"
	BanSourceFed         = "fed"
	BanSourceLock        = "lock"
	BanSourceReport      = "report"
	BanSourceJoinRequest = "join_request"
	BanSourceSpamDB      = "spamdb"
)

// MaxBanSourceDetailRunes caps BanRecord.SourceDetail; the migration declares
// the column VARCHAR(64).
const MaxBanSourceDetailRunes = 64

// BanRecord is one ban the bot issued in a chat. BannedBy is the account
// that issued it, 0 when the bot banned on its own. Until is the end of a
// temporary ban, nil for a permanent one; an open record past Until has
// expired. LiftedAt and LiftedBy are set when the bot unbans the user. A chat
// holds at most one open record (LiftedAt nil) per user.
type BanRecord struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatId       int64      `gorm:"column:chat_id;not null;uniqueIndex:uq_ban_records_open,priority:1,where:lifted_at IS NULL;index:idx_ban_records_chat_user_id,priority:1" json:"chat_id"`
	UserId       int64      `gorm:"column:user_id;not null;uniqueIndex:uq_ban_records_open,priority:2,where:lifted_at IS NULL;index:idx_ban_records_chat_user_id,priority:2" json:"user_id"`
	BannedBy     int64      `gorm:"column:banned_by;not null;default:0" json:"banned_by"`
	Reason       string     `gorm:"column:reason;not null;default:''" json:"reason,omitempty"`
	Source       string     `gorm:"column:source;size:16;not null;check:chk_ban_records_source,source IN ('manual','warn','antiflood','blacklist','antiraid','captcha','fed','lock','report','join_request','spamdb')" json:"source"`
	SourceDetail string     `gorm:"column:source_detail;size:64;not null;default:''" json:"source_detail,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	Until        *time.Time `gorm:"column:until;index:idx_ban_records_open_until,where:lifted_at IS NULL AND until IS NOT NULL" json:"until,omitempty"`
	LiftedAt     *time.Time `gorm:"column:lifted_at;index:idx_ban_records_lifted_at,where:lifted_at IS NOT NULL" json:"lifted_at,omitempty"`
	LiftedBy     *int64     `gorm:"column:lifted_by" json:"lifted_by,omitempty"`
}

func (BanRecord) TableName() string {
	return "ban_records"
}

// IsOpen reports whether the ban is in force at now: not lifted, and
// permanent or not yet past its end.
func (r *BanRecord) IsOpen(now time.Time) bool {
	return r.LiftedAt == nil && (r.Until == nil || r.Until.After(now))
}

package models

import "time"

// Labels of a spam sample.
const (
	SpamLabelSpam = "spam"
	SpamLabelHam  = "ham"
)

// Spam filter modes.
const (
	// SpamFilterOff matches nothing: the default.
	SpamFilterOff = "off"
	// SpamFilterDryRun logs matches to the log channel and acts on none.
	SpamFilterDryRun = "dryrun"
	// SpamFilterOn deletes a match once the chat's dry run is over.
	SpamFilterOn = "on"
)

// DefaultSpamFilterThreshold is the similarity a match needs until an admin
// sets another.
const DefaultSpamFilterThreshold = 0.85

// SpamSample is one text a chat's admins labelled spam or ham. Normalized is
// what matching compares and ContentHash its sha256, so a chat holds one row
// per text. OwnerID and AdminIDs (sorted, comma-separated) are the chat's
// owner and human admins when the text was labelled, which decide whether two
// chats that flagged the same text are unrelated. CreatedAt is UTC.
type SpamSample struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChatID      int64     `gorm:"column:chat_id;not null;uniqueIndex:uq_spam_samples_chat_hash,priority:1;index:idx_spam_samples_chat_label_created_at,priority:1" json:"chat_id"`
	Label       string    `gorm:"column:label;not null;default:'spam';index:idx_spam_samples_chat_label_created_at,priority:2;check:chk_spam_samples_label,label IN ('spam','ham')" json:"label"`
	Text        string    `gorm:"column:text;not null" json:"text"`
	Normalized  string    `gorm:"column:normalized;not null" json:"normalized"`
	ContentHash string    `gorm:"column:content_hash;not null;uniqueIndex:uq_spam_samples_chat_hash,priority:2;index:idx_spam_samples_hash" json:"content_hash"`
	SenderID    int64     `gorm:"column:sender_id;not null;default:0" json:"sender_id,omitempty"`
	AddedBy     int64     `gorm:"column:added_by;not null;default:0" json:"added_by,omitempty"`
	OwnerID     int64     `gorm:"column:owner_id;not null;default:0" json:"owner_id,omitempty"`
	AdminIDs    string    `gorm:"column:admin_ids;not null;default:''" json:"admin_ids,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index:idx_spam_samples_created_at;index:idx_spam_samples_chat_label_created_at,priority:3" json:"created_at"`
}

func (SpamSample) TableName() string {
	return "spam_samples"
}

// SpamFilterSettings is one chat's spam filter switch. EnabledAt is when the
// chat last went from off to dryrun or on; the dry run that precedes acting is
// counted from it. A missing row reads as off.
type SpamFilterSettings struct {
	ChatID    int64      `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Mode      string     `gorm:"column:mode;not null;default:'off';check:chk_spam_filter_settings_mode,mode IN ('off','dryrun','on')" json:"mode"`
	Threshold float64    `gorm:"column:threshold;not null;default:0.85;check:chk_spam_filter_settings_threshold,threshold >= 0.5 AND threshold <= 1" json:"threshold"`
	EnabledAt *time.Time `gorm:"column:enabled_at" json:"enabled_at,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (SpamFilterSettings) TableName() string {
	return "spam_filter_settings"
}

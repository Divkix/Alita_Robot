package models

import "time"

// The limits of the probation settings.
// migrations/20261023006200_add_probation.sql repeats them; change both.
const (
	ProbationDurationMin     = time.Hour
	ProbationDurationMax     = 30 * 24 * time.Hour
	ProbationMessagesMin     = 1
	ProbationMessagesMax     = 100
	ProbationMinLengthMin    = 1
	ProbationMinLengthMax    = 500
	ProbationDefaultDuration = 24 * time.Hour
	ProbationDefaultMessages = 5
	ProbationDefaultMinLen   = 20
)

// ProbationSettings is one chat's new-member probation choice. A chat without
// a row has probation off with the defaults of DefaultProbationSettings.
type ProbationSettings struct {
	ChatID          int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Enabled         bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	DurationSeconds int       `gorm:"column:duration_seconds;not null;default:86400" json:"duration_seconds"`
	MessageCount    int       `gorm:"column:message_count;not null;default:5" json:"message_count"`
	MinLength       int       `gorm:"column:min_length;not null;default:20" json:"min_length"`
	BlockLinks      bool      `gorm:"column:block_links;not null;default:true" json:"block_links"`
	BlockForwards   bool      `gorm:"column:block_forwards;not null;default:true" json:"block_forwards"`
	BlockMedia      bool      `gorm:"column:block_media;not null;default:false" json:"block_media"`
	Notice          bool      `gorm:"column:notice;not null;default:false" json:"notice"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (ProbationSettings) TableName() string {
	return "probation_settings"
}

// DefaultProbationSettings is what a chat without a row has: probation off,
// 24 hours or 5 messages of at least 20 characters, links and forwards
// blocked, no notice.
func DefaultProbationSettings(chatID int64) *ProbationSettings {
	return &ProbationSettings{
		ChatID:          chatID,
		DurationSeconds: int(ProbationDefaultDuration / time.Second),
		MessageCount:    ProbationDefaultMessages,
		MinLength:       ProbationDefaultMinLen,
		BlockLinks:      true,
		BlockForwards:   true,
	}
}

// Duration is how long a member stays on probation after joining.
func (s *ProbationSettings) Duration() time.Duration {
	return time.Duration(s.DurationSeconds) * time.Second
}

// ProbationMember is a member who joined while probation was on and has not
// finished it. Notified is set once the member got the one notice per chat.
type ProbationMember struct {
	ChatID   int64     `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	UserID   int64     `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"user_id"`
	JoinedAt time.Time `gorm:"column:joined_at;not null;index:idx_probation_members_joined_at" json:"joined_at"`
	Notified bool      `gorm:"column:notified;not null;default:false" json:"notified"`
}

func (ProbationMember) TableName() string {
	return "probation_members"
}

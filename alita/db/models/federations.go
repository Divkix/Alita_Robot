package models

import "time"

// Federation is a shared ban list owned by one user.
type Federation struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID         string    `gorm:"column:fed_id;uniqueIndex;not null;size:36" json:"fed_id"`
	OwnerID       int64     `gorm:"column:owner_id;uniqueIndex;not null" json:"owner_id"`
	Name          string    `gorm:"column:name;not null;size:64" json:"name"`
	RequireReason bool      `gorm:"column:require_reason;default:false" json:"require_reason"`
	NotifyOwner   bool      `gorm:"column:notify_owner;default:true" json:"notify_owner"`
	LogChatID     int64     `gorm:"column:log_chat_id;default:0" json:"log_chat_id"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Federation) TableName() string {
	return "federations"
}

// FederationAdmin is a user who can fban in a federation. Only the owner can
// promote or demote admins.
type FederationAdmin struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID     string    `gorm:"column:fed_id;not null;uniqueIndex:idx_fed_admins_fed_user;size:36" json:"fed_id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:idx_fed_admins_fed_user" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (FederationAdmin) TableName() string {
	return "federation_admins"
}

// FederationChat links a Telegram chat to exactly one federation.
type FederationChat struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID     string    `gorm:"column:fed_id;not null;index;size:36" json:"fed_id"`
	ChatID    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id"`
	Quiet     bool      `gorm:"column:quiet;default:false" json:"quiet"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (FederationChat) TableName() string {
	return "federation_chats"
}

// FederationBan is a user banned from a federation.
type FederationBan struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID     string    `gorm:"column:fed_id;not null;uniqueIndex:idx_fed_bans_fed_user;size:36" json:"fed_id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:idx_fed_bans_fed_user" json:"user_id"`
	Reason    string    `gorm:"column:reason;not null;default:''" json:"reason"`
	BannedBy  int64     `gorm:"column:banned_by;not null" json:"banned_by"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (FederationBan) TableName() string {
	return "federation_bans"
}

// FederationSub is a subscription from one federation to another federation's
// ban list. A federation may subscribe to at most five others.
type FederationSub struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID           string    `gorm:"column:fed_id;not null;uniqueIndex:idx_fed_subs_pair;size:36" json:"fed_id"`
	SubscribedFedID string    `gorm:"column:subscribed_fed_id;not null;uniqueIndex:idx_fed_subs_pair;size:36" json:"subscribed_fed_id"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (FederationSub) TableName() string {
	return "federation_subs"
}

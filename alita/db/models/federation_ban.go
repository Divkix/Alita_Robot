package models

import "time"

// FederationBan represents a user banned in a federation.
type FederationBan struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FederationID uint      `gorm:"column:federation_id;uniqueIndex:idx_federation_bans_fed_user;not null" json:"federation_id,omitempty"`
	UserID       int64     `gorm:"column:user_id;uniqueIndex:idx_federation_bans_fed_user;not null" json:"user_id,omitempty"`
	Reason       string    `gorm:"column:reason" json:"reason,omitempty"`
	BannedBy     int64     `gorm:"column:banned_by;not null" json:"banned_by,omitempty"`
	BannedAt     time.Time `gorm:"column:banned_at" json:"banned_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// FederationBanInfo is a denormalized view of a ban used by consumers.
type FederationBanInfo struct {
	FedID    string
	FedName  string
	UserID   int64
	Reason   string
	BannedAt time.Time
}

func (FederationBan) TableName() string {
	return "federation_bans"
}

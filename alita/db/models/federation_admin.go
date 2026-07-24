package models

import "time"

// FederationAdmin represents an admin in a federation (owner is not stored here).
type FederationAdmin struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FederationID uint      `gorm:"column:federation_id;uniqueIndex:idx_federation_admins_fed_user;not null" json:"federation_id,omitempty"`
	UserID       int64     `gorm:"column:user_id;uniqueIndex:idx_federation_admins_fed_user;not null" json:"user_id,omitempty"`
	PromotedBy   int64     `gorm:"column:promoted_by;not null" json:"promoted_by,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (FederationAdmin) TableName() string {
	return "federation_admins"
}

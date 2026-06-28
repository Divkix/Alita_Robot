package models

import "time"

// FederationSubscription represents one federation subscribing to another's ban list.
type FederationSubscription struct {
	ID                       uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FederationID             uint      `gorm:"column:federation_id;uniqueIndex:idx_federation_subs_fed_sub;not null" json:"federation_id,omitempty"`
	SubscribedToFederationID uint      `gorm:"column:subscribed_to_federation_id;uniqueIndex:idx_federation_subs_fed_sub;not null" json:"subscribed_to_federation_id,omitempty"`
	CreatedAt                time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

func (FederationSubscription) TableName() string {
	return "federation_subscriptions"
}

package models

import "time"

// Federation represents a federation of chats sharing ban lists.
type Federation struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	FedID     string    `gorm:"column:fed_id;uniqueIndex;not null" json:"fed_id,omitempty"`
	Name      string    `gorm:"column:name;not null" json:"name,omitempty"`
	OwnerID   int64     `gorm:"column:owner_id;uniqueIndex;not null" json:"owner_id,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (Federation) TableName() string {
	return "federations"
}

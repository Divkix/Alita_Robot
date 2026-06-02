package models

import "time"

// DevSettings represents developer settings
type DevSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id,omitempty"`
	IsDev     bool      `gorm:"column:is_dev;default:false" json:"is_dev,omitempty"`
	Sudo      bool      `gorm:"column:sudo;default:false" json:"sudo,omitempty"` // Sudo privileges
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

func (DevSettings) TableName() string {
	return "devs"
}

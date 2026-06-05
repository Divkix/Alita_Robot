package models

import "time"

// DevSettings represents developer settings
type DevSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-" bson:"-"`
	UserId    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id,omitempty" bson:"user_id"`
	IsDev     bool      `gorm:"column:is_dev;default:false" json:"is_dev,omitempty" bson:"is_dev"`
	Sudo      bool      `gorm:"column:sudo;default:false" json:"sudo,omitempty" bson:"sudo"` // Sudo privileges
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty" bson:"updated_at"`
}

func (DevSettings) TableName() string {
	return "devs"
}

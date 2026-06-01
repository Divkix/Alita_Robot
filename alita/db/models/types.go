package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// Button represents a button structure used in filters, greetings, etc.
type Button struct {
	Name     string `gorm:"column:name" json:"name,omitempty"`
	Url      string `gorm:"column:url" json:"url,omitempty"`
	SameLine bool   `gorm:"column:btn_sameline;default:false" json:"btn_sameline" default:"false"`
}

// ButtonArray is a custom type for handling arrays of buttons as JSONB
type ButtonArray []Button

// Scan implements the Scanner interface for database deserialization of ButtonArray.
func (ba *ButtonArray) Scan(value any) error {
	if value == nil {
		*ba = ButtonArray{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	return json.Unmarshal(data, ba)
}

// Value implements the driver Valuer interface for database serialization of ButtonArray.
func (ba ButtonArray) Value() (driver.Value, error) {
	if len(ba) == 0 {
		return "[]", nil
	}
	return json.Marshal(ba)
}

// StringArray is a custom type for handling arrays of strings as JSONB
type StringArray []string

// Scan implements the Scanner interface for database deserialization of StringArray.
func (sa *StringArray) Scan(value any) error {
	if value == nil {
		*sa = StringArray{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	return json.Unmarshal(data, sa)
}

// Value implements the driver Valuer interface for database serialization of StringArray.
func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return "[]", nil
	}
	return json.Marshal(sa)
}

// Int64Array is a custom type for handling arrays of int64 as JSONB
type Int64Array []int64

// Scan implements the Scanner interface for database deserialization of Int64Array.
func (ia *Int64Array) Scan(value any) error {
	if value == nil {
		*ia = Int64Array{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	return json.Unmarshal(data, ia)
}

// Value implements the driver Valuer interface for database serialization of Int64Array.
func (ia Int64Array) Value() (driver.Value, error) {
	if len(ia) == 0 {
		return "[]", nil
	}
	return json.Marshal(ia)
}

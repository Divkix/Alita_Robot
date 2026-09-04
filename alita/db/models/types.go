package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

func jsonbBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, errors.New("type assertion to []byte or string failed")
	}
}

type Button struct {
	Name     string `gorm:"column:name" json:"name,omitempty"`
	Url      string `gorm:"column:url" json:"url,omitempty"`
	SameLine bool   `gorm:"column:btn_sameline;default:false" json:"btn_sameline" default:"false"`
}

type ButtonArray []Button

func (ba *ButtonArray) Scan(value any) error {
	if value == nil {
		*ba = ButtonArray{}
		return nil
	}

	data, err := jsonbBytes(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, ba)
}

func (ba ButtonArray) Value() (driver.Value, error) {
	if len(ba) == 0 {
		return "[]", nil
	}
	return json.Marshal(ba)
}

type StringArray []string

func (sa *StringArray) Scan(value any) error {
	if value == nil {
		*sa = StringArray{}
		return nil
	}

	data, err := jsonbBytes(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, sa)
}

func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return "[]", nil
	}
	return json.Marshal(sa)
}

type Int64Array []int64

func (ia *Int64Array) Scan(value any) error {
	if value == nil {
		*ia = Int64Array{}
		return nil
	}

	data, err := jsonbBytes(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, ia)
}

func (ia Int64Array) Value() (driver.Value, error) {
	if len(ia) == 0 {
		return "[]", nil
	}
	return json.Marshal(ia)
}

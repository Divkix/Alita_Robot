package models

import (
	"strings"
	"time"
)

type BlacklistSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_blacklist_chat_word" json:"chat_id,omitempty"`
	Word      string    `gorm:"column:word;not null;index:idx_blacklist_chat_word" json:"word,omitempty"`
	Action    string    `gorm:"column:action;default:'warn';check:chk_blacklist_action,action IN ('warn','mute','ban','kick','tban','tmute','delete','none')" json:"action,omitempty"`
	Reason    string    `gorm:"column:reason" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

type BlacklistSettingsSlice []*BlacklistSettings

func (bss BlacklistSettingsSlice) Triggers() []string {
	triggers := make([]string, 0, len(bss))
	for _, bs := range bss {
		triggers = append(triggers, bs.Word)
	}
	return triggers
}

func (bss BlacklistSettingsSlice) Action() string {
	if len(bss) > 0 {
		return bss[0].Action
	}
	return "warn"
}

func (bss BlacklistSettingsSlice) Find(trigger string) *BlacklistSettings {
	for _, bs := range bss {
		if bs != nil && strings.EqualFold(bs.Word, trigger) {
			return bs
		}
	}
	return nil
}

func (bss BlacklistSettingsSlice) Reason() string {
	if len(bss) > 0 && bss[0].Reason != "" {
		return bss[0].Reason
	}
	return "Blacklisted word: '%s'"
}

func (BlacklistSettings) TableName() string {
	return "blacklists"
}

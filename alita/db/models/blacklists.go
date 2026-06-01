package models

import "time"

// BlacklistSettings represents blacklist settings for a chat
type BlacklistSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_blacklist_chat_word" json:"chat_id,omitempty"`
	Word      string    `gorm:"column:word;not null;index:idx_blacklist_chat_word" json:"word,omitempty"`
	Action    string    `gorm:"column:action;default:'warn';check:chk_blacklist_action,action IN ('warn','mute','ban','kick','tban','tmute','delete','none')" json:"action,omitempty"`
	Reason    string    `gorm:"column:reason" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// BlacklistSettingsSlice is a custom type for []*BlacklistSettings with additional methods
type BlacklistSettingsSlice []*BlacklistSettings

// Triggers returns all blacklisted words as a slice of strings for compatibility.
func (bss BlacklistSettingsSlice) Triggers() []string {
	triggers := make([]string, 0, len(bss))
	for _, bs := range bss {
		triggers = append(triggers, bs.Word)
	}
	return triggers
}

// Action returns the action for the first blacklist setting in the slice.
func (bss BlacklistSettingsSlice) Action() string {
	if len(bss) > 0 {
		return bss[0].Action
	}
	return "warn" // default
}

// Reason returns the reason for the first blacklist setting in the slice.
func (bss BlacklistSettingsSlice) Reason() string {
	if len(bss) > 0 && bss[0].Reason != "" {
		return bss[0].Reason
	}
	return "Blacklisted word: '%s'" // default format string with placeholder for trigger word
}

func (BlacklistSettings) TableName() string {
	return "blacklists"
}

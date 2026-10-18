package models

import (
	"slices"
	"strings"
	"time"
)

// blacklistActions are the actions chk_blacklist_action allows.
var blacklistActions = []string{"warn", "mute", "ban", "kick", "tban", "tmute", "delete", "none"}

// ValidBlacklistAction reports whether action passes chk_blacklist_action.
func ValidBlacklistAction(action string) bool {
	return slices.Contains(blacklistActions, action)
}

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

// DefaultBlacklistReason is the reason stored with every new blacklist word.
// It stays English in the database so older rows, backups and older releases
// keep working; the watcher shows the chat's translation of it instead.
const DefaultBlacklistReason = "Blacklisted word: '%s'"

func (bss BlacklistSettingsSlice) Reason() string {
	if len(bss) > 0 && bss[0].Reason != "" {
		return bss[0].Reason
	}
	return DefaultBlacklistReason
}

func (BlacklistSettings) TableName() string {
	return "blacklists"
}

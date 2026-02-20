package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// checkRulesSetting retrieves or creates default rules settings for a chat.
// Used internally before performing any rules-related operation.
// Returns default settings with empty rules if the chat doesn't exist.
func checkRulesSetting(chatID int64) (rulesrc *RulesSettings) {
	rulesrc = &RulesSettings{}
	err := GetRecord(rulesrc, RulesSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating rules to satisfy foreign key constraint
		if err := EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] checkRulesSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &RulesSettings{ChatId: chatID, Rules: ""}
		}

		// Create default settings
		rulesrc = &RulesSettings{ChatId: chatID, Rules: ""}
		err := CreateRecord(rulesrc)
		if err != nil {
			log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
		}
	} else if err != nil {
		// Return default on error
		rulesrc = &RulesSettings{ChatId: chatID, Rules: ""}
		log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
	}
	return rulesrc
}

// GetChatRulesInfo returns the rules settings for the specified chat ID.
// This is the public interface to access chat rules information.
func GetChatRulesInfo(chatId int64) *RulesSettings {
	return checkRulesSetting(chatId)
}

// SetChatRules updates the rules text for the specified chat.
// Creates default rules settings if they don't exist.
func SetChatRules(chatId int64, rules string) {
	err := UpdateRecord(&RulesSettings{}, RulesSettings{ChatId: chatId}, RulesSettings{Rules: rules})
	if err != nil {
		log.Errorf("[Database] SetChatRules: %v - %d", err, chatId)
	}
}

// SetChatRulesButton updates the rules button text for the specified chat.
// The button is used to display rules in a more interactive format.
func SetChatRulesButton(chatId int64, rulesButton string) {
	err := UpdateRecord(&RulesSettings{}, RulesSettings{ChatId: chatId}, RulesSettings{RulesBtn: rulesButton})
	if err != nil {
		log.Errorf("[Database] SetChatRulesButton: %v", err)
	}
}

// SetPrivateRules sets whether rules should be sent privately to users instead of in the group.
// When enabled, rules are sent as a private message to the requesting user.
func SetPrivateRules(chatId int64, pref bool) {
	err := UpdateRecordWithZeroValues(&RulesSettings{}, RulesSettings{ChatId: chatId}, map[string]any{"private": pref})
	if err != nil {
		log.Errorf("[Database] SetPrivateRules: %v", err)
	}
}

// LoadRulesStats returns statistics about rules features across all chats.
// Returns the count of chats with rules set and chats with private rules enabled.
func LoadRulesStats() (setRules, pvtRules int64) {
	// Count chats with rules set (non-empty rules)
	err := DB.Model(&RulesSettings{}).Where("rules != ?", "").Count(&setRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (set rules): %v", err)
	}

	// Count chats with private rules enabled
	err = DB.Model(&RulesSettings{}).Where("private = ?", true).Count(&pvtRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (private rules): %v", err)
	}

	return
}

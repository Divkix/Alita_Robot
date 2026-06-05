package rules

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// checkRulesSetting retrieves or creates default rules settings for a chat.
func checkRulesSetting(chatID int64) (rulesrc *models.RulesSettings) {
	rulesrc = &models.RulesSettings{}
	err := db.GetRecord(rulesrc, bson.M{"chat_id": chatID})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Ensure chat exists in database before creating rules
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] checkRulesSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.RulesSettings{ChatId: chatID, Rules: ""}
		}

		// Create default settings
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: "", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		err := db.CreateRecord(rulesrc)
		if err != nil {
			log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
		}
	} else if err != nil {
		// Return default on error
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
	}
	return rulesrc
}

// GetChatRulesInfo returns the rules settings for the specified chat ID.
func GetChatRulesInfo(chatId int64) *models.RulesSettings {
	return checkRulesSetting(chatId)
}

// SetChatRules updates the rules text for the specified chat.
func SetChatRules(chatId int64, rules string) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, bson.M{"chat_id": chatId}, map[string]any{"rules": rules, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] SetChatRules: %v - %d", err, chatId)
	}
}

// SetChatRulesButton updates the rules button text for the specified chat.
func SetChatRulesButton(chatId int64, rulesButton string) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, bson.M{"chat_id": chatId}, map[string]any{"rules_btn": rulesButton, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] SetChatRulesButton: %v", err)
	}
}

// SetPrivateRules sets whether rules should be sent privately to users instead of in the group.
func SetPrivateRules(chatId int64, pref bool) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, bson.M{"chat_id": chatId}, map[string]any{"private": pref, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] SetPrivateRules: %v", err)
	}
}

// LoadRulesStats returns statistics about rules features across all chats.
func LoadRulesStats() (setRules, pvtRules int64) {
	collection := db.DB.Collection("rules")

	// Count chats with rules set (non-empty rules)
	var err error
	setRules, err = collection.CountDocuments(context.Background(), bson.M{"rules": bson.M{"$ne": ""}})
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (set rules): %v", err)
	}

	// Count chats with private rules enabled
	pvtRules, err = collection.CountDocuments(context.Background(), bson.M{"private": true})
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (private rules): %v", err)
	}

	return
}

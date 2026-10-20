package rules

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func checkRulesSetting(chatID int64) (*models.RulesSettings, error) {
	rulesrc := &models.RulesSettings{}
	err := db.GetRecord(rulesrc, models.RulesSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] checkRulesSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.RulesSettings{ChatId: chatID, Rules: ""}, err
		}

		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		err := db.CreateRecord(rulesrc)
		if err != nil {
			log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
			return rulesrc, err
		}
	} else if err != nil {
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
		return rulesrc, err
	}
	return rulesrc, nil
}

func GetChatRulesInfo(chatId int64) *models.RulesSettings {
	rulesrc, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("rules", chatId),
		cache.CacheTTLRules,
		func(context.Context) (*models.RulesSettings, error) {
			return checkRulesSetting(chatId)
		},
	)
	if err != nil || rulesrc == nil {
		rulesrc, _ = checkRulesSetting(chatId)
	}
	return rulesrc
}

func SetChatRules(chatId int64, rules string) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules": rules})
	if err != nil {
		log.Errorf("[Database] SetChatRules: %v - %d", err, chatId)
		return err
	}
	cache.DeleteCache(cache.CacheKey("rules", chatId))
	return nil
}

func SetChatRulesButton(chatId int64, rulesButton string) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules_btn": rulesButton})
	if err != nil {
		log.Errorf("[Database] SetChatRulesButton: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("rules", chatId))
	return nil
}

func SetPrivateRules(chatId int64, pref bool) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"private": pref})
	if err != nil {
		log.Errorf("[Database] SetPrivateRules: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("rules", chatId))
	return nil
}

func LoadRulesStats() (setRules, pvtRules int64) {
	err := db.DB.Model(&models.RulesSettings{}).Where("rules != ?", "").Count(&setRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (set rules): %v", err)
	}

	err = db.DB.Model(&models.RulesSettings{}).Where("private = ?", true).Count(&pvtRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (private rules): %v", err)
	}

	return
}

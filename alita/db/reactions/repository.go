package reactions

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func reactionsCacheKey(chatID int64) string {
	return cache.CacheKey("reactions", chatID)
}

func GetReactions(chatID int64) map[string]string {
	cacheKey := reactionsCacheKey(chatID)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLReactions, func() (map[string]string, error) {
		var rows []*models.Reactions
		if err := db.GetRecords(&rows, models.Reactions{ChatID: chatID}); err != nil {
			log.Errorf("[Database] GetReactions: %v - chat:%d", err, chatID)
			return map[string]string{}, err
		}
		out := make(map[string]string, len(rows))
		for _, r := range rows {
			out[r.Keyword] = r.Emoji
		}
		return out, nil
	})
	if err != nil || result == nil {
		return map[string]string{}
	}
	return result
}

func AddReaction(chatID int64, keyword, emoji string) error {
	r := &models.Reactions{
		ChatID:  chatID,
		Keyword: keyword,
		Emoji:   emoji,
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "keyword"}},
		DoUpdates: clause.AssignmentColumns([]string{"emoji", "updated_at"}),
	}).Create(r).Error
	if err != nil {
		log.Errorf("[Database] AddReaction: %v - chat:%d keyword:%s", err, chatID, keyword)
		return err
	}
	cache.DeleteCache(reactionsCacheKey(chatID))
	return nil
}

func RemoveReaction(chatID int64, keyword string) error {
	result := db.DB.Where("chat_id = ? AND keyword = ?", chatID, keyword).Delete(&models.Reactions{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveReaction: %v - chat:%d keyword:%s", result.Error, chatID, keyword)
		return result.Error
	}
	cache.DeleteCache(reactionsCacheKey(chatID))
	return nil
}

func ResetReactions(chatID int64) error {
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Reactions{}).Error; err != nil {
		log.Errorf("[Database] ResetReactions: %v - chat:%d", err, chatID)
		return err
	}
	cache.DeleteCache(reactionsCacheKey(chatID))
	return nil
}

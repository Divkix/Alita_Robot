package filters

import (
	"context"
	"errors"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func filterListCacheKey(chatID int64) string {
	return cache.CacheKey("filter_list", chatID)
}

func optimizedFilterCacheKey(chatID int64) string {
	return cache.CacheKey("filters_optimized", chatID)
}

func invalidateFilterCaches(chatID int64) {
	cache.DeleteCache(filterListCacheKey(chatID))
	cache.DeleteCache(optimizedFilterCacheKey(chatID))
}

func GetFiltersList(chatID int64) (allFilterWords []string) {
	return GetFiltersListContext(context.Background(), chatID)
}

func GetFiltersListContext(ctx context.Context, chatID int64) (allFilterWords []string) {
	cacheKey := filterListCacheKey(chatID)
	result, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLFilterList, func(ctx context.Context) ([]string, error) {
		var results []*models.ChatFilters
		err := db.GetRecordsContext(ctx, &results, map[string]any{"chat_id": chatID})
		if err != nil {
			log.Errorf("[Database] GetFiltersList: %v - %d", err, chatID)
			return []string{}, err
		}

		filterWords := make([]string, 0, len(results))
		for _, j := range results {
			filterWords = append(filterWords, j.KeyWord)
		}
		return filterWords, nil
	})
	if err != nil {
		return []string{}
	}
	return result
}

// ListFiltersContext returns the chat's filters ordered by lower-cased keyword
// so pages stay stable. query matches the keyword or the reply
// case-insensitively; limit<=0 means every matching row and offset skips
// matching rows. The panel reads replies through this; the bot's matcher keeps
// using the lighter cached keyword list.
func ListFiltersContext(ctx context.Context, chatID int64, query string, limit, offset int) ([]models.ChatFilters, error) {
	tx := db.DB.WithContext(ctx).Model(&models.ChatFilters{}).Where("chat_id = ?", chatID)
	if query != "" {
		pattern := "%" + strings.ToLower(query) + "%"
		tx = tx.Where("(LOWER(keyword) LIKE ? OR LOWER(filter_reply) LIKE ?)", pattern, pattern)
	}
	tx = tx.Order("LOWER(keyword) ASC, keyword ASC")
	if offset > 0 {
		tx = tx.Offset(offset)
	}
	if limit > 0 {
		tx = tx.Limit(limit)
	}

	rows := make([]models.ChatFilters, 0)
	if err := tx.Find(&rows).Error; err != nil {
		log.Errorf("[Database] ListFiltersContext: %v - %d", err, chatID)
		return nil, err
	}
	return rows, nil
}

func DoesFilterExists(chatId int64, keyword string) bool {
	var filter models.ChatFilters
	err := db.DB.Where("chat_id = ? AND LOWER(keyword) = LOWER(?)", chatId, keyword).Take(&filter).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false
		}
		log.Errorf("[Database] DoesFilterExists: %v - %d", err, chatId)
		return false
	}
	return true
}

func AddFilter(chatID int64, keyWord, replyText, fileID string, buttons []models.Button, filtType int) error {
	now := time.Now().UTC()
	newFilter := map[string]any{
		"chat_id":        chatID,
		"keyword":        keyWord,
		"filter_reply":   replyText,
		"msgtype":        filtType,
		"fileid":         fileID,
		"nonotif":        false,
		"filter_buttons": models.ButtonArray(buttons),
		"created_at":     now,
		"updated_at":     now,
	}

	result := db.DB.Model(&models.ChatFilters{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "keyword"}},
		DoNothing: true,
	}).Create(newFilter)
	if result.Error != nil {
		log.Errorf("[Database][AddFilter]: %d - %v", chatID, result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		invalidateFilterCaches(chatID)
	}
	return nil
}

func UpdateFilter(chatID int64, keyWord, replyText, fileID string, buttons []models.Button, filtType int) (bool, error) {
	result := db.DB.Model(&models.ChatFilters{}).
		Where("chat_id = ? AND keyword = ?", chatID, keyWord).
		Updates(map[string]any{
			"filter_reply":   replyText,
			"msgtype":        filtType,
			"fileid":         fileID,
			"filter_buttons": models.ButtonArray(buttons),
			"updated_at":     time.Now().UTC(),
		})
	if result.Error != nil {
		log.Errorf("[Database][UpdateFilter]: %d - %v", chatID, result.Error)
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		invalidateFilterCaches(chatID)
	}
	return result.RowsAffected > 0, nil
}

func RemoveFilter(chatID int64, keyWord string) error {
	result := db.DB.Where("chat_id = ? AND keyword = ?", chatID, keyWord).Delete(&models.ChatFilters{})
	if result.Error != nil {
		log.Errorf("[Database][RemoveFilter]: %d - %v", chatID, result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		invalidateFilterCaches(chatID)
	}
	return nil
}

func RemoveAllFilters(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatFilters{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllFilters]: %d - %v", chatID, err)
		return err
	}

	invalidateFilterCaches(chatID)
	return nil
}

func CountFilters(chatID int64) int64 {
	return db.CountRows(&models.ChatFilters{}, "chat_id = ?", chatID)
}

func LoadFilterStats() (filtersNum, filtersUsingChats int64) {
	err := db.DB.Model(&models.ChatFilters{}).Count(&filtersNum).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting filters: %v", err)
		return
	}

	err = db.DB.Model(&models.ChatFilters{}).Select("COUNT(DISTINCT chat_id)").Scan(&filtersUsingChats).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting chats: %v", err)
		return
	}

	return
}

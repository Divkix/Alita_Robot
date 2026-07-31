package filters

import (
	"errors"
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

// GetFiltersList retrieves a list of all filter keywords for a specific chat ID.
// Uses caching to improve performance for frequently accessed data.
// Returns an empty slice if no filters are found or an error occurs.
func GetFiltersList(chatID int64) (allFilterWords []string) {
	// Try to get from cache first
	cacheKey := filterListCacheKey(chatID)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFilterList, func() ([]string, error) {
		var results []*models.ChatFilters
		err := db.GetRecords(&results, map[string]any{"chat_id": chatID})
		if err != nil {
			log.Errorf("[Database] GetFiltersList: %v - %d", err, chatID)
			return []string{}, err
		}

		// Pre-allocate slice with known capacity to avoid reallocations
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

// DoesFilterExists checks whether a filter with the given keyword exists in the specified chat.
// Performs a case-insensitive comparison of the keyword.
// Returns false if the filter doesn't exist or an error occurs.
// Uses LIMIT 1 optimization for better performance than COUNT.
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

// AddFilter creates a filter if its keyword is unused.
// Explicit overwrite confirmation uses UpdateFilter.
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

// UpdateFilter replaces an existing filter without recreating one removed while
// an overwrite confirmation was pending.
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

// RemoveFilter deletes a filter with the specified keyword from the chat.
// Invalidates the filter list cache if a filter was successfully removed.
func RemoveFilter(chatID int64, keyWord string) error {
	// Directly attempt to delete the filter without checking existence first
	result := db.DB.Where("chat_id = ? AND keyword = ?", chatID, keyWord).Delete(&models.ChatFilters{})
	if result.Error != nil {
		log.Errorf("[Database][RemoveFilter]: %d - %v", chatID, result.Error)
		return result.Error
	}
	// result.RowsAffected will be 0 if no filter was found, which is fine

	// Invalidate cache after removing filter
	if result.RowsAffected > 0 {
		invalidateFilterCaches(chatID)
	}
	return nil
}

// RemoveAllFilters deletes all filters for the specified chat ID from the database.
// Invalidates the filter list cache after successful removal.
func RemoveAllFilters(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatFilters{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllFilters]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after removing all filters
	invalidateFilterCaches(chatID)
	return nil
}

// CountFilters returns the total number of filters configured for the specified chat ID.
// Returns 0 if an error occurs during the count operation.
func CountFilters(chatID int64) (filtersNum int64) {
	err := db.DB.Model(&models.ChatFilters{}).Where("chat_id = ?", chatID).Count(&filtersNum).Error
	if err != nil {
		log.Errorf("[Database][CountFilters]: %d - %v", chatID, err)
	}
	return
}

// LoadFilterStats returns statistics about filters across the entire system.
// Returns the total number of filters and the number of distinct chats using filters.
func LoadFilterStats() (filtersNum, filtersUsingChats int64) {
	// Count total number of filters
	err := db.DB.Model(&models.ChatFilters{}).Count(&filtersNum).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting filters: %v", err)
		return
	}

	// Count distinct chats using filters
	err = db.DB.Model(&models.ChatFilters{}).Select("COUNT(DISTINCT chat_id)").Scan(&filtersUsingChats).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting chats: %v", err)
		return
	}

	return
}

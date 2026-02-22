package db

import (
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetFiltersList retrieves a list of all filter keywords for a specific chat ID.
// Uses caching to improve performance for frequently accessed data.
// Returns an empty slice if no filters are found or an error occurs.
func GetFiltersList(chatID int64) (allFilterWords []string) {
	// Try to get from cache first
	cacheKey := filterListCacheKey(chatID)
	result, err := getFromCacheOrLoad(cacheKey, CacheTTLFilterList, func() ([]string, error) {
		var results []*ChatFilters
		var filterWords []string
		err := GetRecords(&results, map[string]any{"chat_id": chatID})
		if err != nil {
			log.Errorf("[Database] GetFiltersList: %v - %d", err, chatID)
			return []string{}, err
		}

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
	var filter ChatFilters
	err := DB.Where("chat_id = ? AND LOWER(keyword) = LOWER(?)", chatId, keyword).Take(&filter).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false
		}
		log.Errorf("[Database] DoesFilterExists: %v - %d", err, chatId)
		return false
	}
	return true
}

// AddFilter creates a new filter in the database for the specified chat.
// Does nothing if a filter with the same keyword already exists.
// Invalidates the filter list cache after successful addition.
func AddFilter(chatID int64, keyWord, replyText, fileID string, buttons []Button, filtType int) error {
	// Check if filter already exists using optimized query
	var existingFilter ChatFilters
	err := DB.Where("chat_id = ? AND keyword = ?", chatID, keyWord).Take(&existingFilter).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Errorf("[Database][AddFilter] checking existence: %d - %v", chatID, err)
			return err
		}
		// Filter doesn't exist, continue with creation
	} else {
		return nil // Filter already exists
	}

	// add the filter
	newFilter := ChatFilters{
		ChatId:      chatID,
		KeyWord:     keyWord,
		FilterReply: replyText,
		MsgType:     filtType,
		FileID:      fileID,
		Buttons:     ButtonArray(buttons),
	}

	err = CreateRecord(&newFilter)
	if err != nil {
		log.Errorf("[Database][AddFilter]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after adding filter
	deleteCache(filterListCacheKey(chatID))
	return nil
}

// RemoveFilter deletes a filter with the specified keyword from the chat.
// Invalidates the filter list cache if a filter was successfully removed.
func RemoveFilter(chatID int64, keyWord string) error {
	// Directly attempt to delete the filter without checking existence first
	result := DB.Where("chat_id = ? AND keyword = ?", chatID, keyWord).Delete(&ChatFilters{})
	if result.Error != nil {
		log.Errorf("[Database][RemoveFilter]: %d - %v", chatID, result.Error)
		return result.Error
	}
	// result.RowsAffected will be 0 if no filter was found, which is fine

	// Invalidate cache after removing filter
	if result.RowsAffected > 0 {
		deleteCache(filterListCacheKey(chatID))
	}
	return nil
}

// RemoveAllFilters deletes all filters for the specified chat ID from the database.
// Invalidates the filter list cache after removal.
func RemoveAllFilters(chatID int64) {
	err := DB.Where("chat_id = ?", chatID).Delete(&ChatFilters{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllFilters]: %d - %v", chatID, err)
	}

	// Invalidate cache after removing all filters
	deleteCache(filterListCacheKey(chatID))
}

// CountFilters returns the total number of filters configured for the specified chat ID.
// Returns 0 if an error occurs during the count operation.
func CountFilters(chatID int64) (filtersNum int64) {
	err := DB.Model(&ChatFilters{}).Where("chat_id = ?", chatID).Count(&filtersNum).Error
	if err != nil {
		log.Errorf("[Database][CountFilters]: %d - %v", chatID, err)
	}
	return
}

// LoadFilterStats returns statistics about filters across the entire system.
// Returns the total number of filters and the number of distinct chats using filters.
func LoadFilterStats() (filtersNum, filtersUsingChats int64) {
	// Count total number of filters
	err := DB.Model(&ChatFilters{}).Count(&filtersNum).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting filters: %v", err)
		return
	}

	// Count distinct chats using filters
	err = DB.Model(&ChatFilters{}).Select("COUNT(DISTINCT chat_id)").Scan(&filtersUsingChats).Error
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting chats: %v", err)
		return
	}

	return
}

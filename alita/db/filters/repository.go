package filters

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

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
func GetFiltersList(chatID int64) (allFilterWords []string) {
	// Try to get from cache first
	cacheKey := filterListCacheKey(chatID)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFilterList, func() ([]string, error) {
		var results []*models.ChatFilters
		err := db.GetRecords(&results, bson.M{"chat_id": chatID})
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
func DoesFilterExists(chatId int64, keyword string) bool {
	var filter models.ChatFilters
	collection := db.DB.Collection("filters")
	// Using case-insensitive regex for LOWER check
	err := collection.FindOne(context.Background(), bson.M{
		"chat_id": chatId,
		"keyword": bson.M{"$regex": "^" + keyword + "$", "$options": "i"},
	}).Decode(&filter)

	if err != nil {
		if err == db.ErrRecordNotFound {
			return false
		}
		log.Errorf("[Database] DoesFilterExists: %v - %d", err, chatId)
		return false
	}
	return true
}

// AddFilter creates a new filter in the database for the specified chat.
func AddFilter(chatID int64, keyWord, replyText, fileID string, buttons []models.Button, filtType int) error {
	// Check if filter already exists
	var existingFilter models.ChatFilters
	collection := db.DB.Collection("filters")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID, "keyword": keyWord}).Decode(&existingFilter)

	if err == nil {
		return nil // Filter already exists
	} else if err != db.ErrRecordNotFound {
		log.Errorf("[Database][AddFilter] checking existence: %d - %v", chatID, err)
		return err
	}

	// add the filter
	newFilter := models.ChatFilters{
		ChatId:      chatID,
		KeyWord:     keyWord,
		FilterReply: replyText,
		MsgType:     filtType,
		FileID:      fileID,
		Buttons:     models.ButtonArray(buttons),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateRecord(&newFilter)
	if err != nil {
		log.Errorf("[Database][AddFilter]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after adding filter
	invalidateFilterCaches(chatID)
	return nil
}

// RemoveFilter deletes a filter with the specified keyword from the chat.
func RemoveFilter(chatID int64, keyWord string) error {
	collection := db.DB.Collection("filters")
	result, err := collection.DeleteOne(context.Background(), bson.M{"chat_id": chatID, "keyword": keyWord})
	if err != nil {
		log.Errorf("[Database][RemoveFilter]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after removing filter
	if result.DeletedCount > 0 {
		invalidateFilterCaches(chatID)
	}
	return nil
}

// RemoveAllFilters deletes all filters for the specified chat ID from the database.
func RemoveAllFilters(chatID int64) error {
	collection := db.DB.Collection("filters")
	_, err := collection.DeleteMany(context.Background(), bson.M{"chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][RemoveAllFilters]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after removing all filters
	invalidateFilterCaches(chatID)
	return nil
}

// CountFilters returns the total number of filters configured for the specified chat ID.
func CountFilters(chatID int64) (filtersNum int64) {
	collection := db.DB.Collection("filters")
	filtersNum, err := collection.CountDocuments(context.Background(), bson.M{"chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][CountFilters]: %d - %v", chatID, err)
	}
	return
}

// LoadFilterStats returns statistics about filters across the entire system.
func LoadFilterStats() (filtersNum, filtersUsingChats int64) {
	collection := db.DB.Collection("filters")

	// Count total number of filters
	var err error
	filtersNum, err = collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting filters: %v", err)
		return
	}

	// Count distinct chats using filters
	distinctChats, err := collection.Distinct(context.Background(), "chat_id", bson.M{})
	if err != nil {
		log.Errorf("[Database][LoadFilterStats] counting chats: %v", err)
		return
	}
	filtersUsingChats = int64(len(distinctChats))

	return
}

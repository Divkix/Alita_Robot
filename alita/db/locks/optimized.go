package locks

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetLockStatus retrieves only the lock status for a specific lock type.
func GetLockStatus(chatID int64, lockType string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}

	var lock models.LockSettings
	collection := db.DB.Collection("locks")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID, "lock_type": lockType}, options.FindOne().SetProjection(bson.M{"locked": 1})).Decode(&lock)

	if err != nil {
		if err == db.ErrRecordNotFound {
			return false, nil
		}
		log.Errorf("[OptimizedLockQueries] GetLockStatus: %v", err)
		return false, err
	}

	return lock.Locked, nil
}

// GetChatLocksOptimized retrieves all locks for a chat with minimal column selection.
func GetChatLocksOptimized(chatID int64) (map[string]bool, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var locks []models.LockSettings
	collection := db.DB.Collection("locks")
	cursor, err := collection.Find(context.Background(), bson.M{"chat_id": chatID}, options.Find().SetProjection(bson.M{"lock_type": 1, "locked": 1}))
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetChatLocksOptimized: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err := cursor.All(context.Background(), &locks); err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, lock := range locks {
		result[lock.LockType] = lock.Locked
	}

	return result, nil
}

// GetLockStatusCached retrieves lock status with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func GetLockStatusCached(chatID int64, lockType string) (bool, error) {
	cacheKey := cache.CacheKey("lock", chatID, lockType)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 1*time.Hour, func() (bool, error) {
		return GetLockStatus(chatID, lockType)
	})
	if err != nil {
		// Fallback to direct query on cache error
		return GetLockStatus(chatID, lockType)
	}

	return cached, nil
}

package locks

import (
	"context"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetChatLocks retrieves all lock settings for a specific chat ID.
func GetChatLocks(chatID int64) map[string]bool {
	// Use optimized query with caching
	locks, err := GetChatLocksOptimized(chatID)
	if err != nil {
		log.Errorf("[Database] GetChatLocks: %v - %d", err, chatID)
		return make(map[string]bool)
	}

	return locks
}

// UpdateLock atomically upserts a lock record for the given chat and permission type.
func UpdateLock(chatID int64, perm string, val bool) error {
	collection := db.DB.Collection("locks")
	filter := bson.M{"chat_id": chatID, "lock_type": perm}
	update := bson.M{"$set": bson.M{"locked": val, "updated_at": time.Now()}}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] UpdateLock: %v", err)
		return err
	}

	// Invalidate cache to ensure immediate enforcement
	InvalidateLockCache(chatID, perm)
	return nil
}

// InvalidateLockCache removes the cached lock status for a specific chat and lock type.
func InvalidateLockCache(chatID int64, lockType string) {
	m := utilsCache.GetMarshal()
	if m == nil {
		return
	}

	cacheKey := cache.CacheKey("lock", chatID, lockType)
	err := m.Delete(utilsCache.Context, cacheKey)
	if err != nil {
		log.Debugf("[Cache] Failed to invalidate lock cache for key %s: %v", cacheKey, err)
	}
}

// IsPermLocked checks whether a specific permission type is locked in the given chat.
func IsPermLocked(chatID int64, perm string) bool {
	// Use optimized cached query
	locked, err := GetLockStatusCached(chatID, perm)
	if err != nil {
		log.Errorf("[Database] IsPermLocked: %v - %d", err, chatID)
		return false
	}

	return locked
}

package locks

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

// GetChatLocks retrieves all lock settings for a specific chat ID.
// Uses optimized queries with caching for better performance.
// Returns an empty map if no locks are found or an error occurs.
func GetChatLocks(chatID int64) map[string]bool {
	// Use cached whole-map query to avoid per-message DB round-trips
	locks, err := GetChatLocksCached(chatID)
	if err != nil {
		log.Errorf("[Database] GetChatLocks: %v - %d", err, chatID)
		return make(map[string]bool)
	}

	return locks
}

// UpdateLock atomically upserts a lock record for the given chat and permission type.
// Uses INSERT ... ON CONFLICT DO UPDATE for atomicity under concurrent writes.
// Invalidates the cache after successful update to ensure immediate enforcement.
// Returns an error if the database operation fails.
func UpdateLock(chatID int64, perm string, val bool) error {
	record := models.LockSettings{
		ChatId:   chatID,
		LockType: perm,
		Locked:   val,
	}

	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "lock_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"locked"}),
	}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] UpdateLock: %v", err)
		return err
	}

	// Invalidate cache to ensure immediate enforcement
	InvalidateLockCache(chatID)
	return nil
}

// InvalidateLockCache removes the cached whole-map lock status for a chat so that
// GetChatLocks reflects the change immediately.
// Should be called after updating a lock to ensure immediate enforcement.
func InvalidateLockCache(chatID int64) {
	m := utilsCache.GetMarshal()
	if m == nil {
		return
	}

	mapKey := cache.CacheKey("locks_map", chatID)
	if err := m.Delete(utilsCache.Context, mapKey); err != nil {
		log.Debugf("[Cache] Failed to invalidate locks_map cache for key %s: %v", mapKey, err)
	}
}

// IsPermLocked checks whether a specific permission type is locked in the given chat.
// Uses the cached whole-map lock query for better performance.
// Returns false if the permission is not locked or an error occurs.
func IsPermLocked(chatID int64, perm string) bool {
	return GetChatLocks(chatID)[perm]
}

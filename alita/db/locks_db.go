package db

import (
	"fmt"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

// GetChatLocks retrieves all lock settings for a specific chat ID.
// Uses optimized queries with caching for better performance.
// Returns an empty map if no locks are found or an error occurs.
func GetChatLocks(chatID int64) map[string]bool {
	// Use optimized query with caching
	locks, err := GetOptimizedQueries().lockQueries.GetChatLocksOptimized(chatID)
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
	record := LockSettings{
		ChatId:   chatID,
		LockType: perm,
		Locked:   val,
	}

	err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "lock_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"locked"}),
	}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] UpdateLock: %v", err)
		return err
	}

	// Invalidate cache to ensure immediate enforcement
	InvalidateLockCache(chatID, perm)
	return nil
}

// InvalidateLockCache removes the cached lock status for a specific chat and lock type.
// Should be called after updating a lock to ensure immediate enforcement.
func InvalidateLockCache(chatID int64, lockType string) {
	if cache.Marshal == nil {
		return
	}

	cacheKey := fmt.Sprintf("alita:lock:%d:%s", chatID, lockType)
	err := cache.Marshal.Delete(cache.Context, cacheKey)
	if err != nil {
		log.Debugf("[Cache] Failed to invalidate lock cache for key %s: %v", cacheKey, err)
	}
}

// IsPermLocked checks whether a specific permission type is locked in the given chat.
// Uses optimized cached queries for better performance.
// Returns false if the permission is not locked or an error occurs.
func IsPermLocked(chatID int64, perm string) bool {
	// Use optimized cached query
	locked, err := GetOptimizedQueries().GetLockStatusCached(chatID, perm)
	if err != nil {
		log.Errorf("[Database] IsPermLocked: %v - %d", err, chatID)
		return false
	}

	return locked
}

package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// BackupRateLimiter provides rate limiting for backup operations
type BackupRateLimiter struct {
	mu sync.RWMutex
}

var (
	// Singleton instance
	backupLimiter *BackupRateLimiter
	once          = &sync.Once{}
)

// GetBackupRateLimiter returns the singleton rate limiter instance
func GetBackupRateLimiter() *BackupRateLimiter {
	once.Do(func() {
		backupLimiter = &BackupRateLimiter{}
	})
	return backupLimiter
}

// Cache key prefixes for rate limiting
const (
	exportRatePrefix = "backup:export:"
	importRatePrefix = "backup:import:"
	resetRatePrefix  = "backup:reset:"
)

// Default cooldown periods
const (
	DefaultExportCooldown = 5 * time.Minute
	DefaultImportCooldown = 10 * time.Minute
	DefaultResetCooldown  = 1 * time.Hour
)

// AcquireExport atomically reserves the export cooldown for a chat.
func (r *BackupRateLimiter) AcquireExport(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(exportRatePrefix+strconv.FormatInt(chatID, 10), DefaultExportCooldown)
}

// CanImport checks if an import operation is allowed for the given chat
func (r *BackupRateLimiter) CanImport(chatID int64) (bool, time.Duration) {
	return r.canOperate(importRatePrefix+strconv.FormatInt(chatID, 10), DefaultImportCooldown)
}

// AcquireImport atomically reserves the import cooldown for a chat.
func (r *BackupRateLimiter) AcquireImport(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(importRatePrefix+strconv.FormatInt(chatID, 10), DefaultImportCooldown)
}

// AcquireReset atomically reserves the reset cooldown for a chat.
func (r *BackupRateLimiter) AcquireReset(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(resetRatePrefix+strconv.FormatInt(chatID, 10), DefaultResetCooldown)
}

func (r *BackupRateLimiter) canOperate(cacheKey string, cooldown time.Duration) (bool, time.Duration) {
	if client := cache.GetRedisClient(); client != nil {
		remaining, err := client.TTL(cache.Context, cacheKey).Result()
		if err != nil || remaining <= 0 {
			return true, 0
		}
		return false, remaining
	}

	lastOperation, err := r.getLastOperation(cacheKey)
	if err != nil {
		return true, 0
	}
	elapsed := time.Since(lastOperation)
	if elapsed >= cooldown {
		return true, 0
	}
	return false, cooldown - elapsed
}

func (r *BackupRateLimiter) acquireOperation(cacheKey string, cooldown time.Duration) (bool, time.Duration) {
	if client := cache.GetRedisClient(); client != nil {
		acquired, err := client.SetNX(cache.Context, cacheKey, time.Now().Unix(), cooldown).Result()
		if err != nil {
			log.Debugf("[BackupRateLimit] Failed to reserve operation for key %s: %v", cacheKey, err)
			return true, 0
		}
		if acquired {
			return true, 0
		}
		remaining, err := client.TTL(cache.Context, cacheKey).Result()
		if err != nil || remaining <= 0 {
			return false, cooldown
		}
		return false, remaining
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	m := cache.GetMarshal()
	if m == nil {
		return true, 0
	}
	var timestamp time.Time
	if _, err := m.Get(context.Background(), cacheKey, &timestamp); err == nil {
		if remaining := cooldown - time.Since(timestamp); remaining > 0 {
			return false, remaining
		}
	}
	if err := m.Set(context.Background(), cacheKey, time.Now(), store.WithExpiration(cooldown)); err != nil {
		log.Debugf("[BackupRateLimit] Failed to reserve operation for key %s: %v", cacheKey, err)
		return true, 0
	}
	return true, 0
}

// getLastOperation retrieves the timestamp of the last operation from cache
func (r *BackupRateLimiter) getLastOperation(cacheKey string) (time.Time, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m := cache.GetMarshal()
	if m == nil {
		return time.Time{}, fmt.Errorf("cache not initialized")
	}

	// Try to get from cache
	var timestamp time.Time
	_, err := m.Get(context.Background(), cacheKey, &timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("no record found: %w", err)
	}

	return timestamp, nil
}

// recordOperation stores the current timestamp in cache
func (r *BackupRateLimiter) recordOperation(cacheKey string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := cache.GetMarshal()
	if m == nil {
		return
	}

	if err := m.Set(context.Background(), cacheKey, time.Now(), store.WithExpiration(ttl)); err != nil {
		log.Debugf("[BackupRateLimit] Failed to record operation for key %s: %v", cacheKey, err)
	}
}

// FormatCooldown formats a duration as a human-readable string
func FormatCooldown(duration time.Duration) string {
	if duration < time.Minute {
		seconds := int(duration.Seconds())
		return fmt.Sprintf("%d second%s", seconds, pluralSuffix(seconds))
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%d minute%s %d second%s", minutes, pluralSuffix(minutes), seconds, pluralSuffix(seconds))
		}
		return fmt.Sprintf("%d minute%s", minutes, pluralSuffix(minutes))
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%d hour%s %d minute%s", hours, pluralSuffix(hours), minutes, pluralSuffix(minutes))
	}
	return fmt.Sprintf("%d hour%s", hours, pluralSuffix(hours))
}

func pluralSuffix(value int) string {
	if value == 1 {
		return ""
	}
	return "s"
}

package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/eko/gocache/lib/v4/store"
)

// BackupRateLimiter provides rate limiting for backup operations
type BackupRateLimiter struct {
	mu sync.RWMutex
}

var (
	// Singleton instance
	backupLimiter *BackupRateLimiter
	once          sync.Once
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

// CanExport checks if an export operation is allowed for the given chat
// Returns true if allowed, and remaining cooldown if not
func (r *BackupRateLimiter) CanExport(chatID int64) (bool, time.Duration) {
	cacheKey := exportRatePrefix + strconv.FormatInt(chatID, 10)

	lastExport, err := r.getLastOperation(cacheKey)
	if err != nil {
		// No previous export found or error, allow it
		return true, 0
	}

	elapsed := time.Since(lastExport)
	if elapsed >= DefaultExportCooldown {
		return true, 0
	}

	return false, DefaultExportCooldown - elapsed
}

// RecordExport records an export operation for rate limiting
func (r *BackupRateLimiter) RecordExport(chatID int64) {
	cacheKey := exportRatePrefix + strconv.FormatInt(chatID, 10)
	r.recordOperation(cacheKey)
}

// CanImport checks if an import operation is allowed for the given chat
func (r *BackupRateLimiter) CanImport(chatID int64) (bool, time.Duration) {
	cacheKey := importRatePrefix + strconv.FormatInt(chatID, 10)

	lastImport, err := r.getLastOperation(cacheKey)
	if err != nil {
		return true, 0
	}

	elapsed := time.Since(lastImport)
	if elapsed >= DefaultImportCooldown {
		return true, 0
	}

	return false, DefaultImportCooldown - elapsed
}

// RecordImport records an import operation for rate limiting
func (r *BackupRateLimiter) RecordImport(chatID int64) {
	cacheKey := importRatePrefix + strconv.FormatInt(chatID, 10)
	r.recordOperation(cacheKey)
}

// CanReset checks if a reset operation is allowed for the given chat
func (r *BackupRateLimiter) CanReset(chatID int64) (bool, time.Duration) {
	cacheKey := resetRatePrefix + strconv.FormatInt(chatID, 10)

	lastReset, err := r.getLastOperation(cacheKey)
	if err != nil {
		return true, 0
	}

	elapsed := time.Since(lastReset)
	if elapsed >= DefaultResetCooldown {
		return true, 0
	}

	return false, DefaultResetCooldown - elapsed
}

// RecordReset records a reset operation for rate limiting
func (r *BackupRateLimiter) RecordReset(chatID int64) {
	cacheKey := resetRatePrefix + strconv.FormatInt(chatID, 10)
	r.recordOperation(cacheKey)
}

// getLastOperation retrieves the timestamp of the last operation from cache
func (r *BackupRateLimiter) getLastOperation(cacheKey string) (time.Time, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if cache.Marshal == nil {
		return time.Time{}, fmt.Errorf("cache not initialized")
	}

	// Try to get from cache
	var timestamp time.Time
	_, err := cache.Marshal.Get(context.Background(), cacheKey, &timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("no record found: %w", err)
	}

	return timestamp, nil
}

// recordOperation stores the current timestamp in cache
func (r *BackupRateLimiter) recordOperation(cacheKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cache.Marshal == nil {
		return
	}

	// Determine TTL based on operation type
	var ttl time.Duration
	switch {
	case len(cacheKey) > len(exportRatePrefix) && cacheKey[:len(exportRatePrefix)] == exportRatePrefix:
		ttl = DefaultExportCooldown
	case len(cacheKey) > len(importRatePrefix) && cacheKey[:len(importRatePrefix)] == importRatePrefix:
		ttl = DefaultImportCooldown
	case len(cacheKey) > len(resetRatePrefix) && cacheKey[:len(resetRatePrefix)] == resetRatePrefix:
		ttl = DefaultResetCooldown
	default:
		ttl = 1 * time.Hour
	}

	_ = cache.Marshal.Set(context.Background(), cacheKey, time.Now(), store.WithExpiration(ttl))
}

// FormatCooldown formats a duration as a human-readable string
func FormatCooldown(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%d seconds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%d minutes %d seconds", minutes, seconds)
		}
		return fmt.Sprintf("%d minutes", minutes)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d hours", hours)
}
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

type BackupRateLimiter struct {
	mu sync.RWMutex
}

var (
	backupLimiter *BackupRateLimiter
	once          = &sync.Once{}
)

func GetBackupRateLimiter() *BackupRateLimiter {
	once.Do(func() {
		backupLimiter = &BackupRateLimiter{}
	})
	return backupLimiter
}

const (
	exportRatePrefix = "backup:export:"
	importRatePrefix = "backup:import:"
	resetRatePrefix  = "backup:reset:"
)

const (
	DefaultExportCooldown = 5 * time.Minute
	DefaultImportCooldown = 10 * time.Minute
	DefaultResetCooldown  = 1 * time.Hour
)

func (r *BackupRateLimiter) AcquireExport(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(exportRatePrefix+strconv.FormatInt(chatID, 10), DefaultExportCooldown)
}

func (r *BackupRateLimiter) AcquireImport(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(importRatePrefix+strconv.FormatInt(chatID, 10), DefaultImportCooldown)
}

func (r *BackupRateLimiter) AcquireReset(chatID int64) (bool, time.Duration) {
	return r.acquireOperation(resetRatePrefix+strconv.FormatInt(chatID, 10), DefaultResetCooldown)
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

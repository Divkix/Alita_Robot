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

// FederationRateLimiter provides rate limiting for federation export operations.
type FederationRateLimiter struct {
	mu sync.RWMutex
}

var (
	fedLimiter     *FederationRateLimiter
	fedLimiterOnce = &sync.Once{}
)

// GetFederationRateLimiter returns the singleton federation rate limiter.
func GetFederationRateLimiter() *FederationRateLimiter {
	fedLimiterOnce.Do(func() {
		fedLimiter = &FederationRateLimiter{}
	})
	return fedLimiter
}

const (
	fbanListRatePrefix = "federation:fbanlist:"
	// DefaultFbanListCooldown is the cooldown between /fbanlist exports.
	DefaultFbanListCooldown = 30 * time.Minute
)

// CanExportFbanList checks if a federation banlist export is allowed.
func (r *FederationRateLimiter) CanExportFbanList(ownerID int64) (bool, time.Duration) {
	cacheKey := fbanListRatePrefix + strconv.FormatInt(ownerID, 10)

	lastExport, err := r.getLastOperation(cacheKey)
	if err != nil {
		return true, 0
	}

	elapsed := time.Since(lastExport)
	if elapsed >= DefaultFbanListCooldown {
		return true, 0
	}

	return false, DefaultFbanListCooldown - elapsed
}

// RecordFbanListExport records a federation banlist export for rate limiting.
func (r *FederationRateLimiter) RecordFbanListExport(ownerID int64) {
	cacheKey := fbanListRatePrefix + strconv.FormatInt(ownerID, 10)
	r.recordOperation(cacheKey, DefaultFbanListCooldown)
}

func (r *FederationRateLimiter) getLastOperation(cacheKey string) (time.Time, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m := cache.GetMarshal()
	if m == nil {
		return time.Time{}, fmt.Errorf("cache not initialized")
	}

	var timestamp time.Time
	_, err := m.Get(context.Background(), cacheKey, &timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("no record found: %w", err)
	}

	return timestamp, nil
}

func (r *FederationRateLimiter) recordOperation(cacheKey string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := cache.GetMarshal()
	if m == nil {
		return
	}

	if err := m.Set(context.Background(), cacheKey, time.Now(), store.WithExpiration(ttl)); err != nil {
		log.Debugf("[FederationRateLimit] Failed to record operation for key %s: %v", cacheKey, err)
	}
}

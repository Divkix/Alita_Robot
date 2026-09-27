package ratelimit

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/state"
)

const (
	nsfwWindow       = 24 * time.Hour
	nsfwAdminTokens  = 100
	nsfwMemberTokens = 10
)

type nsfwBucket struct {
	Tokens float64
	Last   time.Time
}

// NSFWRateLimiter atomically reserves scan tokens for each user across allowed chats.
type NSFWRateLimiter struct {
	mu sync.Mutex
}

var defaultNSFWRateLimiter NSFWRateLimiter

// GetNSFWRateLimiter returns the process-wide limiter.
func GetNSFWRateLimiter() *NSFWRateLimiter { return &defaultNSFWRateLimiter }

// Acquire consumes one token. Buckets start full and refill continuously over 24 hours.
func (r *NSFWRateLimiter) Acquire(userID int64, isAdmin bool, now time.Time) (bool, time.Duration) {
	capacity := nsfwMemberTokens
	role := "member"
	if isAdmin {
		capacity = nsfwAdminTokens
		role = "admin"
	}
	key := fmt.Sprintf("nsfw:bucket:%s:%d", role, userID)
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx := context.Background()
	bucket, found := state.Get[nsfwBucket](ctx, key)
	if !found {
		bucket = nsfwBucket{Tokens: float64(capacity), Last: now}
	} else if elapsed := now.Sub(bucket.Last); elapsed > 0 {
		bucket.Tokens = math.Min(float64(capacity), bucket.Tokens+float64(capacity)*float64(elapsed)/float64(nsfwWindow))
		bucket.Last = now
	}
	bucket.Tokens = math.Min(bucket.Tokens, float64(capacity))
	if bucket.Tokens < 1 {
		wait := time.Duration(math.Ceil((1 - bucket.Tokens) * float64(nsfwWindow) / float64(capacity)))
		state.Set(ctx, key, bucket, nsfwWindow)
		return false, wait
	}
	bucket.Tokens--
	state.Set(ctx, key, bucket, nsfwWindow)
	return true, 0
}

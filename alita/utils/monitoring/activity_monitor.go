package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// ActivityMonitor handles automatic tracking and cleanup of chat activity
type ActivityMonitor struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	stopOnce              sync.Once
	checkInterval         time.Duration
	inactivityThreshold   time.Duration
	enableAutoCleanup     bool
	metricsLock           sync.RWMutex
	lastMetrics           *ActivityMetrics
	lastMetricsCalculated time.Time
}

// ActivityMetrics holds calculated activity metrics
type ActivityMetrics struct {
	DailyActiveGroups   int64
	WeeklyActiveGroups  int64
	MonthlyActiveGroups int64
	TotalGroups         int64
	InactiveGroups      int64
	DailyActiveUsers    int64
	WeeklyActiveUsers   int64
	MonthlyActiveUsers  int64
	TotalUsers          int64
	CalculatedAt        time.Time
}

// NewActivityMonitor creates a new activity monitor instance
func NewActivityMonitor() *ActivityMonitor {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel stored in struct field, called in Stop()

	// Default values, can be overridden by environment variables
	checkInterval := 1 * time.Hour
	inactivityThreshold := 30 * 24 * time.Hour // 30 days

	// Check for environment variable overrides
	if config.AppConfig.ActivityCheckInterval > 0 {
		checkInterval = time.Duration(config.AppConfig.ActivityCheckInterval) * time.Hour
	}
	if config.AppConfig.InactivityThresholdDays > 0 {
		inactivityThreshold = time.Duration(config.AppConfig.InactivityThresholdDays) * 24 * time.Hour
	}
	enableAutoCleanup := config.AppConfig.EnableAutoCleanup

	return &ActivityMonitor{
		ctx:                 ctx,
		cancel:              cancel,
		checkInterval:       checkInterval,
		inactivityThreshold: inactivityThreshold,
		enableAutoCleanup:   enableAutoCleanup,
	}
}

// Start begins the activity monitoring background job
func (am *ActivityMonitor) Start() {
	log.Info("[ActivityMonitor] Starting activity monitoring service")
	log.Infof("[ActivityMonitor] Check interval: %v, Inactivity threshold: %v, Auto-cleanup: %v",
		am.checkInterval, am.inactivityThreshold, am.enableAutoCleanup)

	am.wg.Add(1)
	go am.monitorLoop()

	// Calculate initial metrics
	am.calculateMetrics()
}

// Stop gracefully stops the activity monitor
func (am *ActivityMonitor) Stop() {
	am.stopOnce.Do(func() {
		log.Info("[ActivityMonitor] Stopping activity monitoring service")
		am.cancel()
		am.wg.Wait()
	})
}

// monitorLoop runs the periodic activity check
func (am *ActivityMonitor) monitorLoop() {
	defer am.wg.Done()

	ticker := time.NewTicker(am.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			am.performActivityCheck()
		case <-am.ctx.Done():
			return
		}
	}
}

// performActivityCheck checks all chats for activity and marks inactive ones
func (am *ActivityMonitor) performActivityCheck() {
	startTime := time.Now()
	log.Info("[ActivityMonitor] Starting activity check")

	// Calculate current metrics
	am.calculateMetrics()

	if !am.enableAutoCleanup {
		log.Info("[ActivityMonitor] Auto-cleanup disabled, skipping inactive chat marking")
		return
	}

	// Find and mark inactive chats
	inactiveThreshold := time.Now().Add(-am.inactivityThreshold)

	collection := db.DB.Collection("chats")

	res, err := collection.UpdateMany(am.ctx, bson.M{
		"is_inactive":   false,
		"last_activity": bson.M{"$lt": inactiveThreshold},
	}, bson.M{"$set": bson.M{"is_inactive": true, "updated_at": time.Now()}})

	if err != nil {
		log.Errorf("[ActivityMonitor] Error marking inactive chats: %v", err)
		return
	}

	if res.ModifiedCount > 0 {
		log.Infof("[ActivityMonitor] Marked %d chats as inactive (no activity for %v)",
			res.ModifiedCount, am.inactivityThreshold)
	}

	// Reactivate chats that have recent activity
	reactivateRes, err := collection.UpdateMany(am.ctx, bson.M{
		"is_inactive":   true,
		"last_activity": bson.M{"$gte": inactiveThreshold},
	}, bson.M{"$set": bson.M{"is_inactive": false, "updated_at": time.Now()}})

	if err != nil {
		log.Errorf("[ActivityMonitor] Error reactivating chats: %v", err)
		return
	}

	if reactivateRes.ModifiedCount > 0 {
		log.Infof("[ActivityMonitor] Reactivated %d chats with recent activity", reactivateRes.ModifiedCount)
	}

	elapsed := time.Since(startTime)
	log.Infof("[ActivityMonitor] Activity check completed in %v", elapsed)
}

// calculateMetrics calculates activity metrics in parallel for improved performance.
func (am *ActivityMonitor) calculateMetrics() {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	metrics := &ActivityMetrics{
		CalculatedAt: now,
	}

	var wg sync.WaitGroup
	collectionChats := db.DB.Collection("chats")
	collectionUsers := db.DB.Collection("users")

	// Group 1: Chat metrics (5 queries) - run in parallel
	wg.Add(5)

	go func() {
		defer wg.Done()
		metrics.DailyActiveGroups, _ = collectionChats.CountDocuments(am.ctx, bson.M{
			"is_inactive":   false,
			"last_activity": bson.M{"$gte": dayAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.WeeklyActiveGroups, _ = collectionChats.CountDocuments(am.ctx, bson.M{
			"is_inactive":   false,
			"last_activity": bson.M{"$gte": weekAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.MonthlyActiveGroups, _ = collectionChats.CountDocuments(am.ctx, bson.M{
			"is_inactive":   false,
			"last_activity": bson.M{"$gte": monthAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.TotalGroups, _ = collectionChats.CountDocuments(am.ctx, bson.M{})
	}()

	go func() {
		defer wg.Done()
		metrics.InactiveGroups, _ = collectionChats.CountDocuments(am.ctx, bson.M{"is_inactive": true})
	}()

	// Group 2: User metrics (4 queries) - run in parallel
	wg.Add(4)

	go func() {
		defer wg.Done()
		metrics.DailyActiveUsers, _ = collectionUsers.CountDocuments(am.ctx, bson.M{
			"last_activity": bson.M{"$gte": dayAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.WeeklyActiveUsers, _ = collectionUsers.CountDocuments(am.ctx, bson.M{
			"last_activity": bson.M{"$gte": weekAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.MonthlyActiveUsers, _ = collectionUsers.CountDocuments(am.ctx, bson.M{
			"last_activity": bson.M{"$gte": monthAgo},
		})
	}()

	go func() {
		defer wg.Done()
		metrics.TotalUsers, _ = collectionUsers.CountDocuments(am.ctx, bson.M{})
	}()

	// Wait for all queries to complete
	wg.Wait()

	// Store metrics
	am.metricsLock.Lock()
	am.lastMetrics = metrics
	am.lastMetricsCalculated = now
	am.metricsLock.Unlock()

	log.WithFields(log.Fields{
		"daily_active_groups":   metrics.DailyActiveGroups,
		"weekly_active_groups":  metrics.WeeklyActiveGroups,
		"monthly_active_groups": metrics.MonthlyActiveGroups,
		"total_groups":          metrics.TotalGroups,
		"inactive_groups":       metrics.InactiveGroups,
		"daily_active_users":    metrics.DailyActiveUsers,
		"weekly_active_users":   metrics.WeeklyActiveUsers,
		"monthly_active_users":  metrics.MonthlyActiveUsers,
		"total_users":           metrics.TotalUsers,
	}).Info("[ActivityMonitor] Metrics calculated")
}

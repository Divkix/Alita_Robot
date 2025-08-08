package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/divideprojects/Alita_Robot/alita/config"
	"github.com/divideprojects/Alita_Robot/alita/db"
	log "github.com/sirupsen/logrus"
)

// ActivityMonitor handles automatic tracking and cleanup of chat activity
type ActivityMonitor struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
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
	ctx, cancel := context.WithCancel(context.Background())
	
	// Default values, can be overridden by environment variables
	checkInterval := 1 * time.Hour
	inactivityThreshold := 30 * 24 * time.Hour // 30 days
	enableAutoCleanup := true
	
	// Check for environment variable overrides
	if config.ActivityCheckInterval > 0 {
		checkInterval = time.Duration(config.ActivityCheckInterval) * time.Hour
	}
	if config.InactivityThresholdDays > 0 {
		inactivityThreshold = time.Duration(config.InactivityThresholdDays) * 24 * time.Hour
	}
	if config.EnableAutoCleanup != nil {
		enableAutoCleanup = *config.EnableAutoCleanup
	}
	
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
	log.Info("[ActivityMonitor] Stopping activity monitoring service")
	am.cancel()
	am.wg.Wait()
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
	
	result := db.DB.Model(&db.Chat{}).
		Where("is_inactive = ? AND last_activity < ?", false, inactiveThreshold).
		Update("is_inactive", true)
	
	if result.Error != nil {
		log.Errorf("[ActivityMonitor] Error marking inactive chats: %v", result.Error)
		return
	}
	
	if result.RowsAffected > 0 {
		log.Infof("[ActivityMonitor] Marked %d chats as inactive (no activity for %v)",
			result.RowsAffected, am.inactivityThreshold)
	}
	
	// Reactivate chats that have recent activity
	reactivateResult := db.DB.Model(&db.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", true, inactiveThreshold).
		Update("is_inactive", false)
	
	if reactivateResult.Error != nil {
		log.Errorf("[ActivityMonitor] Error reactivating chats: %v", reactivateResult.Error)
		return
	}
	
	if reactivateResult.RowsAffected > 0 {
		log.Infof("[ActivityMonitor] Reactivated %d chats with recent activity", reactivateResult.RowsAffected)
	}
	
	elapsed := time.Since(startTime)
	log.Infof("[ActivityMonitor] Activity check completed in %v", elapsed)
}

// calculateMetrics calculates activity metrics
func (am *ActivityMonitor) calculateMetrics() {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)
	
	metrics := &ActivityMetrics{
		CalculatedAt: now,
	}
	
	// Count daily active groups
	err := db.DB.Model(&db.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
		Count(&metrics.DailyActiveGroups).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting daily active groups: %v", err)
	}
	
	// Count weekly active groups
	err = db.DB.Model(&db.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
		Count(&metrics.WeeklyActiveGroups).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting weekly active groups: %v", err)
	}
	
	// Count monthly active groups
	err = db.DB.Model(&db.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
		Count(&metrics.MonthlyActiveGroups).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting monthly active groups: %v", err)
	}
	
	// Count total groups
	err = db.DB.Model(&db.Chat{}).Count(&metrics.TotalGroups).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting total groups: %v", err)
	}
	
	// Count inactive groups
	err = db.DB.Model(&db.Chat{}).
		Where("is_inactive = ?", true).
		Count(&metrics.InactiveGroups).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting inactive groups: %v", err)
	}
	
	// Count user activity metrics
	// Count daily active users
	err = db.DB.Model(&db.User{}).
		Where("last_activity >= ?", dayAgo).
		Count(&metrics.DailyActiveUsers).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting daily active users: %v", err)
	}
	
	// Count weekly active users
	err = db.DB.Model(&db.User{}).
		Where("last_activity >= ?", weekAgo).
		Count(&metrics.WeeklyActiveUsers).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting weekly active users: %v", err)
	}
	
	// Count monthly active users
	err = db.DB.Model(&db.User{}).
		Where("last_activity >= ?", monthAgo).
		Count(&metrics.MonthlyActiveUsers).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting monthly active users: %v", err)
	}
	
	// Count total users
	err = db.DB.Model(&db.User{}).Count(&metrics.TotalUsers).Error
	if err != nil {
		log.Errorf("[ActivityMonitor] Error counting total users: %v", err)
	}
	
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

// GetMetrics returns the last calculated activity metrics
func (am *ActivityMonitor) GetMetrics() *ActivityMetrics {
	am.metricsLock.RLock()
	defer am.metricsLock.RUnlock()
	
	// If metrics are stale (> 5 minutes old), return nil to trigger recalculation
	if time.Since(am.lastMetricsCalculated) > 5*time.Minute {
		return nil
	}
	
	return am.lastMetrics
}

// GetMetricsForStats returns activity metrics for the stats display
// This function can be called from LoadChatStats to get the latest metrics
func (am *ActivityMonitor) GetMetricsForStats() (dag, wag, mag int64) {
	metrics := am.GetMetrics()
	if metrics == nil {
		// Recalculate if metrics are stale
		am.calculateMetrics()
		metrics = am.GetMetrics()
	}
	
	if metrics != nil {
		return metrics.DailyActiveGroups, metrics.WeeklyActiveGroups, metrics.MonthlyActiveGroups
	}
	
	// Fallback: calculate directly if monitor is not available
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)
	
	db.DB.Model(&db.Chat{}).Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).Count(&dag)
	db.DB.Model(&db.Chat{}).Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).Count(&wag)
	db.DB.Model(&db.Chat{}).Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).Count(&mag)
	
	return dag, wag, mag
}

// GetUserMetricsForStats returns user activity metrics for the stats display
// Returns DAU (Daily Active Users), WAU (Weekly Active Users), and MAU (Monthly Active Users)
func (am *ActivityMonitor) GetUserMetricsForStats() (dau, wau, mau int64) {
	metrics := am.GetMetrics()
	if metrics == nil {
		// Recalculate if metrics are stale
		am.calculateMetrics()
		metrics = am.GetMetrics()
	}
	
	if metrics != nil {
		return metrics.DailyActiveUsers, metrics.WeeklyActiveUsers, metrics.MonthlyActiveUsers
	}
	
	// Fallback: calculate directly if monitor is not available
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)
	
	db.DB.Model(&db.User{}).Where("last_activity >= ?", dayAgo).Count(&dau)
	db.DB.Model(&db.User{}).Where("last_activity >= ?", weekAgo).Count(&wau)
	db.DB.Model(&db.User{}).Where("last_activity >= ?", monthAgo).Count(&mau)
	
	return dau, wau, mau
}

// Global activity monitor instance
var globalActivityMonitor *ActivityMonitor

// GetActivityMonitor returns the global activity monitor instance
func GetActivityMonitor() *ActivityMonitor {
	if globalActivityMonitor == nil {
		globalActivityMonitor = NewActivityMonitor()
	}
	return globalActivityMonitor
}
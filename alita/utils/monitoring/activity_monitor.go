package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	log "github.com/sirupsen/logrus"
)

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

func NewActivityMonitor() *ActivityMonitor {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel stored in struct field, called in Stop()

	checkInterval := 1 * time.Hour
	inactivityThreshold := 30 * 24 * time.Hour

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

func (am *ActivityMonitor) Start() {
	log.Info("[ActivityMonitor] Starting activity monitoring service")
	log.Infof("[ActivityMonitor] Check interval: %v, Inactivity threshold: %v, Auto-cleanup: %v",
		am.checkInterval, am.inactivityThreshold, am.enableAutoCleanup)

	am.wg.Add(1)
	go am.monitorLoop()

	am.calculateMetrics()
}

func (am *ActivityMonitor) Stop() {
	am.stopOnce.Do(func() {
		log.Info("[ActivityMonitor] Stopping activity monitoring service")
		am.cancel()
		am.wg.Wait()
	})
}

func (am *ActivityMonitor) monitorLoop() {
	defer am.wg.Done()

	ticker := time.NewTicker(am.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				defer error_handling.RecoverFromPanic("performActivityCheck", "ActivityMonitor")
				am.performActivityCheck()
			}()
		case <-am.ctx.Done():
			return
		}
	}
}

func (am *ActivityMonitor) performActivityCheck() {
	startTime := time.Now()
	log.Info("[ActivityMonitor] Starting activity check")

	am.calculateMetrics()

	if !am.enableAutoCleanup {
		log.Info("[ActivityMonitor] Auto-cleanup disabled, skipping inactive chat marking")
		return
	}

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

func countAsync(wg *sync.WaitGroup, query func() error, errMsg string) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer error_handling.RecoverFromPanic("calculateMetrics", "ActivityMonitor")
		if err := query(); err != nil {
			log.Errorf("[ActivityMonitor] %s: %v", errMsg, err)
		}
	}()
}

func (am *ActivityMonitor) calculateMetrics() {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	var (
		dailyGroups, weeklyGroups, monthlyGroups, totalGroups, inactiveGroups int64
		dailyUsers, weeklyUsers, monthlyUsers, totalUsers                     int64
	)

	var wg sync.WaitGroup

	// The daily/weekly/monthly predicates are index-friendly
	// on (is_inactive, last_activity).
	countAsync(&wg, func() error {
		return db.DB.Model(&db.Chat{}).
			Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
			Count(&dailyGroups).Error
	}, "Error counting daily active groups")
	countAsync(&wg, func() error {
		return db.DB.Model(&db.Chat{}).
			Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
			Count(&weeklyGroups).Error
	}, "Error counting weekly active groups")
	countAsync(&wg, func() error {
		return db.DB.Model(&db.Chat{}).
			Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
			Count(&monthlyGroups).Error
	}, "Error counting monthly active groups")
	countAsync(&wg, func() error {
		return db.DB.Model(&db.Chat{}).Where("is_inactive = ?", true).Count(&inactiveGroups).Error
	}, "Error counting inactive groups")
	// Windowless total: cheap reltuples estimate (pg_class) instead of an
	// exact COUNT(*) scan.
	totalGroups = db.TableRowCount("chats")

	countAsync(&wg, func() error {
		return db.DB.Model(&db.User{}).
			Where("last_activity >= ?", dayAgo).
			Count(&dailyUsers).Error
	}, "Error counting daily active users")
	countAsync(&wg, func() error {
		return db.DB.Model(&db.User{}).
			Where("last_activity >= ?", weekAgo).
			Count(&weeklyUsers).Error
	}, "Error counting weekly active users")
	countAsync(&wg, func() error {
		return db.DB.Model(&db.User{}).
			Where("last_activity >= ?", monthAgo).
			Count(&monthlyUsers).Error
	}, "Error counting monthly active users")
	// Windowless total: cheap reltuples estimate (pg_class).
	totalUsers = db.TableRowCount("users")

	wg.Wait()

	metrics := &ActivityMetrics{
		DailyActiveGroups:   dailyGroups,
		WeeklyActiveGroups:  weeklyGroups,
		MonthlyActiveGroups: monthlyGroups,
		TotalGroups:         totalGroups,
		InactiveGroups:      inactiveGroups,
		DailyActiveUsers:    dailyUsers,
		WeeklyActiveUsers:   weeklyUsers,
		MonthlyActiveUsers:  monthlyUsers,
		TotalUsers:          totalUsers,
		CalculatedAt:        now,
	}

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

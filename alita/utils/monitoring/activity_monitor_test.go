package monitoring

import (
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
)

var (
	monitoringDBOnce sync.Once
	monitoringDBErr  error
)

func setupMonitoringDB(t *testing.T) {
	t.Helper()

	monitoringDBOnce.Do(func() {
		db.DB, monitoringDBErr = gorm.Open(
			sqlite.Open("file:monitoring_test?mode=memory&cache=shared"),
			&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
		)
		if monitoringDBErr != nil {
			return
		}
		monitoringDBErr = db.DB.AutoMigrate(&db.Chat{}, &db.User{})
	})
	if monitoringDBErr != nil {
		t.Fatalf("setup monitoring DB: %v", monitoringDBErr)
	}
}

func TestNewActivityMonitor_WithConfigOverrides(t *testing.T) {
	// Cannot run in parallel because it mutates global config.
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
		defer func() { config.AppConfig = nil }()
	}
	origInterval := config.AppConfig.ActivityCheckInterval
	origThreshold := config.AppConfig.InactivityThresholdDays
	origCleanup := config.AppConfig.EnableAutoCleanup
	defer func() {
		config.AppConfig.ActivityCheckInterval = origInterval
		config.AppConfig.InactivityThresholdDays = origThreshold
		config.AppConfig.EnableAutoCleanup = origCleanup
	}()

	config.AppConfig.ActivityCheckInterval = 2
	config.AppConfig.InactivityThresholdDays = 7
	config.AppConfig.EnableAutoCleanup = true

	am := NewActivityMonitor()

	if am.checkInterval != 2*time.Hour {
		t.Errorf("checkInterval: got %v, want %v", am.checkInterval, 2*time.Hour)
	}
	if am.inactivityThreshold != 7*24*time.Hour {
		t.Errorf("inactivityThreshold: got %v, want %v", am.inactivityThreshold, 7*24*time.Hour)
	}
	if !am.enableAutoCleanup {
		t.Errorf("enableAutoCleanup: got false, want true")
	}
}

func TestNewActivityMonitor_ConfigZeroValues(t *testing.T) {
	// Cannot run in parallel because it mutates global config.
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
		defer func() { config.AppConfig = nil }()
	}
	origInterval := config.AppConfig.ActivityCheckInterval
	origThreshold := config.AppConfig.InactivityThresholdDays
	origCleanup := config.AppConfig.EnableAutoCleanup
	defer func() {
		config.AppConfig.ActivityCheckInterval = origInterval
		config.AppConfig.InactivityThresholdDays = origThreshold
		config.AppConfig.EnableAutoCleanup = origCleanup
	}()

	config.AppConfig.ActivityCheckInterval = 0
	config.AppConfig.InactivityThresholdDays = 0
	config.AppConfig.EnableAutoCleanup = false

	am := NewActivityMonitor()

	if am.checkInterval != 1*time.Hour {
		t.Errorf("checkInterval: got %v, want %v (default)", am.checkInterval, 1*time.Hour)
	}
	if am.inactivityThreshold != 30*24*time.Hour {
		t.Errorf("inactivityThreshold: got %v, want %v (default)", am.inactivityThreshold, 30*24*time.Hour)
	}
	if am.enableAutoCleanup {
		t.Errorf("enableAutoCleanup: got true, want false")
	}
}

func TestCalculateMetricsCountsActivityBuckets(t *testing.T) {
	setupMonitoringDB(t)
	requireCleanMonitoringTables(t)
	now := time.Now()

	chats := []db.Chat{
		{ChatId: 1001, ChatName: "today", LastActivity: now.Add(-2 * time.Hour), IsInactive: false},
		{ChatId: 1002, ChatName: "week", LastActivity: now.Add(-3 * 24 * time.Hour), IsInactive: false},
		{ChatId: 1003, ChatName: "old inactive", LastActivity: now.Add(-45 * 24 * time.Hour), IsInactive: true},
	}
	if err := db.DB.Create(&chats).Error; err != nil {
		t.Fatalf("Create(chats) error = %v", err)
	}

	users := []db.User{
		{UserId: 2001, Name: "today", LastActivity: now.Add(-2 * time.Hour)},
		{UserId: 2002, Name: "week", LastActivity: now.Add(-3 * 24 * time.Hour)},
		{UserId: 2003, Name: "month", LastActivity: now.Add(-20 * 24 * time.Hour)},
		{UserId: 2004, Name: "old", LastActivity: now.Add(-45 * 24 * time.Hour)},
	}
	if err := db.DB.Create(&users).Error; err != nil {
		t.Fatalf("Create(users) error = %v", err)
	}

	am := NewActivityMonitor()
	am.calculateMetrics()

	am.metricsLock.RLock()
	metrics := *am.lastMetrics
	calculatedAt := am.lastMetricsCalculated
	am.metricsLock.RUnlock()

	if calculatedAt.IsZero() {
		t.Fatal("lastMetricsCalculated was not set")
	}
	if metrics.DailyActiveGroups != 1 || metrics.WeeklyActiveGroups != 2 || metrics.MonthlyActiveGroups != 2 {
		t.Fatalf("group active buckets = daily %d weekly %d monthly %d, want 1/2/2",
			metrics.DailyActiveGroups, metrics.WeeklyActiveGroups, metrics.MonthlyActiveGroups)
	}
	if metrics.TotalGroups != 3 || metrics.InactiveGroups != 1 {
		t.Fatalf("group totals = total %d inactive %d, want 3/1", metrics.TotalGroups, metrics.InactiveGroups)
	}
	if metrics.DailyActiveUsers != 1 || metrics.WeeklyActiveUsers != 2 || metrics.MonthlyActiveUsers != 3 {
		t.Fatalf("user active buckets = daily %d weekly %d monthly %d, want 1/2/3",
			metrics.DailyActiveUsers, metrics.WeeklyActiveUsers, metrics.MonthlyActiveUsers)
	}
	if metrics.TotalUsers != 4 {
		t.Fatalf("TotalUsers = %d, want 4", metrics.TotalUsers)
	}
}

func TestPerformActivityCheckMarksInactiveAndReactivatesRecentChats(t *testing.T) {
	setupMonitoringDB(t)
	requireCleanMonitoringTables(t)
	now := time.Now()

	chats := []db.Chat{
		{ChatId: 3001, ChatName: "stale active", LastActivity: now.Add(-45 * 24 * time.Hour), IsInactive: false},
		{ChatId: 3002, ChatName: "recent inactive", LastActivity: now.Add(-2 * time.Hour), IsInactive: true},
	}
	if err := db.DB.Create(&chats).Error; err != nil {
		t.Fatalf("Create(chats) error = %v", err)
	}

	am := NewActivityMonitor()
	am.enableAutoCleanup = true
	am.inactivityThreshold = 30 * 24 * time.Hour
	am.performActivityCheck()

	var stale db.Chat
	if err := db.DB.Where("chat_id = ?", int64(3001)).First(&stale).Error; err != nil {
		t.Fatalf("load stale chat error = %v", err)
	}
	if !stale.IsInactive {
		t.Fatal("stale chat IsInactive = false, want true")
	}

	var recent db.Chat
	if err := db.DB.Where("chat_id = ?", int64(3002)).First(&recent).Error; err != nil {
		t.Fatalf("load recent chat error = %v", err)
	}
	if recent.IsInactive {
		t.Fatal("recent chat IsInactive = true, want false")
	}
}

func requireCleanMonitoringTables(t *testing.T) {
	t.Helper()

	if err := db.DB.Exec("DELETE FROM users").Error; err != nil {
		t.Fatalf("clean users: %v", err)
	}
	if err := db.DB.Exec("DELETE FROM chats").Error; err != nil {
		t.Fatalf("clean chats: %v", err)
	}
}

func TestActivityMonitorStartCalculatesMetricsAndStopReturns(t *testing.T) {
	setupMonitoringDB(t)

	am := NewActivityMonitor()
	am.Start()

	// Start computes the first metrics synchronously, before any tick.
	am.metricsLock.RLock()
	calculated := am.lastMetrics != nil && !am.lastMetricsCalculated.IsZero()
	am.metricsLock.RUnlock()
	if !calculated {
		am.Stop()
		t.Fatal("Start() returned without calculating metrics")
	}

	stopped := make(chan struct{})
	go func() {
		am.Stop()
		am.Stop() // a second Stop must not block or panic
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return: monitor loop did not exit")
	}
}

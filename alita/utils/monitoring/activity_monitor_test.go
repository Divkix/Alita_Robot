package monitoring

import (
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// ---------------------------------------------------------------------------
// NewActivityMonitor
// ---------------------------------------------------------------------------

func TestNewActivityMonitor(t *testing.T) {
	// Cannot run in parallel because NewActivityMonitor reads global config.

	am := NewActivityMonitor()
	if am == nil {
		t.Fatal("NewActivityMonitor() returned nil")
	}

	t.Run("default check interval is 1h", func(t *testing.T) {
		am2 := NewActivityMonitor()
		if am2.checkInterval != 1*time.Hour {
			t.Fatalf("expected checkInterval=1h, got %v", am2.checkInterval)
		}
	})

	t.Run("default inactivity threshold is 30 days", func(t *testing.T) {
		am2 := NewActivityMonitor()
		expected := 30 * 24 * time.Hour
		if am2.inactivityThreshold != expected {
			t.Fatalf("expected inactivityThreshold=%v, got %v", expected, am2.inactivityThreshold)
		}
	})

	t.Run("context and cancel are set", func(t *testing.T) {
		am2 := NewActivityMonitor()
		if am2.ctx == nil {
			t.Fatal("expected ctx to be non-nil")
		}
		if am2.cancel == nil {
			t.Fatal("expected cancel to be non-nil")
		}
	})

	t.Run("stopOnce is zero value", func(t *testing.T) {
		am2 := NewActivityMonitor()
		// sync.Once has no exported state, but we can verify Stop() works twice
		am2.Stop()
		am2.Stop() // should not panic
	})

	t.Run("metrics fields are zero value", func(t *testing.T) {
		am2 := NewActivityMonitor()
		if am2.lastMetrics != nil {
			t.Fatalf("expected lastMetrics=nil initially, got %v", am2.lastMetrics)
		}
		if !am2.lastMetricsCalculated.IsZero() {
			t.Fatalf("expected lastMetricsCalculated zero, got %v", am2.lastMetricsCalculated)
		}
	})
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

// ---------------------------------------------------------------------------
// Start
// ---------------------------------------------------------------------------

func TestStart(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires PostgreSQL connection")
	}

	am := NewActivityMonitor()
	am.Start()
	defer am.Stop()

	// Verify the monitor is running: lastMetrics should eventually be populated.
	// calculateMetrics runs synchronously inside Start, so lastMetricsCalculated
	// should be set immediately (or very shortly, since it spawns goroutines).
	done := make(chan struct{})
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
			}
			am.metricsLock.RLock()
			calculated := !am.lastMetricsCalculated.IsZero()
			am.metricsLock.RUnlock()
			if calculated {
				close(done)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		close(quit)
		t.Fatal("timeout waiting for metrics calculation after Start()")
	}
}

func TestStart_DoesNotPanic(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires PostgreSQL connection")
	}

	am := NewActivityMonitor()
	defer am.Stop()

	// The sole purpose of this test is to ensure Start() itself does not panic.
	am.Start()
}

// ---------------------------------------------------------------------------
// Stop
// ---------------------------------------------------------------------------

func TestStop_WithoutStart(t *testing.T) {
	am := NewActivityMonitor()
	// Should not panic even if Start() was never called.
	am.Stop()
}

func TestStop_Idempotent(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires PostgreSQL connection")
	}

	am := NewActivityMonitor()
	am.Start()
	am.Stop()
	am.Stop() // second call must not panic
}

func TestStop_GracefullyStopsRunningMonitor(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires PostgreSQL connection")
	}

	am := NewActivityMonitor()
	am.Start()

	// Stop should complete without hanging.
	done := make(chan struct{})
	go func() {
		am.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() hung or timed out")
	}
}

// ---------------------------------------------------------------------------
// monitorLoop lifecycle (no DB required)
// ---------------------------------------------------------------------------

func TestMonitorLoop_ExitsOnCancel(t *testing.T) {
	am := NewActivityMonitor()
	defer am.cancel() // ensure cancellation on timeout so goroutine unblocks

	am.wg.Add(1)
	go am.monitorLoop()

	done := make(chan struct{})
	go func() {
		am.wg.Wait()
		close(done)
	}()

	// cancel the context to trigger monitorLoop exit
	am.cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("monitorLoop did not exit after context cancellation")
	}
}

func TestMonitorLoop_ExitsViaStop(t *testing.T) {
	am := NewActivityMonitor()
	am.wg.Add(1)
	go am.monitorLoop()

	done := make(chan struct{})
	go func() {
		am.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete after starting monitorLoop")
	}
}

// ---------------------------------------------------------------------------
// ActivityMetrics struct fields
// ---------------------------------------------------------------------------

func TestActivityMetrics_Fields(t *testing.T) {
	t.Parallel()

	m := ActivityMetrics{
		DailyActiveGroups:   1,
		WeeklyActiveGroups:  2,
		MonthlyActiveGroups: 3,
		TotalGroups:         4,
		InactiveGroups:      5,
		DailyActiveUsers:    6,
		WeeklyActiveUsers:   7,
		MonthlyActiveUsers:  8,
		TotalUsers:          9,
	}

	if m.DailyActiveGroups != 1 {
		t.Errorf("DailyActiveGroups: got %d, want 1", m.DailyActiveGroups)
	}
	if m.WeeklyActiveGroups != 2 {
		t.Errorf("WeeklyActiveGroups: got %d, want 2", m.WeeklyActiveGroups)
	}
	if m.MonthlyActiveGroups != 3 {
		t.Errorf("MonthlyActiveGroups: got %d, want 3", m.MonthlyActiveGroups)
	}
	if m.TotalGroups != 4 {
		t.Errorf("TotalGroups: got %d, want 4", m.TotalGroups)
	}
	if m.InactiveGroups != 5 {
		t.Errorf("InactiveGroups: got %d, want 5", m.InactiveGroups)
	}
	if m.DailyActiveUsers != 6 {
		t.Errorf("DailyActiveUsers: got %d, want 6", m.DailyActiveUsers)
	}
	if m.WeeklyActiveUsers != 7 {
		t.Errorf("WeeklyActiveUsers: got %d, want 7", m.WeeklyActiveUsers)
	}
	if m.MonthlyActiveUsers != 8 {
		t.Errorf("MonthlyActiveUsers: got %d, want 8", m.MonthlyActiveUsers)
	}
	if m.TotalUsers != 9 {
		t.Errorf("TotalUsers: got %d, want 9", m.TotalUsers)
	}
}

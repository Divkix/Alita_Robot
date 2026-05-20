package modules

import (
	"testing"
	"time"
)

func resetAntiSpamMapForTest(t *testing.T) {
	t.Helper()

	antiSpamMutex.Lock()
	previous := antiSpamMap
	antiSpamMap = make(map[spamKey]*antiSpamInfo)
	antiSpamMutex.Unlock()

	t.Cleanup(func() {
		antiSpamMutex.Lock()
		antiSpamMap = previous
		antiSpamMutex.Unlock()
	})
}

func TestCheckSpammedInitializesAndTripsThreshold(t *testing.T) {
	resetAntiSpamMapForTest(t)

	key := spamKey{chatId: -1001, userId: 42}
	levels := []antiSpamLevel{{Limit: 2, Expiry: time.Minute}}

	if checkSpammed(key, levels) {
		t.Fatal("first message marked as spam")
	}
	if !checkSpammed(key, levels) {
		t.Fatal("second message did not trip spam threshold")
	}

	antiSpamMutex.Lock()
	info := antiSpamMap[key]
	antiSpamMutex.Unlock()
	if info == nil || len(info.Levels) != 1 {
		t.Fatalf("antiSpamMap[%+v] = %+v, want one level", key, info)
	}
	if !info.Levels[0].Spammed {
		t.Fatal("spam level was not marked spammed")
	}
}

func TestCheckSpammedResetsExpiredWindow(t *testing.T) {
	resetAntiSpamMapForTest(t)

	key := spamKey{chatId: -1002, userId: 43}
	antiSpamMap[key] = &antiSpamInfo{Levels: []antiSpamLevel{{
		Count:    10,
		Limit:    3,
		CurrTime: time.Now().Add(-2 * time.Minute),
		Expiry:   time.Second,
		Spammed:  true,
	}}}

	if checkSpammed(key, []antiSpamLevel{{Limit: 3, Expiry: time.Second}}) {
		t.Fatal("expired spam window should not stay spammed after reset")
	}

	got := antiSpamMap[key].Levels[0]
	if got.Count != 1 {
		t.Fatalf("reset Count = %d, want 1 after current message", got.Count)
	}
	if got.Spammed {
		t.Fatal("reset level remained spammed")
	}
}

func TestCleanupExpiredAntiSpamDeletesNilAndExpiredEntries(t *testing.T) {
	resetAntiSpamMapForTest(t)

	now := time.Now()
	nilKey := spamKey{chatId: -1003, userId: 44}
	expiredKey := spamKey{chatId: -1003, userId: 45}
	activeKey := spamKey{chatId: -1003, userId: 46}

	antiSpamMap[nilKey] = nil
	antiSpamMap[expiredKey] = &antiSpamInfo{Levels: []antiSpamLevel{{
		CurrTime: now.Add(-10 * time.Minute),
		Expiry:   time.Minute,
	}}}
	antiSpamMap[activeKey] = &antiSpamInfo{Levels: []antiSpamLevel{{
		CurrTime: now.Add(-30 * time.Second),
		Expiry:   time.Minute,
	}}}

	cleanupExpiredAntiSpam(now)

	if _, ok := antiSpamMap[nilKey]; ok {
		t.Fatal("nil anti-spam entry was not deleted")
	}
	if _, ok := antiSpamMap[expiredKey]; ok {
		t.Fatal("expired anti-spam entry was not deleted")
	}
	if _, ok := antiSpamMap[activeKey]; !ok {
		t.Fatal("active anti-spam entry was deleted")
	}
}

func TestSpamCheckUsesDefaultThreshold(t *testing.T) {
	resetAntiSpamMapForTest(t)

	key := spamKey{chatId: -1004, userId: 47}
	for i := 0; i < 17; i++ {
		if spamCheck(key) {
			t.Fatalf("spamCheck() marked message %d as spam before default threshold", i+1)
		}
	}
	if !spamCheck(key) {
		t.Fatal("spamCheck() did not mark eighteenth message as spam")
	}
}

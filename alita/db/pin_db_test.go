package db

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGetPinData_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&PinSettings{}).Error
	})

	data := GetPinData(chatID)
	if data == nil {
		t.Fatal("expected non-nil PinSettings")
	}
	if data.MsgId != 0 {
		t.Fatalf("expected default MsgId=0, got %d", data.MsgId)
	}
	if data.CleanLinked {
		t.Fatal("expected default CleanLinked=false")
	}
	if data.AntiChannelPin {
		t.Fatal("expected default AntiChannelPin=false")
	}
}

func TestSetCleanLinked_BooleanRoundTrip(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&PinSettings{}).Error
	})

	// Create default settings first
	_ = GetPinData(chatID)

	// Enable CleanLinked
	if err := SetCleanLinked(chatID, true); err != nil {
		t.Fatalf("SetCleanLinked(true) failed: %v", err)
	}
	data := GetPinData(chatID)
	if !data.CleanLinked {
		t.Fatal("expected CleanLinked=true after SetCleanLinked(true)")
	}

	// Disable CleanLinked — zero value boolean must persist
	if err := SetCleanLinked(chatID, false); err != nil {
		t.Fatalf("SetCleanLinked(false) failed: %v", err)
	}
	data = GetPinData(chatID)
	if data.CleanLinked {
		t.Fatal("expected CleanLinked=false after SetCleanLinked(false)")
	}
}

func TestSetAntiChannelPin_BooleanRoundTrip(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&PinSettings{}).Error
	})

	// Create default settings first
	_ = GetPinData(chatID)

	// Enable AntiChannelPin
	if err := SetAntiChannelPin(chatID, true); err != nil {
		t.Fatalf("SetAntiChannelPin(true) failed: %v", err)
	}
	data := GetPinData(chatID)
	if !data.AntiChannelPin {
		t.Fatal("expected AntiChannelPin=true after SetAntiChannelPin(true)")
	}

	// Disable AntiChannelPin — zero value boolean must persist
	if err := SetAntiChannelPin(chatID, false); err != nil {
		t.Fatalf("SetAntiChannelPin(false) failed: %v", err)
	}
	data = GetPinData(chatID)
	if data.AntiChannelPin {
		t.Fatal("expected AntiChannelPin=false after SetAntiChannelPin(false)")
	}
}

func TestGetPinData_IdempotentCreate(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&PinSettings{}).Error
	})

	// Call multiple times — should not produce duplicate records
	for i := range 3 {
		data := GetPinData(chatID)
		if data == nil {
			t.Fatalf("call %d: expected non-nil PinSettings", i+1)
		}
	}

	var count int64
	DB.Model(&PinSettings{}).Where("chat_id = ?", chatID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 pin record, got %d", count)
	}
}

func TestConcurrentPinSettings(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&PinSettings{}).Error
	})

	// Create default settings first
	_ = GetPinData(chatID)

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)

	for i := range workers {
		go func(i int) {
			defer wg.Done()
			pref := i%2 == 0
			if err := SetAntiChannelPin(chatID, pref); err != nil {
				errs <- fmt.Errorf("SetAntiChannelPin(%v): %w", pref, err)
				return
			}
			if err := SetCleanLinked(chatID, pref); err != nil {
				errs <- fmt.Errorf("SetCleanLinked(%v): %w", pref, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent pin update error: %v", err)
	}

	// Verify record still exists and is accessible
	data := GetPinData(chatID)
	if data == nil {
		t.Fatal("expected non-nil PinSettings after concurrent updates")
	}

	var count int64
	DB.Model(&PinSettings{}).Where("chat_id = ?", chatID).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 pin record after concurrent writes, got %d", count)
	}
}

func TestLoadPinStats_Returns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// Just verify the function executes without error and returns non-negative values
	acCount, clCount := LoadPinStats()
	if acCount < 0 {
		t.Fatalf("expected non-negative acCount, got %d", acCount)
	}
	if clCount < 0 {
		t.Fatalf("expected non-negative clCount, got %d", clCount)
	}
}

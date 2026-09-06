package pins

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestPinSettingsCRUD(t *testing.T) {
	skipIfNoDb(t)

	t.Run("defaults", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.PinSettings{}).Error
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
	})

	t.Run("cleanLinked roundtrip", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.PinSettings{}).Error
		})

		_ = GetPinData(chatID)

		if err := SetCleanLinked(chatID, true); err != nil {
			t.Fatalf("SetCleanLinked(true) failed: %v", err)
		}
		data := GetPinData(chatID)
		if !data.CleanLinked {
			t.Fatal("expected CleanLinked=true after SetCleanLinked(true)")
		}

		if err := SetCleanLinked(chatID, false); err != nil {
			t.Fatalf("SetCleanLinked(false) failed: %v", err)
		}
		data = GetPinData(chatID)
		if data.CleanLinked {
			t.Fatal("expected CleanLinked=false after SetCleanLinked(false)")
		}
	})

	t.Run("antiChannelPin roundtrip", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.PinSettings{}).Error
		})

		_ = GetPinData(chatID)

		if err := SetAntiChannelPin(chatID, true); err != nil {
			t.Fatalf("SetAntiChannelPin(true) failed: %v", err)
		}
		data := GetPinData(chatID)
		if !data.AntiChannelPin {
			t.Fatal("expected AntiChannelPin=true after SetAntiChannelPin(true)")
		}

		if err := SetAntiChannelPin(chatID, false); err != nil {
			t.Fatalf("SetAntiChannelPin(false) failed: %v", err)
		}
		data = GetPinData(chatID)
		if data.AntiChannelPin {
			t.Fatal("expected AntiChannelPin=false after SetAntiChannelPin(false)")
		}
	})

	t.Run("idempotent create", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.PinSettings{}).Error
		})

		for i := range 3 {
			data := GetPinData(chatID)
			if data == nil {
				t.Fatalf("call %d: expected non-nil PinSettings", i+1)
			}
		}

		var count int64
		db.DB.Model(&models.PinSettings{}).Where("chat_id = ?", chatID).Count(&count)
		if count != 1 {
			t.Fatalf("expected exactly 1 pin record, got %d", count)
		}
	})
}

func TestLoadPinStats_Returns(t *testing.T) {
	skipIfNoDb(t)

	acCount, clCount := LoadPinStats()
	if acCount < 0 {
		t.Fatalf("expected non-negative acCount, got %d", acCount)
	}
	if clCount < 0 {
		t.Fatalf("expected non-negative clCount, got %d", clCount)
	}
}

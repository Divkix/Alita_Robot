package db

import (
	"sync"
	"testing"
	"time"
)

func TestGetGreetingSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	settings := GetGreetingSettings(chatID)
	if settings == nil {
		t.Fatalf("GetGreetingSettings() returned nil")
	}
	if settings.WelcomeSettings == nil {
		t.Fatalf("GetGreetingSettings() WelcomeSettings is nil")
	}
	if settings.GoodbyeSettings == nil {
		t.Fatalf("GetGreetingSettings() GoodbyeSettings is nil")
	}
	if !settings.WelcomeSettings.ShouldWelcome {
		t.Fatalf("expected default ShouldWelcome=true, got false")
	}
	if settings.WelcomeSettings.WelcomeText != DefaultWelcome {
		t.Fatalf("expected default WelcomeText=%q, got %q", DefaultWelcome, settings.WelcomeSettings.WelcomeText)
	}
	if !settings.GoodbyeSettings.ShouldGoodbye {
		t.Fatalf("expected default ShouldGoodbye=true (DB column default), got false")
	}
}

func TestSetWelcomeToggle_ZeroValueBoolean(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	SetWelcomeToggle(chatID, true)
	settings := GetGreetingSettings(chatID)
	if settings.WelcomeSettings == nil || !settings.WelcomeSettings.ShouldWelcome {
		t.Fatalf("expected ShouldWelcome=true after SetWelcomeToggle(true)")
	}

	SetWelcomeToggle(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.WelcomeSettings == nil || settings.WelcomeSettings.ShouldWelcome {
		t.Fatalf("expected ShouldWelcome=false after SetWelcomeToggle(false)")
	}
}

func TestSetWelcomeText(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	buttons := []Button{{Name: "btn1", Url: "https://example.com", SameLine: false}}
	SetWelcomeText(chatID, "Hello {first}!", "file123", buttons, PHOTO)

	settings := GetGreetingSettings(chatID)
	if settings.WelcomeSettings == nil {
		t.Fatalf("WelcomeSettings is nil")
	}
	if settings.WelcomeSettings.WelcomeText != "Hello {first}!" {
		t.Fatalf("expected WelcomeText=%q, got %q", "Hello {first}!", settings.WelcomeSettings.WelcomeText)
	}
	if settings.WelcomeSettings.FileID != "file123" {
		t.Fatalf("expected FileID=%q, got %q", "file123", settings.WelcomeSettings.FileID)
	}
	if settings.WelcomeSettings.WelcomeType != PHOTO {
		t.Fatalf("expected WelcomeType=%d, got %d", PHOTO, settings.WelcomeSettings.WelcomeType)
	}
	if len(settings.WelcomeSettings.Button) != 1 {
		t.Fatalf("expected 1 button, got %d", len(settings.WelcomeSettings.Button))
	}
	if settings.WelcomeSettings.Button[0].Name != "btn1" {
		t.Fatalf("expected button name=%q, got %q", "btn1", settings.WelcomeSettings.Button[0].Name)
	}
}

func TestSetGoodbyeText(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	buttons := []Button{{Name: "bye", Url: "https://example.com/bye", SameLine: true}}
	SetGoodbyeText(chatID, "Goodbye {first}!", "gbfile456", buttons, STICKER)

	settings := GetGreetingSettings(chatID)
	if settings.GoodbyeSettings == nil {
		t.Fatalf("GoodbyeSettings is nil")
	}
	if settings.GoodbyeSettings.GoodbyeText != "Goodbye {first}!" {
		t.Fatalf("expected GoodbyeText=%q, got %q", "Goodbye {first}!", settings.GoodbyeSettings.GoodbyeText)
	}
	if settings.GoodbyeSettings.FileID != "gbfile456" {
		t.Fatalf("expected FileID=%q, got %q", "gbfile456", settings.GoodbyeSettings.FileID)
	}
	if settings.GoodbyeSettings.GoodbyeType != STICKER {
		t.Fatalf("expected GoodbyeType=%d, got %d", STICKER, settings.GoodbyeSettings.GoodbyeType)
	}
	if len(settings.GoodbyeSettings.Button) != 1 {
		t.Fatalf("expected 1 button, got %d", len(settings.GoodbyeSettings.Button))
	}
}

func TestSetGoodbyeToggle_ZeroValueBoolean(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	SetGoodbyeToggle(chatID, true)
	settings := GetGreetingSettings(chatID)
	if settings.GoodbyeSettings == nil || !settings.GoodbyeSettings.ShouldGoodbye {
		t.Fatalf("expected ShouldGoodbye=true after SetGoodbyeToggle(true)")
	}

	SetGoodbyeToggle(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.GoodbyeSettings == nil || settings.GoodbyeSettings.ShouldGoodbye {
		t.Fatalf("expected ShouldGoodbye=false after SetGoodbyeToggle(false)")
	}
}

func TestSetShouldCleanService(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	SetShouldCleanService(chatID, true)
	settings := GetGreetingSettings(chatID)
	if !settings.ShouldCleanService {
		t.Fatalf("expected ShouldCleanService=true, got false")
	}

	SetShouldCleanService(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.ShouldCleanService {
		t.Fatalf("expected ShouldCleanService=false after reset")
	}
}

func TestSetShouldAutoApprove(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	SetShouldAutoApprove(chatID, true)
	settings := GetGreetingSettings(chatID)
	if !settings.ShouldAutoApprove {
		t.Fatalf("expected ShouldAutoApprove=true, got false")
	}

	SetShouldAutoApprove(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.ShouldAutoApprove {
		t.Fatalf("expected ShouldAutoApprove=false after reset")
	}
}

func TestSetCleanWelcomeSetting(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	SetCleanWelcomeSetting(chatID, true)
	settings := GetGreetingSettings(chatID)
	if settings.WelcomeSettings == nil || !settings.WelcomeSettings.CleanWelcome {
		t.Fatalf("expected CleanWelcome=true, got false")
	}

	SetCleanWelcomeSetting(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.WelcomeSettings == nil || settings.WelcomeSettings.CleanWelcome {
		t.Fatalf("expected CleanWelcome=false after reset")
	}
}

func TestSetCleanMsgId(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	cases := []struct {
		name       string
		msgID      int64
		setFunc    func(int64, int64)
		getLastMsg func(*GreetingSettings) int64
		nilCheck   func(*GreetingSettings) bool
	}{
		{
			name:       "WelcomeMsgId",
			msgID:      99999,
			setFunc:    SetCleanWelcomeMsgId,
			getLastMsg: func(s *GreetingSettings) int64 { return s.WelcomeSettings.LastMsgId },
			nilCheck:   func(s *GreetingSettings) bool { return s.WelcomeSettings == nil },
		},
		{
			name:       "GoodbyeMsgId",
			msgID:      77777,
			setFunc:    SetCleanGoodbyeMsgId,
			getLastMsg: func(s *GreetingSettings) int64 { return s.GoodbyeSettings.LastMsgId },
			nilCheck:   func(s *GreetingSettings) bool { return s.GoodbyeSettings == nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			chatID := time.Now().UnixNano()
			if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
				t.Fatalf("EnsureChatInDb() error = %v", err)
			}
			t.Cleanup(func() {
				DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
				DB.Where("chat_id = ?", chatID).Delete(&Chat{})
			})

			_ = GetGreetingSettings(chatID)

			tc.setFunc(chatID, tc.msgID)
			settings := GetGreetingSettings(chatID)
			if tc.nilCheck(settings) {
				t.Fatalf("settings sub-struct is nil")
			}
			if tc.getLastMsg(settings) != tc.msgID {
				t.Fatalf("expected LastMsgId=%d, got %d", tc.msgID, tc.getLastMsg(settings))
			}

			tc.setFunc(chatID, 0)
			settings = GetGreetingSettings(chatID)
			if tc.getLastMsg(settings) != 0 {
				t.Fatalf("expected LastMsgId=0 after reset, got %d", tc.getLastMsg(settings))
			}
		})
	}
}

func TestSetCleanGoodbyeSetting(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	_ = GetGreetingSettings(chatID)

	SetCleanGoodbyeSetting(chatID, true)
	settings := GetGreetingSettings(chatID)
	if settings.GoodbyeSettings == nil || !settings.GoodbyeSettings.CleanGoodbye {
		t.Fatalf("expected CleanGoodbye=true, got false")
	}

	SetCleanGoodbyeSetting(chatID, false)
	settings = GetGreetingSettings(chatID)
	if settings.GoodbyeSettings == nil || settings.GoodbyeSettings.CleanGoodbye {
		t.Fatalf("expected CleanGoodbye=false after reset")
	}
}

func TestGetWelcomeButtons_Empty(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	buttons := GetWelcomeButtons(chatID)
	if buttons == nil {
		t.Fatalf("GetWelcomeButtons() returned nil, expected empty slice")
	}
	if len(buttons) != 0 {
		t.Fatalf("expected 0 buttons, got %d", len(buttons))
	}
}

func TestGetGoodbyeButtons_Empty(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	buttons := GetGoodbyeButtons(chatID)
	if buttons == nil {
		t.Fatalf("GetGoodbyeButtons() returned nil, expected empty slice")
	}
	if len(buttons) != 0 {
		t.Fatalf("expected 0 buttons, got %d", len(buttons))
	}
}

func TestLoadGreetingsStats_EmptyDB(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// Just verify the function returns without error and returns int64 values.
	// The DB may have other rows from other tests, so we just check the function runs.
	enabledWelcome, enabledGoodbye, cleanService, cleanWelcome, cleanGoodbye := LoadGreetingsStats()
	// All values should be >= 0
	if enabledWelcome < 0 {
		t.Fatalf("enabledWelcome is negative: %d", enabledWelcome)
	}
	if enabledGoodbye < 0 {
		t.Fatalf("enabledGoodbye is negative: %d", enabledGoodbye)
	}
	if cleanService < 0 {
		t.Fatalf("cleanService is negative: %d", cleanService)
	}
	if cleanWelcome < 0 {
		t.Fatalf("cleanWelcome is negative: %d", cleanWelcome)
	}
	if cleanGoodbye < 0 {
		t.Fatalf("cleanGoodbye is negative: %d", cleanGoodbye)
	}
}

func TestGreetingSettings_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
	})

	// Ensure initial record exists
	_ = GetGreetingSettings(chatID)

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				SetWelcomeToggle(chatID, true)
			} else {
				SetGoodbyeToggle(chatID, true)
			}
		}(i)
	}

	wg.Wait()

	// Verify the record is still consistent (no corruption/panic)
	settings := GetGreetingSettings(chatID)
	if settings == nil {
		t.Fatalf("GetGreetingSettings() returned nil after concurrent writes")
	}
	if settings.WelcomeSettings == nil {
		t.Fatalf("WelcomeSettings is nil after concurrent writes")
	}
	if settings.GoodbyeSettings == nil {
		t.Fatalf("GoodbyeSettings is nil after concurrent writes")
	}
}

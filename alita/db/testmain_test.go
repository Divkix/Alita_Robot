package db

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if DB == nil {
		fmt.Println("Skipping DB tests: PostgreSQL not available (DB == nil)")
		os.Exit(0)
	}

	err := DB.AutoMigrate(
		&User{},
		&Chat{},
		&WarnSettings{},
		&Warns{},
		&GreetingSettings{},
		&ChatFilters{},
		&AdminSettings{},
		&BlacklistSettings{},
		&PinSettings{},
		&ReportChatSettings{},
		&ReportUserSettings{},
		&DevSettings{},
		&ChannelSettings{},
		&AntifloodSettings{},
		&ConnectionSettings{},
		&ConnectionChatSettings{},
		&DisableSettings{},
		&DisableChatSettings{},
		&RulesSettings{},
		&LockSettings{},
		&NotesSettings{},
		&Notes{},
		&CaptchaSettings{},
		&CaptchaAttempts{},
		&StoredMessages{},
		&CaptchaMutedUsers{},
	)
	if err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("requires PostgreSQL connection")
	}
}

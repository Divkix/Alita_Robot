package repo_checks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func TestMigrationsCreateDisableChatSettingsTable(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected migration files to exist")
	}

	createTable := regexp.MustCompile(`(?is)create\s+table\s+(?:if\s+not\s+exists\s+)?(?:public\.)?disable_chat_settings\b`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", file, err)
		}
		if createTable.Match(data) {
			return
		}
	}

	t.Fatal("disable_chat_settings model has no CREATE TABLE statement in SQL migrations")
}

func TestClearConnectionsDataPersistsFalseAllowConnect(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "db", "backup_db.go")
	start := strings.Index(source, "func clearConnectionsData")
	if start == -1 {
		t.Fatal("clearConnectionsData is missing")
	}

	body := source[start:]
	end := strings.Index(body, "\n}\n")
	if end == -1 {
		t.Fatal("clearConnectionsData body is malformed")
	}
	body = body[:end]

	if !strings.Contains(body, "UpdateRecordWithZeroValues") {
		t.Fatal("clearConnectionsData must use UpdateRecordWithZeroValues to persist false")
	}
	if !strings.Contains(body, `"allow_connect"`) ||
		!strings.Contains(body, "false") {
		t.Fatal("clearConnectionsData must set allow_connect to false")
	}
}

func TestImportConnectionsDataUsesTargetChatAndZeroValueUpdate(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "db", "backup_db.go")
	start := strings.Index(source, "func importConnectionsData")
	if start == -1 {
		t.Fatal("importConnectionsData is missing")
	}

	body := source[start:]
	end := strings.Index(body, "\n}\n\nfunc ")
	if end == -1 {
		t.Fatal("importConnectionsData body is malformed")
	}
	body = body[:end]

	if !strings.Contains(body, "GetChatConnectionSetting(chatID)") {
		t.Fatal("importConnectionsData must ensure the target chat settings row exists")
	}
	if !strings.Contains(body, "UpdateRecordWithZeroValues") {
		t.Fatal("importConnectionsData must use UpdateRecordWithZeroValues to import false")
	}
	if !strings.Contains(body, `"allow_connect"`) {
		t.Fatal("importConnectionsData must update allow_connect on the target chat")
	}
	if strings.Contains(body, "settings.ChatId") {
		t.Fatal("importConnectionsData must not write the source backup chat_id into the target row")
	}
}

func TestDisablingBackupImportExportAndClearAreFunctional(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "db", "backup_db.go")
	required := []string{
		"ShouldDel(chatID)",
		"ToggleDel(chatID",
		"DisableCMD(chatID",
		"EnableCMD(chatID",
	}

	for _, want := range required {
		if !strings.Contains(source, want) {
			t.Fatalf("backup disabling support missing %s", want)
		}
	}
}

func TestPollingLoadsModulesBeforeStartingPolling(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "main.go")
	start := strings.Index(source, "// Use polling mode")
	if start == -1 {
		t.Fatal("polling branch marker is missing")
	}
	end := strings.Index(source[start:], "// Set Commands of Bot")
	if end == -1 {
		t.Fatal("polling startup region marker is missing")
	}

	pollingBranch := source[start : start+end]
	loadModules := strings.Index(pollingBranch, "alita.LoadModules(dispatcher)")
	startPolling := strings.Index(pollingBranch, "updater.StartPolling")

	if loadModules == -1 {
		t.Fatal("polling branch does not load modules")
	}
	if startPolling == -1 {
		t.Fatal("polling branch does not start polling")
	}
	if loadModules > startPolling {
		t.Fatal("polling branch starts polling before loading modules")
	}
}

func TestAntiSpamCleanupDefersUnlockAndRecovers(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "modules", "antispam.go")
	if !strings.Contains(source, "error_handling.RecoverFromPanic") {
		t.Fatal("antiSpamCleanupLoop must recover from panics")
	}
	if !strings.Contains(source, "defer antiSpamMutex.Unlock()") {
		t.Fatal("antiSpamCleanupLoop must defer mutex unlock after locking")
	}
}

func TestCaptchaBackgroundGoroutinesRecoverFromPanics(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "modules", "captcha.go")
	required := []string{
		`error_handling.RecoverFromPanic("CaptchaDisableCleanup"`,
		`error_handling.RecoverFromPanic("CaptchaDisableDeleteAttempts"`,
		`error_handling.RecoverFromPanic("CaptchaCleanup"`,
		`error_handling.RecoverFromPanic("CaptchaCleanupExpiredAttempts"`,
		`error_handling.RecoverFromPanic("CaptchaUnmute"`,
	}

	for _, want := range required {
		if !strings.Contains(source, want) {
			t.Fatalf("captcha.go missing background panic recovery %s", want)
		}
	}
}

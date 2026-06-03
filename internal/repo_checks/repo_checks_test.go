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

	source := readRepoFile(t, "alita", "db", "backup", "backup.go")
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

	if !strings.Contains(body, "db.UpdateRecordWithZeroValues") {
		t.Fatal("clearConnectionsData must use UpdateRecordWithZeroValues to persist false")
	}
	if !strings.Contains(body, `"allow_connect"`) ||
		!strings.Contains(body, "false") {
		t.Fatal("clearConnectionsData must set allow_connect to false")
	}
}

func TestImportConnectionsDataUsesTargetChatAndZeroValueUpdate(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "db", "backup", "backup.go")
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

	if !strings.Contains(body, "connections.GetChatConnectionSetting(chatID)") {
		t.Fatal("importConnectionsData must ensure the target chat settings row exists")
	}
	if !strings.Contains(body, "db.UpdateRecordWithZeroValues") {
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

	source := readRepoFile(t, "alita", "db", "backup", "backup.go")
	required := []string{
		"disabling.ShouldDel(chatID)",
		"disabling.ToggleDel(chatID",
		"disabling.DisableCMD(chatID",
		"disabling.EnableCMD(chatID",
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

	// Find the polling branch in main().
	pollingStart := strings.Index(source, "// Use polling mode")
	if pollingStart == -1 {
		t.Fatal("polling branch marker is missing")
	}

	// The polling block in main() calls postInit(...) then updater.StartPolling(...).
	// postInit itself (defined after main()) calls alita.LoadModules(dispatcher).
	// Check execution order by verifying the call site in the polling branch.
	pollingEnd := strings.Index(source[pollingStart:], "\n}")
	if pollingEnd == -1 {
		t.Fatal("could not find end of polling branch")
	}
	pollingBranch := source[pollingStart : pollingStart+pollingEnd]

	postInitCall := strings.Index(pollingBranch, "postInit(b, dispatcher")
	startPollingCall := strings.Index(pollingBranch, "updater.StartPolling")

	if postInitCall == -1 {
		t.Fatal("polling branch does not call postInit")
	}
	if startPollingCall == -1 {
		t.Fatal("polling branch does not start polling")
	}
	if postInitCall > startPollingCall {
		t.Fatal("polling branch starts polling before calling postInit")
	}

	// Verify that postInit itself calls alita.LoadModules before returning.
	sourceAfterPolling := source[pollingStart+pollingEnd:]
	postInitFunc := strings.Index(sourceAfterPolling, "func postInit(")
	if postInitFunc == -1 {
		t.Fatal("postInit function definition is missing")
	}
	postInitBody := sourceAfterPolling[postInitFunc:]
	loadModules := strings.Index(postInitBody, "alita.LoadModules(")
	postInitEnd := strings.Index(postInitBody, "\n}\n")
	if loadModules == -1 || (postInitEnd != -1 && loadModules > postInitEnd) {
		t.Fatal("postInit must call alita.LoadModules")
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

func TestLoadModulesDelegatesToRegistryOnly(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "main.go")
	start := strings.Index(source, "func LoadModules(")
	if start == -1 {
		t.Fatal("LoadModules function is missing")
	}

	body := source[start:]
	end := strings.Index(body, "\n}\n")
	if end == -1 {
		t.Fatal("LoadModules body is malformed")
	}
	body = body[:end]

	if !strings.Contains(body, "modules.LoadAllModules(dispatcher)") {
		t.Fatal("LoadModules must delegate module startup to modules.LoadAllModules")
	}

	explicitLoader := regexp.MustCompile(`modules\.(Load[A-Z][A-Za-z0-9_]*)\s*\(`)
	for _, match := range explicitLoader.FindAllStringSubmatch(body, -1) {
		if match[1] == "LoadAllModules" || match[1] == "LoadHelp" {
			continue
		}
		t.Fatalf("LoadModules must not call individual module loaders directly; found %s", match[0])
	}
}

func TestModuleLoadersAreRegisteredWithRegistry(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("..", "..", "alita", "modules", "*.go"))
	if err != nil {
		t.Fatalf("failed to list module files: %v", err)
	}

	loaderPattern := regexp.MustCompile(`func\s+(Load[A-Z][A-Za-z0-9_]*)\s*\(\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)?\*ext\.Dispatcher\s*\)`)
	registrationPattern := regexp.MustCompile(`RegisterLegacyModule\(\s*"[^"]+"\s*,\s*[^,]+\s*,\s*(Load[A-Z][A-Za-z0-9_]*)\s*\)`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") ||
			strings.HasSuffix(file, string(filepath.Separator)+"registry.go") ||
			strings.HasSuffix(file, string(filepath.Separator)+"help.go") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read module file %s: %v", file, err)
		}
		source := string(data)
		registeredLoaders := make(map[string]bool)
		for _, match := range registrationPattern.FindAllStringSubmatch(source, -1) {
			registeredLoaders[match[1]] = true
		}

		for _, match := range loaderPattern.FindAllStringSubmatch(source, -1) {
			loaderName := match[1]
			if loaderName == "LoadAllModules" || loaderName == "LoadHelp" || loaderName == "LoadBotUpdates" {
				continue
			}

			if !registeredLoaders[loaderName] {
				t.Fatalf("%s defines %s but does not register it with RegisterLegacyModule", file, loaderName)
			}
		}
	}
}

func TestHelpRegistryDoesNotExposeGlobalMutableSingleton(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "modules", "core.go")
	if regexp.MustCompile(`(?m)^var\s+HelpModule\b`).MatchString(source) {
		t.Fatal("help registry must not expose a package-level HelpModule singleton")
	}
	if !strings.Contains(source, "func NewHelpRegistry() *moduleStruct") {
		t.Fatal("help registry must expose a constructor for isolated tests")
	}
}

func TestBotLockApprovedBypassRequiresPositiveSenderID(t *testing.T) {
	t.Parallel()

	source := readRepoFile(t, "alita", "modules", "locks.go")
	start := strings.Index(source, "func (moduleStruct) botLockHandler")
	if start == -1 {
		t.Fatal("botLockHandler function is missing")
	}

	body := source[start:]
	end := strings.Index(body, "\n}\n")
	if end == -1 {
		t.Fatal("botLockHandler body is malformed")
	}
	body = body[:end]

	if !strings.Contains(body, "senderID > 0 && chat_status.IsApproved") {
		t.Fatal("botLockHandler must not call IsApproved unless senderID is positive")
	}
}

func TestCodeUsesSafeCallbackQueryAccessor(t *testing.T) {
	t.Parallel()

	var files []string
	root := filepath.Join("..", "..", "alita")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to list Go files: %v", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read Go file %s: %v", file, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "ctx.CallbackQuery") {
				continue
			}
			if strings.Contains(line, "ctx.CallbackQuery =") {
				continue
			}
			t.Fatalf("%s:%d reads ctx.CallbackQuery directly; use a safe accessor", file, lineNumber+1)
		}
	}
}

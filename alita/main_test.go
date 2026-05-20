package alita

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/modules"
)

type alitaTestBotClient struct{}

func (alitaTestBotClient) RequestWithContext(_ context.Context, _ string, method string, _ map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getMe" {
		return json.RawMessage(`{"id":999,"is_bot":true,"first_name":"Alita","username":"AlitaTestBot"}`), nil
	}
	return json.RawMessage(`true`), nil
}

func (alitaTestBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (alitaTestBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

var (
	alitaMainDBOnce sync.Once
	alitaMainDBErr  error
)

func setupAlitaMainDB(t *testing.T) {
	t.Helper()

	alitaMainDBOnce.Do(func() {
		if db.DB == nil {
			db.DB, alitaMainDBErr = gorm.Open(
				sqlite.Open("file:alita_main_test?mode=memory&cache=shared"),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
			)
			if alitaMainDBErr != nil {
				return
			}
		}
		alitaMainDBErr = db.DB.AutoMigrate(&db.User{})
	})
	if alitaMainDBErr != nil {
		t.Fatalf("setup alita main DB: %v", alitaMainDBErr)
	}
}

func resetHelpRegistryForTest(t *testing.T) {
	t.Helper()

	registry := modules.DefaultHelpRegistry()
	registry.AbleMap.Init()
	registry.AltHelpOptions = make(map[string][]string)
	t.Cleanup(func() {
		registry.AbleMap.Init()
		registry.AltHelpOptions = make(map[string][]string)
	})
}

func TestListModulesSortsEnabledModuleNames(t *testing.T) {
	resetHelpRegistryForTest(t)

	registry := modules.DefaultHelpRegistry()

	registry.AbleMap.Store("Warns", true)
	registry.AbleMap.Store("Admin", true)
	registry.AbleMap.Store("Filters", true)

	if got, want := ListModules(), "[Admin, Filters, Warns]"; got != want {
		t.Fatalf("ListModules() = %q, want %q", got, want)
	}
}

func TestLoadModulesLoadsRegistryAndHelp(t *testing.T) {
	resetHelpRegistryForTest(t)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadModules(dispatcher)

	for _, moduleName := range []string{"Admin", "Captcha", "Filters", "Greetings", "Warns"} {
		_, enabled := modules.DefaultHelpRegistry().AbleMap.Load(moduleName)
		if !enabled {
			t.Fatalf("%s was not enabled after LoadModules", moduleName)
		}
	}
}

func TestCheckDuplicateAliasesAllowsUniqueAliases(t *testing.T) {
	resetHelpRegistryForTest(t)

	registry := modules.DefaultHelpRegistry()
	registry.AltHelpOptions["Admin"] = []string{"admin", "promote"}
	registry.AltHelpOptions["Warns"] = []string{"warn", "dwarn"}

	checkDuplicateAliases()
}

func TestInitialChecksEnsuresBotAndValidatesAliases(t *testing.T) {
	setupAlitaMainDB(t)
	resetHelpRegistryForTest(t)
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: alitaTestBotClient{},
		User: gotgbot.User{
			Id:        999,
			IsBot:     true,
			FirstName: "Fallback",
			Username:  "fallback_bot",
		},
	}

	if err := InitialChecks(bot); err != nil {
		t.Fatalf("InitialChecks() error = %v", err)
	}

	var user db.User
	if err := db.DB.Where("user_id = ?", int64(999)).First(&user).Error; err != nil {
		t.Fatalf("bot user was not created: %v", err)
	}
	if user.UserName != "AlitaTestBot" || user.Name != "Alita" {
		t.Fatalf("bot user = username %q name %q, want AlitaTestBot/Alita", user.UserName, user.Name)
	}
}

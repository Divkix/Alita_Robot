package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/modules"
	alitaerrors "github.com/divkix/Alita_Robot/alita/utils/errors"
)

type mainBotCall struct {
	method string
	params map[string]any
}

type mainBotClient struct {
	calls []mainBotCall
	// failMethods fails one API method, for refusal and error-path tests.
	failMethods map[string]error
}

func (c *mainBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.calls = append(c.calls, mainBotCall{method: method, params: params})
	if err := c.failMethods[method]; err != nil {
		return nil, err
	}
	switch method {
	case "getMe":
		return json.RawMessage(`{"id":999,"is_bot":true,"first_name":"Alita","username":"AlitaTestBot"}`), nil
	case "setMyCommands":
		return json.RawMessage(`true`), nil
	case "setChatMenuButton":
		return json.RawMessage(`true`), nil
	case "sendMessage":
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup"}}`), nil
	default:
		return nil, gotgbot.ErrInvalidTokenFormat
	}
}

// callsFor returns the recorded calls of one API method.
func (c *mainBotClient) callsFor(method string) []mainBotCall {
	var calls []mainBotCall
	for _, call := range c.calls {
		if call.method == method {
			calls = append(calls, call)
		}
	}
	return calls
}

func (c *mainBotClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	if opts != nil && opts.APIURL != "" {
		return strings.TrimSuffix(opts.APIURL, "/")
	}
	return "https://api.telegram.org"
}

func (c *mainBotClient) FileURL(token string, tgFilePath string, opts *gotgbot.RequestOpts) string {
	return c.GetAPIURL(opts) + "/file/bot" + token + "/" + tgFilePath
}

func TestResolveBotAPIURL(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{name: "empty", output: gotgbot.DefaultAPIURL},
		{name: "default", input: gotgbot.DefaultAPIURL, output: gotgbot.DefaultAPIURL},
		{name: "path prefix", input: "https://bot-api.example/internal/", output: "https://bot-api.example/internal"},
		{
			name:   "drops unsupported components",
			input:  "https://user:secret@bot-api.example/internal/?x=1#fragment",
			output: "https://bot-api.example/internal",
		},
		{name: "invalid URL", input: "://bad-url", output: gotgbot.DefaultAPIURL},
		{name: "missing scheme", input: "bot-api.example/internal", output: gotgbot.DefaultAPIURL},
		{name: "unsupported scheme", input: "ftp://bot-api.example/internal", output: gotgbot.DefaultAPIURL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveBotAPIURL(test.input); got != test.output {
				t.Fatalf("resolveBotAPIURL(%q) = %q, want %q", test.input, got, test.output)
			}
		})
	}
}

func TestBaseBotClientUsesResolvedAPIURL(t *testing.T) {
	client := &gotgbot.BaseBotClient{
		DefaultRequestOpts: &gotgbot.RequestOpts{
			APIURL: resolveBotAPIURL("https://bot-api.example/internal/"),
		},
	}
	if got := client.GetAPIURL(nil); got != "https://bot-api.example/internal" {
		t.Fatalf("GetAPIURL(nil) = %q, want custom API URL", got)
	}
	if got := client.FileURL("123:token", "photos/file.jpg", nil); got != "https://bot-api.example/internal/file/bot123:token/photos/file.jpg" {
		t.Fatalf("FileURL() = %q, want custom API file URL", got)
	}
}

func TestPostInitSetsCommandsAndStartupMessage(t *testing.T) {
	previousConfig := config.AppConfig
	config.AppConfig.MessageDump = -100123
	config.AppConfig.WorkingMode = ""
	t.Cleanup(func() {
		config.AppConfig = previousConfig
	})

	previousDB := db.DB
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open captcha lifecycle test database: %v", err)
	}
	if err := testDB.AutoMigrate(&db.CaptchaAttempts{}); err != nil {
		t.Fatalf("migrate captcha lifecycle test database: %v", err)
	}
	db.DB = testDB
	t.Cleanup(func() {
		modules.StopCaptchaLifecycle()
		db.DB = previousDB
	})

	client := &mainBotClient{}
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "AlitaTestBot",
		},
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	postInit(bot, dispatcher, bot.Username, "polling")

	if config.AppConfig.WorkingMode != "polling" {
		t.Fatalf("WorkingMode = %q, want polling", config.AppConfig.WorkingMode)
	}
	if len(client.calls) != 2 {
		t.Fatalf("got %d bot calls, want setMyCommands and sendMessage", len(client.calls))
	}
	if client.calls[0].method != "setMyCommands" {
		t.Fatalf("first call = %s, want setMyCommands", client.calls[0].method)
	}
	if client.calls[1].method != "sendMessage" {
		t.Fatalf("second call = %s, want sendMessage", client.calls[1].method)
	}
	if got := client.calls[1].params["chat_id"]; got != int64(-100123) {
		t.Fatalf("startup message chat_id = %#v, want MessageDump", got)
	}
	// The panel is disabled by default: postInit must leave the menu button as
	// it was, so there is no third call at all.
	if calls := client.callsFor("setChatMenuButton"); len(calls) != 0 {
		t.Fatalf("setChatMenuButton calls = %d, want none while the panel is disabled", len(calls))
	}
}

var mainTestLocalesOnce sync.Once

// initTestLocales loads the embedded locales once, as main() does, so a test
// compares against the real user-visible labels instead of empty strings.
func initTestLocales(t *testing.T) {
	t.Helper()
	mainTestLocalesOnce.Do(func() {
		if err := i18n.GetManager().Initialize(&Locales, "locales", i18n.DefaultManagerConfig()); err != nil {
			t.Fatalf("initialize locales: %v", err)
		}
	})
}

// With the panel enabled the default menu button becomes a Mini App button in
// every private chat; with it disabled the button is left untouched.
func TestPanelMenuButtonFollowsPanelConfig(t *testing.T) {
	previousEnabled, previousURL := config.AppConfig.PanelEnabled, config.AppConfig.PanelPublicURL
	t.Cleanup(func() {
		config.AppConfig.PanelEnabled, config.AppConfig.PanelPublicURL = previousEnabled, previousURL
	})
	initTestLocales(t)
	label, err := i18n.MustNewTranslator("en").GetString("panel_open_button")
	if err != nil || label == "" {
		t.Fatalf("panel_open_button = %q, %v; want a label", label, err)
	}

	config.AppConfig.PanelEnabled = false
	config.AppConfig.PanelPublicURL = "https://panel.example.com"
	disabled := &mainBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: disabled})
	if calls := disabled.callsFor("setChatMenuButton"); len(calls) != 0 {
		t.Fatalf("disabled panel sent %d setChatMenuButton calls, want none", len(calls))
	}

	config.AppConfig.PanelEnabled = true
	enabled := &mainBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: enabled})
	calls := enabled.callsFor("setChatMenuButton")
	if len(calls) != 1 {
		t.Fatalf("setChatMenuButton calls = %d, want 1", len(calls))
	}
	if got, ok := calls[0].params["chat_id"]; ok {
		t.Errorf("chat_id = %v, want no chat_id so the default button of every private chat changes", got)
	}
	button, ok := calls[0].params["menu_button"].(gotgbot.MenuButtonWebApp)
	if !ok {
		t.Fatalf("menu_button type = %T, want MenuButtonWebApp", calls[0].params["menu_button"])
	}
	if button.Text != label {
		t.Errorf("menu button text = %q, want %q", button.Text, label)
	}
	if button.WebApp.Url != "https://panel.example.com" {
		t.Errorf("menu button url = %q, want the configured panel URL", button.WebApp.Url)
	}

	// A restart repeats the same write, so the button never drifts.
	restart := &mainBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: restart})
	again := restart.callsFor("setChatMenuButton")
	if len(again) != 1 {
		t.Fatalf("restart setChatMenuButton calls = %d, want 1", len(again))
	}
	if got := again[0].params["menu_button"]; got != button {
		t.Errorf("restart menu button = %#v, want the same value %#v", got, button)
	}
}

// A refusal from Telegram is a warning, never a startup failure: the command
// menu and the group buttons keep working.
func TestPanelMenuButtonRefusalOnlyWarns(t *testing.T) {
	initTestLocales(t)
	previousLevel := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() { log.SetLevel(previousLevel) })
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)
	previousEnabled, previousURL := config.AppConfig.PanelEnabled, config.AppConfig.PanelPublicURL
	t.Cleanup(func() {
		config.AppConfig.PanelEnabled, config.AppConfig.PanelPublicURL = previousEnabled, previousURL
	})
	config.AppConfig.PanelEnabled = true
	config.AppConfig.PanelPublicURL = "https://panel.example.com"

	client := &mainBotClient{failMethods: map[string]error{
		"setChatMenuButton": errors.New("Bad Request: BUTTON_URL_INVALID"),
	}}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: client})

	var warned bool
	for _, entry := range hook.AllEntries() {
		if entry.Level <= log.ErrorLevel {
			t.Fatalf("refusal logged at %s: %q", entry.Level, entry.Message)
		}
		if entry.Level == log.WarnLevel && strings.Contains(entry.Message, "menu button") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("no warning about the menu button; entries = %v", hook.AllEntries())
	}
}

func TestResolveBotUsernameReadsGetMeResponse(t *testing.T) {
	client := &mainBotClient{}
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true},
	}

	if got := resolveBotUsername(bot); got != "AlitaTestBot" {
		t.Fatalf("resolveBotUsername() = %q, want AlitaTestBot", got)
	}
}

func TestNewDispatcherHandlesExpectedAndWrappedErrors(t *testing.T) {
	dispatcher := newConfiguredDispatcher(7)
	if dispatcher == nil {
		t.Fatal("newConfiguredDispatcher() = nil")
	}
	if dispatcher.Error == nil {
		t.Fatal("dispatcher Error handler is nil")
	}

	ctx := &ext.Context{Update: &gotgbot.Update{UpdateId: 42}}
	action := dispatcher.Error(nil, ctx, &gotgbot.TelegramError{Description: "Bad Request: message to delete not found"})
	if action != ext.DispatcherActionNoop {
		t.Fatalf("expected Telegram error action = %s, want noop", action)
	}

	action = dispatcher.Error(nil, ctx, alitaerrors.Wrap(assertErr{}, "wrapped failure"))
	if action != ext.DispatcherActionNoop {
		t.Fatalf("wrapped error action = %s, want noop", action)
	}
}

func TestDispatcherErrorHandlerDoesNotLogControlFlowAsError(t *testing.T) {
	var buf bytes.Buffer
	previous := log.StandardLogger()
	log.SetOutput(&buf)
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		log.SetOutput(previous.Out)
		log.SetFormatter(previous.Formatter)
		log.SetLevel(previous.GetLevel())
	})

	ctx := &ext.Context{Update: &gotgbot.Update{UpdateId: 42}}
	action := dispatcherErrorHandler(nil, ctx, ext.EndGroups)
	if action != ext.DispatcherActionNoop {
		t.Fatalf("EndGroups action = %s, want noop", action)
	}
	if strings.Contains(buf.String(), "level=error") {
		t.Fatalf("EndGroups was logged as an error: %s", buf.String())
	}

	buf.Reset()
	action = dispatcherErrorHandler(nil, ctx, ext.ContinueGroups)
	if action != ext.DispatcherActionNoop {
		t.Fatalf("ContinueGroups action = %s, want noop", action)
	}
	if strings.Contains(buf.String(), "level=error") {
		t.Fatalf("ContinueGroups was logged as an error: %s", buf.String())
	}
}

type assertErr struct{}

func (assertErr) Error() string {
	return "assert error"
}

func TestCloseDBConnectionsNilDBReturnsNil(t *testing.T) {
	previousDB := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = previousDB })

	if err := closeDBConnections(); err != nil {
		t.Fatalf("closeDBConnections() with nil DB = %v, want nil", err)
	}
}

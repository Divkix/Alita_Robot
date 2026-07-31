package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/modules"
	alitaerrors "github.com/divkix/Alita_Robot/alita/utils/errors"
)

type mainBotCall struct {
	method string
	params map[string]any
}

type mainBotClient struct {
	calls []mainBotCall
}

func (c *mainBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.calls = append(c.calls, mainBotCall{method: method, params: params})
	switch method {
	case "getMe":
		return json.RawMessage(`{"id":999,"is_bot":true,"first_name":"Alita","username":"AlitaTestBot"}`), nil
	case "setMyCommands":
		return json.RawMessage(`true`), nil
	case "sendMessage":
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup"}}`), nil
	default:
		return nil, gotgbot.ErrInvalidTokenFormat
	}
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

func TestNewBotAPITransportKeepsConnectionTuning(t *testing.T) {
	transport := newBotAPITransport(12, 4)
	if transport.MaxIdleConns != 12 || transport.MaxIdleConnsPerHost != 4 {
		t.Fatalf(
			"transport limits = (%d, %d), want (12, 4)",
			transport.MaxIdleConns,
			transport.MaxIdleConnsPerHost,
		)
	}
	if transport.MaxConnsPerHost <= transport.MaxIdleConnsPerHost {
		t.Fatalf("MaxConnsPerHost = %d, want more than MaxIdleConnsPerHost", transport.MaxConnsPerHost)
	}
}

func TestHealthCheckPortUsesProviderEnvironment(t *testing.T) {
	previousConfig := config.AppConfig
	config.AppConfig = &config.Config{HTTPPort: 8080}
	t.Cleanup(func() { config.AppConfig = previousConfig })

	t.Setenv("HTTP_PORT", "")
	t.Setenv("PORT", "9090")
	if got := healthCheckPort(); got != 9090 {
		t.Fatalf("healthCheckPort() = %d, want Railway PORT 9090", got)
	}

	t.Setenv("HTTP_PORT", "7070")
	if got := healthCheckPort(); got != 7070 {
		t.Fatalf("healthCheckPort() = %d, want HTTP_PORT 7070", got)
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

func TestMainVersionModeExitsWithConfiguredVersion(t *testing.T) {
	cmd := helperMainCommand(t, "--version")
	cmd.Env = append(cmd.Env, "ALITA_TEST_MAIN_VERSION=v9.9.9")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main --version exited with error: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "v9.9.9" {
		t.Fatalf("main --version output = %q, want configured version", got)
	}
}

func TestMainHealthModeExitsByStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "healthy", statusCode: http.StatusOK},
		{name: "unhealthy", statusCode: http.StatusServiceUnavailable, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			t.Cleanup(server.Close)

			port := serverPort(t, server.URL)
			cmd := helperMainCommand(t, "--health")
			cmd.Env = append(cmd.Env, "HTTP_PORT="+port, "PORT=")

			output, err := cmd.CombinedOutput()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("main --health succeeded, want exit error\n%s", output)
				}
				return
			}
			if err != nil {
				t.Fatalf("main --health exited with error: %v\n%s", err, output)
			}
		})
	}
}

func TestCloseDBConnectionsAllowsNilDatabase(t *testing.T) {
	if err := closeDBConnections(); err != nil {
		t.Fatalf("close nil database: %v", err)
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

type assertErr struct{}

func (assertErr) Error() string {
	return "assert error"
}

func TestHelperMainProcess(t *testing.T) {
	if os.Getenv("ALITA_TEST_MAIN_PROCESS") != "1" {
		return
	}

	if version := os.Getenv("ALITA_TEST_MAIN_VERSION"); version != "" {
		config.AppConfig.BotVersion = version
	}
	args := []string{os.Args[0]}
	if sep := slicesIndex(os.Args, "--"); sep >= 0 && sep+1 < len(os.Args) {
		args = append(args, os.Args[sep+1:]...)
	}
	os.Args = args
	main()
}

func helperMainCommand(t *testing.T, arg string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperMainProcess$", "--", arg)
	cmd.Env = append(os.Environ(), "ALITA_TEST_MAIN_PROCESS=1")
	return cmd
}

func serverPort(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	_, port, ok := strings.Cut(parsed.Host, ":")
	if !ok || port == "" {
		t.Fatalf("server URL has no port: %s", rawURL)
	}
	return port
}

func slicesIndex(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

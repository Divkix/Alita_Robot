package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	alitaerrors "github.com/divkix/Alita_Robot/alita/utils/errors"
)

type captureRoundTripper struct {
	req *http.Request
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

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

func TestAPIServerRewriteTransportRewritesTelegramRequests(t *testing.T) {
	base := &captureRoundTripper{}
	target, err := url.Parse("https://bot-api.example/internal")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	transport := &apiServerRewriteTransport{base: base, target: target}
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot123/sendMessage", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	if base.req == nil {
		t.Fatal("base transport did not receive rewritten request")
	}
	if got := base.req.URL.String(); got != "https://bot-api.example/internal/bot123/sendMessage" {
		t.Fatalf("rewritten URL = %q", got)
	}
	if base.req.Host != "bot-api.example" {
		t.Fatalf("rewritten Host = %q", base.req.Host)
	}
	if req.URL.String() != "https://api.telegram.org/bot123/sendMessage" {
		t.Fatalf("original request was mutated: %q", req.URL.String())
	}
}

func TestNewBotAPITransportUsesRewriteForCustomServer(t *testing.T) {
	transport := newBotAPITransport(12, 4, "https://bot-api.example/internal")
	rewrite, ok := transport.(*apiServerRewriteTransport)
	if !ok {
		t.Fatalf("newBotAPITransport() = %T, want apiServerRewriteTransport", transport)
	}
	if rewrite.target.String() != "https://bot-api.example/internal" {
		t.Fatalf("rewrite target = %q", rewrite.target.String())
	}
	if base, ok := rewrite.base.(*http.Transport); !ok || base.MaxIdleConns != 12 {
		t.Fatalf("rewrite base = %#v, want configured *http.Transport", rewrite.base)
	}
}

func TestNewBotAPITransportFallsBackForDefaultOrInvalidServer(t *testing.T) {
	for _, apiServer := range []string{"", "https://api.telegram.org", "://bad-url"} {
		transport := newBotAPITransport(9, 3, apiServer)
		if _, ok := transport.(*apiServerRewriteTransport); ok {
			t.Fatalf("newBotAPITransport(%q) returned rewrite transport", apiServer)
		}
		base, ok := transport.(*http.Transport)
		if !ok {
			t.Fatalf("newBotAPITransport(%q) = %T, want *http.Transport", apiServer, transport)
		}
		if base.MaxIdleConns != 9 || base.MaxIdleConnsPerHost != 3 {
			t.Fatalf("transport limits = (%d, %d), want (9, 3)", base.MaxIdleConns, base.MaxIdleConnsPerHost)
		}
	}
}

func TestAPIServerRewriteTransportPreservesNonTelegramRequests(t *testing.T) {
	base := &captureRoundTripper{}
	target, err := url.Parse("https://bot-api.example/internal")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	transport := &apiServerRewriteTransport{base: base, target: target}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	if base.req != req {
		t.Fatal("non-Telegram request should be passed through unchanged")
	}
	if base.req.URL.String() != "https://example.com/status" {
		t.Fatalf("pass-through URL = %q", base.req.URL.String())
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
			cmd.Env = append(cmd.Env, "ALITA_TEST_MAIN_HTTP_PORT="+port)

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
	if port := os.Getenv("ALITA_TEST_MAIN_HTTP_PORT"); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil {
			t.Fatalf("invalid ALITA_TEST_MAIN_HTTP_PORT: %v", err)
		}
		config.AppConfig.HTTPPort = parsed
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

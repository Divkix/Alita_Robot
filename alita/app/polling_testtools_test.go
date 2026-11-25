//go:build testtools

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

// longPollClient answers the first getUpdates with an empty list and holds
// every later one open until the updater cancels it, like a long poll. It
// records the timeout the first getUpdates asked Telegram to hold it for.
type longPollClient struct {
	*appBotClient
	polled      chan struct{}
	held        chan struct{}
	once        sync.Once
	cancelled   func()
	pollTimeout any
}

func (c *longPollClient) RequestWithContext(ctx context.Context, _ string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method != "getUpdates" {
		return c.appBotClient.RequestWithContext(ctx, "", method, params, opts)
	}
	first := false
	c.once.Do(func() { first = true })
	if first {
		c.pollTimeout = params["timeout"]
		close(c.polled)
		return json.RawMessage(`[]`), nil
	}
	close(c.held)
	<-ctx.Done()
	c.cancelled()
	return nil, ctx.Err()
}

// The long-poll timeout leaves the transport room to read Telegram's empty
// answer: an idle poll must end as an empty list, never a header timeout.
func TestPollingTimeoutStaysBelowResponseHeaderTimeout(t *testing.T) {
	headerTimeout := newBotAPITransport(1, 1).ResponseHeaderTimeout
	if headerTimeout-pollingTimeout < 2*time.Second {
		t.Errorf("polling timeout %v is not at least 2s below the response-header timeout %v", pollingTimeout, headerTimeout)
	}
}

// The updater stop is a shutdown step: a poll held for pollingTimeout must
// end at least 2s before the manager gives up on that step.
func TestPollingTimeoutStaysBelowShutdownHandlerLimit(t *testing.T) {
	if limit := shutdown.HandlerTimeoutForTest(); limit-pollingTimeout < 2*time.Second {
		t.Errorf("polling timeout %v is not at least 2s below the shutdown per-handler limit %v", pollingTimeout, limit)
	}
}

// A stop signal in polling mode runs every shutdown step, updater first and
// tracer last, and the polling function keeps the process alive until the
// manager's exit: returning from it is returning from main. The getUpdates
// it sends asks for a long poll, and a poll held open does not delay the
// shutdown: the updater stop is not skipped after its handler limit.
func TestPollingShutdownRunsEveryDrainBeforeExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := shutdown.NewManager()
		var mu sync.Mutex
		var order []string
		record := func(step string) {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, step)
		}
		drain := func(step string) func(context.Context) error {
			return func(context.Context) error {
				record(step)
				return nil
			}
		}
		manager.Register("tracer", shutdown.PhaseTelemetry, drain("tracer"))
		RegisterShutdownSteps(manager, ShutdownDrainSpecs(
			drain("db close"),
			drain("stats flush"),
			drain("jobs stop"),
			[]DrainSpec{
				{Name: "users drain", Handler: drain("users drain")},
				{Name: "captcha stop", Handler: drain("captcha stop")},
				{Name: "aispam drain", Handler: drain("aispam drain")},
			},
		)...)
		httpStopStarted := make(chan struct{})
		releaseHTTPStop := make(chan struct{})
		manager.Register("http stop", shutdown.PhaseHTTPServer, func(context.Context) error {
			close(httpStopStarted)
			<-releaseHTTPStop
			record("http stop")
			return nil
		})

		returned := make(chan struct{})
		exited := make(chan int, 1)
		var orderAtExit []string
		var returnedBeforeExit bool
		restore := shutdown.SetExitForTest(func(code int) {
			mu.Lock()
			orderAtExit = slices.Clone(order)
			mu.Unlock()
			select {
			case <-returned:
				returnedBeforeExit = true
			default:
			}
			exited <- code
		})
		defer restore()
		startShutdown, restoreSignals := shutdown.StubSignalsForTest()
		defer restoreSignals()

		client := &longPollClient{
			appBotClient: &appBotClient{},
			polled:       make(chan struct{}),
			held:         make(chan struct{}),
			cancelled:    func() { record("updater stop") },
		}
		bot := &gotgbot.Bot{Token: "999:test", BotClient: client}
		updater := ext.NewUpdater(ext.NewDispatcher(nil), nil)

		go func() {
			defer close(returned)
			runPolling(bot, updater, &config.Config{}, updaterStopSpec(updater), manager, func(*shutdown.Manager) { manager.WaitForShutdown() })
		}()

		<-client.polled
		pollTimeout, _ := strconv.Atoi(fmt.Sprint(client.pollTimeout))
		if pollTimeout <= 0 {
			t.Errorf("getUpdates timeout = %v, want a long poll above 0s", client.pollTimeout)
		}
		// Ensure shutdown cancels a held poll instead of stopping between polls.
		<-client.held
		shutdownStarted := time.Now()
		startShutdown()

		// With a later drain held open, let every goroutine settle: a polling
		// function released by the updater stop has returned by now.
		<-httpStopStarted
		synctest.Wait()
		close(releaseHTTPStop)

		if code := <-exited; code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if elapsed := time.Since(shutdownStarted); elapsed >= pollingTimeout {
			t.Errorf("shutdown took %v, waiting out the %v poll held open", elapsed, pollingTimeout)
		}
		<-returned

		if returnedBeforeExit {
			t.Error("runPolling returned before the shutdown manager exited; the remaining drains would not run")
		}
		want := []string{"updater stop", "http stop", "jobs stop", "aispam drain", "captcha stop", "users drain", "stats flush", "db close", "tracer"}
		if !slices.Equal(orderAtExit, want) {
			t.Errorf("drains run before exit = %v, want %v", orderAtExit, want)
		}
	})
}

// Run in webhook mode registers the webhook, runs postInit, starts the HTTP
// server and waits on the shutdown manager — with no re-validation of the
// webhook contract config already owns.
func TestRunWebhookForkRegistersWebhookAndWaits(t *testing.T) {
	client := &appBotClient{}
	manager := shutdown.NewManager()
	waitCalled := make(chan struct{})
	var drainOrder []string
	deps := runDeps(client, func(m *shutdown.Manager) {
		drainOrder = m.ExecuteHandlersForTest()
		close(waitCalled)
	})
	deps.NewShutdownManager = func() *shutdown.Manager { return manager }

	cfg := &config.Config{
		UseWebhooks:    true,
		WebhookDomain:  "https://bot.example.com",
		WebhookSecret:  "secret",
		HTTPPort:       0,
		MessageDump:    -100123,
		AllowedUpdates: []string{"message"},
	}
	Run("webhook", cfg, deps)

	<-waitCalled
	if cfg.WorkingMode != "webhook" {
		t.Fatalf("WorkingMode = %q, want webhook", cfg.WorkingMode)
	}
	webhookCalls := client.callsFor("setWebhook")
	if len(webhookCalls) != 1 {
		t.Fatalf("setWebhook calls = %d, want 1", len(webhookCalls))
	}
	if got := webhookCalls[0].params["url"]; got != "https://bot.example.com/webhook" {
		t.Errorf("setWebhook url = %v, want domain + /webhook", got)
	}
	if got := webhookCalls[0].params["secret_token"]; got != "secret" {
		t.Errorf("setWebhook secret_token = %v, want the configured secret", got)
	}
	if calls := client.callsFor("deleteWebhook"); len(calls) != 0 {
		t.Fatalf("deleteWebhook calls = %d, want 0 in webhook mode", len(calls))
	}
	if calls := client.callsFor("setMyCommands"); len(calls) != 1 {
		t.Fatalf("setMyCommands calls = %d, want 1 (postInit ran)", len(calls))
	}

	want := []string{"http stop", "jobs stop", "aispam drain", "captcha stop", "users drain", "stats flush", "db close", "tracer"}
	if !slices.Equal(drainOrder, want) {
		t.Errorf("drains executed = %v, want %v", drainOrder, want)
	}
}

// Run in polling mode removes the webhook, runs postInit and registers the
// updater stop in the update-source phase before waiting on shutdown.
func TestRunPollingForkDeletesWebhookAndRegistersUpdaterDrain(t *testing.T) {
	client := &longPollClient{
		appBotClient: &appBotClient{},
		polled:       make(chan struct{}),
		held:         make(chan struct{}),
		cancelled:    func() {},
	}
	manager := shutdown.NewManager()
	waitCalled := make(chan struct{})
	var drainOrder []string
	deps := runDeps(client, func(m *shutdown.Manager) {
		<-client.held // let the updater hold a poll open, like a real shutdown signal would
		drainOrder = m.ExecuteHandlersForTest()
		close(waitCalled)
	})
	deps.NewShutdownManager = func() *shutdown.Manager { return manager }

	cfg := &config.Config{
		UseWebhooks:    false,
		HTTPPort:       0,
		MessageDump:    -100123,
		AllowedUpdates: []string{"message"},
	}
	Run("polling", cfg, deps)

	<-waitCalled
	if cfg.WorkingMode != "polling" {
		t.Fatalf("WorkingMode = %q, want polling", cfg.WorkingMode)
	}
	if calls := client.callsFor("deleteWebhook"); len(calls) != 1 {
		t.Fatalf("deleteWebhook calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("setWebhook"); len(calls) != 0 {
		t.Fatalf("setWebhook calls = %d, want 0 in polling mode", len(calls))
	}
	if calls := client.callsFor("setMyCommands"); len(calls) != 1 {
		t.Fatalf("setMyCommands calls = %d, want 1 (postInit ran)", len(calls))
	}

	want := []string{"updater stop", "http stop", "jobs stop", "aispam drain", "captcha stop", "users drain", "stats flush", "db close", "tracer"}
	if !slices.Equal(drainOrder, want) {
		t.Errorf("drains executed = %v, want %v", drainOrder, want)
	}
}

//go:build testtools

package main

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

	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

// longPollClient answers the first getUpdates with an empty list and holds
// every later one open until the updater cancels it, like a long poll. It
// records the timeout the first getUpdates asked Telegram to hold it for.
type longPollClient struct {
	*mainBotClient
	polled      chan struct{}
	once        sync.Once
	cancelled   func()
	pollTimeout any
}

func (c *longPollClient) RequestWithContext(ctx context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method != "getUpdates" {
		return nil, fmt.Errorf("unexpected method %s", method)
	}
	first := false
	c.once.Do(func() { first = true })
	if first {
		c.pollTimeout = params["timeout"]
		close(c.polled)
		return json.RawMessage(`[]`), nil
	}
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
		drain := func(step string) func() error {
			return func() error {
				record(step)
				return nil
			}
		}
		manager.RegisterHandler(drain("tracer"))
		registerShutdownDrains(manager, drain("db close"), drain("stats flush"), drain("jobs stop"))
		manager.RegisterHandler(drain("users drain"))
		manager.RegisterHandler(drain("captcha stop"))
		manager.RegisterHandler(drain("aispam drain"))
		httpStopStarted := make(chan struct{})
		releaseHTTPStop := make(chan struct{})
		manager.RegisterHandler(func() error {
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
			mainBotClient: &mainBotClient{},
			polled:        make(chan struct{}),
			cancelled:     func() { record("updater stop") },
		}
		bot := &gotgbot.Bot{Token: "999:test", BotClient: client}
		updater := ext.NewUpdater(ext.NewDispatcher(nil), nil)

		go func() {
			defer close(returned)
			runPolling(bot, updater, manager)
		}()

		<-client.polled
		pollTimeout, _ := strconv.Atoi(fmt.Sprint(client.pollTimeout))
		if pollTimeout <= 0 {
			t.Errorf("getUpdates timeout = %v, want a long poll above 0s", client.pollTimeout)
		}
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
		want := []string{"updater stop", "http stop", "aispam drain", "captcha stop", "users drain", "jobs stop", "stats flush", "db close", "tracer"}
		if !slices.Equal(orderAtExit, want) {
			t.Errorf("drains run before exit = %v, want %v", orderAtExit, want)
		}
	})
}

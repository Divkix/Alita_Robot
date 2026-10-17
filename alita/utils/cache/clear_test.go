package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/config"
)

// commandHook delays every Redis command, optionally fails it, and pages SCAN
// replies. miniredis ignores SCAN's COUNT and returns every key in one reply,
// so without paging a test cannot reproduce the many-step walk a large
// keyspace needs. Because the walk deletes what it scans, each page is simply
// the first pageSize keys left, with cursor 1 while more remain. miniredis
// answers any non-zero cursor with nothing, so the hook sends cursor 0.
type commandHook struct {
	delay    time.Duration
	fail     func(cmd redis.Cmder) error
	pageSize int
}

func (h commandHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h commandHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		time.Sleep(h.delay)
		if h.fail != nil {
			if err := h.fail(cmd); err != nil {
				return err
			}
		}
		if _, ok := cmd.(*redis.ScanCmd); ok && h.pageSize > 0 {
			cmd.Args()[1] = 0
		}
		if err := next(ctx, cmd); err != nil {
			return err
		}
		if scan, ok := cmd.(*redis.ScanCmd); ok && h.pageSize > 0 {
			keys, _ := scan.Val()
			if len(keys) > h.pageSize {
				scan.SetVal(keys[:h.pageSize], 1)
			}
		}
		return nil
	}
}

func (h commandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func seedDataKeys(t *testing.T, server *miniredis.Miniredis, n int) {
	t.Helper()
	for i := range n {
		if err := server.Set(fmt.Sprintf("%schat:%d", DataCachePrefix, i), "v"); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.Set("alita:antiraid:state:-1001", "operational"); err != nil {
		t.Fatal(err)
	}
}

func newClearClient(t *testing.T, server *miniredis.Miniredis, hook commandHook) *redis.Client {
	t.Helper()
	options, err := newRedisOptions(&config.Config{RedisAddress: server.Addr()})
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	client.AddHook(hook)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func setContextTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	previous := contextTimeout
	contextTimeout = d
	t.Cleanup(func() { contextTimeout = previous })
}

func dataKeyCount(server *miniredis.Miniredis) int {
	n := 0
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, DataCachePrefix) {
			n++
		}
	}
	return n
}

// The walk is many SCAN/UNLINK round trips. The 5 s budget is per step: a big
// keyspace whose whole walk outlasts one budget must still be cleared.
func TestClearWithClientBudgetIsPerStepNotPerWalk(t *testing.T) {
	server := miniredis.RunT(t)
	seedDataKeys(t, server, 600) // 30 SCAN + 30 UNLINK steps at a page of 20
	setContextTimeout(t, time.Second)
	// 60 steps x 20 ms = at least 1.2 s of sleeping, so the walk always
	// outlasts the budget (sleeps only ever add time). A single step has 50x
	// headroom, so scheduler stalls under -race and parallel packages do not
	// push one past the budget.
	client := newClearClient(t, server, commandHook{delay: 20 * time.Millisecond, pageSize: 20})

	if err := clearWithClient(client); err != nil {
		t.Fatalf("clearWithClient() error = %v", err)
	}
	if left := dataKeyCount(server); left != 0 {
		t.Errorf("%d cache keys survived the clear", left)
	}
	if !server.Exists("alita:antiraid:state:-1001") {
		t.Error("clear deleted an operational key")
	}
}

// A clear that stops early must say so loudly, with how far it got.
func TestClearOnStartupLogsErrorWithDeletedCount(t *testing.T) {
	server := miniredis.RunT(t)
	seedDataKeys(t, server, 300)
	scans := 0
	client := newClearClient(t, server, commandHook{pageSize: 100, fail: func(cmd redis.Cmder) error {
		if strings.EqualFold(cmd.Name(), "scan") {
			scans++
			if scans == 2 {
				return errors.New("redis went away")
			}
		}
		return nil
	}})
	// NewGlobal adds its hook to the process-wide logger for good; swap the
	// hooks out for this test and put them back afterwards.
	previousHooks := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(previousHooks) })
	hook := logtest.NewGlobal()

	clearOnStartup(client)

	var entry *log.Entry
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "clear") {
			entry = e
		}
	}
	if entry == nil {
		t.Fatal("no log entry about the failed clear")
	}
	if entry.Level != log.ErrorLevel {
		t.Errorf("failed clear logged at %v, want error", entry.Level)
	}
	for _, want := range []string{"redis went away", "after clearing 100 keys"} {
		if !strings.Contains(entry.Message, want) {
			t.Errorf("log message %q does not mention %q", entry.Message, want)
		}
	}
	if left := dataKeyCount(server); left != 200 {
		t.Errorf("%d cache keys left, want 200 (one batch deleted before the failure)", left)
	}
}

package spamdb

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func TestCheckReportsLolsListing(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	lols := newFakeService(t, http.StatusOK, lolsBanned)
	c := newBothChecker(cas, lols, Options{CASAPIKey: "cas-test-key", Redis: func() *redis.Client { return nil }})

	src, err := c.Check(context.Background(), testUserID, bothSources)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if src != SourceLols {
		t.Fatalf("Check() = %q, want %q", src, SourceLols)
	}
	req := lols.lastRequest(t)
	if req.URL.Path != "/account" {
		t.Fatalf("lols path = %q, want /account", req.URL.Path)
	}
	if got := req.URL.Query().Get("id"); got != strconv.FormatInt(testUserID, 10) {
		t.Fatalf("lols id = %q, want %d", got, testUserID)
	}
	// The CAS key belongs to CAS; lols.bot needs no key and must not get it.
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("lols Authorization = %q, want empty", got)
	}
}

func TestCheckCleanInEverySelectedSourceIsNotListed(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	lols := newFakeService(t, http.StatusOK, lolsClean)
	c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return nil }})

	src, err := c.Check(context.Background(), testUserID, bothSources)
	if err != nil || src != "" {
		t.Fatalf("Check() = %q, %v; want not listed", src, err)
	}
	if cas.hits.Load() != 1 || lols.hits.Load() != 1 {
		t.Fatalf("hits CAS=%d lols=%d, want 1 each", cas.hits.Load(), lols.hits.Load())
	}
}

// A chat's choice of databases decides where a user ID is sent.
func TestCheckQueriesOnlySelectedSources(t *testing.T) {
	cases := []struct {
		name     string
		sources  []Source
		wantCAS  int32
		wantLols int32
	}{
		{"cas only", []Source{SourceCAS}, 1, 0},
		{"lols only", []Source{SourceLols}, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cas := newFakeService(t, http.StatusOK, casListed)
			lols := newFakeService(t, http.StatusOK, lolsBanned)
			c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return nil }})

			src, err := c.Check(context.Background(), testUserID, tc.sources)
			if err != nil || src != tc.sources[0] {
				t.Fatalf("Check() = %q, %v; want %q", src, err, tc.sources[0])
			}
			if cas.hits.Load() != tc.wantCAS || lols.hits.Load() != tc.wantLols {
				t.Fatalf("hits CAS=%d lols=%d, want %d/%d", cas.hits.Load(), lols.hits.Load(), tc.wantCAS, tc.wantLols)
			}
		})
	}
}

func TestCheckRequiresASource(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	lols := newFakeService(t, http.StatusOK, lolsBanned)
	c := newBothChecker(cas, lols, Options{})

	for _, sources := range [][]Source{nil, {}, {"unknown"}} {
		if src, err := c.Check(context.Background(), testUserID, sources); err == nil || src != "" {
			t.Fatalf("Check(%v) = %q, %v; want an error", sources, src, err)
		}
	}
	if cas.hits.Load() != 0 || lols.hits.Load() != 0 {
		t.Fatalf("hits CAS=%d lols=%d, want none", cas.hits.Load(), lols.hits.Load())
	}
}

// Anything but a documented lols.bot verdict is a failure, never "clean".
func TestCheckLolsFailureIsAnError(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"no input", http.StatusBadRequest, ``},
		{"invalid input", http.StatusInternalServerError, ``},
		{"ok false", http.StatusOK, `{"ok":false,"user_id":1234567890,"banned":false}`},
		{"no verdict", http.StatusOK, `{"ok":true,"user_id":1234567890}`},
		{"other user", http.StatusOK, `{"ok":true,"user_id":42,"banned":false}`},
		{"garbage", http.StatusOK, `<html>`},
		{"null", http.StatusOK, `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hook := captureLogs(t)
			cas := newFakeService(t, http.StatusOK, casNotFound)
			lols := newFakeService(t, tc.status, tc.body)
			c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return nil }})

			src, err := c.Check(context.Background(), testUserID, []Source{SourceLols})
			if err == nil || src != "" {
				t.Fatalf("Check() = %q, %v; want an error", src, err)
			}
			var warned bool
			for _, e := range hook.AllEntries() {
				warned = warned || (e.Level == logrus.WarnLevel && strings.Contains(e.Message, "unexpected lols.bot response"))
			}
			if !warned {
				t.Fatalf("no warning logged; entries = %v", hook.AllEntries())
			}

			// A failure is not cached: the next check asks again.
			if _, err := c.Check(context.Background(), testUserID, []Source{SourceLols}); err == nil {
				t.Fatal("second Check() error = nil, want the failure again")
			}
			if got := lols.hits.Load(); got != 2 {
				t.Fatalf("lols hit %d times, want 2", got)
			}
		})
	}
}

// The user is clean only when every selected database said so.
func TestCheckSourceFailureIsAnError(t *testing.T) {
	cases := []struct {
		name               string
		casStatus, lStatus int
		casBody, lBody     string
	}{
		{"lols down, cas clean", http.StatusOK, http.StatusInternalServerError, casNotFound, ``},
		{"cas down, lols clean", http.StatusBadGateway, http.StatusOK, `down`, lolsClean},
		{"both down", http.StatusBadGateway, http.StatusInternalServerError, `down`, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cas := newFakeService(t, tc.casStatus, tc.casBody)
			lols := newFakeService(t, tc.lStatus, tc.lBody)
			c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return nil }})

			if src, err := c.Check(context.Background(), testUserID, bothSources); err == nil || src != "" {
				t.Fatalf("Check() = %q, %v; want an error", src, err)
			}
		})
	}
}

func TestCheckListingWinsOverOtherSourceFailure(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	lols := newFakeService(t, http.StatusInternalServerError, ``)
	c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return nil }})

	src, err := c.Check(context.Background(), testUserID, bothSources)
	if err != nil || src != SourceCAS {
		t.Fatalf("Check() = %q, %v; want %q", src, err, SourceCAS)
	}
}

func TestCheckReturnsListingWithoutWaitingForSlowSource(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	cas.setDelay(slowerThanAny)
	lols := newFakeService(t, http.StatusOK, lolsBanned)
	c := newBothChecker(cas, lols, Options{Timeout: 2 * slowerThanAny, Redis: func() *redis.Client { return nil }})

	start := time.Now()
	src, err := c.Check(context.Background(), testUserID, bothSources)
	if elapsed := time.Since(start); elapsed > slowerThanAny/2 {
		t.Fatalf("Check() took %v; it waited for the slow source", elapsed)
	}
	if err != nil || src != SourceLols {
		t.Fatalf("Check() = %q, %v; want %q", src, err, SourceLols)
	}
}

// Verdicts are cached per database: a chat checking only CAS must not read a
// lols.bot verdict another chat cached, and the reverse.
func TestCheckCachesVerdictsPerSource(t *testing.T) {
	mr, client := newMiniredis(t)
	cas := newFakeService(t, http.StatusOK, casNotFound)
	lols := newFakeService(t, http.StatusOK, lolsBanned)
	c := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return client }})

	if src, err := c.Check(context.Background(), testUserID, []Source{SourceLols}); err != nil || src != SourceLols {
		t.Fatalf("lols-only Check() = %q, %v; want %q", src, err, SourceLols)
	}
	if src, err := c.Check(context.Background(), testUserID, []Source{SourceCAS}); err != nil || src != "" {
		t.Fatalf("CAS-only Check() = %q, %v; want not listed", src, err)
	}
	if src, err := c.Check(context.Background(), testUserID, bothSources); err != nil || src != SourceLols {
		t.Fatalf("both Check() = %q, %v; want %q from cache", src, err, SourceLols)
	}
	if cas.hits.Load() != 1 || lols.hits.Load() != 1 {
		t.Fatalf("hits CAS=%d lols=%d, want 1 each", cas.hits.Load(), lols.hits.Load())
	}

	id := strconv.FormatInt(testUserID, 10)
	waitForRedisKey(t, mr, "alita:spamdb:cas:"+id)
	waitForRedisKey(t, mr, "alita:spamdb:lols:"+id)
	if got, _ := mr.Get("alita:spamdb:lols:" + id); got != listedValue {
		t.Fatalf("lols cache value = %q, want %q", got, listedValue)
	}
	if got, _ := mr.Get("alita:spamdb:cas:" + id); got != cleanValue {
		t.Fatalf("CAS cache value = %q, want %q", got, cleanValue)
	}

	// A fresh replica sharing Redis reads each verdict without a lookup.
	other := newBothChecker(cas, lols, Options{Redis: func() *redis.Client { return client }})
	if src, err := other.Check(context.Background(), testUserID, []Source{SourceCAS}); err != nil || src != "" {
		t.Fatalf("other replica CAS-only Check() = %q, %v; want not listed", src, err)
	}
	if cas.hits.Load() != 1 || lols.hits.Load() != 1 {
		t.Fatalf("other replica hit CAS=%d lols=%d, want 1 each", cas.hits.Load(), lols.hits.Load())
	}
}

// The name reaches the log-channel notice, so it is the admin-facing label.
func TestSourceNames(t *testing.T) {
	want := map[Source]string{
		SourceCAS:    "CAS",
		SourceLols:   "lols.bot",
		SourceShared: "Shared bans",
	}
	for src, name := range want {
		if got := src.Name(); got != name {
			t.Errorf("%q.Name() = %q, want %q", src, got, name)
		}
	}
}

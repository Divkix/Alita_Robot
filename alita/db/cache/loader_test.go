//go:build testtools

package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestCacheLoadCancellationReleasesDatabaseWork(t *testing.T) {
	utilsCache.SetupTestMemoryMarshaler(t)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	stopped := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := GetFromCacheOrLoad(ctx, "cancelled-load", time.Minute, func(loadCtx context.Context) (string, error) {
			close(started)
			<-loadCtx.Done()
			close(stopped)
			return "", loadCtx.Err()
		})
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("load error = %v, want cancellation", err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("database work survived cancellation of its only caller")
	}
}

func TestDeleteCachePreventsInFlightLoaderFromRepopulatingStaleValue(t *testing.T) {
	utilsCache.SetupTestMemoryMarshaler(t)

	const key = "alita:test:loader-race"
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan string, 1)

	go func() {
		got, err := GetFromCacheOrLoad(context.Background(), key, time.Minute, func(ctx context.Context) (string, error) {
			close(started)
			<-release
			return "stale", nil
		})
		if err != nil {
			result <- "error: " + err.Error()
			return
		}
		result <- got
	}()

	<-started
	DeleteCache(key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fresh, err := GetFromCacheOrLoad(ctx, key, time.Minute, func(context.Context) (string, error) {
		return "fresh", nil
	})
	close(release)
	if err != nil || fresh != "fresh" {
		t.Fatalf("lookup after invalidation = %q, %v; want fresh value", fresh, err)
	}

	if got := <-result; got != "stale" {
		t.Fatalf("GetFromCacheOrLoad() = %q, want caller snapshot stale", got)
	}

	var cached string
	if _, err := utilsCache.GetMarshal().Get(utilsCache.Context, key, &cached); err != nil || cached != "fresh" {
		t.Fatalf("cache contains %q, error %v after invalidation raced with load", cached, err)
	}
}

func TestCacheLoadCancellationPreservesAnotherWaitingCaller(t *testing.T) {
	utilsCache.SetupTestMemoryMarshaler(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		_, err := GetFromCacheOrLoad(ctx, "shared-load", time.Minute, func(loadCtx context.Context) (string, error) {
			close(started)
			select {
			case <-release:
				return "shared result", nil
			case <-loadCtx.Done():
				return "", loadCtx.Err()
			}
		})
		first <- err
	}()
	<-started
	second := make(chan string, 1)
	go func() {
		value, err := GetFromCacheOrLoad(context.Background(), "shared-load", time.Minute, func(context.Context) (string, error) {
			return "duplicate lookup", nil
		})
		if err != nil {
			second <- err.Error()
			return
		}
		second <- value
	}()
	select {
	case result := <-second:
		t.Fatalf("second caller returned before the shared lookup completed: %q", result)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("first caller error = %v, want cancellation", err)
	}
	close(release)
	if result := <-second; result != "shared result" {
		t.Fatalf("remaining caller got %q, want shared result", result)
	}
}

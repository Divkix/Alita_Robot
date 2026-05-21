//go:build testtools

package cache


import (
	"sync"
	"testing"

	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
)

func TestGetMarshalSetMarshalRoundTrip(t *testing.T) {
	withMemoryMarshaler(t)

	const key = "alita:test:marshal"
	if err := GetMarshal().Set(Context, key, "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var got string
	if _, err := GetMarshal().Get(Context, key, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "value" {
		t.Fatalf("Get() = %q, want value", got)
	}
}

func TestGetMarshalSetMarshalConcurrentAccess(t *testing.T) {
	withMemoryMarshaler(t)

	manager := gocache.New[any](newCacheMemoryStore())
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()
			SetMarshal(marshaler.New(manager))
			if GetMarshal() == nil {
				t.Error("GetMarshal() = nil after SetMarshal")
			}
		}()
	}
	wg.Wait()

	if GetMarshal() == nil {
		t.Fatal("GetMarshal() = nil after concurrent SetMarshal calls")
	}
}

func TestInitTestMarshalSetsAndRestores(t *testing.T) {
	// Capture pre-call state
	before := GetMarshal()

	restore := InitTestMarshal()
	t.Cleanup(restore)
	if GetMarshal() == nil {
		t.Fatal("GetMarshal() = nil after InitTestMarshal")
	}

	restore()
	if GetMarshal() != before {
		t.Fatal("GetMarshal() did not restore original marshaler")
	}
}

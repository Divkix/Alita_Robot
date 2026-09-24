package updatememo

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type testKey struct{ id int }

func TestGetLoadsOncePerKey(t *testing.T) {
	memo := New()
	var loads int
	load := func() int {
		loads++
		return 7
	}

	for range 3 {
		if got := Get(memo, testKey{1}, load); got != 7 {
			t.Fatalf("Get() = %d, want 7", got)
		}
	}
	if loads != 1 {
		t.Fatalf("load calls = %d, want 1", loads)
	}
}

func TestGetLoadsDifferentKeysSeparately(t *testing.T) {
	memo := New()
	var loads int

	a := Get(memo, testKey{1}, func() bool { loads++; return true })
	b := Get(memo, testKey{2}, func() bool { loads++; return false })

	if !a || b {
		t.Fatalf("Get() = (%v, %v), want (true, false)", a, b)
	}
	if loads != 2 {
		t.Fatalf("load calls = %d, want 2", loads)
	}
}

func TestGetNilMemoLoadsEveryTime(t *testing.T) {
	var loads int
	for range 3 {
		Get(nil, testKey{1}, func() int { loads++; return loads })
	}
	if loads != 3 {
		t.Fatalf("load calls = %d, want 3", loads)
	}
}

func TestFromReturnsNilWithoutMemo(t *testing.T) {
	cases := map[string]*ext.Context{
		"nil ctx":     nil,
		"nil data":    {},
		"missing key": {Data: map[string]any{"other": 1}},
		"wrong type":  {Data: map[string]any{DataKey: "not a memo"}},
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			if got := From(ctx); got != nil {
				t.Fatalf("From() = %v, want nil", got)
			}
		})
	}
}

func TestFromReturnsStoredMemo(t *testing.T) {
	memo := New()
	ctx := &ext.Context{Data: map[string]any{DataKey: memo}}
	if got := From(ctx); got != memo {
		t.Fatalf("From() = %p, want %p", got, memo)
	}
}

func TestGetConcurrent(t *testing.T) {
	memo := New()
	var loads atomic.Int32
	var wg sync.WaitGroup

	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 50 {
				key := testKey{(i + j) % 4}
				if got := Get(memo, key, func() int { loads.Add(1); return key.id }); got != key.id {
					t.Errorf("Get(%v) = %d, want %d", key, got, key.id)
				}
			}
		}()
	}
	wg.Wait()

	if n := loads.Load(); n < 4 {
		t.Fatalf("load calls = %d, want at least one per key (4)", n)
	}
}

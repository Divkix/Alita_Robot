// Package updatememo holds values memoised for the lifetime of one update.
//
// It imports only the standard library and gotgbot so any package can use it
// without import cycles.
package updatememo

import (
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// DataKey is the ext.Context.Data key holding the per-update *Memo.
const DataKey = "updatememo"

// Memo caches lookups for a single update. Create it with New.
type Memo struct {
	mu sync.Mutex
	m  map[any]any
}

// New returns an empty memo.
func New() *Memo { return &Memo{m: make(map[any]any)} }

// From returns the update's memo, or nil when there is none (tests, callbacks built by hand).
func From(ctx *ext.Context) *Memo {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	memo, _ := ctx.Data[DataKey].(*Memo)
	return memo
}

// Get returns the memoised value for key, calling load at most once per memo.
// A nil memo calls load every time, so callers never need a nil check.
//
// The mutex is held only around map access, never while load runs, because
// load may do network I/O. Two concurrent loads of the same key may both run;
// the last one to finish wins.
func Get[T any](memo *Memo, key any, load func() T) T {
	if memo == nil {
		return load()
	}

	memo.mu.Lock()
	v, ok := memo.m[key]
	memo.mu.Unlock()
	if ok {
		if typed, ok := v.(T); ok {
			return typed
		}
	}

	loaded := load()

	memo.mu.Lock()
	memo.m[key] = loaded
	memo.mu.Unlock()
	return loaded
}

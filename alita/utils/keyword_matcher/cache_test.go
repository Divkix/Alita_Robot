package keyword_matcher

import (
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	t.Parallel()

	ttl := 5 * time.Minute
	c := NewCache(ttl)

	if c == nil {
		t.Fatal("NewCache() returned nil")
	}
	if c.ttl != ttl {
		t.Errorf("ttl: got %v, want %v", c.ttl, ttl)
	}
	if c.matchers == nil {
		t.Error("matchers map is nil, want initialized")
	}
	if len(c.matchers) != 0 {
		t.Errorf("matchers map is not empty, got %d entries", len(c.matchers))
	}
	if c.lastUsed == nil {
		t.Error("lastUsed map is nil, want initialized")
	}
	if len(c.lastUsed) != 0 {
		t.Errorf("lastUsed map is not empty, got %d entries", len(c.lastUsed))
	}
}

func TestGetOrCreateMatcher(t *testing.T) {
	t.Parallel()

	t.Run("new chatID creates matcher", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		m := c.GetOrCreateMatcher(100, []string{"hello", "world"})
		if m == nil {
			t.Fatal("GetOrCreateMatcher() returned nil for new chatID")
		}
	})

	t.Run("same chatID and patterns returns cached pointer", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		patterns := []string{"foo", "bar"}
		m1 := c.GetOrCreateMatcher(200, patterns)
		m2 := c.GetOrCreateMatcher(200, patterns)
		if m1 != m2 {
			t.Error("expected same pointer for same chatID and patterns, got different instances")
		}
	})

	t.Run("same chatID with different patterns creates new matcher", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		m1 := c.GetOrCreateMatcher(300, []string{"alpha"})
		m2 := c.GetOrCreateMatcher(300, []string{"beta"})
		if m1 == m2 {
			t.Error("expected different pointer for changed patterns, got same instance")
		}
	})

	t.Run("empty patterns slice", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		m := c.GetOrCreateMatcher(400, []string{})
		if m == nil {
			t.Fatal("GetOrCreateMatcher() returned nil for empty patterns")
		}
	})

	t.Run("concurrent 10 goroutines", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		patterns := []string{"concurrent", "test"}

		var wg sync.WaitGroup
		const goroutines = 10
		wg.Add(goroutines)

		matchers := make([]*KeywordMatcher, goroutines)
		for i := 0; i < goroutines; i++ {
			i := i
			go func() {
				defer wg.Done()
				matchers[i] = c.GetOrCreateMatcher(500, patterns)
			}()
		}

		wg.Wait()

		// All should have received a non-nil matcher
		for i, m := range matchers {
			if m == nil {
				t.Errorf("goroutine %d got nil matcher", i)
			}
		}
	})
}

func TestCleanupExpired(t *testing.T) {
	t.Parallel()

	t.Run("expired entries removed with 1ms TTL after 5ms sleep", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Millisecond)
		c.GetOrCreateMatcher(1001, []string{"pattern"})

		// Wait for TTL to expire
		time.Sleep(5 * time.Millisecond)

		c.CleanupExpired()

		c.mu.RLock()
		_, exists := c.matchers[1001]
		c.mu.RUnlock()

		if exists {
			t.Error("expected expired entry to be removed, but it still exists")
		}
	})

	t.Run("unexpired entries kept with 1h TTL", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Hour)
		c.GetOrCreateMatcher(1002, []string{"pattern"})

		c.CleanupExpired()

		c.mu.RLock()
		_, exists := c.matchers[1002]
		c.mu.RUnlock()

		if !exists {
			t.Error("expected unexpired entry to remain, but it was removed")
		}
	})

	t.Run("empty cache cleanup is a no-op", func(t *testing.T) {
		t.Parallel()

		c := NewCache(time.Minute)
		// Should not panic or have side effects on empty cache
		c.CleanupExpired()

		c.mu.RLock()
		count := len(c.matchers)
		c.mu.RUnlock()

		if count != 0 {
			t.Errorf("expected 0 entries after cleanup of empty cache, got %d", count)
		}
	})

	t.Run("zero TTL — all entries expired immediately", func(t *testing.T) {
		t.Parallel()

		c := NewCache(0)
		c.GetOrCreateMatcher(1003, []string{"pattern"})

		// With zero TTL, any elapsed time > 0 means expired.
		// Sleep a tiny bit to ensure time.Now().Sub(lastUsed) > 0.
		time.Sleep(time.Millisecond)

		c.CleanupExpired()

		c.mu.RLock()
		_, exists := c.matchers[1003]
		c.mu.RUnlock()

		if exists {
			t.Error("expected zero-TTL entry to be removed after cleanup")
		}
	})
}

func TestPatternsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "identical slices same order",
			a:    []string{"a", "b"},
			b:    []string{"a", "b"},
			want: true,
		},
		{
			name: "same elements different order",
			a:    []string{"b", "a"},
			b:    []string{"a", "b"},
			want: true,
		},
		{
			name: "different lengths",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b"},
			want: false,
		},
		{
			name: "nil vs nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "different content same length",
			a:    []string{"x", "y"},
			b:    []string{"x", "z"},
			want: false,
		},
		{
			name: "duplicates in a vs unique in b — different logical sets",
			a:    []string{"a", "a"},
			b:    []string{"a"},
			want: false,
		},
		{
			name: "empty slices",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "one nil one empty",
			a:    nil,
			b:    []string{},
			want: true,
		},
		{
			name: "one empty one non-empty",
			a:    []string{},
			b:    []string{"a"},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := patternsEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("patternsEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

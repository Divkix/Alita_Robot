package modules

import (
	"slices"
	"testing"
)

func TestGetLockMapAsArray(t *testing.T) {
	t.Parallel()

	var m moduleStruct
	got := m.getLockMapAsArray()

	// Compute expected keys from lockMap + restrMap
	expected := make(map[string]struct{}, len(lockMap)+len(restrMap))
	for k := range lockMap {
		expected[k] = struct{}{}
	}
	for k := range restrMap {
		expected[k] = struct{}{}
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d lock types, got %d", len(expected), len(got))
	}

	// Ensure sorted
	if !slices.IsSorted(got) {
		t.Fatalf("expected sorted slice, got %v", got)
	}

	// Ensure every expected key is present
	for k := range expected {
		if !slices.Contains(got, k) {
			t.Fatalf("expected lock type %q missing from %v", k, got)
		}
	}
}

func TestGetLockMapAsArrayCachesResult(t *testing.T) {
	// Since sync.Once caches the result, both calls should return identical slices.
	var m moduleStruct
	first := m.getLockMapAsArray()
	second := m.getLockMapAsArray()

	if len(first) == 0 {
		t.Fatal("expected non-empty lock types")
	}

	if !slices.Equal(first, second) {
		t.Fatalf("cached result should be identical: first=%v second=%v", first, second)
	}
}

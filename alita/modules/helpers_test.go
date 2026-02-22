package modules

import (
	"slices"
	"testing"
)

func TestModuleEnabled_StoreAndLoad(t *testing.T) {
	t.Parallel()

	var m moduleEnabled
	m.Init()

	// Store true and load
	m.Store("admin", true)
	_, got := m.Load("admin")
	if !got {
		t.Fatalf("Load(\"admin\") = false, want true after Store(\"admin\", true)")
	}

	// Load non-existent key returns false
	_, got = m.Load("nonexistent")
	if got {
		t.Fatalf("Load(\"nonexistent\") = true, want false")
	}

	// Overwrite with false
	m.Store("admin", false)
	_, got = m.Load("admin")
	if got {
		t.Fatalf("Load(\"admin\") = true, want false after Store(\"admin\", false)")
	}

	// Empty string key
	m.Store("", true)
	_, got = m.Load("")
	if !got {
		t.Fatalf("Load(\"\") = false, want true after Store(\"\", true)")
	}
}

func TestModuleEnabled_LoadModules(t *testing.T) {
	t.Parallel()

	t.Run("no stores returns empty slice", func(t *testing.T) {
		t.Parallel()

		var m moduleEnabled
		m.Init()

		result := m.LoadModules()
		if len(result) != 0 {
			t.Fatalf("LoadModules() with no stores = %v (len %d), want empty slice", result, len(result))
		}
	})

	t.Run("enabled modules returned, disabled excluded", func(t *testing.T) {
		t.Parallel()

		var m moduleEnabled
		m.Init()
		m.Store("a", true)
		m.Store("b", true)
		m.Store("c", false)

		result := m.LoadModules()
		if len(result) != 2 {
			t.Fatalf("LoadModules() = %v (len %d), want 2 elements", result, len(result))
		}
		if !slices.Contains(result, "a") {
			t.Fatalf("LoadModules() = %v, want to contain \"a\"", result)
		}
		if !slices.Contains(result, "b") {
			t.Fatalf("LoadModules() = %v, want to contain \"b\"", result)
		}
		if slices.Contains(result, "c") {
			t.Fatalf("LoadModules() = %v, must not contain \"c\" (disabled)", result)
		}
	})
}

// TestListModules modifies package-level HelpModule state -- do NOT use t.Parallel().
func TestListModules(t *testing.T) {
	// Re-initialize HelpModule.AbleMap before and after to avoid contaminating other tests.
	t.Cleanup(func() {
		HelpModule.AbleMap.Init()
	})

	HelpModule.AbleMap.Init()
	HelpModule.AbleMap.Store("admin", true)
	HelpModule.AbleMap.Store("filters", true)
	HelpModule.AbleMap.Store("help", true)

	result := listModules()

	if len(result) != 3 {
		t.Fatalf("listModules() = %v (len %d), want 3 elements", result, len(result))
	}

	// Result must be sorted alphabetically.
	expected := []string{"admin", "filters", "help"}
	for i, name := range expected {
		if result[i] != name {
			t.Fatalf("listModules()[%d] = %q, want %q (not sorted); full result: %v", i, result[i], name, result)
		}
	}
}

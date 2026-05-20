package cache

import "testing"

func TestAdminCacheHelpersHandleNilMarshal(t *testing.T) {
	originalMarshal := Marshal
	Marshal = nil
	t.Cleanup(func() {
		Marshal = originalMarshal
	})

	found, adminCache := GetAdminCacheList(-100123)
	if found {
		t.Fatalf("GetAdminCacheList() found = true, cache = %+v", adminCache)
	}

	found, member := GetAdminCacheUser(-100123, 42)
	if found {
		t.Fatalf("GetAdminCacheUser() found = true, member = %+v", member)
	}

	InvalidateAdminCache(-100123)
}

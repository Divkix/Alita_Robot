package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestRecentJoinProcessingNoCacheFallback(t *testing.T) {
	originalMarshal := cache.Marshal
	cache.Marshal = nil
	t.Cleanup(func() {
		cache.Marshal = originalMarshal
		clearRecentJoinProcessing(-100123, 456)
	})

	key := recentJoinProcessingKey(-100123, 456)
	if key != "alita:recentJoinProcessing:-100123:456" {
		t.Fatalf("recentJoinProcessingKey() = %q", key)
	}

	if !claimRecentJoinProcessing(-100123, 456) {
		t.Fatal("first claimRecentJoinProcessing() = false, want true")
	}
	if claimRecentJoinProcessing(-100123, 456) {
		t.Fatal("second claimRecentJoinProcessing() = true, want false duplicate")
	}

	clearRecentJoinProcessing(-100123, 456)
	if !claimRecentJoinProcessing(-100123, 456) {
		t.Fatal("claim after clear = false, want true")
	}
}

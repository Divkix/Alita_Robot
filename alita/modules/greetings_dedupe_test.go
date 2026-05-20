package modules

import (
	"fmt"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestRecentJoinProcessingNoCacheFallback(t *testing.T) {
	withNilCacheMarshal(t)
	t.Cleanup(func() {
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

func TestPendingJoinsCacheNilMarshal(t *testing.T) {
	withNilCacheMarshal(t)

	if greetingsModule.loadPendingJoins(-100123, 456) {
		t.Fatal("loadPendingJoins(nil marshal) = true, want false")
	}
	greetingsModule.setPendingJoins(-100123, 456)
}

func TestPendingJoinsCacheRoundTrip(t *testing.T) {
	if cache.GetMarshal() == nil {
		t.Skip("requires cache marshal")
	}

	chatID := uniqueModuleChatID()
	const userID int64 = 456789
	key := fmt.Sprintf("alita:pendingJoins:%d:%d", chatID, userID)
	t.Cleanup(func() {
		_ = cache.GetMarshal().Delete(cache.Context, key)
	})

	greetingsModule.setPendingJoins(chatID, userID)
	if !greetingsModule.loadPendingJoins(chatID, userID) {
		t.Fatal("loadPendingJoins() = false after setPendingJoins, want true")
	}
}

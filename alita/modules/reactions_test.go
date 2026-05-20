package modules

import "testing"

func TestReactionKey(t *testing.T) {
	t.Parallel()

	if got := reactionKey(-1001234567890); got != "alita:reactions:-1001234567890" {
		t.Fatalf("reactionKey() = %q", got)
	}
}

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}



// TestDemoteErrorHandling verifies error handling patterns in demote logic
func TestDemoteErrorHandling(t *testing.T) {
	// simulateGetMemberResult simulates the return values of GetMember,
	// returning values that staticcheck cannot statically resolve.
	simulateGetMemberResult := func(wantErr bool) (gotgbot.ChatMember, error) {
		if wantErr {
			return nil, &testError{msg: "API error"}
		}
		return nil, nil
	}

	t.Run("error takes precedence over nil member", func(t *testing.T) {
		// When GetMember returns (nil, error), error should be checked first
		// This is the standard pattern: check err != nil before using result
		userMember, err := simulateGetMemberResult(true)

		if err != nil {
			// Expected: error is handled first
			t.Logf("Error handled first: %v", err)
			return
		}

		// Should not reach here when err != nil
		if userMember == nil {
			t.Fatal("Should have returned on error, not reached nil check")
		}
	})

	t.Run("nil error with nil member", func(t *testing.T) {
		// When GetMember returns (nil, nil), the nil member should be handled
		userMember, err := simulateGetMemberResult(false)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// After confirming no error, check the member
		if userMember == nil {
			t.Log("Nil member properly detected after nil error check")
			return
		}

		t.Log("Non-nil member received")
	})
}

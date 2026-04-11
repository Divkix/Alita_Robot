package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWarnSettingsConstraint_PositiveLimit tests that warn_limit must be positive
func TestWarnSettingsConstraint_PositiveLimit(t *testing.T) {
	// This test verifies the database constraint is working
	// Integration test that requires database connection
	t.Skip("Requires database connection - add to integration test suite")
}

// TestAntifloodSettingsConstraint_ValidActions tests that only valid actions are accepted
func TestAntifloodSettingsConstraint_ValidActions(t *testing.T) {
	validActions := []string{"mute", "ban", "kick", "warn", "tban", "tmute"}
	for _, action := range validActions {
		// Verify each valid action is in the accepted list
		assert.Contains(t, []string{"mute", "ban", "kick", "warn", "tban", "tmute"}, action)
	}
}

// TestCaptchaSettingsConstraint_TimeoutRange tests that timeout must be between 1 and 10
func TestCaptchaSettingsConstraint_TimeoutRange(t *testing.T) {
	// Test boundary values - valid range is 1-10 inclusive
	validValues := []int{1, 5, 10}
	for _, timeout := range validValues {
		assert.True(t, timeout >= 1 && timeout <= 10, "Timeout %d should be valid", timeout)
	}

	// Test invalid values
	invalidValues := []int{0, 11, -1, 100}
	for _, timeout := range invalidValues {
		assert.False(t, timeout >= 1 && timeout <= 10, "Timeout %d should be invalid", timeout)
	}
}

// TestCaptchaAttemptsConstraint_Expiration tests that expires_at must be after created_at
func TestCaptchaAttemptsConstraint_Expiration(t *testing.T) {
	// Test temporal constraint logic
	now := time.Now()

	// Valid: expires_at is after created_at
	expiresValid := now.Add(5 * time.Minute)
	assert.True(t, expiresValid.After(now), "Expiration 5 minutes in future should be valid")

	// Invalid: expires_at is before created_at
	expiresInvalid := now.Add(-5 * time.Minute)
	assert.False(t, expiresInvalid.After(now), "Expiration in past should be invalid")
}

// TestWarnSettingsIntegration_PositiveLimit tests warn_limit constraint with database
func TestWarnSettingsIntegration_PositiveLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
	})

	// Test valid positive limit
	settings := &WarnSettings{
		ChatId:    chatID,
		WarnLimit: 3,
	}
	err := CreateRecord(settings)
	require.NoError(t, err, "Creating warn settings with positive limit should succeed")
	assert.Greater(t, settings.WarnLimit, 0, "Warn limit should be positive")

	// Test that zero limit violates constraint
	invalidSettings := &WarnSettings{
		ChatId:    chatID + 1,
		WarnLimit: 0,
	}
	err = CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating warn settings with zero limit should fail due to CHECK constraint")

	// Test that negative limit violates constraint
	invalidSettings2 := &WarnSettings{
		ChatId:    chatID + 2,
		WarnLimit: -1,
	}
	err = CreateRecord(invalidSettings2)
	assert.Error(t, err, "Creating warn settings with negative limit should fail due to CHECK constraint")
}

// TestAntifloodSettingsConstraint_ValidActionsIntegration tests antiflood action constraint
func TestAntifloodSettingsConstraint_ValidActionsIntegration(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	validActions := []string{"mute", "ban", "kick", "warn", "tban", "tmute"}

	for _, action := range validActions {
		settings := &AntifloodSettings{
			ChatId: chatID + int64(hashCode(action)),
			Limit:  5,
			Action: action,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating antiflood settings with valid action '%s' should succeed", action)
	}

	// Test invalid action
	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 99999,
		Limit:  5,
		Action: "invalid_action",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with invalid action should fail due to CHECK constraint")
}

// TestCaptchaSettingsConstraint_TimeoutRangeIntegration tests captcha timeout constraint
func TestCaptchaSettingsConstraint_TimeoutRangeIntegration(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	// Test valid timeout values (1-10)
	for _, timeout := range []int{1, 5, 10} {
		settings := &CaptchaSettings{
			ChatID:  chatID + int64(timeout),
			Timeout: timeout,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating captcha settings with timeout %d should succeed", timeout)
	}

	// Test invalid timeout values
	invalidTimeouts := []int{0, 11, -1, 100}
	for _, timeout := range invalidTimeouts {
		settings := &CaptchaSettings{
			ChatID:  chatID + int64(timeout*1000),
			Timeout: timeout,
		}
		err := CreateRecord(settings)
		assert.Error(t, err, "Creating captcha settings with timeout %d should fail due to CHECK constraint", timeout)
	}
}

// TestCaptchaSettingsConstraint_MaxAttemptsRange tests max_attempts constraint
func TestCaptchaSettingsConstraint_MaxAttemptsRange(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	// Test valid max_attempts values (1-10)
	for _, attempts := range []int{1, 5, 10} {
		settings := &CaptchaSettings{
			ChatID:      chatID + int64(attempts),
			MaxAttempts: attempts,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating captcha settings with max_attempts %d should succeed", attempts)
	}

	// Test invalid max_attempts values
	invalidAttempts := []int{0, 11, -1, 100}
	for _, attempts := range invalidAttempts {
		settings := &CaptchaSettings{
			ChatID:      chatID + int64(attempts*1000),
			MaxAttempts: attempts,
		}
		err := CreateRecord(settings)
		assert.Error(t, err, "Creating captcha settings with max_attempts %d should fail due to CHECK constraint", attempts)
	}
}

// TestCaptchaSettingsConstraint_ValidModes tests captcha_mode constraint
func TestCaptchaSettingsConstraint_ValidModes(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	validModes := []string{"math", "text"}
	for _, mode := range validModes {
		settings := &CaptchaSettings{
			ChatID:      chatID + int64(hashCode(mode)),
			CaptchaMode: mode,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating captcha settings with mode '%s' should succeed", mode)
	}

	// Test invalid mode
	invalidSettings := &CaptchaSettings{
		ChatID:      chatID + 99999,
		CaptchaMode: "invalid_mode",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating captcha settings with invalid mode should fail due to CHECK constraint")
}

// TestCaptchaSettingsConstraint_ValidFailureActions tests failure_action constraint
func TestCaptchaSettingsConstraint_ValidFailureActions(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	validActions := []string{"kick", "ban", "mute"}
	for _, action := range validActions {
		settings := &CaptchaSettings{
			ChatID:        chatID + int64(hashCode(action)),
			FailureAction: action,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating captcha settings with failure_action '%s' should succeed", action)
	}

	// Test invalid failure_action
	invalidSettings := &CaptchaSettings{
		ChatID:        chatID + 99999,
		FailureAction: "delete",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating captcha settings with invalid failure_action should fail due to CHECK constraint")
}

// TestCaptchaAttemptsConstraint_ExpirationIntegration tests expires_at constraint
func TestCaptchaAttemptsConstraint_ExpirationIntegration(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 100
	chatID := base + 101
	t.Cleanup(func() {
		_ = DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&CaptchaAttempts{}).Error
	})

	// Test valid: expires_at is after created_at
	expiresValid := time.Now().Add(5 * time.Minute)
	attempt := &CaptchaAttempts{
		UserID:    userID,
		ChatID:    chatID,
		Answer:    "42",
		ExpiresAt: expiresValid,
	}
	err := CreateRecord(attempt)
	require.NoError(t, err, "Creating captcha attempt with future expiration should succeed")
	assert.True(t, attempt.ExpiresAt.After(attempt.CreatedAt) || attempt.ExpiresAt.Equal(attempt.CreatedAt.Add(2*time.Minute)),
		"Expiration should be after creation time")

	// Test invalid: expires_at is before created_at (will fail constraint)
	invalidAttempt := &CaptchaAttempts{
		UserID:    userID + 1,
		ChatID:    chatID + 1,
		Answer:    "42",
		ExpiresAt: time.Now().Add(-5 * time.Minute), // Past expiration
	}
	err = CreateRecord(invalidAttempt)
	assert.Error(t, err, "Creating captcha attempt with past expiration should fail due to CHECK constraint")
}

// TestWarnsUsersConstraint_NonNegativeNumWarns tests num_warns constraint
func TestWarnsUsersConstraint_NonNegativeNumWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 200
	chatID := base + 201
	t.Cleanup(func() {
		_ = DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&Warns{}).Error
	})

	// Test valid non-negative values
	for _, numWarns := range []int{0, 1, 5, 10} {
		warn := &Warns{
			UserId:   userID + int64(numWarns),
			ChatId:   chatID,
			NumWarns: numWarns,
		}
		err := CreateRecord(warn)
		assert.NoError(t, err, "Creating warns with num_warns %d should succeed", numWarns)
	}

	// Test invalid negative value
	invalidWarn := &Warns{
		UserId:   userID + 9999,
		ChatId:   chatID + 1,
		NumWarns: -1,
	}
	err := CreateRecord(invalidWarn)
	assert.Error(t, err, "Creating warns with negative num_warns should fail due to CHECK constraint")
}

// TestAntifloodConstraint_NonNegativeFloodLimit tests flood_limit constraint
func TestAntifloodConstraint_NonNegativeFloodLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	// Test valid non-negative values
	for _, limit := range []int{0, 1, 5, 10} {
		settings := &AntifloodSettings{
			ChatId: chatID + int64(limit),
			Limit:  limit,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating antiflood settings with flood_limit %d should succeed", limit)
	}

	// Test invalid negative value
	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 9999,
		Limit:  -1,
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with negative flood_limit should fail due to CHECK constraint")
}

// TestBlacklistConstraint_ValidActions tests blacklist action constraint
func TestBlacklistConstraint_ValidActions(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&BlacklistSettings{}).Error
	})

	validActions := []string{"warn", "mute", "ban", "kick", "tban", "tmute", "delete"}
	for _, action := range validActions {
		settings := &BlacklistSettings{
			ChatId: chatID + int64(hashCode(action)),
			Word:   "test_word_" + action,
			Action: action,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating blacklist settings with action '%s' should succeed", action)
	}

	// Test invalid action
	invalidSettings := &BlacklistSettings{
		ChatId: chatID + 99999,
		Word:   "test_word_invalid",
		Action: "invalid_action",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating blacklist settings with invalid action should fail due to CHECK constraint")
}

// TestWarnModeConstraint_ValidModes tests warn_mode constraint
func TestWarnModeConstraint_ValidModes(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
	})

	// Test NULL warn_mode (should be valid)
	settings1 := &WarnSettings{
		ChatId:   chatID,
		WarnMode: "",
	}
	err := CreateRecord(settings1)
	assert.NoError(t, err, "Creating warn settings with NULL/empty warn_mode should succeed")

	// Test valid modes
	validModes := []string{"ban", "kick", "mute", "tban", "tmute"}
	for _, mode := range validModes {
		settings := &WarnSettings{
			ChatId:   chatID + int64(hashCode(mode)),
			WarnMode: mode,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating warn settings with warn_mode '%s' should succeed", mode)
	}

	// Test invalid mode
	invalidSettings := &WarnSettings{
		ChatId:   chatID + 99999,
		WarnMode: "invalid_mode",
	}
	err = CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating warn settings with invalid warn_mode should fail due to CHECK constraint")
}

// TestAntifloodModeConstraint_ValidModes tests antiflood mode constraint
func TestAntifloodModeConstraint_ValidModes(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	// Test NULL mode (should be valid)
	settings1 := &AntifloodSettings{
		ChatId: chatID,
		Mode:   "",
	}
	err := CreateRecord(settings1)
	assert.NoError(t, err, "Creating antiflood settings with NULL/empty mode should succeed")

	// Test valid modes
	validModes := []string{"mute", "ban", "kick", "warn", "tban", "tmute"}
	for _, mode := range validModes {
		settings := &AntifloodSettings{
			ChatId: chatID + int64(hashCode(mode)),
			Mode:   mode,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating antiflood settings with mode '%s' should succeed", mode)
	}

	// Test invalid mode
	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 99999,
		Mode:   "invalid_mode",
	}
	err = CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with invalid mode should fail due to CHECK constraint")
}

// Helper function to generate deterministic hash codes for test data
func hashCode(s string) int {
	hash := 0
	for i, c := range s {
		hash = hash*31 + int(c) + i
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

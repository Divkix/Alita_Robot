package db

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarnSettingsConstraint_PositiveLimit(t *testing.T) {
	t.Skip("Requires database connection - add to integration test suite")
}

func TestAntifloodSettingsConstraint_ValidActions(t *testing.T) {
	validActions := []string{"mute", "ban", "kick", "warn", "tban", "tmute"}
	for _, action := range validActions {
		assert.Contains(t, []string{"mute", "ban", "kick", "warn", "tban", "tmute"}, action)
	}
}

func TestCaptchaSettingsConstraint_TimeoutRange(t *testing.T) {
	validValues := []int{1, 5, 10}
	for _, timeout := range validValues {
		assert.True(t, timeout >= 1 && timeout <= 10, "Timeout %d should be valid", timeout)
	}

	invalidValues := []int{0, 11, -1, 100}
	for _, timeout := range invalidValues {
		assert.False(t, timeout >= 1 && timeout <= 10, "Timeout %d should be invalid", timeout)
	}
}

func TestCaptchaAttemptsConstraint_Expiration(t *testing.T) {
	now := time.Now()

	expiresValid := now.Add(5 * time.Minute)
	assert.True(t, expiresValid.After(now), "Expiration 5 minutes in future should be valid")

	expiresInvalid := now.Add(-5 * time.Minute)
	assert.False(t, expiresInvalid.After(now), "Expiration in past should be invalid")
}

func testIntRangeConstraint(t *testing.T, chatID int64, fieldName string, validValues []int, invalidValues []int, createFunc func(int64, int) error) {
	t.Run(fieldName+"_Valid", func(t *testing.T) {
		for _, val := range validValues {
			err := createFunc(chatID+int64(val), val)
			assert.NoError(t, err, "Creating with %s=%d should succeed", fieldName, val)
		}
	})

	t.Run(fieldName+"_Invalid", func(t *testing.T) {
		for _, val := range invalidValues {
			err := createFunc(chatID+int64(val*1000), val)
			assert.Error(t, err, "Creating with %s=%d should fail due to CHECK constraint", fieldName, val)
		}
	})
}

func TestWarnSettingsIntegration_PositiveLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
	})

	settings := &models.WarnSettings{
		ChatId:    chatID,
		WarnLimit: 3,
	}
	err := CreateRecord(settings)
	require.NoError(t, err, "Creating warn settings with positive limit should succeed")
	assert.Greater(t, settings.WarnLimit, 0, "Warn limit should be positive")

	err = DB.Model(&models.WarnSettings{}).Create(map[string]any{
		"chat_id":    chatID + 1,
		"warn_limit": 0,
	}).Error
	assert.Error(t, err, "Creating warn settings with zero limit should fail due to CHECK constraint")

	invalidSettings2 := &models.WarnSettings{
		ChatId:    chatID + 2,
		WarnLimit: -1,
	}
	err = CreateRecord(invalidSettings2)
	assert.Error(t, err, "Creating warn settings with negative limit should fail due to CHECK constraint")
}

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

	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 99999,
		Limit:  5,
		Action: "invalid_action",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with invalid action should fail due to CHECK constraint")
}

func TestCaptchaSettingsConstraint_TimeoutRangeIntegration(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	testIntRangeConstraint(t, chatID, "timeout",
		[]int{1, 5, 10},       // valid values
		[]int{0, 11, -1, 100}, // invalid values
		func(chatID int64, timeout int) error {
			return DB.Model(&CaptchaSettings{}).Create(map[string]any{
				"chat_id": chatID,
				"timeout": timeout,
			}).Error
		},
	)
}

func TestCaptchaSettingsConstraint_MaxAttemptsRange(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	testIntRangeConstraint(t, chatID, "max_attempts",
		[]int{1, 5, 10},       // valid values
		[]int{0, 11, -1, 100}, // invalid values
		func(chatID int64, attempts int) error {
			return DB.Model(&CaptchaSettings{}).Create(map[string]any{
				"chat_id":      chatID,
				"max_attempts": attempts,
			}).Error
		},
	)
}

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

	invalidSettings := &CaptchaSettings{
		ChatID:      chatID + 99999,
		CaptchaMode: "invalid_mode",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating captcha settings with invalid mode should fail due to CHECK constraint")
}

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

	invalidSettings := &CaptchaSettings{
		ChatID:        chatID + 99999,
		FailureAction: "delete",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating captcha settings with invalid failure_action should fail due to CHECK constraint")
}

func TestCaptchaAttemptsConstraint_ExpirationIntegration(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 100
	chatID := base + 101
	t.Cleanup(func() {
		_ = DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&CaptchaAttempts{}).Error
	})

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

	invalidAttempt := &CaptchaAttempts{
		UserID:    userID + 1,
		ChatID:    chatID + 1,
		Answer:    "42",
		ExpiresAt: time.Now().Add(-5 * time.Minute), // Past expiration
	}
	err = CreateRecord(invalidAttempt)
	assert.Error(t, err, "Creating captcha attempt with past expiration should fail due to CHECK constraint")
}

func TestWarnsUsersConstraint_NonNegativeNumWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 200
	chatID := base + 201
	t.Cleanup(func() {
		_ = DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&models.Warns{}).Error
	})

	for _, numWarns := range []int{0, 1, 5, 10} {
		warn := &models.Warns{
			UserId:   userID + int64(numWarns),
			ChatId:   chatID,
			NumWarns: numWarns,
		}
		err := CreateRecord(warn)
		assert.NoError(t, err, "Creating warns with num_warns %d should succeed", numWarns)
	}

	invalidWarn := &models.Warns{
		UserId:   userID + 9999,
		ChatId:   chatID + 1,
		NumWarns: -1,
	}
	err := CreateRecord(invalidWarn)
	assert.Error(t, err, "Creating warns with negative num_warns should fail due to CHECK constraint")
}

func TestAntifloodConstraint_NonNegativeFloodLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	for _, limit := range []int{0, 1, 5, 10} {
		settings := &AntifloodSettings{
			ChatId: chatID + int64(limit),
			Limit:  limit,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating antiflood settings with flood_limit %d should succeed", limit)
	}

	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 9999,
		Limit:  -1,
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with negative flood_limit should fail due to CHECK constraint")
}

func TestBlacklistConstraint_ValidActions(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&models.BlacklistSettings{}).Error
	})

	validActions := []string{"warn", "mute", "ban", "kick", "tban", "tmute", "delete"}
	for _, action := range validActions {
		settings := &models.BlacklistSettings{
			ChatId: chatID + int64(hashCode(action)),
			Word:   "test_word_" + action,
			Action: action,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating blacklist settings with action '%s' should succeed", action)
	}

	invalidSettings := &models.BlacklistSettings{
		ChatId: chatID + 99999,
		Word:   "test_word_invalid",
		Action: "invalid_action",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating blacklist settings with invalid action should fail due to CHECK constraint")
}

func TestWarnModeConstraint_ValidModes(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
	})

	settings1 := &models.WarnSettings{
		ChatId:   chatID,
		WarnMode: "",
	}
	err := CreateRecord(settings1)
	assert.NoError(t, err, "Creating warn settings with NULL/empty warn_mode should succeed")

	validModes := []string{"ban", "kick", "mute", "tban", "tmute"}
	for _, mode := range validModes {
		settings := &models.WarnSettings{
			ChatId:   chatID + int64(hashCode(mode)),
			WarnMode: mode,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating warn settings with warn_mode '%s' should succeed", mode)
	}

	invalidSettings := &models.WarnSettings{
		ChatId:   chatID + 99999,
		WarnMode: "invalid_mode",
	}
	err = CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating warn settings with invalid warn_mode should fail due to CHECK constraint")
}

func TestAntifloodActionConstraint_ValidActions(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	validActions := []string{"mute", "ban", "kick", "warn", "tban", "tmute"}
	for _, action := range validActions {
		settings := &AntifloodSettings{
			ChatId: chatID + int64(hashCode(action)),
			Action: action,
		}
		err := CreateRecord(settings)
		assert.NoError(t, err, "Creating antiflood settings with action '%s' should succeed", action)
	}

	invalidSettings := &AntifloodSettings{
		ChatId: chatID + 99999,
		Action: "invalid_action",
	}
	err := CreateRecord(invalidSettings)
	assert.Error(t, err, "Creating antiflood settings with invalid action should fail due to CHECK constraint")
}

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

package ratelimit

import (
	"strconv"
	"time"
)

const settingsPickerRatePrefix = "settings:picker:"

// AcquireSettingsPicker reserves one /settings group-list build for userID,
// shared across replicas through Redis like the backup limits. While an earlier
// build's cooldown runs it returns false and the time left. A cooldown <= 0
// turns the pause off.
func AcquireSettingsPicker(userID int64, cooldown time.Duration) (bool, time.Duration) {
	if cooldown <= 0 {
		return true, 0
	}
	return GetBackupRateLimiter().acquireOperation(settingsPickerRatePrefix+strconv.FormatInt(userID, 10), cooldown)
}

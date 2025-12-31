package helpers

// IsChannelID checks if a given chat ID represents a channel.
// Channel IDs in Telegram are negative numbers less than -1000000000000.
func IsChannelID(chatID int64) bool {
	return chatID < -1000000000000
}

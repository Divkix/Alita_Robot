package constants

import "time"

const (
	AdminCacheTTL           = 30 * time.Minute
	RestrictedCacheTTL      = 30 * time.Minute
	RestrictedProbeInterval = 5 * time.Minute
	ShortCacheTTL           = 1 * time.Minute

	// UserUpdateInterval and ChatUpdateInterval are both the in-process
	// throttle in modules/users.go and the last_activity touch window in the
	// users/chats upserts. One value keeps the throttle from letting through
	// writes that the upsert's WHERE then discards. Renames bypass both.
	UserUpdateInterval    = time.Hour
	ChatUpdateInterval    = time.Hour
	ChannelUpdateInterval = time.Hour

	DefaultTimeout  = 10 * time.Second
	ShortTimeout    = 5 * time.Second
	LongTimeout     = 30 * time.Second
	VeryLongTimeout = 120 * time.Second

	DefaultHTTPPort         = 8080
	MaxIdleConnsExtraBuffer = 20
)

package constants

import "time"

// Common time durations used throughout the application
const (
	// Cache durations
	AdminCacheTTL           = 30 * time.Minute
	RestrictedCacheTTL      = 30 * time.Minute
	RestrictedProbeInterval = 5 * time.Minute
	ShortCacheTTL           = 1 * time.Minute

	// Update intervals
	UserUpdateInterval    = 5 * time.Minute
	ChatUpdateInterval    = 5 * time.Minute
	ChannelUpdateInterval = 5 * time.Minute

	// Timeout durations
	DefaultTimeout  = 10 * time.Second
	ShortTimeout    = 5 * time.Second
	LongTimeout     = 30 * time.Second
	VeryLongTimeout = 120 * time.Second

	// Connection pooling
	DefaultHTTPPort         = 8080
	MaxIdleConnsExtraBuffer = 20
)

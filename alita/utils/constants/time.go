package constants

import "time"

const (
	AdminCacheTTL           = 30 * time.Minute
	RestrictedCacheTTL      = 30 * time.Minute
	RestrictedProbeInterval = 5 * time.Minute
	ShortCacheTTL           = 1 * time.Minute

	UserUpdateInterval    = 5 * time.Minute
	ChatUpdateInterval    = 5 * time.Minute
	ChannelUpdateInterval = 5 * time.Minute

	DefaultTimeout  = 10 * time.Second
	ShortTimeout    = 5 * time.Second
	LongTimeout     = 30 * time.Second
	VeryLongTimeout = 120 * time.Second

	DefaultHTTPPort         = 8080
	MaxIdleConnsExtraBuffer = 20
)

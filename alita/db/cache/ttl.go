package cache

import "time"

// TTL constants for cache entries. Each domain uses its own TTL to balance
// freshness vs DB load. Warn settings (30m) are separate from language (1h).
const (
	CacheTTLChatSettings    = 30 * time.Minute
	CacheTTLLanguage        = 1 * time.Hour
	CacheTTLFilterList      = 30 * time.Minute
	CacheTTLBlacklist       = 30 * time.Minute
	CacheTTLGreetings       = 30 * time.Minute
	CacheTTLNotesList       = 30 * time.Minute
	CacheTTLNotesSettings   = 30 * time.Minute
	CacheTTLWarnSettings    = 30 * time.Minute
	CacheTTLAntiflood       = 30 * time.Minute
	CacheTTLDisabledCmds    = 30 * time.Minute
	CacheTTLCaptchaSettings = 30 * time.Minute
	CacheTTLApprovals       = 30 * time.Minute
	CacheTTLAntiRaid        = 30 * time.Minute
	CacheTTLChannels        = 30 * time.Minute
	CacheTTLReactions       = 30 * time.Minute
)

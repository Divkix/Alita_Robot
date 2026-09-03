package cache

import "time"

// TTL constants for cache entries. Most domains share DefaultCacheTTL (30m);
// language uses LangCacheTTL (1h). Per-domain aliases remain for call-site
// clarity and grep-ability.
const (
	DefaultCacheTTL = 30 * time.Minute
	LangCacheTTL    = 1 * time.Hour
)

const (
	CacheTTLChatSettings    = DefaultCacheTTL
	CacheTTLLanguage        = LangCacheTTL
	CacheTTLFilterList      = DefaultCacheTTL
	CacheTTLBlacklist       = DefaultCacheTTL
	CacheTTLGreetings       = DefaultCacheTTL
	CacheTTLNotesList       = DefaultCacheTTL
	CacheTTLNotesSettings   = DefaultCacheTTL
	CacheTTLWarnSettings    = DefaultCacheTTL
	CacheTTLAntiflood       = DefaultCacheTTL
	CacheTTLDisabledCmds    = DefaultCacheTTL
	CacheTTLCaptchaSettings = DefaultCacheTTL
	CacheTTLApprovals       = DefaultCacheTTL
	CacheTTLAntiRaid        = DefaultCacheTTL
	CacheTTLChannels        = DefaultCacheTTL
	CacheTTLReactions       = DefaultCacheTTL
	CacheTTLFederation      = DefaultCacheTTL
	CacheTTLLogChannel      = DefaultCacheTTL
)

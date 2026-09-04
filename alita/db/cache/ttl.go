package cache

import "time"

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

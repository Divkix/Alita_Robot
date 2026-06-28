package cache

import "time"

const (
	CacheTTLChatSettings     = 30 * time.Minute
	CacheTTLLanguage         = 1 * time.Hour
	CacheTTLFilterList       = 30 * time.Minute
	CacheTTLBlacklist        = 30 * time.Minute
	CacheTTLGreetings        = 30 * time.Minute
	CacheTTLNotesList        = 30 * time.Minute
	CacheTTLNotesSettings    = 30 * time.Minute
	CacheTTLWarnSettings     = 30 * time.Minute
	CacheTTLAntiflood        = 30 * time.Minute
	CacheTTLDisabledCmds     = 30 * time.Minute
	CacheTTLCaptchaSettings  = 30 * time.Minute
	CacheTTLApprovals        = 30 * time.Minute
	CacheTTLAntiRaid         = 30 * time.Minute
	CacheTTLChannels         = 30 * time.Minute
	CacheTTLFederation       = 30 * time.Minute
	CacheTTLFederationBans   = 1 * time.Hour
	CacheTTLFederationExport = 30 * time.Minute
)

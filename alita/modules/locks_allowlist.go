package modules

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

// The allowlist kinds stored in lock_allowlist.kind.
const (
	allowlistKindHost   = "host"
	allowlistKindInvite = "invite"
)

var (
	// inviteHostRe matches the Telegram hosts whose links can be invites.
	inviteHostRe = regexp.MustCompile(`(?i)^(?:www\.)?(?:t\.me|telegram\.me|telegram\.dog)$`)
	// allowlistHostRe is a dotted hostname; IP literals and single labels are
	// refused so an entry cannot silently cover more than its host.
	allowlistHostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	// A numeric +path up to 32 digits names a phone profile in Telegram.
	telegramPhoneRe  = regexp.MustCompile(`^[0-9]{1,32}$`)
	inviteHashBodyRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// parseAllowlistURL resolves raw into the URL it denotes, tolerating the
// scheme-less forms Telegram entities and admins type ("example.com/x",
// "t.me/+hash") and the opaque tg:join?invite= spelling. ok is false when
// raw cannot be a link at all.
func parseAllowlistURL(raw string) (*url.URL, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), ".,;:!?)]}")
	if trimmed == "" {
		return nil, false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, false
	}
	if u.Host == "" && !(strings.EqualFold(u.Scheme, "tg") && u.Opaque != "") {
		u, err = url.Parse("https://" + trimmed)
		if err != nil || u.Host == "" {
			return nil, false
		}
	}
	return u, true
}

// allowlistHostOf returns the normalized host of raw when raw is a URL whose
// host is a real hostname; credentials, Telegram invite hosts and
// non-hostname hosts are refused so an entry cannot silently cover more than
// its host.
func allowlistHostOf(raw string) (string, bool) {
	u, ok := parseAllowlistURL(raw)
	if !ok || u.User != nil || inviteURLHost(u) {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", false
	}
	host = strings.TrimPrefix(host, "www.")
	if !allowlistHostRe.MatchString(host) {
		return "", false
	}
	return host, true
}

// inviteHashIdentity normalizes a private invite to its stored form "+HASH":
// Telegram's "+HASH" and "joinchat/HASH" spellings and the tg://join
// ?invite=HASH parameter all name the same hash, which stays case-sensitive.
func inviteHashIdentity(raw string) string {
	if body, ok := strings.CutPrefix(raw, "joinchat/"); ok && inviteHashBodyRe.MatchString(body) {
		return "+" + body
	}
	if body, ok := strings.CutPrefix(raw, "+"); ok && inviteHashBodyRe.MatchString(body) && !telegramPhoneRe.MatchString(body) {
		return raw
	}
	return ""
}

// inviteTgURIIdentity recognizes tg://join?invite=HASH and its opaque
// spelling tg:join?invite=HASH.
func inviteTgURIIdentity(u *url.URL) string {
	target := u.Hostname()
	if target == "" {
		// An opaque tg: URI (tg:join?invite=HASH) has no host; its
		// scheme-specific part names the target instead.
		target = u.Opaque
	}
	if strings.EqualFold(target, "join") && u.Path == "" {
		if hash := u.Query().Get("invite"); inviteHashBodyRe.MatchString(hash) {
			return "+" + hash
		}
	}
	return ""
}

// inviteURLHost reports whether host is a Telegram host whose links can be
// invites.
func inviteURLHost(u *url.URL) bool {
	return inviteHostRe.MatchString(strings.ToLower(u.Hostname()))
}

// parseInviteEntry normalizes an admin-typed invite exception to its stored
// value "+hash": a bare "+hash"/"joinchat/hash", a t.me private-invite path
// or a tg://join?invite= link (also spelled tg:join?invite=). Public
// usernames, previews, discussion links, phone lookups and phone-number
// profiles are not invite exceptions.
func parseInviteEntry(raw string) (string, bool) {
	trimmed := strings.TrimSpace(strings.Trim(raw, `"'`))
	if trimmed == "" {
		return "", false
	}
	// "+hash" and "joinchat/hash" are invite identities on their own.
	if identity := inviteHashIdentity(trimmed); identity != "" {
		return identity, true
	}
	if identity := inviteIdentityOfURL(trimmed); identity != "" {
		return identity, true
	}
	return "", false
}

// allowlistEntryKind classifies an admin-typed exception: an invite identity
// wins over a host reading, so "t.me/+hash" is an invite, never the t.me
// host. A tg:// link that is not a definite invite is refused outright.
func allowlistEntryKind(raw string) (kind, value string, ok bool) {
	if value, ok = parseInviteEntry(raw); ok {
		return allowlistKindInvite, value, true
	}
	if u, parsed := parseAllowlistURL(raw); parsed && strings.EqualFold(u.Scheme, "tg") {
		return "", "", false
	}
	if value, ok = allowlistHostOf(raw); ok {
		return allowlistKindHost, value, true
	}
	return "", "", false
}

// inviteIdentityOfURL returns the case-sensitive "+HASH" for a Telegram
// private invite, or "" for other URLs (including phone profiles).
func inviteIdentityOfURL(raw string) string {
	u, ok := parseAllowlistURL(raw)
	if !ok || u.User != nil {
		return ""
	}
	if strings.EqualFold(u.Scheme, "tg") {
		return inviteTgURIIdentity(u)
	}
	if !inviteURLHost(u) {
		return ""
	}
	return inviteHashIdentity(strings.Trim(u.Path, "/"))
}

// lockExceptions is the chat's allowlist split by kind for one update.
type lockExceptions struct {
	hosts   map[string]bool
	invites map[string]bool
}

func lockExceptionsForUpdate(ctx *ext.Context, chatID int64, chatLocks map[string]bool) lockExceptions {
	if !chatLocks["url"] && !chatLocks["invitelink"] {
		return lockExceptions{}
	}
	return updatememo.Get(updatememo.From(ctx), chatID, func() lockExceptions {
		return lockExceptionsOf(tracing.UpdateContext(ctx), chatID)
	})
}

func lockExceptionsOf(ctx context.Context, chatID int64) lockExceptions {
	rows, err := locks.GetChatAllowlistCachedContext(ctx, chatID)
	if err != nil {
		log.WithError(err).WithField("chat_id", chatID).Error("[Locks] Failed to load allowlist")
		return lockExceptions{}
	}
	ex := lockExceptions{
		hosts:   make(map[string]bool, len(rows)),
		invites: make(map[string]bool, len(rows)),
	}
	for _, row := range rows {
		switch row.Kind {
		case allowlistKindHost:
			ex.hosts[row.Value] = true
		case allowlistKindInvite:
			ex.invites[row.Value] = true
		}
	}
	return ex
}

// hostAllowed reports whether host is excepted: an entry covers itself and
// its subdomains, never a lookalike suffix (example.com.evil.test).
func (ex lockExceptions) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for allowed := range ex.hosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// urlEntityHit reports whether the message carries a URL entity no host
// exception covers. Entities that yield no parseable URL still count, as the
// url lock always treated them.
func (ex lockExceptions) urlEntityHit(msg *gotgbot.Message) bool {
	for _, link := range linkEntitiesOf(msg) {
		host := ""
		if raw := link.text(msg); raw != "" {
			if h, ok := allowlistHostOf(raw); ok {
				host = h
			}
		}
		if host == "" || !ex.hostAllowed(host) {
			return true
		}
	}
	return false
}

// inviteEntityHit reports whether the message carries a Telegram invite link
// no invite exception covers.
func (ex lockExceptions) inviteEntityHit(msg *gotgbot.Message) bool {
	for _, link := range linkEntitiesOf(msg) {
		raw := link.text(msg)
		if raw == "" {
			continue
		}
		if invite := inviteIdentityOfURL(raw); invite != "" && !ex.invites[invite] {
			return true
		}
	}
	return false
}

// linkEntity is one entity that can carry a link, with the text it was found
// in (the message text or the caption) so url entities slice correctly.
type linkEntity struct {
	entity gotgbot.MessageEntity
	source string
}

func (l linkEntity) text(msg *gotgbot.Message) string {
	if l.entity.Url != "" {
		return l.entity.Url
	}
	return extraction.ExtractEntityText(l.source, l.entity.Offset, l.entity.Length)
}

// linkEntitiesOf lists the entities that can carry a link: "url" and
// "text_link", plus any entity with an explicit Url, in text and caption.
func linkEntitiesOf(msg *gotgbot.Message) []linkEntity {
	var out []linkEntity
	for _, pair := range []struct {
		source   string
		entities []gotgbot.MessageEntity
	}{
		{msg.Text, msg.Entities},
		{msg.Caption, msg.CaptionEntities},
	} {
		for _, entity := range pair.entities {
			if entity.Type == "url" || entity.Type == "text_link" || entity.Url != "" {
				out = append(out, linkEntity{entity: entity, source: pair.source})
			}
		}
	}
	return out
}

// permLockHit is the exception-aware form of a lockMap filter for the url
// and invitelink locks: every link entity is checked on its own, so one
// allowed host or invite never exempts another unapproved link in the same
// message. Other lock types keep their plain filter.
func permLockHit(perm string, chatLocks map[string]bool, ex lockExceptions, msg *gotgbot.Message) bool {
	if !chatLocks[perm] {
		return false
	}
	switch perm {
	case "url":
		return ex.urlEntityHit(msg)
	case "invitelink":
		return ex.inviteEntityHit(msg)
	default:
		return lockMap[perm](msg)
	}
}

// anyPermLockHit is anyLockHit for the permission locks with exceptions.
func anyPermLockHit(chatLocks map[string]bool, ex lockExceptions, msg *gotgbot.Message, skip string) bool {
	for key := range lockMap {
		if key != skip && permLockHit(key, chatLocks, ex, msg) {
			return true
		}
	}
	return false
}

// locksAllowlistClearAll is the /rmallowlistall owner confirmation: the
// shared skeleton asks, authorizes and answers; the nonce makes the
// destructive clear single-use.
var locksAllowlistClearAll = ownerConfirmSpec{
	Namespace: "rmallowlist",
	AskKey:    "locks_allowlist_clear_all_confirm",
	Ask: func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator) (bool, error) {
		entries, err := locks.GetChatAllowlistError(chat.Id)
		if err != nil {
			log.WithError(err).WithField("chat_id", chat.Id).Error("[Locks] Failed to read allowlist")
			_, replyErr := ctx.EffectiveMessage.Reply(b, tr.String("error_generic"), formatting.Shtml())
			if replyErr != nil {
				return false, replyErr
			}
			return false, nil
		}
		if len(entries) > 0 {
			return true, nil
		}
		_, err = ctx.EffectiveMessage.Reply(b, tr.String("locks_allowlist_empty"), formatting.Shtml())
		return false, err
	},
	Commit: removeAllCommit("Failed to clear lock allowlist", func(chatID int64) error {
		return locks.ClearAllowlist(chatID)
	}),
	Keys: ownerConfirmKeys{
		Confirmed: "locks_allowlist_cleared",
		Cancelled: "locks_allowlist_clear_all_cancelled",
		Failed:    "error_generic",
		Expired:   "locks_allowlist_expired",
	},
	OneUse: true,
}

var locksAllowlistClearAllMessage, locksAllowlistClearAllCallback = registerOwnerConfirm(locksAllowlistClearAll)

var allowlistChecks = []helpers.Check{
	helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.RequireBotAdmin(), helpers.RequireUserAdmin(),
}

// allowlist replies to /allowlist: with no arguments it lists the exceptions,
// otherwise it adds each item and reports the outcome per item.
func (moduleStruct) allowlist(b *gotgbot.Bot, ctx *ext.Context) error {
	return locksModule.allowlistArgs(b, ctx, true)
}

// rmallowlist replies to /rmallowlist: with no arguments it lists the
// exceptions, otherwise it removes each item and reports the outcome.
func (moduleStruct) rmallowlist(b *gotgbot.Bot, ctx *ext.Context) error {
	return locksModule.allowlistArgs(b, ctx, false)
}

func (moduleStruct) allowlistArgs(b *gotgbot.Bot, ctx *ext.Context, add bool) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if len(args) == 0 {
		return locksModule.replyAllowlist(b, ctx, tr)
	}

	current, err := locks.GetChatAllowlistError(chat.Id)
	if err != nil {
		log.WithError(err).WithField("chat_id", chat.Id).Error("[Locks] Failed to read allowlist")
		_, replyErr := msg.Reply(b, withGroupLine(tr, msg, chat, tr.String("error_generic")), formatting.Shtml())
		if replyErr != nil {
			return replyErr
		}
		return ext.EndGroups
	}
	listed := make(map[string]bool, len(current))
	for _, row := range current {
		listed[row.Kind+"\x00"+row.Value] = true
	}

	var added, removed, skipped, invalid []string
	var failed bool
	for _, arg := range args {
		kind, value, ok := allowlistEntryKind(arg)
		if !ok {
			invalid = append(invalid, arg)
			continue
		}
		key := kind + "\x00" + value
		if add {
			if listed[key] {
				skipped = append(skipped, arg)
				continue
			}
			if err := locks.AddAllowlist(chat.Id, kind, value); err != nil {
				log.WithError(err).WithField("chat_id", chat.Id).Error("[Locks] Failed to add allowlist entry")
				failed = true
				continue
			}
			listed[key] = true
			added = append(added, arg)
		} else {
			if !listed[key] {
				skipped = append(skipped, arg)
				continue
			}
			if err := locks.RemoveAllowlist(chat.Id, kind, value); err != nil {
				log.WithError(err).WithField("chat_id", chat.Id).Error("[Locks] Failed to remove allowlist entry")
				failed = true
				continue
			}
			delete(listed, key)
			removed = append(removed, arg)
		}
	}

	var sb strings.Builder
	if failed {
		sb.WriteString(tr.String("error_generic"))
	}
	write := func(key string, items []string) {
		if len(items) == 0 {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "%s\n - %s\n", tr.String(key), html.EscapeString(strings.Join(items, "\n - ")))
	}
	if add {
		write("locks_allowlist_added", added)
		write("locks_allowlist_already_listed", skipped)
	} else {
		write("locks_allowlist_removed", removed)
		write("locks_allowlist_not_listed", skipped)
	}
	write("locks_allowlist_invalid", invalid)
	if sb.Len() == 0 {
		sb.WriteString(tr.String("locks_allowlist_nothing_to_do"))
	}
	_, err = msg.Reply(b, withGroupLine(tr, msg, chat, sb.String()), formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// replyAllowlist answers /allowlist with no arguments: the chat's current
// exceptions, or the empty-list text.
func (moduleStruct) replyAllowlist(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	rows, err := locks.GetChatAllowlistError(chat.Id)
	if err != nil {
		log.WithError(err).WithField("chat_id", chat.Id).Error("[Locks] Failed to read allowlist")
		_, replyErr := msg.Reply(b, withGroupLine(tr, msg, chat, tr.String("error_generic")), formatting.Shtml())
		if replyErr != nil {
			return replyErr
		}
		return ext.EndGroups
	}
	var sb strings.Builder
	if len(rows) == 0 {
		sb.WriteString(tr.String("locks_allowlist_empty"))
	} else {
		sb.WriteString(tr.String("locks_allowlist_list_header"))
		for _, row := range rows {
			fmt.Fprintf(&sb, "\n - <code>%s</code> (%s)", html.EscapeString(row.Value), row.Kind)
		}
	}
	_, err = msg.Reply(b, withGroupLine(tr, msg, chat, sb.String()), formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// rmallallowlist is the /rmallowlistall message handler.
func (moduleStruct) rmallallowlist(b *gotgbot.Bot, ctx *ext.Context) error {
	return locksAllowlistClearAllMessage(b, ctx)
}

// LoadLocksAllowlist registers the allowlist commands and the clear-all
// callback of the Locks module.
func LoadLocksAllowlist(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "allowlist", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: allowlistChecks,
	}, pipelineHandler(locksModule.allowlist))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "rmallowlist", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: allowlistChecks,
	}, pipelineHandler(locksModule.rmallowlist))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "rmallowlistall", PMGroupArg: helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{groupOnlyChatCheck, helpers.RequireGroup(), helpers.RequireUserOwner()},
	}, pipelineHandler(locksModule.rmallallowlist))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmallowlist|"), locksAllowlistClearAllCallback))
}

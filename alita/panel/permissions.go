package panel

import (
	"net/http"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/settings"
)

// permission is the Telegram right the equivalent bot command checks before a
// write.
type permission int

const (
	permNone       permission = iota // admin only, as requireWritable already checks
	permChangeInfo                   // caller can_change_info (creator always has it)
	permRestrict                     // caller AND bot can_restrict_members
)

// settingsPermissions maps a settings module to the right its command checks.
// Modules not listed are admin-only, which requireWritable already enforces.
var settingsPermissions = map[string]permission{
	"language": permNone, // /lang group requires the caller to be an admin
	"rules":    permNone, // /setrules requires an admin and the bot admin
	// Captcha mutes new members, and switching it off unmutes the users it
	// still holds: both writes restrict members, so the caller and the bot
	// need the same right the captcha watcher uses.
	"captcha":     permRestrict,
	"antiflood":   permNone,       // /setflood requires an admin and the bot admin
	"antispam":    permNone,       // /aispam and /spamdb only require an admin; /probation: settingsFieldPermission
	"warns":       permNone,       // /setwarnlimit, /setwarnmode and /setwarntime
	"digest":      permNone,       // /digest requires an admin and the bot admin
	"reports":     permNone,       // /reports; auto-mute: settingsFieldPermission
	"timezone":    permChangeInfo, // /timezone checks can_change_info
	"nightmode":   permNone,       // /nightmode checks an admin and the bot admin
	"locks":       permNone,       // read-only summary
	"log_channel": permNone,       // read-only summary
}

// settingsFieldPermission is the right one "<module>.<field>" change needs:
// its module's, raised for a field whose command checks more than the rest
// of the module.
func settingsFieldPermission(module, field string) permission {
	if declaration, ok := settings.Lookup(module + "." + field); ok {
		switch declaration.Right {
		case settings.Admin, settings.AdminBot:
			// The panel's access envelope already requires both admins.
			return permNone
		case settings.ChangeInfo, settings.ChangeInfoBot:
			return permChangeInfo
		default:
			panic("unmapped setting right")
		}
	}
	perm := settingsPermissions[module]
	// /probation checks can_restrict_members, unlike /aispam and /spamdb.
	if module == "antispam" && strings.HasPrefix(field, "probation_") {
		perm = max(perm, permRestrict)
	}
	if module == "reports" && field == "mute_threshold" {
		perm = max(perm, permRestrict)
	}
	return perm
}

// settingsPermission reports the union of the independent rights a batch needs.
func settingsPermission(changes map[string]any) permission {
	required := permNone
	for key := range changes {
		module, field, ok := strings.Cut(key, ".")
		if !ok {
			continue
		}
		required |= settingsFieldPermission(module, field)
	}
	return required
}

// requirePermission runs requireWritable, then checks perm with fresh
// getChatMember calls for the caller (and the bot for permRestrict). On refusal
// it writes 403 {"code":"missing_permission"} for the caller or
// 409 {"code":"bot_missing_permission"} for the bot and returns false.
func (p *Panel) requirePermission(w http.ResponseWriter, r *http.Request, access chatAccess, userID int64, perm permission) bool {
	if !requireWritable(w, access) {
		return false
	}
	if perm == permNone {
		return true
	}

	caller, err := p.bot.GetChatMemberWithContext(r.Context(), access.ChatID, userID, nil)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "telegram_error", "Telegram could not return this member: "+err.Error())
		return false
	}
	for _, right := range []permission{permChangeInfo, permRestrict} {
		if perm&right != 0 && !memberCan(caller, right) {
			writeJSONError(w, http.StatusForbidden, "missing_permission", "you need the "+permissionRight(right)+" right in this chat")
			return false
		}
	}
	if perm&permRestrict != 0 {
		botMember, err := p.bot.GetChatMemberWithContext(r.Context(), access.ChatID, p.bot.Id, nil)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "telegram_error", "Telegram could not return the bot's membership: "+err.Error())
			return false
		}
		if !memberCan(botMember, permRestrict) {
			writeJSONError(w, http.StatusConflict, "bot_missing_permission", "the bot is missing the "+permissionRight(perm)+" right in this chat, so changes are disabled")
			return false
		}
	}
	return true
}

// permissionRight names the Telegram right a permission stands for in a
// refusal message.
func permissionRight(perm permission) string {
	if perm == permChangeInfo {
		return "can_change_info"
	}
	return "can_restrict_members"
}

// memberCan reports whether a member holds perm. A creator holds every right.
func memberCan(member gotgbot.ChatMember, perm permission) bool {
	merged := member.MergeChatMember()
	if merged.Status == "creator" {
		return true
	}
	switch perm {
	case permChangeInfo:
		return merged.CanChangeInfo
	case permRestrict:
		return merged.CanRestrictMembers
	default:
		return true
	}
}

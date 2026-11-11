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

const permNone permission = 0 // admin only, as requireWritable already checks

const (
	permChangeInfo  permission = 1 << iota // caller can_change_info (creator always has it)
	permRestrict                           // caller can_restrict_members
	permBotRestrict                        // bot can_restrict_members
)

// settingsFieldPermission maps a declaration's neutral policy to Telegram rights.
func settingsFieldPermission(module, field string) permission {
	if declaration, ok := settings.Lookup(module + "." + field); ok {
		switch declaration.Right {
		case settings.Admin, settings.AdminBot:
			// The panel's access envelope already requires both admins.
			return permNone
		case settings.ChangeInfo, settings.ChangeInfoBot:
			return permChangeInfo
		case settings.Restrict:
			return permRestrict | permBotRestrict
		case settings.RestrictUser:
			return permRestrict
		case settings.OwnerBot, settings.FederationAdmin:
			panic("owner/federation settings have no panel editor")
		default:
			panic("unmapped setting right")
		}
	}
	return permNone
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
// getChatMember calls for the caller (and the bot for permBotRestrict). On refusal
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
	if perm&permBotRestrict != 0 {
		botMember, err := p.bot.GetChatMemberWithContext(r.Context(), access.ChatID, p.bot.Id, nil)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "telegram_error", "Telegram could not return the bot's membership: "+err.Error())
			return false
		}
		if !memberCan(botMember, permRestrict) {
			writeJSONError(w, http.StatusConflict, "bot_missing_permission", "the bot is missing the "+permissionRight(permRestrict)+" right in this chat, so changes are disabled")
			return false
		}
	}
	return true
}

// permissionRight names one Telegram right in a refusal message, not a union.
func permissionRight(perm permission) string {
	switch perm {
	case permChangeInfo:
		return "can_change_info"
	case permRestrict:
		return "can_restrict_members"
	default:
		panic("permission text requires one right")
	}
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

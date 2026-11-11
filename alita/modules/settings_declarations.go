package modules

import (
	"fmt"

	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// declaredSettingChecks maps neutral setting rights into the command pipeline.
func declaredSettingChecks(declaration *chatsettings.Declaration) []helpers.CheckFunc {
	switch declaration.Right {
	case chatsettings.Admin:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired)}
	case chatsettings.AdminBot:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired)}
	case chatsettings.ChangeInfo:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), helpers.CanUserChangeInfo()}
	case chatsettings.ChangeInfoBot:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserChangeInfo()}
	case chatsettings.Restrict:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case chatsettings.RestrictUser:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict()}
	case chatsettings.OwnerBot:
		return []helpers.CheckFunc{helpers.RequireConnectedMember(helpers.BotAdminRequired), settingOwnerWriteCheck}
	case chatsettings.FederationAdmin:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

// declaredGroupSettingChecks keeps group-only commands' permission replies while
// deriving the same policy as connected commands.
func declaredGroupSettingChecks(declaration *chatsettings.Declaration) []helpers.CheckFunc {
	switch declaration.Right {
	case chatsettings.Admin:
		return []helpers.CheckFunc{helpers.RequireUserAdmin()}
	case chatsettings.AdminBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin()}
	case chatsettings.ChangeInfo:
		return []helpers.CheckFunc{helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case chatsettings.ChangeInfoBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case chatsettings.Restrict:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case chatsettings.RestrictUser:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict()}
	case chatsettings.OwnerBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), settingOwnerWriteCheck}
	case chatsettings.FederationAdmin:
		return []helpers.CheckFunc{helpers.RequireUserAdmin(), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

func settingOwnerWriteCheck(c *helpers.CommandContext) bool {
	// Members may view the mode; only the owner may change it.
	return len(c.Ctx.Args()) == 1 || helpers.RequireUserOwner()(c)
}

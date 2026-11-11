package modules

import (
	"fmt"

	"github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// declaredSettingChecks maps neutral setting rights into the command pipeline.
func declaredSettingChecks(declaration *settings.Declaration) []helpers.CheckFunc {
	switch declaration.Right {
	case settings.Admin:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired)}
	case settings.AdminBot:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired)}
	case settings.ChangeInfo:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), helpers.CanUserChangeInfo()}
	case settings.ChangeInfoBot:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserChangeInfo()}
	case settings.Restrict:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case settings.RestrictUser:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict()}
	case settings.OwnerBot:
		return []helpers.CheckFunc{helpers.RequireConnectedMember(helpers.BotAdminRequired), settingOwnerWriteCheck}
	case settings.FederationAdmin:
		return []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

// declaredGroupSettingChecks keeps group-only commands' permission replies while
// deriving the same policy as connected commands.
func declaredGroupSettingChecks(declaration *settings.Declaration) []helpers.CheckFunc {
	switch declaration.Right {
	case settings.Admin:
		return []helpers.CheckFunc{helpers.RequireUserAdmin()}
	case settings.AdminBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin()}
	case settings.ChangeInfo:
		return []helpers.CheckFunc{helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case settings.ChangeInfoBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case settings.Restrict:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case settings.RestrictUser:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict()}
	case settings.OwnerBot:
		return []helpers.CheckFunc{helpers.RequireBotAdmin(), settingOwnerWriteCheck}
	case settings.FederationAdmin:
		return []helpers.CheckFunc{helpers.RequireUserAdmin(), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

func settingOwnerWriteCheck(c *helpers.CommandContext) bool {
	// Members may view the mode; only the owner may change it.
	return len(c.Ctx.Args()) == 1 || helpers.RequireUserOwner()(c)
}

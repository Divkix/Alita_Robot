package modules

import (
	"fmt"

	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// declaredSettingChecks maps neutral setting rights into the command pipeline.
func declaredSettingChecks(declaration *chatsettings.Declaration) []helpers.Check {
	switch declaration.Right {
	case chatsettings.Admin:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired)}
	case chatsettings.AdminBot:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired)}
	case chatsettings.ChangeInfo:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), helpers.CanUserChangeInfo()}
	case chatsettings.ChangeInfoBot:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserChangeInfo()}
	case chatsettings.Restrict:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case chatsettings.RestrictUser:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict()}
	case chatsettings.OwnerBot:
		return []helpers.Check{helpers.RequireConnectedMember(helpers.BotAdminRequired), settingOwnerWriteCheck}
	case chatsettings.FederationAdmin:
		return []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

// declaredGroupSettingChecks keeps group-only commands' permission replies while
// deriving the same policy as connected commands.
func declaredGroupSettingChecks(declaration *chatsettings.Declaration) []helpers.Check {
	switch declaration.Right {
	case chatsettings.Admin:
		return []helpers.Check{helpers.RequireUserAdmin()}
	case chatsettings.AdminBot:
		return []helpers.Check{helpers.RequireBotAdmin(), helpers.RequireUserAdmin()}
	case chatsettings.ChangeInfo:
		return []helpers.Check{helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case chatsettings.ChangeInfoBot:
		return []helpers.Check{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserChangeInfo()}
	case chatsettings.Restrict:
		return []helpers.Check{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	case chatsettings.RestrictUser:
		return []helpers.Check{helpers.RequireBotAdmin(), helpers.RequireUserAdmin(), helpers.CanUserRestrict()}
	case chatsettings.OwnerBot:
		return []helpers.Check{helpers.RequireBotAdmin(), settingOwnerWriteCheck}
	case chatsettings.FederationAdmin:
		return []helpers.Check{helpers.RequireUserAdmin(), federationChatCheck}
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

// settingOwnerWriteCheck keeps view access for members; only the owner may
// change the value (a lone argument is the view form).
var settingOwnerWriteCheck = helpers.NewCheck(helpers.CheckKindMember, func(c *helpers.CommandContext) bool {
	// Members may view the mode; only the owner may change it.
	return len(c.Ctx.Args()) == 1 || helpers.RequireUserOwner().Run(c)
})

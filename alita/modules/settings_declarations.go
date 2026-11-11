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
	default:
		panic(fmt.Sprintf("unmapped setting right: %d", declaration.Right))
	}
}

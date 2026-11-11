package settings

import (
	"context"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/pins"
)

var PrivateNotes = &Declaration{
	Key: "notes.private_notes", Type: Switch, Default: false, Right: Admin,
	read: func(ctx context.Context, chatID int64) any {
		row := notes.GetNotes(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.Private
	},
	write: func(input WriteContext, value any) error {
		return notes.TooglePrivateNote(input.ChatID, value.(bool))
	},
}

var AntiChannelPin = &Declaration{
	Key: "pins.anti_channel_pin", Type: Switch, Default: false, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row := pins.GetPinData(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.AntiChannelPin
	},
	write: func(input WriteContext, value any) error {
		return pins.SetAntiChannelPin(input.ChatID, value.(bool))
	},
}

var CleanLinked = &Declaration{
	Key: "pins.clean_linked", Type: Switch, Default: false, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row := pins.GetPinData(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.CleanLinked
	},
	write: func(input WriteContext, value any) error {
		return pins.SetCleanLinked(input.ChatID, value.(bool))
	},
}

var AllowConnect = &Declaration{
	Key: "connections.allow_connect", Type: Switch, Default: true, Right: Admin,
	read: func(ctx context.Context, chatID int64) any {
		row := connections.GetChatConnectionSetting(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.AllowConnect
	},
	write: func(input WriteContext, value any) error {
		return connections.ToggleAllowConnect(input.ChatID, value.(bool))
	},
}

var AnonAdmin = &Declaration{
	Key: "admin.anon_admin", Type: Switch, Default: false, Right: OwnerBot,
	read: func(ctx context.Context, chatID int64) any {
		row := admin.GetAdminSettings(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.AnonAdmin
	},
	write: func(input WriteContext, value any) error {
		return admin.SetAnonAdminMode(input.ChatID, value.(bool))
	},
}

var DeleteDisabledCommands = &Declaration{
	Key: "disabling.delete_commands", Type: Switch, Default: false, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row := disabling.GetChatSettings(chatID)
		if row == nil {
			return nil
		}
		return row.DeleteCommands
	},
	write: func(input WriteContext, value any) error {
		return disabling.ToggleDel(input.ChatID, value.(bool))
	},
}

var QuietFed = &Declaration{
	Key: "federations.quiet", Type: Switch, Default: false, Right: FederationAdmin,
	read: func(ctx context.Context, chatID int64) any {
		row := federations.GetChatFedContext(ctx, chatID)
		if row == nil {
			return nil
		}
		return row.Quiet
	},
	write: func(input WriteContext, value any) error {
		return federations.SetQuietFed(input.ChatID, value.(bool))
	},
}

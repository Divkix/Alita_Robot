package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

type adminMemoKey struct{ chatID, userID int64 }
type approvedMemoKey struct{ chatID, userID int64 }

// IsUserAdminForUpdate is IsUserAdmin memoised for the current update. Use it
// only in watchers: the result is not refreshed if the update itself changes admins.
func IsUserAdminForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), adminMemoKey{chatID, userID}, func() bool {
		return IsUserAdmin(b, chatID, userID)
	})
}

// IsApprovedForUpdate is IsApproved memoised for the current update. Use it
// only in watchers: the result is not refreshed if the update itself changes approvals.
func IsApprovedForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), approvedMemoKey{chatID, userID}, func() bool {
		return IsApproved(b, chatID, userID)
	})
}

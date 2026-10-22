package helpers

import "fmt"

// BotAdminRights are the admin rights the bot checks before acting
// (chat_status.CanBot*). Requesting them in the add-to-group link makes Telegram
// add the bot as an admin; without `admin=` it joins as a plain member and every
// admin-only command fails until someone promotes it by hand.
const BotAdminRights = "change_info+delete_messages+restrict_members+invite_users+pin_messages+promote_members"

// AddToGroupURL is the one description of the add-to-group link. The docs
// landing page and quick-start page must contain this URL verbatim;
// TestDocsAddToGroupLinksMatchHelper fails when either drifts.
func AddToGroupURL(botUsername string) string {
	return fmt.Sprintf("https://t.me/%s?startgroup=botstart&admin=%s", botUsername, BotAdminRights)
}

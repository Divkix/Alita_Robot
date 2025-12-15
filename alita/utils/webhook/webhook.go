package webhook

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
)

// DeleteWebhook removes the webhook configuration from Telegram
// This is useful when switching from webhook mode to polling mode
func DeleteWebhook(bot *gotgbot.Bot) error {
	_, err := bot.DeleteWebhook(nil)
	if err != nil {
		log.Errorf("[Webhook] Failed to delete webhook: %v", err)
		return err
	}
	log.Info("[Webhook] Webhook deleted successfully")
	return nil
}

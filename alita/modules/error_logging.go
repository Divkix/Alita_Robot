package modules

import (
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"
)

// logJoinOperationError logs a join/greeting operation failure at warning level
// when the cause is an expected external Telegram condition (see
// helpers.IsExpectedTelegramError) and at error level otherwise. The dispatcher
// already treats these conditions as expected, so error-level entries from the
// join pipeline only duplicate those warnings and pollute error monitoring.
func logJoinOperationError(msg string, err error) {
	if err == nil {
		return
	}
	if helpers.IsExpectedTelegramError(err) {
		log.Warnf("%s: %v", msg, err)
		return
	}
	log.Errorf("%s: %v", msg, err)
}

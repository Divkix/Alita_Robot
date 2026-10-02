package modules

import (
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"
)

// logJoinOperationError logs a join/greeting operation failure at debug level
// when the cause is an expected external Telegram condition (see
// helpers.IsExpectedTelegramError) and at error level otherwise. A returned
// error is already logged once by the dispatcher, so anything above debug for
// expected conditions only repeats that entry for every pipeline stage.
func logJoinOperationError(msg string, err error) {
	if err == nil {
		return
	}
	if helpers.IsExpectedTelegramError(err) {
		log.Debugf("%s: %v", msg, err)
		return
	}
	log.Errorf("%s: %v", msg, err)
}

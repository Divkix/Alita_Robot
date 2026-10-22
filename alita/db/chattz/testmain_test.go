//go:build testtools

package chattz

import (
	"os"
	"testing"
	// The tests must not depend on the host's zoneinfo, as production
	// binaries do not (main imports time/tzdata).
	_ "time/tzdata"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ChatTimezone{}))
}

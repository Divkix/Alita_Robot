//go:build testtools

package nightmode

import (
	"os"
	"testing"
	// The DST tests load America/New_York; production binaries embed the
	// zone database through main.
	_ "time/tzdata"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.NightModeSettings{}))
}

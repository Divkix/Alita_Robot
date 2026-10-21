package federations

import (
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.Federation{},
		&models.FederationAdmin{},
		&models.FederationChat{},
		&models.FederationBan{},
		&models.FederationSub{},
	))
}

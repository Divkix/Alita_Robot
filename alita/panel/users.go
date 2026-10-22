package panel

import (
	"context"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/user"
)

// userName is how the dashboard names a user: the first name and @username
// the bot last saw, either of which may be empty.
type userName struct {
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
}

// userNames looks up the stored names of ids for display, keyed by decimal
// user ID. Non-positive IDs (automated actors, anonymous admins) and users the
// bot never stored are left out. A failed lookup costs only the names, so it
// is logged and the screen falls back to IDs.
func userNames(ctx context.Context, ids []int64) map[string]userName {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil
	}

	rows, err := user.GetUserNamesContext(ctx, unique)
	if err != nil {
		log.WithError(err).Warn("[Panel] Failed to look up user names")
		return nil
	}
	names := make(map[string]userName, len(rows))
	for id, row := range rows {
		if row.Name == "" && row.UserName == "" {
			continue
		}
		names[strconv.FormatInt(id, 10)] = userName{Name: row.Name, Username: row.UserName}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

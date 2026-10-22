// Package chattz stores each chat's timezone and turns it into a
// *time.Location for features that act on the chat's local time. A chat that
// never set one is on UTC.
package chattz

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// DefaultName is the timezone of a chat without a row.
const DefaultName = "UTC"

// maxNameLen matches chk_chat_timezones_timezone. The longest IANA name is
// 32 bytes.
const maxNameLen = 64

const cacheKeyPrefix = "chat_timezone"

// ErrUnknownTimezone is returned for a name the time zone database does not
// know, and for names that are not IANA zones (empty, "Local").
var ErrUnknownTimezone = errors.New("chattz: unknown timezone")

// locations memoises time.LoadLocation, which reads and parses the zone file
// on every call, for canonical and stored names only. Those are a closed set,
// so the map stays small.
var locations sync.Map // name -> *time.Location

// unloadable remembers stored names that did not load, so a stale row costs
// one lookup and one log line per process, not one per message.
var unloadable sync.Map // name -> struct{}

func cacheKey(chatID int64) string {
	return cache.CacheKey(cacheKeyPrefix, chatID)
}

// Parse validates an IANA timezone name and returns its canonical spelling
// with the loaded location. The name is tried as typed and then with each
// word capitalised, so "asia/kolkata" finds Asia/Kolkata. "UTC" matches in
// any case. "Local" is refused: it is the server's zone, not a chat's.
func Parse(input string) (string, *time.Location, error) {
	name := strings.TrimSpace(input)
	if strings.EqualFold(name, DefaultName) {
		return DefaultName, time.UTC, nil
	}
	if name == "" || len(name) > maxNameLen || strings.EqualFold(name, "Local") || !validNameChars(name) {
		return "", nil, ErrUnknownTimezone
	}
	for _, candidate := range []string{name, capitalised(name)} {
		// Not memoised: on a file system that ignores case every spelling of
		// a zone loads, and user input must not grow the memo.
		if _, err := time.LoadLocation(candidate); err != nil {
			continue
		}
		canonical := diskCase(candidate)
		loc, err := load(canonical)
		if err != nil {
			continue
		}
		return canonical, loc, nil
	}
	return "", nil, ErrUnknownTimezone
}

// zoneDirs are the directories time.LoadLocation reads on Unix before it
// falls back to the copy embedded through time/tzdata.
var zoneDirs = []string{"/usr/share/zoneinfo/", "/usr/share/lib/zoneinfo/", "/usr/lib/locale/TZ/", "/etc/zoneinfo/"}

// diskCase returns name spelled as its zone file is. On a file system that
// ignores case (macOS), time.LoadLocation also loads "asia/kolkata", which
// would not load on a Linux host; the stored name must load everywhere. When
// no zone directory holds the name, the embedded copy loaded it, and that
// lookup is already case-sensitive.
func diskCase(name string) string {
	for _, dir := range zoneDirs {
		if real, ok := realPath(dir, name); ok {
			return real
		}
	}
	return name
}

// realPath walks name's segments under dir, matching each against the
// directory's entries: exactly when one matches so, else ignoring case.
func realPath(dir, name string) (string, bool) {
	parts := strings.Split(name, "/")
	out := make([]string, 0, len(parts))
	current := dir
	for _, part := range parts {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", false
		}
		found := ""
		for _, entry := range entries {
			if entry.Name() == part {
				found = part
				break
			}
			if found == "" && strings.EqualFold(entry.Name(), part) {
				found = entry.Name()
			}
		}
		if found == "" {
			return "", false
		}
		out = append(out, found)
		current = filepath.Join(current, found)
	}
	return strings.Join(out, "/"), true
}

// validNameChars keeps the lookup to the characters zone names use, so user
// input never reaches the file system as anything but a zone path.
func validNameChars(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/' || r == '_' || r == '-' || r == '+':
		default:
			return false
		}
	}
	return !strings.Contains(name, "//") && !strings.HasPrefix(name, "/") && !strings.HasSuffix(name, "/")
}

// capitalised upper-cases the first letter of every word and lower-cases the
// rest, where words are split by '/', '_' and '-': "america/new_york" becomes
// "America/New_York". Names that do not follow that pattern (Etc/GMT+5,
// America/Port-au-Prince) must be typed as listed.
func capitalised(name string) string {
	b := []byte(strings.ToLower(name))
	upper := true
	for i, c := range b {
		if upper && c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
		upper = c == '/' || c == '_' || c == '-'
	}
	return string(b)
}

// Load returns the location of a canonical name, as Parse or Name return it.
func Load(name string) (*time.Location, error) {
	if name == DefaultName {
		return time.UTC, nil
	}
	return load(name)
}

func load(name string) (*time.Location, error) {
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	locations.Store(name, loc)
	return loc, nil
}

// Name returns the chat's timezone name, DefaultName when it never set one.
func Name(chatID int64) (string, error) {
	return NameContext(context.Background(), chatID)
}

func NameContext(ctx context.Context, chatID int64) (string, error) {
	row, err := cache.GetFromCacheOrLoad(ctx, cacheKey(chatID), cache.CacheTTLChatTimezone,
		func(ctx context.Context) (*models.ChatTimezone, error) {
			row := &models.ChatTimezone{}
			err := db.GetRecordContext(ctx, row, models.ChatTimezone{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.ChatTimezone{ChatID: chatID, Timezone: DefaultName}, nil
			}
			if err != nil {
				return nil, err
			}
			return row, nil
		})
	if err != nil {
		return "", err
	}
	if row == nil || row.Timezone == "" {
		return DefaultName, nil
	}
	return row.Timezone, nil
}

// Location returns the chat's timezone. It never fails: a read error or a
// stored name the time zone database no longer knows falls back to UTC, with
// a log line, so a caller acting on local time keeps working.
func Location(chatID int64) *time.Location {
	return LocationContext(context.Background(), chatID)
}

func LocationContext(ctx context.Context, chatID int64) *time.Location {
	name, err := NameContext(ctx, chatID)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "error": err}).
			Warn("[ChatTZ] could not read the chat's timezone; using UTC")
		return time.UTC
	}
	if name == DefaultName {
		return time.UTC
	}
	if _, bad := unloadable.Load(name); bad {
		return time.UTC
	}
	loc, err := load(name)
	if err != nil {
		unloadable.Store(name, struct{}{})
		log.WithFields(log.Fields{"chat_id": chatID, "timezone": name, "error": err}).
			Warn("[ChatTZ] stored timezone does not load; using UTC for it")
		return time.UTC
	}
	return loc
}

// Set validates name with Parse and stores its canonical spelling, which it
// returns. Setting DefaultName stores it too, so the row records the choice.
func Set(chatID int64, name string) (string, error) {
	canonical, _, err := Parse(name)
	if err != nil {
		return "", err
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return "", err
	}
	now := time.Now()
	err = db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"timezone", "updated_at"}),
	}).Create(&models.ChatTimezone{
		ChatID:    chatID,
		Timezone:  canonical,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
	if err != nil {
		return "", err
	}
	cache.DeleteCache(cacheKey(chatID))
	return canonical, nil
}

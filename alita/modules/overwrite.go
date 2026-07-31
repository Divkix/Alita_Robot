package modules

import (
	"fmt"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/vmihailenco/msgpack/v5"
)

const overwriteCacheTTL = 5 * time.Minute

var overwriteConsumeMu sync.Mutex

// overwriteBase holds common fields for temporary state storage during command flows.
type overwriteBase struct {
	ChatID   int64
	UserID   int64
	ItemName string // filterWord or noteWord
	Text     string
	FileID   string
	Buttons  []db.Button
	DataType int
}

// struct for filters module
type overwriteFilter struct {
	overwriteBase
}

// struct for notes module
type overwriteNote struct {
	overwriteBase
	PvtOnly     bool
	GrpOnly     bool
	AdminOnly   bool
	WebPrev     bool
	IsProtected bool
	NoNotif     bool
}

func overwriteCacheKey(kind, token string) string {
	return fmt.Sprintf("alita:%s_overwrite:%s", kind, token)
}

func setOverwriteCache(key string, data any) error {
	m := cache.GetMarshal()
	if m == nil {
		return fmt.Errorf("cache not initialized")
	}
	return m.Set(cache.Context, key, data, store.WithExpiration(overwriteCacheTTL))
}

func getOverwriteCache[T any](key string) (*T, error) {
	m := cache.GetMarshal()
	if m == nil {
		return nil, fmt.Errorf("cache not initialized")
	}
	var data T
	if _, err := m.Get(cache.Context, key, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func consumeOverwriteCache[T any](key string) (*T, error) {
	if rdb := cache.GetRedisClient(); rdb != nil {
		raw, err := rdb.GetDel(cache.Context, key).Bytes()
		if err != nil {
			return nil, err
		}
		var data T
		if err := msgpack.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
		return &data, nil
	}

	// Test/fallback stores lack GETDEL; serialize the read and delete locally.
	overwriteConsumeMu.Lock()
	defer overwriteConsumeMu.Unlock()
	data, err := getOverwriteCache[T](key)
	if err != nil {
		return nil, err
	}
	deleteOverwriteCache(key)
	return data, nil
}

func deleteOverwriteCache(key string) {
	if m := cache.GetMarshal(); m != nil {
		_ = m.Delete(cache.Context, key)
	}
}

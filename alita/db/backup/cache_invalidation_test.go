//go:build testtools

package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// TestClearModuleDataInvalidatesCreatedChatCache primes the negative chat
// cache for an unknown chat, lets a clear create its chats row, and checks the
// cached read returns that row instead of the stale not-found entry.
func TestClearModuleDataInvalidatesCreatedChatCache(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	_, err := chats.GetChatBasicInfoCached(chatID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, ClearModuleData(chatID, BackupModuleNotes))

	chat, err := chats.GetChatBasicInfoCached(chatID)
	require.NoError(t, err)
	assert.Equal(t, chatID, chat.ChatId)
}

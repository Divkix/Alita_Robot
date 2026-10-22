package panel

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// Chat roles returned by the chat access helper.
const (
	RoleAdmin    = "admin"
	RoleReadonly = "readonly"
)

// chatAccessMemoTTL is how long an admin check is trusted. Writes with the
// panel are gated by the same answer, so keep it short.
const chatAccessMemoTTL = 60 * time.Second

// chatAccess is one verified chat: its Telegram title and the caller's role.
type chatAccess struct {
	ChatID   int64
	Title    string
	Role     string
	BotAdmin bool
}

type accessKey struct {
	chatID int64
	userID int64
}

type accessEntry struct {
	access  chatAccess
	expires time.Time
}

// accessChat verifies that userID may open chatID through the existing admin
// cache and returns the caller's role. A non-admin is readonly, not an error.
// Results are memoised per (chat, user) for 60 seconds; later panel endpoints
// reuse this helper through the same memo.
func (p *Panel) accessChat(ctx context.Context, userID, chatID int64) (chatAccess, error) {
	key := accessKey{chatID: chatID, userID: userID}
	now := p.deps.Now()

	p.accessMu.Lock()
	entry, ok := p.access[key]
	p.accessMu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.access, nil
	}

	info, err := p.bot.GetChatWithContext(ctx, chatID, nil)
	if err != nil {
		return chatAccess{}, fmt.Errorf("get chat: %w", err)
	}

	isAdmin, err := chat_status.IsUserAdminWithContext(ctx, p.bot, chatID, userID)
	if err != nil {
		return chatAccess{}, fmt.Errorf("check admin: %w", err)
	}

	access := chatAccess{
		ChatID:   chatID,
		Title:    info.Title,
		Role:     RoleReadonly,
		BotAdmin: chat_status.IsBotAdmin(p.bot, nil, &gotgbot.Chat{Id: info.Id, Type: info.Type, Title: info.Title}),
	}
	if isAdmin {
		access.Role = RoleAdmin
	}

	p.accessMu.Lock()
	if len(p.access) > 4096 {
		for k, e := range p.access {
			if !now.Before(e.expires) {
				delete(p.access, k)
			}
		}
	}
	p.access[key] = accessEntry{access: access, expires: now.Add(chatAccessMemoTTL)}
	p.accessMu.Unlock()

	return access, nil
}

package panel

import (
	"context"
	"errors"
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

// chatAccessMemoTTL is how long an access answer is trusted. Writes with the
// panel are gated by the same answer, so keep it short.
const chatAccessMemoTTL = 60 * time.Second

// chatAccessMemoCap bounds the memo. One (chat, user) pair is one entry;
// reaching the cap clears the map instead of scanning it on every request, so
// the worst case is a few extra Telegram calls, never unbounded growth.
const chatAccessMemoCap = 4096

// Errors returned by accessChat. chatRequest maps them to stable panel error
// codes; every other error is a Telegram failure.
var (
	errNotGroup  = errors.New("chat is not a group")
	errNotMember = errors.New("user is not a member of the chat")
)

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

// accessChat verifies that userID is a current member of chatID and returns
// the caller's role. Only groups and supergroups are served; a non-member is
// refused with errNotMember, and a member who is not an admin is readonly.
// Results are memoised per (chat, user) for 60 seconds, successes only; later
// panel endpoints reuse this helper through the same memo.
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
	if info.Type != "group" && info.Type != "supergroup" {
		return chatAccess{}, fmt.Errorf("%w: chat %d has type %q", errNotGroup, chatID, info.Type)
	}

	member, err := p.bot.GetChatMemberWithContext(ctx, chatID, userID, nil)
	if err != nil {
		return chatAccess{}, fmt.Errorf("get chat member: %w", err)
	}
	role, ok := memberRole(member)
	if !ok {
		status := "unknown"
		if member != nil {
			status = member.GetStatus()
		}
		return chatAccess{}, fmt.Errorf("%w: status %q", errNotMember, status)
	}

	access := chatAccess{
		ChatID:   chatID,
		Title:    info.Title,
		Role:     role,
		BotAdmin: chat_status.IsBotAdmin(p.bot, nil, &gotgbot.Chat{Id: info.Id, Type: info.Type, Title: info.Title}),
	}

	p.accessMu.Lock()
	if _, cached := p.access[key]; !cached && len(p.access) >= chatAccessMemoCap {
		clear(p.access)
	}
	p.access[key] = accessEntry{access: access, expires: now.Add(chatAccessMemoTTL)}
	p.accessMu.Unlock()

	return access, nil
}

// memberRole is the panel role for one chat member. It reports false when the
// member is not in the chat: left, kicked, or restricted with is_member false.
func memberRole(member gotgbot.ChatMember) (string, bool) {
	if member == nil {
		return "", false
	}
	switch member.GetStatus() {
	case gotgbot.ChatMemberStatusCreator, gotgbot.ChatMemberStatusAdministrator:
		return RoleAdmin, true
	case gotgbot.ChatMemberStatusMember:
		return RoleReadonly, true
	case gotgbot.ChatMemberStatusRestricted:
		if restricted, ok := member.(gotgbot.ChatMemberRestricted); ok && restricted.IsMember {
			return RoleReadonly, true
		}
	}
	return "", false
}

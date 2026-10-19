//go:build testtools

package chat_status

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

type errorBotClient struct {
	chatErr  bool
	chatType string
}

func (c *errorBotClient) RequestWithContext(
	_ context.Context, _ string, method string,
	_ map[string]any, _ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	switch method {
	case "getChat":
		if c.chatErr {
			return nil, fmt.Errorf("chat not found")
		}
		chatJSON := fmt.Sprintf(`{"id":-4001,"type":%q,"title":"Error Test Chat"}`, c.chatType)
		return json.RawMessage(chatJSON), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *errorBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *errorBotClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

// statusOnlyBotClient stubs only the API calls that IsUserAdmin makes when the
// admin cache is empty and the bot is NOT a chat administrator.  Under that
// condition LoadAdminCache sees a non-admin bot, returns an empty-but-cached
// admin list, and IsUserAdmin falls back to a direct GetChatMemberWithContext
// call for the target user.
//
// Requests handled:
//   - getChat             → supergroup (used by GetChatWithContext inside IsUserAdmin)
//   - getChatMember:<bot> → "member" (bot is NOT admin → triggers the empty-list path)
//   - getChatMember:<uid> → the status registered for that uid in statusMap
type statusOnlyBotClient struct {
	botID     int64
	statusMap map[int64]string // userId → Telegram status string
}

func (c *statusOnlyBotClient) RequestWithContext(
	_ context.Context, _ string, method string,
	params map[string]any, _ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	switch method {
	case "getChat":
		return json.RawMessage(`{"id":-2001,"type":"supergroup","title":"Status Test Chat"}`), nil

	case "getChatMember":
		uidStr := fmt.Sprint(params["user_id"])

		if uidStr == fmt.Sprint(c.botID) {
			return json.RawMessage(fmt.Sprintf(
				`{"status":"member","user":{"id":%d,"is_bot":true,"first_name":"FallbackBot"}}`,
				c.botID,
			)), nil
		}

		for uid, status := range c.statusMap {
			if uidStr == fmt.Sprint(uid) {
				return json.RawMessage(fmt.Sprintf(
					`{"status":%q,"user":{"id":%d,"is_bot":false,"first_name":"TestUser"}}`,
					status, uid,
				)), nil
			}
		}
		return json.RawMessage(`{"status":"member","user":{"id":0,"is_bot":false,"first_name":"Unknown"}}`), nil

	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *statusOnlyBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *statusOnlyBotClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func newStatusBot(botID int64, statusMap map[int64]string) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     fmt.Sprintf("%d:statustest", botID),
		BotClient: &statusOnlyBotClient{botID: botID, statusMap: statusMap},
		User:      gotgbot.User{Id: botID, IsBot: true, FirstName: "StatusBot"},
	}
}

type recordingStatusClient struct {
	inner *statusOnlyBotClient
	calls []string
}

func (c *recordingStatusClient) RequestWithContext(
	ctx context.Context, token, method string,
	params map[string]any, opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	c.calls = append(c.calls, method)
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *recordingStatusClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *recordingStatusClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// NOTE: this test must NOT use t.Parallel() because SetupTestMemoryMarshaler
// writes to package-level globals that are not safe to write concurrently.
func TestIsUserAdminMemberStatuses(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	chatID := int64(-2001)

	tests := []struct {
		name   string
		userID int64
		status string
		want   bool
	}{
		{name: "creator", userID: 201, status: "creator", want: true},
		{name: "administrator", userID: 202, status: "administrator", want: true},
		{name: "member", userID: 203, status: "member", want: false},
		{name: "restricted", userID: 204, status: "restricted", want: false},
		{name: "left", userID: 205, status: "left", want: false},
		{name: "kicked", userID: 206, status: "kicked", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subChatID := chatID - tc.userID
			bot := newStatusBot(8800+tc.userID, map[int64]string{tc.userID: tc.status})

			got := IsUserAdmin(bot, subChatID, tc.userID)
			if got != tc.want {
				t.Errorf("IsUserAdmin(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// Guards against privilege escalation where a channel acting as a group member could be treated as an admin.
func TestIsUserAdminChannelIDReturnsFalse(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	inner := &statusOnlyBotClient{
		botID:     9900,
		statusMap: map[int64]string{},
	}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9900:chantest",
		BotClient: rec,
		User:      gotgbot.User{Id: 9900, IsBot: true, FirstName: "ChanBot"},
	}

	channelIDs := []int64{
		-1001234567890, // standard supergroup-channel ID format
		-1009999999999, // another channel ID
		-1000000000001, // just below the threshold
	}

	for _, chanID := range channelIDs {
		if !IsChannelId(chanID) {
			t.Fatalf("test setup error: %d is not recognised as a channel ID", chanID)
		}

		got := IsUserAdmin(bot, -100555, chanID)
		if got {
			t.Errorf("IsUserAdmin(channelID=%d) = true, want false", chanID)
		}
	}

	if len(rec.calls) != 0 {
		t.Errorf("IsUserAdmin made unexpected API calls for channel IDs: %v", rec.calls)
	}
}

func TestIsUserAdminUsesCache(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const (
		chatID     = int64(-3001)
		adminID    = int64(301)
		nonAdminID = int64(302)
	)

	adminEntry := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: adminID, FirstName: "CachedAdmin"},
	}
	seedCache := cache.AdminCache{
		ChatId:   chatID,
		UserInfo: []gotgbot.MergedChatMember{adminEntry},
		Cached:   true,
	}
	if err := cache.GetMarshal().Set(
		cache.Context,
		fmt.Sprintf("alita:cache:adminCache:%d", chatID),
		seedCache,
	); err != nil {
		t.Fatalf("seeding admin cache: %v", err)
	}

	inner := &statusOnlyBotClient{
		botID:     9901,
		statusMap: map[int64]string{nonAdminID: "member"},
	}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9901:cachetest",
		BotClient: rec,
		User:      gotgbot.User{Id: 9901, IsBot: true, FirstName: "CacheBot"},
	}

	if !IsUserAdmin(bot, chatID, adminID) {
		t.Error("cache HIT: IsUserAdmin(adminID) = false, want true (admin is in seeded cache)")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT: unexpected API calls (should have used cache): %v", rec.calls)
	}

	if IsUserAdmin(bot, chatID, nonAdminID) {
		t.Error("cache HIT: IsUserAdmin(nonAdminID) = true, want false (user not in admin cache)")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT: unexpected API calls for non-admin lookup: %v", rec.calls)
	}

	cache.InvalidateAdminCache(chatID)
	rec.calls = nil

	inner.statusMap[adminID] = "administrator"
	rec.inner = &statusOnlyBotClient{
		botID:     9901,
		statusMap: map[int64]string{adminID: "administrator", nonAdminID: "member"},
	}

	const cacheMissChatID = int64(-3002)
	innerFresh := &statusOnlyBotClient{
		botID:     9902,
		statusMap: map[int64]string{adminID: "administrator", nonAdminID: "member"},
	}
	recFresh := &recordingStatusClient{inner: innerFresh}
	botFresh := &gotgbot.Bot{
		Token:     "9902:freshbot",
		BotClient: recFresh,
		User:      gotgbot.User{Id: 9902, IsBot: true, FirstName: "FreshBot"},
	}

	if !IsUserAdmin(botFresh, cacheMissChatID, adminID) {
		t.Error("cache MISS: IsUserAdmin(adminID) = false, want true (API stub returns administrator)")
	}
	if len(recFresh.calls) == 0 {
		t.Error("cache MISS: expected at least one API call after cache invalidation, got none")
	}
}

func TestIsUserAdminTelegramServiceAccounts(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	inner := &statusOnlyBotClient{
		botID:     9903,
		statusMap: map[int64]string{},
	}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9903:svctest",
		BotClient: rec,
		User:      gotgbot.User{Id: 9903, IsBot: true, FirstName: "SvcBot"},
	}

	serviceIDs := []struct {
		name string
		id   int64
	}{
		{name: "groupAnonymousBot", id: groupAnonymousBot},
		{name: "tgUserId", id: tgUserId},
	}

	for _, svc := range serviceIDs {
		got := IsUserAdmin(bot, -100999, svc.id)
		if !got {
			t.Errorf("IsUserAdmin(%s=%d) = false, want true (Telegram service accounts are always admin)", svc.name, svc.id)
		}
	}

	if len(rec.calls) != 0 {
		t.Errorf("IsUserAdmin made unexpected API calls for service accounts: %v", rec.calls)
	}
}

func TestIsUserAdminInvalidNonChannelUserIDs(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	inner := &statusOnlyBotClient{botID: 9904, statusMap: map[int64]string{}}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9904:invtest",
		BotClient: rec,
		User:      gotgbot.User{Id: 9904, IsBot: true, FirstName: "InvBot"},
	}

	nonChannelInvalidIDs := []int64{0, -1, -999, -100000000000}
	for _, id := range nonChannelInvalidIDs {
		if IsChannelId(id) {
			continue
		}
		if IsUserAdmin(bot, -100888, id) {
			t.Errorf("IsUserAdmin(invalid userId=%d) = true, want false", id)
		}
	}

	if len(rec.calls) != 0 {
		t.Errorf("IsUserAdmin made unexpected API calls for invalid user IDs: %v", rec.calls)
	}
}

func TestIsUserAdminGetChatError(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	bot := &gotgbot.Bot{
		Token:     "9905:errtest",
		BotClient: &errorBotClient{chatErr: true},
		User:      gotgbot.User{Id: 9905, IsBot: true, FirstName: "ErrBot"},
	}

	got := IsUserAdmin(bot, -4001, 42)
	if got {
		t.Error("IsUserAdmin(GetChatWithContext error) = true, want false")
	}
}

func TestIsUserAdminNonGroupChatType(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	bot := &gotgbot.Bot{
		Token:     "9906:privtest",
		BotClient: &errorBotClient{chatErr: false, chatType: "private"},
		User:      gotgbot.User{Id: 9906, IsBot: true, FirstName: "PrivBot"},
	}

	got := IsUserAdmin(bot, -4001, 42)
	if got {
		t.Error("IsUserAdmin(private chat type) = true, want false")
	}
}

func TestIsUserAdminCacheHitLinearScan(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const (
		chatID  = int64(-5001)
		adminID = int64(501)
	)

	adminEntry := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: adminID, FirstName: "LinearAdmin"},
	}
	seedCache := cache.AdminCache{
		ChatId:   chatID,
		UserInfo: []gotgbot.MergedChatMember{adminEntry},
		Cached:   true,
	}
	if err := cache.GetMarshal().Set(
		cache.Context,
		fmt.Sprintf("alita:cache:adminCache:%d", chatID),
		seedCache,
	); err != nil {
		t.Fatalf("seeding admin cache: %v", err)
	}

	inner := &statusOnlyBotClient{botID: 9907, statusMap: map[int64]string{}}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9907:linscan",
		BotClient: rec,
		User:      gotgbot.User{Id: 9907, IsBot: true, FirstName: "LinBot"},
	}

	if !IsUserAdmin(bot, chatID, adminID) {
		t.Error("cache HIT linear scan: IsUserAdmin(adminID) = false, want true")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT linear scan: unexpected API calls: %v", rec.calls)
	}

	if IsUserAdmin(bot, chatID, 502) {
		t.Error("cache HIT linear scan: IsUserAdmin(non-admin) = true, want false")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT linear scan (non-admin): unexpected API calls: %v", rec.calls)
	}
}

// NOTE: must NOT use t.Parallel() at sub-test level because SetupTestMemoryMarshaler
// writes package-level globals that are not concurrency-safe across goroutines.
func TestIsUserAdminFallbackGetChatMemberErrors(t *testing.T) {
	errTests := []struct {
		name   string
		errMsg string
	}{
		{name: "CHAT_ADMIN_REQUIRED", errMsg: "CHAT_ADMIN_REQUIRED"},
		{name: "invalid_user_id", errMsg: "invalid user_id specified"},
		{name: "generic_error", errMsg: "some unexpected API error"},
	}

	for _, tc := range errTests {
		t.Run(tc.name, func(t *testing.T) {
			cache.SetupTestMemoryMarshaler(t)

			chatID := int64(-6000) - int64(len(tc.name))
			userID := int64(601)

			client := &fallbackErrBotClient{
				botID:   int64(9910),
				chatID:  chatID,
				userID:  userID,
				userErr: fmt.Errorf("%s", tc.errMsg),
			}
			bot := &gotgbot.Bot{
				Token:     fmt.Sprintf("9910:%s", tc.name),
				BotClient: client,
				User:      gotgbot.User{Id: 9910, IsBot: true, FirstName: "ErrFallbackBot"},
			}

			got := IsUserAdmin(bot, chatID, userID)
			if got {
				t.Errorf("IsUserAdmin(%s) = true, want false when GetChatMember errors", tc.name)
			}
		})
	}
}

// fallbackErrBotClient handles the three-call sequence in IsUserAdmin's fallback:
//
//  1. getChat → valid supergroup
//  2. getChatMember:<botID> → "member" (bot not admin → triggers empty-list path)
//  3. getChatMember:<userID> → returns the configured error
type fallbackErrBotClient struct {
	botID   int64
	chatID  int64
	userID  int64
	userErr error
}

func (c *fallbackErrBotClient) RequestWithContext(
	_ context.Context, _ string, method string,
	params map[string]any, _ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	switch method {
	case "getChat":
		return json.RawMessage(fmt.Sprintf(
			`{"id":%d,"type":"supergroup","title":"Fallback Err Chat"}`,
			c.chatID,
		)), nil
	case "getChatMember":
		uidStr := fmt.Sprint(params["user_id"])
		if uidStr == fmt.Sprint(c.botID) {
			return json.RawMessage(fmt.Sprintf(
				`{"status":"member","user":{"id":%d,"is_bot":true,"first_name":"ErrFallbackBot"}}`,
				c.botID,
			)), nil
		}
		if uidStr == fmt.Sprint(c.userID) {
			return nil, c.userErr
		}
		return json.RawMessage(`{"status":"member","user":{"id":0}}`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *fallbackErrBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *fallbackErrBotClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

// panickingAdminListClient reports the bot as an admin and panics while
// listing the chat's administrators.
type panickingAdminListClient struct {
	statusOnlyBotClient
}

func (c *panickingAdminListClient) RequestWithContext(
	ctx context.Context, token, method string,
	params map[string]any, opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	switch {
	case method == "getChatAdministrators":
		panic("admin list exploded")
	case method == "getChatMember" && fmt.Sprint(params["user_id"]) == fmt.Sprint(c.botID):
		return json.RawMessage(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":true,"first_name":"Bot"}}`, c.botID)), nil
	}
	return c.statusOnlyBotClient.RequestWithContext(ctx, token, method, params, opts)
}

// A panicking admin-list load must abort IsUserAdmin as before rather than
// read as "not an admin".
func TestIsUserAdminPassesOnAdminListPanic(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	bot := &gotgbot.Bot{
		Token:     "8899:statustest",
		BotClient: &panickingAdminListClient{statusOnlyBotClient{botID: 8899, statusMap: map[int64]string{301: "administrator"}}},
		User:      gotgbot.User{Id: 8899, IsBot: true, FirstName: "StatusBot"},
	}

	recovered := func() (r any) {
		defer func() { r = recover() }()
		IsUserAdmin(bot, -2301, 301)
		return nil
	}()
	if recovered == nil {
		t.Fatal("IsUserAdmin returned after the admin-list load panicked, want the panic passed on")
	}
}

// NOTE: must NOT use t.Parallel() because SetupTestMemoryMarshaler writes to
// package-level globals.
func TestRequireHelpersFollowConnectedChatSwap(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const (
		groupID    = int64(-3101)
		adminID    = int64(311)
		nonAdminID = int64(312)
	)
	adminEntry := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: adminID, FirstName: "ConnectedAdmin"},
	}
	// The bot is absent from the group's admin list, so a bot-admin check
	// that reaches the group fails, while one on the PM passes.
	if err := cache.GetMarshal().Set(
		cache.Context,
		fmt.Sprintf("alita:cache:adminCache:%d", groupID),
		cache.AdminCache{
			ChatId:   groupID,
			UserInfo: []gotgbot.MergedChatMember{adminEntry},
			Cached:   true,
		},
	); err != nil {
		t.Fatalf("seeding admin cache: %v", err)
	}
	t.Cleanup(func() { cache.InvalidateAdminCache(groupID) })

	// getChat answers "private" for every id, so an admin check that runs on
	// the PM instead of the group refuses everyone.
	bot := &gotgbot.Bot{
		Token:     "9911:swaptest",
		BotClient: &errorBotClient{chatType: "private"},
		User:      gotgbot.User{Id: 9911, IsBot: true, FirstName: "SwapBot"},
	}
	group := &gotgbot.Chat{Id: groupID, Type: "supergroup", Title: "Connected"}

	pmCtx := func(userID int64) *ext.Context {
		from := gotgbot.User{Id: userID, FirstName: "PM"}
		msg := &gotgbot.Message{MessageId: 1, Date: 1, Chat: gotgbot.Chat{Id: userID, Type: "private"}, From: &from}
		return ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
	}

	swapped := pmCtx(adminID)
	swapped.EffectiveChat = group
	if got := extractChatFromContext(swapped, nil); got == nil || got.Id != groupID {
		t.Errorf("extractChatFromContext(swapped) = %#v, want connected group %d", got, groupID)
	}
	if !RequireUserAdmin(bot, swapped, nil, adminID) {
		t.Error("RequireUserAdmin(swapped, admin) = false, want true: the user is a cached admin of the connected group")
	}
	if RequireBotAdmin(bot, swapped, nil) {
		t.Error("RequireBotAdmin(swapped) = true, want false: the bot is not an admin of the connected group")
	}

	swappedMember := pmCtx(nonAdminID)
	swappedMember.EffectiveChat = group
	if RequireUserAdmin(bot, swappedMember, nil, nonAdminID) {
		t.Error("RequireUserAdmin(swapped, non-admin) = true, want false")
	}

	explicit := &gotgbot.Chat{Id: -3102, Type: "supergroup"}
	if got := extractChatFromContext(swapped, explicit); got != explicit {
		t.Fatalf("extractChatFromContext(swapped, explicit) = %#v, want the explicit chat", got)
	}

	unswapped := pmCtx(adminID)
	if got := extractChatFromContext(unswapped, nil); got == nil || got.Id != adminID {
		t.Fatalf("extractChatFromContext(unswapped) = %#v, want the PM chat %d", got, adminID)
	}
	if RequireUserAdmin(bot, unswapped, nil, adminID) {
		t.Error("RequireUserAdmin(unswapped PM) = true, want false: the check must stay on the PM")
	}
	if !RequirePrivate(bot, unswapped, nil) {
		t.Error("RequirePrivate(unswapped PM) = false, want true")
	}
}

// A stored list with no admins in it (the bot is not an admin there, or it
// could not check its own status) cannot answer, but reloading it would only
// store the same list again: IsUserAdmin asks Telegram for the user's own
// membership and nothing else until the entry expires or is invalidated.
func TestIsUserAdminCachedListWithoutAdminsAsksOnlyForTheMember(t *testing.T) {
	const (
		adminID  = int64(601)
		memberID = int64(602)
	)
	for name, entry := range map[string]cache.AdminCache{
		"bot is not an admin":     {UserInfo: []gotgbot.MergedChatMember{}, Cached: true, Negative: true},
		"bot status not verified": {UserInfo: []gotgbot.MergedChatMember{}, Cached: true},
	} {
		t.Run(name, func(t *testing.T) {
			cache.SetupTestMemoryMarshaler(t)
			chatID := int64(-6001)
			entry.ChatId = chatID
			if err := cache.GetMarshal().Set(cache.Context, fmt.Sprintf("alita:cache:adminCache:%d", chatID), entry); err != nil {
				t.Fatalf("seeding admin cache: %v", err)
			}
			rec := &recordingStatusClient{inner: &statusOnlyBotClient{
				botID:     9908,
				statusMap: map[int64]string{adminID: "administrator", memberID: "member"},
			}}
			bot := &gotgbot.Bot{
				Token:     "9908:noadmins",
				BotClient: rec,
				User:      gotgbot.User{Id: 9908, IsBot: true, FirstName: "NoAdminsBot"},
			}

			if !IsUserAdmin(bot, chatID, adminID) {
				t.Error("IsUserAdmin(admin) = false, want true from the member lookup")
			}
			if IsUserAdmin(bot, chatID, memberID) {
				t.Error("IsUserAdmin(member) = true, want false")
			}
			if want := []string{"getChatMember", "getChatMember"}; fmt.Sprint(rec.calls) != fmt.Sprint(want) {
				t.Errorf("API calls = %v, want only the users' own lookups %v", rec.calls, want)
			}
		})
	}
}

// privateChatClient answers getChat with a private chat and getChatMember with
// "administrator" for anyone, so a test sees whether IsUserAdmin ever lets a
// member lookup decide in a private chat.
type privateChatClient struct {
	chatID int64
	calls  []string
}

func (c *privateChatClient) RequestWithContext(
	_ context.Context, _ string, method string,
	params map[string]any, _ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	c.calls = append(c.calls, method)
	switch method {
	case "getChat":
		return json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"private","first_name":"PM"}`, c.chatID)), nil
	case "getChatMember":
		return json.RawMessage(fmt.Sprintf(
			`{"status":"administrator","user":{"id":%v,"is_bot":false,"first_name":"TestUser"}}`,
			params["user_id"],
		)), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *privateChatClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *privateChatClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

// /report in a private chat loads the admin list for the user's own ID, which
// stores an empty list there. A private chat has no admins: IsUserAdmin keeps
// answering false from getChat and never asks for the user's membership.
func TestIsUserAdminCachedEmptyListInPrivateChatIsNotAdmin(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	const userID = int64(603)
	entry := cache.AdminCache{ChatId: userID, UserInfo: []gotgbot.MergedChatMember{}, Cached: true, Negative: true}
	if err := cache.GetMarshal().Set(cache.Context, fmt.Sprintf("alita:cache:adminCache:%d", userID), entry); err != nil {
		t.Fatalf("seeding admin cache: %v", err)
	}
	rec := &privateChatClient{chatID: userID}
	bot := &gotgbot.Bot{
		Token:     "9909:private",
		BotClient: rec,
		User:      gotgbot.User{Id: 9909, IsBot: true, FirstName: "PrivateBot"},
	}

	if IsUserAdmin(bot, userID, userID) {
		t.Error("IsUserAdmin(user, own private chat) = true, want false")
	}
	for _, call := range rec.calls {
		if call == "getChatMember" {
			t.Errorf("API calls = %v, want no getChatMember in a private chat", rec.calls)
			break
		}
	}
}

//go:build testtools

package chat_status

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// errorBotClient is a bot client that returns an error for getChat requests,
// used to exercise the GetChatWithContext error path inside IsUserAdmin.
type errorBotClient struct {
	chatErr bool // if true, getChat returns an error
	chatType string // if non-empty and chatErr is false, getChat returns this type
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

		// Bot is not an admin → LoadAdminCache will return an empty cached list,
		// which triggers the direct-GetChatMember fallback in IsUserAdmin.
		if uidStr == fmt.Sprint(c.botID) {
			return json.RawMessage(fmt.Sprintf(
				`{"status":"member","user":{"id":%d,"is_bot":true,"first_name":"FallbackBot"}}`,
				c.botID,
			)), nil
		}

		// Resolve the requested user ID to the stubbed status.
		for uid, status := range c.statusMap {
			if uidStr == fmt.Sprint(uid) {
				return json.RawMessage(fmt.Sprintf(
					`{"status":%q,"user":{"id":%d,"is_bot":false,"first_name":"TestUser"}}`,
					status, uid,
				)), nil
			}
		}
		// Unknown user → treat as regular member.
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

// recordingClient wraps statusOnlyBotClient and tracks every method called.
// Used by TestIsUserAdminChannelIDReturnsFalse to assert no API call is made.
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

// TestIsUserAdminMemberStatuses verifies the documented contract:
//   - "creator" and "administrator" → IsUserAdmin returns true
//   - "member", "restricted", "left", "kicked" → IsUserAdmin returns false
//
// The test forces the GetChatMember fallback path inside IsUserAdmin by
// using a bot that is not a chat administrator (so LoadAdminCache returns an
// empty list) and then seeding the response for the target user.
//
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
			// Each sub-test uses a unique chatID so cache entries do not interfere.
			subChatID := chatID - tc.userID
			bot := newStatusBot(8800+tc.userID, map[int64]string{tc.userID: tc.status})

			got := IsUserAdmin(bot, subChatID, tc.userID)
			if got != tc.want {
				t.Errorf("IsUserAdmin(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// TestIsUserAdminChannelIDReturnsFalse asserts that IsUserAdmin short-circuits
// to false for channel IDs (< -1000000000000) without making any Telegram API
// call.  This guards against the privilege-escalation class of bugs where a
// channel acting as a group member could be treated as an admin.
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
		-1001234567890,  // standard supergroup-channel ID format
		-1009999999999,  // another channel ID
		-1000000000001,  // just below the threshold
	}

	for _, chanID := range channelIDs {
		// Confirm the ID is indeed recognised as a channel ID.
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

// TestIsUserAdminUsesCache verifies two things:
//
//  1. Cache HIT: when the admin cache is pre-populated with a user's entry,
//     IsUserAdmin returns true without touching the Telegram API.
//
//  2. Cache MISS: after the cache is invalidated, IsUserAdmin falls back to the
//     live API (stubbed here) and returns the correct value.
func TestIsUserAdminUsesCache(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const (
		chatID     = int64(-3001)
		adminID    = int64(301)
		nonAdminID = int64(302)
	)

	// ── Part 1: cache HIT ────────────────────────────────────────────────────

	// Seed the admin cache directly, bypassing the API entirely.
	adminEntry := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: adminID, FirstName: "CachedAdmin"},
	}
	seedCache := cache.AdminCache{
		ChatId:   chatID,
		UserInfo: []gotgbot.MergedChatMember{adminEntry},
		UserMap:  map[int64]gotgbot.MergedChatMember{adminID: adminEntry},
		Cached:   true,
	}
	if err := cache.GetMarshal().Set(
		cache.Context,
		fmt.Sprintf("alita:adminCache:%d", chatID),
		seedCache,
	); err != nil {
		t.Fatalf("seeding admin cache: %v", err)
	}

	// A recording bot that must NOT be called during the cache-hit phase.
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

	// A non-admin user should return false via the cache as well.
	if IsUserAdmin(bot, chatID, nonAdminID) {
		t.Error("cache HIT: IsUserAdmin(nonAdminID) = true, want false (user not in admin cache)")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT: unexpected API calls for non-admin lookup: %v", rec.calls)
	}

	// ── Part 2: cache MISS (invalidated → falls back to API) ─────────────────

	cache.InvalidateAdminCache(chatID)
	rec.calls = nil // reset the call log

	// After invalidation the cache miss triggers: GetChatWithContext →
	// LoadAdminCache (bot is non-admin → empty list) → GetChatMemberWithContext.
	// The stub returns "administrator" for adminID.
	inner.statusMap[adminID] = "administrator"
	rec.inner = &statusOnlyBotClient{
		botID:     9901,
		statusMap: map[int64]string{adminID: "administrator", nonAdminID: "member"},
	}

	// Use a unique chatID to avoid any lingering cache from part 1.
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

// TestIsUserAdminTelegramServiceAccounts verifies that the special Telegram
// service account IDs (groupAnonymousBot and tgUserId) are always treated as
// admins regardless of what the cache or API would return.
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

	// Service accounts are handled before any cache or API lookup.
	if len(rec.calls) != 0 {
		t.Errorf("IsUserAdmin made unexpected API calls for service accounts: %v", rec.calls)
	}
}

// TestIsUserAdminInvalidNonChannelUserIDs covers the warning branch for IDs that
// are ≤ 0 but not channel IDs (i.e. zero and small-negative group IDs).
// This exercises the else-if branch at chat_status.go:212.
func TestIsUserAdminInvalidNonChannelUserIDs(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	inner := &statusOnlyBotClient{botID: 9904, statusMap: map[int64]string{}}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9904:invtest",
		BotClient: rec,
		User:      gotgbot.User{Id: 9904, IsBot: true, FirstName: "InvBot"},
	}

	// These IDs are ≤ 0 but are NOT channel IDs (not < -1000000000000),
	// so they trigger the Warning branch rather than the channel-ID Debug branch.
	nonChannelInvalidIDs := []int64{0, -1, -999, -100000000000}
	for _, id := range nonChannelInvalidIDs {
		if IsChannelId(id) {
			// skip any that happen to be channel IDs to keep the branch distinct
			continue
		}
		if IsUserAdmin(bot, -100888, id) {
			t.Errorf("IsUserAdmin(invalid userId=%d) = true, want false", id)
		}
	}

	// The guard fires before any API call.
	if len(rec.calls) != 0 {
		t.Errorf("IsUserAdmin made unexpected API calls for invalid user IDs: %v", rec.calls)
	}
}

// TestIsUserAdminGetChatError covers the error path when GetChatWithContext
// fails (cache miss scenario where the API is unreachable).
// Exercises chat_status.go:251-258.
func TestIsUserAdminGetChatError(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	bot := &gotgbot.Bot{
		Token:     "9905:errtest",
		BotClient: &errorBotClient{chatErr: true},
		User:      gotgbot.User{Id: 9905, IsBot: true, FirstName: "ErrBot"},
	}

	// Cache is empty so IsUserAdmin will try GetChatWithContext which returns an error.
	got := IsUserAdmin(bot, -4001, 42)
	if got {
		t.Error("IsUserAdmin(GetChatWithContext error) = true, want false")
	}
}

// TestIsUserAdminNonGroupChatType covers the guard at chat_status.go:261-263
// where the function returns false if the chat is not a group or supergroup.
func TestIsUserAdminNonGroupChatType(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	// Use a bot whose getChat returns a "private" type — the guard fires immediately.
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

// TestIsUserAdminCacheHitLinearScan covers the backwards-compatibility linear
// scan path (chat_status.go:241-246) where a cached AdminCache has a nil
// UserMap and must be searched via UserInfo slice.
func TestIsUserAdminCacheHitLinearScan(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const (
		chatID  = int64(-5001)
		adminID = int64(501)
	)

	// Seed cache WITHOUT a UserMap (nil) to trigger the linear scan path.
	adminEntry := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: adminID, FirstName: "LinearAdmin"},
	}
	seedCache := cache.AdminCache{
		ChatId:   chatID,
		UserInfo: []gotgbot.MergedChatMember{adminEntry},
		UserMap:  nil, // explicitly nil to force linear scan
		Cached:   true,
	}
	if err := cache.GetMarshal().Set(
		cache.Context,
		fmt.Sprintf("alita:adminCache:%d", chatID),
		seedCache,
	); err != nil {
		t.Fatalf("seeding admin cache without UserMap: %v", err)
	}

	inner := &statusOnlyBotClient{botID: 9907, statusMap: map[int64]string{}}
	rec := &recordingStatusClient{inner: inner}
	bot := &gotgbot.Bot{
		Token:     "9907:linscan",
		BotClient: rec,
		User:      gotgbot.User{Id: 9907, IsBot: true, FirstName: "LinBot"},
	}

	// Cache hit with linear scan → should find the admin and return true.
	if !IsUserAdmin(bot, chatID, adminID) {
		t.Error("cache HIT linear scan: IsUserAdmin(adminID) = false, want true")
	}
	// No API call should have been made.
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT linear scan: unexpected API calls: %v", rec.calls)
	}

	// A user NOT in the UserInfo list must return false.
	if IsUserAdmin(bot, chatID, 502) {
		t.Error("cache HIT linear scan: IsUserAdmin(non-admin) = true, want false")
	}
	if len(rec.calls) != 0 {
		t.Errorf("cache HIT linear scan (non-admin): unexpected API calls: %v", rec.calls)
	}
}

// TestIsUserAdminFallbackGetChatMemberErrors covers the error sub-paths inside
// the GetChatMember fallback (chat_status.go:292-321): CHAT_ADMIN_REQUIRED,
// invalid user_id, and an unexpected generic error must all return false without
// panicking.
//
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

			// Use a unique chatID per sub-test to avoid cache bleed.
			chatID := int64(-6000) - int64(len(tc.name))
			userID := int64(601)

			// A client where:
			//   getChat → valid supergroup (so IsUserAdmin proceeds past type guard)
			//   getChatMember:<botID> → "member" (bot not admin → LoadAdminCache returns empty)
			//   getChatMember:<userID> → error
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
	botID  int64
	chatID int64
	userID int64
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
			// Bot is not an admin → LoadAdminCache returns empty list.
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

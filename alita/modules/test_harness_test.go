package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// withNilCacheMarshal clears the cache marshaler for the duration of a test.
func withNilCacheMarshal(t *testing.T) {
	t.Helper()

	previousMarshal := cache.GetMarshal()
	cache.SetMarshal(nil)
	t.Cleanup(func() {
		cache.SetMarshal(previousMarshal)
	})
}

type moduleBotCall struct {
	Method string
	Params map[string]any
}

type moduleBotClient struct {
	mu        sync.Mutex
	calls     []moduleBotCall
	responses map[string]json.RawMessage
	errors    map[string]error
	// errorsByChat fails a method for one chat_id, keyed "method|chat_id".
	errorsByChat map[string]error
	// adminsByChat overrides getChatAdministrators per chat_id.
	adminsByChat map[string]json.RawMessage
	// membersByUser overrides getChatMember per user_id.
	membersByUser map[string]json.RawMessage
	// membersByChat overrides getChatMember for one member of one chat, keyed
	// "chat_id|user_id"; it wins over every other getChatMember answer.
	membersByChat map[string]json.RawMessage
	// chatsByID overrides getChat per chat_id.
	chatsByID map[string]json.RawMessage
}

func newModuleTestBot(client *moduleBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "AlitaTestBot",
		},
	}
}

func newModuleBotClient() *moduleBotClient {
	return &moduleBotClient{
		responses: map[string]json.RawMessage{
			"sendMessage": json.RawMessage(
				`{"message_id":9001,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"}}`,
			),
			"editMessageText": json.RawMessage(
				`{"message_id":9001,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"}}`,
			),
			"deleteMessage":          json.RawMessage(`true`),
			"banChatMember":          json.RawMessage(`true`),
			"banChatSenderChat":      json.RawMessage(`true`),
			"restrictChatMember":     json.RawMessage(`true`),
			"unbanChatMember":        json.RawMessage(`true`),
			"unbanChatSenderChat":    json.RawMessage(`true`),
			"leaveChat":              json.RawMessage(`true`),
			"approveChatJoinRequest": json.RawMessage(`true`),
			"declineChatJoinRequest": json.RawMessage(`true`),
			"pinChatMessage":         json.RawMessage(`true`),
			"unpinChatMessage":       json.RawMessage(`true`),
			"unpinAllChatMessages":   json.RawMessage(`true`),
			"getChatMemberCount":     json.RawMessage(`17`),
			"sendDocument": json.RawMessage(
				`{"message_id":9002,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"},"document":{"file_id":"doc-1","file_unique_id":"doc-u1","file_name":"chatlist.txt"}}`,
			),
			"sendPhoto": json.RawMessage(
				`{"message_id":9003,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"},"photo":[{"file_id":"photo-1","file_unique_id":"photo-u1","width":160,"height":80}]}`,
			),
			"answerCallbackQuery": json.RawMessage(`true`),
			"getMe": json.RawMessage(
				`{"id":999,"is_bot":true,"first_name":"Alita","username":"AlitaTestBot"}`,
			),
			"getChat": json.RawMessage(
				`{"id":-1001,"type":"supergroup","title":"Test Chat"}`,
			),
			"getChatMember": json.RawMessage(
				`{"status":"member","user":{"id":42,"is_bot":false,"first_name":"Member"}}`,
			),
			"getChatAdministrators": json.RawMessage(
				`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_pin_messages":true,"can_delete_messages":true,"can_restrict_members":true,"can_promote_members":true,"can_change_info":true,"can_invite_users":true,"can_manage_chat":true}]`,
			),
		},
		errors: make(map[string]error),
	}
}

func (c *moduleBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	c.calls = append(c.calls, moduleBotCall{Method: method, Params: copied})

	if err := c.errors[method]; err != nil {
		return nil, err
	}
	if err := c.errorsByChat[method+"|"+fmt.Sprint(params["chat_id"])]; err != nil {
		return nil, err
	}
	if method == "getChatMember" {
		if response, ok := c.membersByChat[fmt.Sprint(params["chat_id"])+"|"+fmt.Sprint(params["user_id"])]; ok {
			return response, nil
		}
	}
	if method == "getChatMember" && fmt.Sprint(params["user_id"]) == "999" {
		return json.RawMessage(
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_pin_messages":true,"can_delete_messages":true,"can_restrict_members":true,"can_promote_members":true,"can_change_info":true,"can_invite_users":true,"can_manage_chat":true}`,
		), nil
	}
	if method == "getChatMember" && fmt.Sprint(params["user_id"]) == "777000" {
		return json.RawMessage(
			`{"status":"creator","user":{"id":777000,"is_bot":false,"first_name":"Telegram"}}`,
		), nil
	}
	if method == "getChatMember" && fmt.Sprint(params["user_id"]) == "13" {
		return json.RawMessage(
			`{"status":"left","user":{"id":13,"is_bot":false,"first_name":"Left User"}}`,
		), nil
	}
	if method == "getChatMember" && fmt.Sprint(params["user_id"]) == "14" {
		return json.RawMessage(
			`{"status":"kicked","user":{"id":14,"is_bot":false,"first_name":"Kicked User"}}`,
		), nil
	}
	if method == "getChatMember" {
		if response, ok := c.membersByUser[fmt.Sprint(params["user_id"])]; ok {
			return response, nil
		}
	}
	if method == "getChatAdministrators" {
		if response, ok := c.adminsByChat[fmt.Sprint(params["chat_id"])]; ok {
			return response, nil
		}
	}
	if method == "getChat" {
		if response, ok := c.chatsByID[fmt.Sprint(params["chat_id"])]; ok {
			return response, nil
		}
	}
	if response, ok := c.responses[method]; ok {
		return response, nil
	}
	return json.RawMessage(`true`), nil
}

func (c *moduleBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *moduleBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func (c *moduleBotClient) callsFor(method string) []moduleBotCall {
	c.mu.Lock()
	defer c.mu.Unlock()

	var calls []moduleBotCall
	for _, call := range c.calls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}

// onlyCall fails unless method was called exactly once, and returns that call.
func (c *moduleBotClient) onlyCall(t *testing.T, method string) moduleBotCall {
	t.Helper()
	calls := c.callsFor(method)
	if len(calls) != 1 {
		t.Fatalf("%s calls = %d, want 1", method, len(calls))
	}
	return calls[0]
}

// assertParams fails unless each wanted param prints the same as the sent one.
func assertParams(t *testing.T, call moduleBotCall, want map[string]any) {
	t.Helper()
	for key, value := range want {
		got, ok := call.Params[key]
		if !ok {
			t.Errorf("%s: param %q missing, want %v", call.Method, key, value)
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(value) {
			t.Errorf("%s: param %q = %v, want %v", call.Method, key, got, value)
		}
	}
}

// assertDeleted fails unless deleteMessage was sent for each id in chatID.
func assertDeleted(t *testing.T, client *moduleBotClient, chatID int64, ids ...int64) {
	t.Helper()
	deleted := map[string]bool{}
	for _, call := range client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			deleted[fmt.Sprint(call.Params["message_id"])] = true
		}
	}
	for _, id := range ids {
		if !deleted[fmt.Sprint(id)] {
			t.Errorf("message %d in chat %d was not deleted; deleted %v", id, chatID, deleted)
		}
	}
}

// targetParams is the chat_id/user_id pair a member-targeting call must carry.
func targetParams(chatID, userID int64) map[string]any {
	return map[string]any{"chat_id": chatID, "user_id": userID}
}

// adminChatFixture is a supergroup whose admin is an ordinary user ID. The
// admin passes permission checks only through the real admin-cache load
// (getChatMember for the bot, then getChatAdministrators), unlike 777000 and
// 1087968824, which chat_status treats as admins without any lookup.
type adminChatFixture struct {
	client *moduleBotClient
	bot    *gotgbot.Bot
	chat   gotgbot.Chat
	admin  gotgbot.User
	// member is a plain member, absent from the admin list.
	member gotgbot.User
}

const fixtureAdminRights = `"can_manage_chat":true,"can_delete_messages":true,"can_restrict_members":true,` +
	`"can_promote_members":true,"can_change_info":true,"can_invite_users":true,"can_pin_messages":true`

// newAdminChatFixture lists a unique admin with every right next to the bot.
func newAdminChatFixture(t *testing.T) adminChatFixture {
	t.Helper()
	client := newModuleBotClient()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Fixture Chat"}
	admin := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	client.adminsByChat = map[string]json.RawMessage{}
	client.membersByUser = map[string]json.RawMessage{}
	client.chatsByID = map[string]json.RawMessage{
		fmt.Sprint(chat.Id): json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, chat.Id, chat.Title)),
	}
	t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })

	f := adminChatFixture{client: client, bot: newModuleTestBot(client), chat: chat, admin: admin, member: member}
	f.setAdminStatus("administrator")
	return f
}

// makeOwner turns the fixture's admin into the chat creator. Call it before
// the first permission check, which caches the admin list.
func (f adminChatFixture) makeOwner() {
	f.setAdminStatus("creator")
}

// addAdmins appends raw ChatMember JSON entries to the chat's admin list. Call
// it after makeOwner, which rewrites the list.
func (f adminChatFixture) addAdmins(entries ...string) {
	key := fmt.Sprint(f.chat.Id)
	list := strings.TrimSuffix(string(f.client.adminsByChat[key]), "]")
	f.client.adminsByChat[key] = json.RawMessage(list + "," + strings.Join(entries, ",") + "]")
}

func (f adminChatFixture) setAdminStatus(status string) {
	adminJSON := fmt.Sprintf(`{"status":%q,"user":{"id":%d,"is_bot":false,"first_name":"Admin"},%s}`, status, f.admin.Id, fixtureAdminRights)
	botJSON := `{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},` + fixtureAdminRights + `}`
	f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + botJSON + "," + adminJSON + "]")
	f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(adminJSON)
}

// connectAdmin connects the fixture's admin to the chat, as /connect does,
// and returns the admin's PM, where connected-chat commands are sent.
func (f adminChatFixture) connectAdmin(t *testing.T) gotgbot.Chat {
	t.Helper()
	if err := connections.ConnectId(f.admin.Id, f.chat.Id); err != nil {
		t.Fatalf("ConnectId(%d, %d) error = %v", f.admin.Id, f.chat.Id, err)
	}
	t.Cleanup(func() {
		if err := connections.DisconnectId(f.admin.Id); err != nil {
			t.Errorf("DisconnectId(%d) error = %v", f.admin.Id, err)
		}
	})
	return privateChatOf(f.admin)
}

func newModuleMessageContext(bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User, text string) *ext.Context {
	msg := &gotgbot.Message{
		MessageId: 101,
		Date:      1,
		Chat:      chat,
		From:      &from,
		Text:      text,
	}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)
}

func newModuleCallbackContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	from gotgbot.User,
	data string,
) *ext.Context {
	msg := &gotgbot.Message{
		MessageId: 102,
		Date:      1,
		Chat:      chat,
		From:      &from,
		Text:      "callback source",
	}
	// Match gotgbot's JSON decoder, which stores Message as a value.
	query := &gotgbot.CallbackQuery{
		Id:           "callback-1",
		From:         from,
		Message:      *msg,
		Data:         data,
		ChatInstance: "test-chat-instance",
	}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 2, CallbackQuery: query}, nil)
}

type moduleMemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
	ttls map[string]time.Time
}

func newModuleMemoryStore() *moduleMemoryStore {
	return &moduleMemoryStore{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

func (m *moduleMemoryStore) Get(_ context.Context, key any) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	if expiry, ok := m.ttls[k]; ok && time.Now().After(expiry) {
		return nil, fmt.Errorf("key expired")
	}
	v, ok := m.data[k]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return v, nil
}

func (m *moduleMemoryStore) GetWithTTL(_ context.Context, key any) (any, time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	v, ok := m.data[k]
	if !ok {
		return nil, 0, fmt.Errorf("key not found")
	}
	var ttl time.Duration
	if expiry, ok := m.ttls[k]; ok {
		ttl = time.Until(expiry)
		if ttl < 0 {
			return nil, 0, fmt.Errorf("key expired")
		}
	}
	return v, ttl, nil
}

func (m *moduleMemoryStore) Set(_ context.Context, key, value any, options ...store.Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	opts := store.ApplyOptions(options...)
	switch v := value.(type) {
	case []byte:
		m.data[k] = v
	case string:
		m.data[k] = []byte(v)
	default:
		return fmt.Errorf("unsupported value type %T", value)
	}
	if opts.Expiration > 0 {
		m.ttls[k] = time.Now().Add(opts.Expiration)
	} else {
		delete(m.ttls, k)
	}
	return nil
}

func (m *moduleMemoryStore) Delete(_ context.Context, key any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	delete(m.data, k)
	delete(m.ttls, k)
	return nil
}

func (m *moduleMemoryStore) Invalidate(_ context.Context, _ ...store.InvalidateOption) error {
	return nil
}

func (m *moduleMemoryStore) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	return nil
}

func (m *moduleMemoryStore) GetType() string {
	return "memory"
}

func TestMain(m *testing.M) {
	var dbFileName string
	cache.SetMarshal(marshaler.New(gocache.New[any](newModuleMemoryStore())))
	if db.DB == nil {
		dbFile, err := os.CreateTemp("", "alita_modules_test_*.db")
		if err != nil {
			fmt.Printf("temp file creation failed: %v\n", err)
			os.Exit(1)
		}
		dbFileName = dbFile.Name()
		if closeErr := dbFile.Close(); closeErr != nil {
			fmt.Printf("temp file close failed: %v\n", closeErr)
			os.Exit(1)
		}

		sqliteDB, err := gorm.Open(sqlite.Open(dbFileName+"?_busy_timeout=10000&_journal_mode=WAL"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			fmt.Printf("SQLite init failed: %v\n", err)
			os.Exit(1)
		}
		db.DB = sqliteDB
	}

	if err := db.DB.AutoMigrate(
		&db.User{},
		&db.Chat{},
		&models.ConnectionSettings{},
		&models.ConnectionChatSettings{},
		&models.AdminSettings{},
		&models.DisableSettings{},
		&models.DisableChatSettings{},
		&models.RulesSettings{},
		&models.PinSettings{},
		&models.WarnSettings{},
		&models.Warns{},
		&models.WarnEntry{},
		&db.NotesSettings{},
		&db.Notes{},
		&models.GreetingSettings{},
		&db.CaptchaSettings{},
		&db.CaptchaAttempts{},
		&models.StoredMessages{},
		&models.CaptchaMutedUsers{},
		&db.ApprovedUsers{},
		&db.ChatFilters{},
		&models.BlacklistSettings{},
		&db.LockSettings{},
		&models.ChannelSettings{},
		&models.ReportChatSettings{},
		&models.ReportUserSettings{},
		&models.Report{},
		&models.ReportAutoMute{},
		&models.ModerationEvent{},
		&db.AntifloodSettings{},
		&models.AntiRaidSettings{},
		&db.DevSettings{},
		&models.Reactions{},
		&models.Federation{},
		&models.FederationAdmin{},
		&models.FederationChat{},
		&models.FederationBan{},
		&models.FederationSub{},
		&models.LogChannel{},
		&models.AISpamSettings{},
		&models.SpamDBSettings{},
		&models.ChatDailyStat{},
		&models.ChatHourlyStat{},
		&models.ChatMemberDailyStat{},
		&models.DigestSettings{},
	); err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	if db.DB != nil {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if dbFileName != "" {
		_ = os.Remove(dbFileName)
	}

	os.Exit(exitCode)
}

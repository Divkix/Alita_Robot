package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

type moduleBotCall struct {
	Method string
	Params map[string]any
}

type moduleBotClient struct {
	mu        sync.Mutex
	calls     []moduleBotCall
	responses map[string]json.RawMessage
	errors    map[string]error
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
			"approveChatJoinRequest": json.RawMessage(`true`),
			"declineChatJoinRequest": json.RawMessage(`true`),
			"pinChatMessage":         json.RawMessage(`true`),
			"unpinChatMessage":       json.RawMessage(`true`),
			"unpinAllChatMessages":   json.RawMessage(`true`),
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
				`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}]`,
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
	query := &gotgbot.CallbackQuery{
		Id:           "callback-1",
		From:         from,
		Message:      msg,
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
	cache.Marshal = marshaler.New(gocache.New[any](newModuleMemoryStore()))
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
		&db.ConnectionSettings{},
		&db.ConnectionChatSettings{},
		&db.DisableSettings{},
		&db.DisableChatSettings{},
		&db.RulesSettings{},
		&db.PinSettings{},
		&db.WarnSettings{},
		&db.Warns{},
		&db.NotesSettings{},
		&db.Notes{},
		&db.GreetingSettings{},
		&db.CaptchaSettings{},
		&db.CaptchaAttempts{},
		&db.ApprovedUsers{},
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

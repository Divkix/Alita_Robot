package alita

import (
	"reflect"
	"slices"
	"testing"
	"unsafe"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/modules"
)

type watcherContract struct {
	Group int
	Kind  string
	Count int
}

type productContract struct {
	Commands           []string
	CallbackNamespaces []string
	Watchers           []watcherContract
	HelpModules        []string
}

func TestProductContract(t *testing.T) {
	resetHelpRegistryForTest(t)

	bot, err := gotgbot.NewBot("999:test", &gotgbot.BotOpts{BotClient: alitaTestBotClient{}})
	if err != nil {
		t.Fatalf("create fake Telegram bot: %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadModules(dispatcher)

	want := productContract{
		Commands: []string{
			"addblacklist", "addfilter", "addnote", "addreaction", "antiraid", "approval",
			"approve", "approved", "autoantiraid", "ban",
			"blacklistaction", "blacklists", "chatinfo",
			"chatlist", "checknsfw", "cleanservice", "cleanwelcome", "clear", "clearall", "connect",
			"connection", "curse", "delflood", "disconnect", "export",
			"filters", "flood", "formatting", "help", "id", "import",
			"info", "kick", "kickme", "leavechat", "mute", "notes", "nsfw", "ping", "purge",
			"raidactiontime", "raidtime", "reactions",
			"removereaction", "report", "reset", "resetallwarns", "resetwarns", "rmallbl",
			"rmblacklist", "rmfilter", "rmnote", "rmwarn", "setflood", "setfloodmode", "setwarnlimit",
			"setwelcome", "start", "stat", "stats",
			"tell", "unapprove", "unban", "unmute", "warn",
			"warnings", "warns", "welcome",
		},
		CallbackNamespaces: []string{
			"antiraid", "backup", "connbtns", "filters_overwrite", "formatting", "helpq",
			"notes.overwrite",
			"rmAllBlacklist", "rmAllChatWarns", "rmAllNotes", "rmWarn",
		},
		Watchers: []watcherContract{
			{Group: -5, Kind: "message", Count: 1},
			{Group: -1, Kind: "chat_member", Count: 1},
			{Group: -1, Kind: "message", Count: 1},
			{Group: -1, Kind: "my_chat_member", Count: 1},
			{Group: 0, Kind: "chat_member", Count: 2},
			{Group: 0, Kind: "message", Count: 2},
			{Group: 4, Kind: "message", Count: 1},
			{Group: 7, Kind: "message", Count: 1},
			{Group: 8, Kind: "message", Count: 1},
			{Group: 9, Kind: "message", Count: 1},
		},
		HelpModules: []string{
			"AntiRaid", "Antiflood", "Approvals", "Backup", "Bans", "Blacklists",
			"Connections", "Filters", "Formatting", "Greetings", "Misc", "Mutes",
			"NSFW", "Notes", "Purges", "Reactions", "Reports", "Warns",
		},
	}

	got := inspectProductContract(t, bot, dispatcher, want.CallbackNamespaces)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("product contract mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func inspectProductContract(
	t *testing.T,
	bot *gotgbot.Bot,
	dispatcher *ext.Dispatcher,
	callbackCandidates []string,
) productContract {
	t.Helper()

	contract := productContract{}
	watcherCounts := make(map[struct {
		group int
		kind  string
	}]int)
	callbackHandlers := make([]ext.Handler, 0)

	for group, groupHandlers := range dispatcherHandlers(dispatcher) {
		for _, handler := range groupHandlers {
			switch handler := handler.(type) {
			case handlers.Command:
				contract.Commands = append(contract.Commands, handler.Command)
			case handlers.CallbackQuery:
				callbackHandlers = append(callbackHandlers, handler)
			case handlers.Message:
				watcherCounts[struct {
					group int
					kind  string
				}{group, "message"}]++
			case handlers.ChatMember:
				watcherCounts[struct {
					group int
					kind  string
				}{group, "chat_member"}]++
			case handlers.MyChatMember:
				watcherCounts[struct {
					group int
					kind  string
				}{group, "my_chat_member"}]++
			case handlers.ChatJoinRequest:
				watcherCounts[struct {
					group int
					kind  string
				}{group, "chat_join_request"}]++
			default:
				t.Fatalf("unclassified dispatcher handler %T in group %d", handler, group)
			}
		}
	}

	for key, count := range watcherCounts {
		contract.Watchers = append(contract.Watchers, watcherContract{
			Group: key.group,
			Kind:  key.kind,
			Count: count,
		})
	}

	for _, namespace := range callbackCandidates {
		ctx := ext.NewContext(bot, &gotgbot.Update{CallbackQuery: &gotgbot.CallbackQuery{
			Id:   "contract-query",
			From: gotgbot.User{Id: 1, FirstName: "Contract"},
			Data: namespace + "|v1|",
		}}, nil)
		matches := 0
		for _, handler := range callbackHandlers {
			if handler.CheckUpdate(bot, ctx) {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("callback namespace %q matched %d handlers, want 1", namespace, matches)
			continue
		}
		contract.CallbackNamespaces = append(contract.CallbackNamespaces, namespace)
	}
	if len(callbackHandlers) != len(contract.CallbackNamespaces) {
		t.Errorf(
			"dispatcher has %d callback handlers, but the contract identified %d namespaces",
			len(callbackHandlers),
			len(contract.CallbackNamespaces),
		)
	}

	for module, enabled := range modules.DefaultHelpRegistry().AbleMap {
		if enabled {
			contract.HelpModules = append(contract.HelpModules, module)
		}
	}

	slices.Sort(contract.Commands)
	slices.Sort(contract.CallbackNamespaces)
	slices.SortFunc(contract.Watchers, func(a, b watcherContract) int {
		if a.Group != b.Group {
			return a.Group - b.Group
		}
		return compareStrings(a.Kind, b.Kind)
	})
	slices.Sort(contract.HelpModules)
	return contract
}

func dispatcherHandlers(dispatcher *ext.Dispatcher) map[int][]ext.Handler {
	// gotgbot deliberately keeps its handler mapping private and provides no read API.
	// Keep this dependency-specific inspection at the high-level test seam so runtime
	// registration remains untouched and a dependency change fails in one obvious place.
	dispatcherValue := reflect.ValueOf(dispatcher).Elem()
	mappingField := dispatcherValue.FieldByName("handlers")
	mappingValue := reflect.NewAt(mappingField.Type(), unsafe.Pointer(mappingField.UnsafeAddr())).Elem()
	handlersField := mappingValue.FieldByName("handlers")
	return *(*map[int][]ext.Handler)(unsafe.Pointer(handlersField.UnsafeAddr()))
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

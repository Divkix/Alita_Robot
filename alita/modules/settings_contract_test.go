//go:build testtools

package modules

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/panel"
)

type settingContractEdge struct {
	value  any
	input  string
	label  string
	accept bool
}

type settingContract struct {
	key          string
	command      string
	defaultValue any
	defaultLabel string
	statusToken  string
	refusalKey   string
	seedInvalid  func(int64) error
	edges        []settingContractEdge
	button       string
	noPanel      bool
	changeInfo   bool
	botAdmin     bool
	statusOn     string
	statusOff    string
	setOn        string
	setOff       string
}

// Expectations come from the command contract, not the declarations. Later
// settings add rows here, including their optional button input and side effect.
func TestDeclaredSettingsAcrossSurfaces(t *testing.T) {
	useRealLocales(t)
	for _, setting := range []settingContract{
		{
			key: "antiraid.raid_action_time", command: "/raidactiontime",
			botAdmin:     true,
			defaultValue: 3600, defaultLabel: "1h", statusToken: "Action: ",
			seedInvalid: func(chatID int64) error { return antiraid.SetRaidActionTime(chatID, 5) },
			refusalKey:  "antiraid_invalid_duration",
			edges: []settingContractEdge{
				{29, "29s", "29s", false},
				{30, "30s", "30s", true},
				{31622400, "366d", "366d", true},
				{31622401, "31622401s", "31622401s", false},
			},
		},
		{
			key: "antiraid.raid_time", command: "/raidtime",
			botAdmin:     true,
			defaultValue: 21600, defaultLabel: "6h", statusToken: "Raid: ",
			refusalKey: "antiraid_invalid_duration",
			seedInvalid: func(chatID int64) error {
				if err := antiraid.SetRaidTime(chatID, 60); err != nil {
					return err
				}
				return antiraid.SetRaidTime(chatID, 0)
			},
			edges: []settingContractEdge{
				{0, "0s", "0s", false},
				{1, "1s", "1s", true},
				{31622400, "366d", "366d", true},
				{31622401, "31622401s", "31622401s", false},
			},
		},
		{
			key: "antiraid.auto_threshold", command: "/autoantiraid",
			botAdmin:     true,
			defaultValue: 0, defaultLabel: "off", statusToken: "Auto: ",
			refusalKey: "antiraid_invalid_threshold",
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.AntiRaidSettings{ChatID: chatID, AutoAntiRaidThreshold: -1}).Error
			},
			edges: []settingContractEdge{
				{-1, "-1", "-1", false},
				{0, "off", "off", true},
				{1, "1", "1", true},
				{2147483647, "2147483647", "2147483647", true},
				{2147483648, "2147483648", "2147483648", false},
			},
		},
		{
			key: "greetings.goodbye_enabled", command: "/goodbye", button: "gb",
			defaultValue: true, defaultLabel: "on",
			statusToken: "I am currently goodbying users: <code>",
			refusalKey:  "greetings_goodbye_invalid",
			setOn:       "greetings_goodbye_enable", setOff: "greetings_goodbye_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.welcome_enabled", command: "/welcome", button: "we",
			defaultValue: true, defaultLabel: "on",
			statusToken: "I am currently welcoming users: <code>",
			refusalKey:  "greetings_welcome_invalid_option",
			setOn:       "greetings_welcome_enabled", setOff: "greetings_welcome_disabled",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_welcome", command: "/cleanwelcome", button: "cw", changeInfo: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_welcome_should", statusOff: "greetings_clean_welcome_not",
			refusalKey: "greetings_clean_welcome_invalid_option",
			setOn:      "greetings_clean_welcome_enable", setOff: "greetings_clean_welcome_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_goodbye", command: "/cleangoodbye", button: "cg", changeInfo: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_goodbye_should", statusOff: "greetings_clean_goodbye_not",
			refusalKey: "greetings_clean_goodbye_invalid_option",
			setOn:      "greetings_clean_goodbye_enable", setOff: "greetings_clean_goodbye_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_service", command: "/cleanservice", button: "cs", changeInfo: true, botAdmin: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_service_should", statusOff: "greetings_clean_service_not",
			refusalKey: "greetings_clean_service_invalid_option",
			setOn:      "greetings_clean_service_enable", setOff: "greetings_clean_service_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.auto_approve", command: "/autoapprove", button: "aa", changeInfo: true, botAdmin: true, noPanel: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_auto_approve_enabled", statusOff: "greetings_auto_approve_disabled",
			refusalKey: "greetings_auto_approve_invalid_option",
			setOn:      "greetings_auto_approve_enable", setOff: "greetings_auto_approve_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
	} {
		t.Run(setting.key, func(t *testing.T) {
			t.Run("default without creating a row", func(t *testing.T) {
				f := newSettingContractFixture(t, setting)
				assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
				assertContractRowCount(t, f, setting, 0)
			})
			t.Run("invalid stored value uses default", func(t *testing.T) {
				if setting.seedInvalid == nil {
					return
				}
				f := newSettingContractFixture(t, setting)
				if err := setting.seedInvalid(f.chat.Id); err != nil {
					t.Fatal(err)
				}
				assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
			})
			for _, rights := range []struct {
				name        string
				callerAdmin bool
				botAdmin    bool
				changeInfo  bool
			}{
				{"member", false, true, false},
				{"exact right", true, true, setting.changeInfo},
				{"no change info", true, true, false},
				{"bot member", true, false, setting.changeInfo},
			} {
				t.Run("rights/"+rights.name, func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					contractRights(f, rights.changeInfo, rights.botAdmin, false)
					actor := f.member
					if rights.callerAdmin {
						actor = f.admin
					}
					edge := setting.edges[1]
					accepted := rights.callerAdmin && (!setting.botAdmin || rights.botAdmin) && (!setting.changeInfo || rights.changeInfo)
					refusal := "connections_is_user_connected_user_not_admin"
					if setting.botAdmin && !rights.botAdmin {
						refusal = "connections_is_user_connected_bot_not_admin"
					} else if rights.callerAdmin && setting.changeInfo && !rights.changeInfo {
						refusal = "chat_status_change_info_cmd_error"
					}
					contractDispatch(t, f, actor, setting.command+" "+edge.input)
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if !accepted && reply != translated(t, refusal) {
						t.Fatalf("command refusal = %q, want %s", reply, refusal)
					}
					if !accepted {
						assertContractRowCount(t, f, setting, 0)
					}
					if setting.button != "" {
						contractPress(t, f, actor, setting.button, edge.input)
						text, alert := lastAnswer(t, f.client)
						if accepted == alert || (!accepted && text != reply) {
							t.Fatalf("button answer = %q alert %t, command = %q accepted %t", text, alert, reply, accepted)
						}
					}
					if !setting.noPanel {
						response := contractPanelRequest(t, f, actor, http.MethodPatch, contractChanges(t, map[string]any{setting.key: edge.value}))
						if accepted && rights.botAdmin {
							if response.Code != http.StatusOK {
								t.Fatalf("exact right refused: %d %s", response.Code, response.Body.String())
							}
						} else if response.Code != http.StatusForbidden && response.Code != http.StatusConflict {
							t.Fatalf("panel accepted missing right: %d %s", response.Code, response.Body.String())
						}
					}
					if !accepted {
						assertContractRowCount(t, f, setting, 0)
					} else if rights.botAdmin {
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					}
				})
			}
			for _, edge := range setting.edges {
				t.Run(edge.input, func(t *testing.T) {
					if setting.noPanel {
						return
					}
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					changes := map[string]any{setting.key: edge.value}
					if !edge.accept {
						sibling := "antiraid.raid_time"
						if sibling == setting.key {
							sibling = "antiraid.raid_action_time"
						}
						changes[sibling] = 60
					}
					payload, err := json.Marshal(map[string]any{"changes": changes})
					if err != nil {
						t.Fatal(err)
					}
					response := contractPanelRequest(t, f, f.admin, http.MethodPatch, string(payload))
					var result struct {
						Saved  []string          `json:"saved"`
						Failed map[string]string `json:"failed"`
						Errors map[string]string `json:"errors"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if edge.accept {
						if response.Code != http.StatusOK {
							t.Fatalf("panel refused %v: %d %s", edge.value, response.Code, response.Body.String())
						}
						if !slices.Contains(result.Saved, setting.key) || len(result.Failed) != 0 {
							t.Fatalf("per-field outcomes = %+v", result)
						}
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					} else {
						if response.Code != http.StatusBadRequest || result.Errors[setting.key] == "" || len(result.Saved) != 0 || len(result.Failed) != 0 {
							t.Fatalf("panel accepted invalid %v: %d %s", edge.value, response.Code, response.Body.String())
						}
						assertContractRowCount(t, f, setting, 0)
					}
				})
				t.Run("command/"+edge.input, func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					contractDispatch(t, f, f.admin, setting.command+" "+edge.input)
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if edge.accept {
						if setting.setOn != "" {
							key := setting.setOff
							if edge.value == true {
								key = setting.setOn
							}
							if reply != translated(t, key) {
								t.Fatalf("command reply = %q, want %s", reply, key)
							}
						} else if edge.input == "off" {
							if reply != translated(t, "antiraid_auto_disabled") {
								t.Fatalf("off reply = %q", reply)
							}
						} else if !strings.Contains(reply, edge.label) {
							t.Fatalf("command reply = %q, want accepted %s", reply, edge.label)
						}
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					} else {
						if reply != translated(t, setting.refusalKey) {
							t.Fatalf("command accepted invalid value: %q", reply)
						}
						assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					}
				})
				if setting.button != "" {
					t.Run("button/"+edge.input, func(t *testing.T) {
						f := newSettingContractFixture(t, setting)
						assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
						contractPress(t, f, f.admin, setting.button, edge.input)
						text, alert := lastAnswer(t, f.client)
						if edge.accept {
							if alert {
								t.Fatalf("button refused %s: %q", edge.input, text)
							}
							assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
						} else {
							if text != translated(t, "common_callback_invalid_request") {
								t.Fatalf("button accepted invalid %s: %q", edge.input, text)
							}
							assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
							assertContractRowCount(t, f, setting, 0)
						}
					})
				}
			}
			if setting.changeInfo && !setting.noPanel {
				t.Run("rights remain independent in a batch", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					contractRights(f, false, true, true)
					response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
						setting.key: true, "reports.mute_threshold": 2,
					}))
					if response.Code != http.StatusForbidden {
						t.Fatalf("batch bypassed change-info: %d %s", response.Code, response.Body.String())
					}
					assertContractRowCount(t, f, setting, 0)
				})
			}
			if setting.button != "" {
				t.Run("write failure", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					name := "fail_greeting_write"
					if err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
						if tx.Statement.Table == "greetings" {
							_ = tx.AddError(errors.New("greeting write failed"))
						}
					}); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(name) })
					contractDispatch(t, f, f.admin, setting.command+" on")
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if reply != translated(t, "common_settings_save_failed") {
						t.Fatalf("write failure reply = %q", reply)
					}
					contractPress(t, f, f.admin, setting.button, "on")
					if text, alert := lastAnswer(t, f.client); !alert || text != reply {
						t.Fatalf("button failure = %q alert %t, command = %q", text, alert, reply)
					}
					if !setting.noPanel {
						response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: true}))
						var result struct {
							Saved  []string          `json:"saved"`
							Failed map[string]string `json:"failed"`
						}
						if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if response.Code != http.StatusInternalServerError || len(result.Saved) != 0 || result.Failed[setting.key] == "" {
							t.Fatalf("panel failure = %d %s", response.Code, response.Body.String())
						}
					}
					assertContractRowCount(t, f, setting, 0)
				})
			}
		})
	}
}

func TestAntiRaidRepairsStoredTimesThroughDispatcher(t *testing.T) {
	useRealLocales(t)
	withMiniredis(t)
	for _, surface := range []string{"command", "settings button", "status button", "automatic"} {
		t.Run(surface, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := antiraid.SetRaidTime(f.chat.Id, 60); err != nil {
				t.Fatal(err)
			}
			if err := antiraid.SetRaidTime(f.chat.Id, 0); err != nil {
				t.Fatal(err)
			}
			if err := antiraid.SetRaidActionTime(f.chat.Id, 5); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(f.chat.Id) })
			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadAntiRaid(dispatcher)
			LoadConnections(dispatcher)
			started := time.Now().Unix()
			switch surface {
			case "command":
				contractDispatch(t, f, f.admin, "/antiraid on")
				if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != "AntiRaid enabled for 6h." {
					t.Fatalf("enable reply = %q", reply)
				}
			case "settings button", "status button":
				namespace, fields := "settings", map[string]string{"a": "t", "c": fmt.Sprint(f.chat.Id), "k": "ar", "v": "on"}
				if surface == "status button" {
					namespace, fields = "antiraid", map[string]string{"a": "on", "c": fmt.Sprint(f.chat.Id)}
				}
				update := newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, encodeCallbackData(namespace, fields)).Update
				if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatal(err)
				}
			case "automatic":
				if err := antiraid.SetAutoAntiRaidThreshold(f.chat.Id, 1); err != nil {
					t.Fatal(err)
				}
			}
			newcomer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			before := time.Now().Unix()
			update := &gotgbot.Update{UpdateId: 3, Message: &gotgbot.Message{
				MessageId: 103, Date: before, Chat: f.chat, From: &newcomer, NewChatMembers: []gotgbot.User{newcomer},
			}}
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatal(err)
			}
			ban := f.client.onlyCall(t, "banChatMember")
			until, err := strconv.ParseInt(fmt.Sprint(ban.Params["until_date"]), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if until < before+3600 || until > time.Now().Unix()+3600 {
				t.Fatalf("ban until = %d, want the default hour after %d", until, before)
			}
			assertParams(t, ban, map[string]any{"chat_id": f.chat.Id, "user_id": newcomer.Id})
			contractDispatch(t, f, f.admin, "/antiraid")
			reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
			if !strings.Contains(reply, "AntiRaid ACTIVE") || !strings.Contains(reply, "Raid: 6h | Action: 1h") {
				t.Fatalf("repaired raid status = %q", reply)
			}
			var expiry string
			if _, err := fmt.Sscanf(reply, "AntiRaid ACTIVE | Expires: %s", &expiry); err != nil {
				t.Fatal(err)
			}
			seconds, ok := parseDuration(expiry)
			if !ok || seconds > 21600 || seconds < 21600-int(time.Now().Unix()-started)-1 {
				t.Fatalf("raid expires in %q, want default six hours", expiry)
			}
		})
	}
}

func newSettingContractFixture(t *testing.T, setting settingContract) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	contractRights(f, setting.changeInfo, true, false)
	if err := chats.EnsureChatInDb(f.chat.Id, f.chat.Title); err != nil {
		t.Fatal(err)
	}
	return f
}

func contractRights(f adminChatFixture, changeInfo, botAdmin, restrict bool) {
	admin := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"first_name":"Admin"},"can_change_info":%t,"can_restrict_members":%t}`, f.admin.Id, changeInfo, restrict)
	bot := fmt.Sprintf(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_restrict_members":%t}`, restrict)
	if !botAdmin {
		bot = `{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`
	}
	f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(admin)
	f.client.membersByChat = map[string]json.RawMessage{fmt.Sprintf("%d|999", f.chat.Id): json.RawMessage(bot)}
	f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + admin + "]")
	if botAdmin {
		f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + bot + "," + admin + "]")
	}
}

func contractPress(t *testing.T, f adminChatFixture, actor gotgbot.User, key, value string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	data := encodeCallbackData("settings", map[string]string{"a": "t", "c": fmt.Sprint(f.chat.Id), "k": key, "v": value})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, privateChatOf(actor), actor, data).Update, nil); err != nil {
		t.Fatal(err)
	}
}

func contractDispatch(t *testing.T, f adminChatFixture, actor gotgbot.User, text string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntiRaid(dispatcher)
	LoadGreetings(dispatcher)
	LoadConnections(dispatcher)
	if err := dispatcher.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, text).Update, nil); err != nil {
		t.Fatal(err)
	}
}

func assertContractRead(t *testing.T, f adminChatFixture, setting settingContract, value any, label string, changed bool) {
	t.Helper()
	command := "/antiraid"
	if setting.button != "" {
		command = setting.command
	}
	before := len(f.client.callsFor("sendMessage"))
	contractDispatch(t, f, f.admin, command)
	replies := f.client.callsFor("sendMessage")[before:]
	found := false
	for _, reply := range replies {
		text := fmt.Sprint(reply.Params["text"])
		if setting.statusOn != "" {
			key := setting.statusOff
			if value == true {
				key = setting.statusOn
			}
			found = found || text == translated(t, key)
			continue
		}
		if strings.Contains(text, setting.statusToken+fmt.Sprint(value)) || strings.Contains(text, setting.statusToken+label) {
			found = true
		}
	}
	if !found {
		t.Fatalf("status replies = %v, want %s%v", replies, setting.statusToken, value)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	data := encodeCallbackData("settings", map[string]string{"a": "r", "c": fmt.Sprint(f.chat.Id)})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, data).Update, nil); err != nil {
		t.Fatal(err)
	}
	assertContractSummary(t, fmt.Sprint(lastCall(t, f.client, "editMessageText").Params["text"]), setting.command, label, changed)
	if setting.button != "" {
		next := "on"
		if value == true {
			next = "off"
		}
		if actual := toggleButtons(t, markupOf(t, lastCall(t, f.client, "editMessageText")))[setting.button]; actual != next {
			t.Fatalf("button offers %q, want %q", actual, next)
		}
	}
	if setting.noPanel {
		return
	}
	response := contractPanelRequest(t, f, f.admin, http.MethodGet, "")
	var body struct {
		Modules []struct {
			Key    string `json:"key"`
			Fields []struct {
				Key     string          `json:"key"`
				Value   json.RawMessage `json:"value"`
				Default json.RawMessage `json:"default"`
				Changed bool            `json:"changed"`
			} `json:"fields"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, module := range body.Modules {
		for _, field := range module.Fields {
			if module.Key+"."+field.Key != setting.key {
				continue
			}
			if string(field.Value) != contractJSON(t, value) || string(field.Default) != contractJSON(t, setting.defaultValue) || field.Changed != changed {
				t.Fatalf("panel field = %+v, want value %v default %v changed %t", field, value, setting.defaultValue, changed)
			}
			return
		}
	}
	t.Fatalf("panel missing %s: %d %s", setting.key, response.Code, response.Body.String())
}

func assertContractSummary(t *testing.T, text, command, label string, changed bool) {
	t.Helper()
	line := "<code>" + command + "</code> — " + label
	if changed {
		line = "• " + line
	}
	for _, actual := range strings.Split(text, "\n") {
		if actual == line {
			return
		}
	}
	t.Fatalf("summary missing line %q: %s", line, text)
}

func contractJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func contractChanges(t *testing.T, changes map[string]any) string {
	t.Helper()
	return contractJSON(t, map[string]any{"changes": changes})
}

func assertContractRowCount(t *testing.T, f adminChatFixture, setting settingContract, want int64) {
	t.Helper()
	var model any = &models.AntiRaidSettings{}
	if setting.button != "" {
		model = &models.GreetingSettings{}
	}
	var count int64
	if err := db.DB.Model(model).Where("chat_id = ?", f.chat.Id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("settings rows = %d, want %d", count, want)
	}
}

func contractPanelRequest(t *testing.T, f adminChatFixture, actor gotgbot.User, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now()
	user, err := json.Marshal(actor)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}, "user": {string(user)}}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(f.bot.Token))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte("auth_date=" + values.Get("auth_date") + "\nuser=" + values.Get("user")))
	values.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	request := httptest.NewRequest(method, fmt.Sprintf("/panel/api/v1/chats/%d/settings", f.chat.Id), strings.NewReader(body))
	request.Header.Set("X-Telegram-Init-Data", values.Encode())
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	panel.New(f.bot, panel.Deps{Now: func() time.Time { return now }}).ServeHTTP(response, request)
	return response
}

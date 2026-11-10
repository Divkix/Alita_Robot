//go:build testtools

package modules

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/panel"
)

type settingContractEdge struct {
	value  int
	input  string
	label  string
	accept bool
}

type settingContract struct {
	key          string
	command      string
	defaultValue int
	defaultLabel string
	statusToken  string
	refusalKey   string
	seedInvalid  func(int64) error
	edges        []settingContractEdge
}

// Expectations come from the command contract, not the declarations. Later
// settings add rows here, including their optional button input and side effect.
func TestDeclaredSettingsAcrossSurfaces(t *testing.T) {
	useRealLocales(t)
	for _, setting := range []settingContract{
		{
			key: "antiraid.raid_action_time", command: "/raidactiontime",
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
	} {
		t.Run(setting.key, func(t *testing.T) {
			t.Run("default without creating a row", func(t *testing.T) {
				f := newSettingContractFixture(t)
				assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
				var count int64
				if err := db.DB.Model(&models.AntiRaidSettings{}).Where("chat_id = ?", f.chat.Id).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatalf("reading settings created %d rows", count)
				}
			})
			t.Run("invalid stored value uses default", func(t *testing.T) {
				f := newSettingContractFixture(t)
				if err := setting.seedInvalid(f.chat.Id); err != nil {
					t.Fatal(err)
				}
				assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
			})
			for _, callerAdmin := range []bool{false, true} {
				for _, botAdmin := range []bool{false, true} {
					t.Run(fmt.Sprintf("rights/caller=%t/bot=%t", callerAdmin, botAdmin), func(t *testing.T) {
						f := newSettingContractFixture(t)
						actor := f.member
						if callerAdmin {
							actor = f.admin
						}
						if !botAdmin {
							f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(fmt.Sprintf(`[{"status":"administrator","user":{"id":%d,"first_name":"Admin"}}]`, f.admin.Id))
							f.client.membersByChat = map[string]json.RawMessage{
								fmt.Sprintf("%d|999", f.chat.Id): json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
							}
						}
						edge := setting.edges[1]
						contractDispatch(t, f, actor, setting.command+" "+edge.input)
						reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
						response := contractPanelRequest(t, f, actor, http.MethodPatch, fmt.Sprintf(`{"changes":{%q:%d}}`, setting.key, edge.value))
						if callerAdmin && botAdmin {
							if response.Code != http.StatusOK {
								t.Fatalf("exact right refused: %d %s", response.Code, response.Body.String())
							}
							assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
						} else {
							key := "connections_is_user_connected_user_not_admin"
							if !botAdmin {
								key = "connections_is_user_connected_bot_not_admin"
							}
							if reply != translated(t, key) {
								t.Fatalf("command refusal = %q, want %s", reply, key)
							}
							if response.Code != http.StatusForbidden && response.Code != http.StatusConflict {
								t.Fatalf("panel accepted missing right: %d %s", response.Code, response.Body.String())
							}
							var count int64
							if err := db.DB.Model(&models.AntiRaidSettings{}).Where("chat_id = ?", f.chat.Id).Count(&count).Error; err != nil {
								t.Fatal(err)
							}
							if count != 0 {
								t.Fatalf("missing right stored %d rows", count)
							}
						}
					})
				}
			}
			for _, edge := range setting.edges {
				t.Run(edge.input, func(t *testing.T) {
					f := newSettingContractFixture(t)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					changes := map[string]int{setting.key: edge.value}
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
							t.Fatalf("panel refused %d: %d %s", edge.value, response.Code, response.Body.String())
						}
						if !slices.Contains(result.Saved, setting.key) || len(result.Failed) != 0 {
							t.Fatalf("per-field outcomes = %+v", result)
						}
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					} else {
						if response.Code != http.StatusBadRequest || result.Errors[setting.key] == "" || len(result.Saved) != 0 || len(result.Failed) != 0 {
							t.Fatalf("panel accepted invalid %d: %d %s", edge.value, response.Code, response.Body.String())
						}
						var count int64
						if err := db.DB.Model(&models.AntiRaidSettings{}).Where("chat_id = ?", f.chat.Id).Count(&count).Error; err != nil {
							t.Fatal(err)
						}
						if count != 0 {
							t.Fatalf("invalid batch wrote %d rows", count)
						}
					}
				})
				t.Run("command/"+edge.input, func(t *testing.T) {
					f := newSettingContractFixture(t)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					contractDispatch(t, f, f.admin, setting.command+" "+edge.input)
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if edge.accept {
						if edge.input == "off" {
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

func newSettingContractFixture(t *testing.T) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	// Raid settings need only caller + bot admin, no change-info or restrict.
	admin := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"first_name":"Admin"}}`, f.admin.Id)
	f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` + admin + `]`)
	f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(admin)
	if err := chats.EnsureChatInDb(f.chat.Id, f.chat.Title); err != nil {
		t.Fatal(err)
	}
	return f
}

func contractDispatch(t *testing.T, f adminChatFixture, actor gotgbot.User, text string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntiRaid(dispatcher)
	LoadConnections(dispatcher)
	if err := dispatcher.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, text).Update, nil); err != nil {
		t.Fatal(err)
	}
}

func assertContractRead(t *testing.T, f adminChatFixture, setting settingContract, value int, label string, changed bool) {
	t.Helper()
	contractDispatch(t, f, f.admin, "/antiraid")
	if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); !strings.Contains(reply, setting.statusToken+strconv.Itoa(value)) && !strings.Contains(reply, setting.statusToken+label) {
		t.Fatalf("status reply = %q, want %s%s", reply, setting.statusToken, label)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	data := encodeCallbackData("settings", map[string]string{"a": "r", "c": fmt.Sprint(f.chat.Id)})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, data).Update, nil); err != nil {
		t.Fatal(err)
	}
	assertContractSummary(t, fmt.Sprint(lastCall(t, f.client, "editMessageText").Params["text"]), setting.command, label, changed)
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
			if string(field.Value) != strconv.Itoa(value) || string(field.Default) != strconv.Itoa(setting.defaultValue) || field.Changed != changed {
				t.Fatalf("panel field = %+v, want value %d default %d changed %t", field, value, setting.defaultValue, changed)
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

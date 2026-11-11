//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
)

// The approved seam is punishment.Apply/Lift, observed through Telegram and SQLite.
func TestPunishmentMatrix(t *testing.T) {
	useRealLocales(t)
	testPunishmentLifts(t)
	for _, mode := range []punishment.Mode{punishment.Ban, punishment.Kick, punishment.Mute} {
		for _, senderChat := range []bool{false, true} {
			for _, duration := range []time.Duration{0, time.Hour, 29 * time.Second, 367 * 24 * time.Hour, 30 * time.Second, 366 * 24 * time.Hour, -time.Second} {
				for _, failure := range []string{"", "telegram", "record"} {
					t.Run(fmt.Sprintf("apply/%s/channel=%t/%s/failure=%s", mode, senderChat, duration, failure), func(t *testing.T) {
						f := newAdminChatFixture(t)
						target := punishment.Target{ID: f.member.Id, SenderChat: senderChat}
						if senderChat {
							target.ID = -100123456789
						}
						method := map[punishment.Mode]string{punishment.Ban: "banChatMember", punishment.Kick: "unbanChatMember", punishment.Mute: "restrictChatMember"}[mode]
						if senderChat && mode == punishment.Ban {
							method = "banChatSenderChat"
						}
						requestErr := errors.New("Telegram refused punishment")
						if failure == "telegram" {
							f.client.errors[method] = requestErr
						}
						previous := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
						t.Cleanup(func() { log.StandardLogger().ReplaceHooks(previous) })
						hook := logtest.NewGlobal()
						if failure == "record" {
							if err := db.DB.Migrator().DropTable(&models.BanRecord{}); err != nil {
								t.Fatal(err)
							}
							t.Cleanup(func() {
								if err := db.DB.AutoMigrate(&models.BanRecord{}); err != nil {
									t.Error(err)
								}
							})
						}
						before := time.Now().Unix()
						result, err := punishment.Apply(t.Context(), f.bot, punishment.Request{
							ChatID: f.chat.Id, Target: target, Mode: mode, Duration: duration,
							Source: models.BanSourceAntiflood, ActorID: f.admin.Id, Reason: "flood", Detail: "limit",
						})
						after := time.Now().Unix()
						supported := (!senderChat || mode == punishment.Ban && duration == 0) && (mode != punishment.Kick || duration == 0) &&
							(duration == 0 || duration >= 30*time.Second && duration <= 366*24*time.Hour)
						if !supported {
							if result.Applied || result.Refusal == "" || err != nil || len(f.client.callsFor(method)) != 0 {
								t.Fatalf("unsupported action = %+v, %v; calls = %+v", result, err, f.client.calls)
							}
						} else {
							call := f.client.onlyCall(t, method)
							params := targetParams(f.chat.Id, target.ID)
							if senderChat {
								params = map[string]any{"chat_id": f.chat.Id, "sender_chat_id": target.ID}
							}
							assertParams(t, call, params)
							if failure == "telegram" {
								if result.Applied || result.Refusal == "" || !errors.Is(err, requestErr) {
									t.Fatalf("refused action = %+v, %v", result, err)
								}
							} else if !result.Applied || result.Refusal != "" || err != nil {
								t.Fatalf("accepted action = %+v, %v", result, err)
							}
							if duration != 0 {
								until := untilDateParam(t, call)
								if until < before+int64(duration/time.Second) || until > after+int64(duration/time.Second) || failure != "telegram" && result.UntilDate != until {
									t.Fatalf("until_date = %d, result = %+v", until, result)
								}
							} else if call.Params["until_date"] != nil || result.UntilDate != 0 {
								t.Fatalf("untimed action has until_date: %+v / %+v", call, result)
							}
							if mode == punishment.Kick && call.Params["only_if_banned"] == true {
								t.Fatal("kick would leave an existing member in the group")
							}
							if mode == punishment.Mute && !reflect.DeepEqual(call.Params["permissions"], gotgbot.ChatPermissions{CanManageTopics: new(bool)}) {
								t.Fatalf("mute permissions = %+v, want all denied", call.Params["permissions"])
							}
						}
						wantRecord := supported && mode == punishment.Ban && failure != "telegram"
						if failure == "record" {
							if wantRecord && (hook.LastEntry() == nil || !strings.Contains(hook.LastEntry().Message, "Failed to record")) {
								t.Fatal("record write failure was not logged")
							}
							wantRecord = false
						} else {
							record, loadErr := bans.Latest(f.chat.Id, target.ID)
							if loadErr != nil || (record != nil) != wantRecord {
								t.Fatalf("ban record = %+v, %v, want record=%t", record, loadErr, wantRecord)
							}
							if wantRecord {
								if record.ID != result.RecordID || record.BannedBy != f.admin.Id || record.Source != models.BanSourceAntiflood || record.Reason != "flood" || record.SourceDetail != "limit" || (record.Until != nil) != (duration != 0) || record.Until != nil && record.Until.Unix() != result.UntilDate {
									t.Fatalf("ban record = %+v, result = %+v", record, result)
								}
							}
						}
						if !wantRecord {
							if result.RecordID != 0 || result.LogMarkup != nil {
								t.Fatalf("action without record returned log button: %+v", result)
							}
						} else {
							if result.LogMarkup == nil {
								t.Fatal("recorded ban has no Unban button")
							}
							button := result.LogMarkup.InlineKeyboard[0][0]
							decoded, decodeErr := callbackcodec.Decode(button.CallbackData)
							if decodeErr != nil || decoded.Namespace != "banlogunban" || decoded.Fields["r"] != fmt.Sprint(result.RecordID) || button.Text != translated(t, "bans_log_unban_button") {
								t.Fatalf("log button = %+v, %v", button, decodeErr)
							}
						}
					})
				}
			}
		}
	}
}

func testPunishmentLifts(t *testing.T) {
	t.Helper()
	for _, mode := range []punishment.Mode{punishment.Ban, punishment.Mute, punishment.Kick} {
		for _, senderChat := range []bool{false, true} {
			for _, defaults := range []bool{false, true} {
				for _, failure := range []string{"", "telegram", "getChat"} {
					t.Run(fmt.Sprintf("lift/%s/channel=%t/defaults=%t/failure=%s", mode, senderChat, defaults, failure), func(t *testing.T) {
						f := newAdminChatFixture(t)
						target := punishment.Target{ID: f.member.Id, SenderChat: senderChat}
						if senderChat {
							target.ID = -100123456789
						}
						seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: target.ID, Source: models.BanSourceManual})
						method := "unbanChatMember"
						if senderChat {
							method = "unbanChatSenderChat"
						}
						if mode == punishment.Mute {
							method = "restrictChatMember"
						}
						// Without chat defaults: send anything and invite, but no info, pin or topic rights.
						wantPermissions := gotgbot.ChatPermissions{
							CanSendMessages: true, CanSendPhotos: true, CanSendVideos: true, CanSendAudios: true,
							CanSendDocuments: true, CanSendVideoNotes: true, CanSendVoiceNotes: true,
							CanAddWebPagePreviews: true, CanInviteUsers: true, CanManageTopics: new(bool),
							CanSendPolls: true, CanSendOtherMessages: true,
						}
						if defaults {
							wantPermissions = gotgbot.ChatPermissions{CanSendMessages: true, CanSendPolls: true}
							f.client.chatsByID[fmt.Sprint(f.chat.Id)] = []byte(`{"id":-1001,"type":"supergroup","permissions":{"can_send_messages":true,"can_send_polls":true}}`)
						}
						requestErr := errors.New("Telegram refused lift")
						switch failure {
						case "telegram":
							f.client.errors[method] = requestErr
						case "getChat":
							f.client.errors["getChat"] = requestErr
						}
						result, err := punishment.Lift(t.Context(), f.bot, punishment.LiftRequest{
							ChatID: f.chat.Id, Target: target, Mode: mode, ActorID: f.admin.Id,
						})
						supported := mode == punishment.Ban || mode == punishment.Mute && !senderChat
						refused := supported && (failure == "telegram" || failure == "getChat" && mode == punishment.Mute)
						if result.Applied != (supported && !refused) || errors.Is(err, requestErr) != refused || result.Applied == (result.Refusal != "") {
							t.Fatalf("Lift = %+v, %v", result, err)
						}
						record, loadErr := bans.Latest(f.chat.Id, target.ID)
						if loadErr != nil || record == nil {
							t.Fatalf("load ban = %+v, %v", record, loadErr)
						}
						closed := supported && !refused && mode == punishment.Ban
						if (record.LiftedAt != nil) != closed || closed && (record.LiftedBy == nil || *record.LiftedBy != f.admin.Id) {
							t.Fatalf("ban after lift = %+v", record)
						}
						if !supported || failure == "getChat" && mode == punishment.Mute {
							if len(f.client.callsFor(method)) != 0 {
								t.Fatal("unsupported or failed lookup still changed permissions")
							}
							return
						}
						call := f.client.onlyCall(t, method)
						if mode == punishment.Mute {
							if !reflect.DeepEqual(call.Params["permissions"], wantPermissions) || call.Params["until_date"] != nil {
								t.Fatalf("unmute = %+v, want defaults %+v", call.Params, wantPermissions)
							}
						} else if !senderChat {
							assertParams(t, call, map[string]any{"only_if_banned": true})
						}
					})
				}
			}
		}
	}
}

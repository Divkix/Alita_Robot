//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

func TestWarnLimitDispatcherDecidesPunishment(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		name     string
		settings models.WarnSettings
		method   string
		duration time.Duration
		timed    bool
		text     string
	}{
		{"kick", models.WarnSettings{WarnMode: "kick"}, "unbanChatMember", 0, false, "kicked"},
		{"mute", models.WarnSettings{WarnMode: "mute"}, "restrictChatMember", 0, false, "Muted"},
		{"ban", models.WarnSettings{WarnMode: "ban"}, "banChatMember", 0, false, "banned"},
		{"tban", models.WarnSettings{WarnMode: "tban", WarnActionTime: 86400}, "banChatMember", 24 * time.Hour, true, "banned for 1d"},
		{"tmute", models.WarnSettings{WarnMode: "tmute", WarnActionTime: 7200}, "restrictChatMember", 2 * time.Hour, true, "muted for 2h"},
		{"longest", models.WarnSettings{WarnMode: "tban", WarnActionTime: 366 * 86400}, "banChatMember", 366 * 24 * time.Hour, true, "banned for 366d"},
		// Rows without a usable duration keep the one-hour fallback.
		{"legacy zero", models.WarnSettings{WarnMode: "tmute"}, "restrictChatMember", time.Hour, true, "muted for 1h"},
		{"too short", models.WarnSettings{WarnMode: "tban", WarnActionTime: 30}, "banChatMember", time.Hour, true, "banned for 1h"},
		{"too long", models.WarnSettings{WarnMode: "tmute", WarnActionTime: 366*86400 + 1}, "restrictChatMember", time.Hour, true, "muted for 1h"},
	} {
		// The bot's own warns (blacklists) punish as the bot.
		for _, issuer := range []string{"admin", "bot"} {
			t.Run(tc.name+"/"+issuer, func(t *testing.T) {
				f := newAdminChatFixture(t)
				settings := tc.settings
				settings.ChatId, settings.WarnLimit = f.chat.Id, 2
				if err := db.DB.Create(&settings).Error; err != nil {
					t.Fatal(err)
				}
				warnedBy, actorID := f.admin.Id, f.admin.Id
				if issuer == "bot" {
					warnedBy, actorID = f.bot.Id, 0
				}
				for i := range 2 {
					ctx := newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn limit reached")
					before := time.Now().Unix()
					err := warnsModule.warnThisUser(f.bot, ctx, f.member.Id, "limit reached", "warn", models.WarnEntry{WarnedBy: warnedBy})
					after := time.Now().Unix()
					if err != ext.EndGroups {
						t.Fatalf("warnThisUser() error = %v, want EndGroups", err)
					}
					calls := punishmentCalls(f.client)
					if i == 0 {
						if len(calls) != 0 {
							t.Fatalf("below the limit punishment calls = %+v, want none", calls)
						}
						continue
					}
					if len(calls) != 1 || calls[0].Method != tc.method {
						t.Fatalf("punishment calls = %+v, want one %s", calls, tc.method)
					}
					assertParams(t, calls[0], targetParams(f.chat.Id, f.member.Id))
					if !tc.timed {
						if calls[0].Params["until_date"] != nil {
							t.Fatalf("%s params = %#v, want no until_date", calls[0].Method, calls[0].Params)
						}
					} else {
						secs := int64(tc.duration / time.Second)
						if until := untilDateParam(t, calls[0]); until < before+secs || until > after+secs {
							t.Fatalf("until_date = %d, want now+%d (%d..%d)", until, secs, before+secs, after+secs)
						}
					}
					if tc.method == "banChatMember" {
						record := singleBanRecord(t, f.chat.Id)
						if record.Source != models.BanSourceWarn || record.BannedBy != actorID || record.Reason != "limit reached" {
							t.Fatalf("warn-limit record = %+v", record)
						}
					}
				}
				if numWarns, _ := warns.GetWarns(f.member.Id, f.chat.Id); numWarns != 0 {
					t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
				}
				if reply := lastReply(t, f.client); !strings.Contains(reply, "2/2") || !strings.Contains(reply, tc.text) {
					t.Fatalf("limit reply = %q, want %q", reply, tc.text)
				}
			})
		}
	}
}

// Admins cannot be warned, so they never reach the limit's punishment.
func TestWarnLimitExemptsAdmins(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
		t.Fatal(err)
	}
	ctx := newModuleMessageContext(f.bot, f.chat, f.admin, "/warn")
	err := warnsModule.warnThisUser(f.bot, ctx, f.admin.Id, "", "warn", warnIssuer(ctx))
	if err != ext.EndGroups {
		t.Fatalf("warn of an admin: err = %v, want EndGroups", err)
	}
	if calls := punishmentCalls(f.client); len(calls) != 0 {
		t.Fatalf("admin punished: %+v", calls)
	}
}

// A warn-limit ban keeps its record, its log entry's Unban button and reply.
func TestWarnLimitBanKeepsRecordAndLog(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	// An admin's warn logs under the opt-in warns category.
	if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryWarns, true); err != nil {
		t.Fatal(err)
	}
	if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
		t.Fatal(err)
	}
	if err := warns.SetWarnMode(f.chat.Id, "tban", 600); err != nil {
		t.Fatal(err)
	}
	if err := warnsModule.warnUser(f.bot, newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn flooding")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)
	if record.Source != models.BanSourceWarn || record.BannedBy != f.admin.Id || record.Reason != "flooding" || record.Until == nil {
		t.Fatalf("warn-limit record = %+v", record)
	}
	if entries := logUnbanButtons(t, f); len(entries) != 1 || len(entries[0]) != 1 || entries[0][0] != unbanLogData(record.ID) {
		t.Fatalf("warn-limit log buttons = %+v", entries)
	}
	logs := callsToChat(f.client, "sendMessage", banLogChannelID)
	if text := fmt.Sprint(logs[0].Params["text"]); !strings.Contains(text, "#WARNLIMIT") || !strings.Contains(text, "tban 10m") {
		t.Fatalf("warn-limit log = %q", text)
	}
	if reply := lastReply(t, f.client); !strings.Contains(reply, "banned for 10m") {
		t.Fatalf("warn-limit reply = %q", reply)
	}
}

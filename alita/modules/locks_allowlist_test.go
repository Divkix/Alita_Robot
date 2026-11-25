package modules

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/locks"
)

// An allowlisted host exempts only itself: the exact host survives the URL
// lock, while a second unapproved host or a lookalike in the same caption
// still gets the message deleted.
func TestURLHostAllowlistExemptsExactHostOnly(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := locks.UpdateLock(f.chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/allowlist example.com").Update, nil); err != nil {
		t.Fatal(err)
	}

	urlEntity := linkURLEntity
	send := func(id int64, text string, entities []gotgbot.MessageEntity) {
		t.Helper()
		ctx := newModuleMessageContext(f.bot, f.chat, f.member, text)
		ctx.EffectiveMessage.MessageId = id
		ctx.EffectiveMessage.Entities = entities
		if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatal(err)
		}
	}
	sendCaption := func(id int64, caption string, entities []gotgbot.MessageEntity) {
		t.Helper()
		ctx := newModuleMessageContext(f.bot, f.chat, f.member, "")
		ctx.EffectiveMessage.MessageId = id
		ctx.EffectiveMessage.Photo = []gotgbot.PhotoSize{{FileId: "photo-1"}}
		ctx.EffectiveMessage.Caption = caption
		ctx.EffectiveMessage.CaptionEntities = entities
		if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatal(err)
		}
	}

	// The allowlisted host, with a path, is exempt.
	send(201, "visit https://example.com/docs", []gotgbot.MessageEntity{
		urlEntity("visit https://example.com/docs", "https://example.com/docs"),
	})
	// A caption pairing the allowlisted host with an unapproved one still matches.
	mixed := "https://example.com https://blocked.test"
	sendCaption(202, mixed, []gotgbot.MessageEntity{
		urlEntity(mixed, "https://example.com"),
		urlEntity(mixed, "https://blocked.test"),
	})
	// A lookalike host does not inherit the exception.
	lookalike := "https://example.com https://example.com.evil.test"
	sendCaption(203, lookalike, []gotgbot.MessageEntity{
		urlEntity(lookalike, "https://example.com"),
		urlEntity(lookalike, "https://example.com.evil.test"),
	})

	deleted := deletedMessageIDs(f.client, f.chat.Id)
	if deleted["201"] {
		t.Error("message whose only URL host is allowlisted was deleted")
	}
	for _, id := range []string{"202", "203"} {
		if !deleted[id] {
			t.Errorf("message %s was not deleted; deleted %v", id, deleted)
		}
	}
}

// linkURLEntity builds a url entity covering target inside text.
func linkURLEntity(text, target string) gotgbot.MessageEntity {
	return gotgbot.MessageEntity{Type: "url", Offset: int64(strings.Index(text, target)), Length: int64(len(target))}
}

func deletedMessageIDs(client *moduleBotClient, chatID int64) map[string]bool {
	deleted := map[string]bool{}
	for _, call := range client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			deleted[fmt.Sprint(call.Params["message_id"])] = true
		}
	}
	return deleted
}

// The invitelink lock works on its own: with the url lock off, ordinary URLs
// survive while Telegram invite links are deleted, and an allowlisted invite
// identity exempts only itself.
func TestInviteLinkLockIndependentAndAllowlisted(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	if err := locks.UpdateLock(f.chat.Id, "invitelink", true); err != nil {
		t.Fatalf("UpdateLock invitelink setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/allowlist t.me/+GoodHash").Update, nil); err != nil {
		t.Fatal(err)
	}

	send := func(id int64, text, target string) {
		t.Helper()
		ctx := newModuleMessageContext(f.bot, f.chat, f.member, text)
		ctx.EffectiveMessage.MessageId = id
		ctx.EffectiveMessage.Entities = []gotgbot.MessageEntity{linkURLEntity(text, target)}
		if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatal(err)
		}
	}

	send(301, "https://example.com/page", "https://example.com/page") // not an invite: url lock is off
	send(302, "https://t.me/+GoodHash", "https://t.me/+GoodHash")     // allowlisted invite
	send(303, "https://t.me/+OtherHash", "https://t.me/+OtherHash")   // unapproved invite
	send(304, "join us at https://t.me/+OtherHash now", "https://t.me/+OtherHash")

	deleted := deletedMessageIDs(f.client, f.chat.Id)
	for _, id := range []string{"301", "302"} {
		if deleted[id] {
			t.Errorf("message %s was deleted; deleted %v", id, deleted)
		}
	}
	for _, id := range []string{"303", "304"} {
		if !deleted[id] {
			t.Errorf("message %s was not deleted; deleted %v", id, deleted)
		}
	}
}

// /allowlist and /rmallowlist add, list and remove entries through the real
// dispatcher; entries round-trip through the repository.
func TestAllowlistCommandsAddListRemove(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	process := func(text string) {
		t.Helper()
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text).Update, nil); err != nil {
			t.Fatal(err)
		}
	}

	process("/allowlist example.com t.me/+HashOne t.me/+HashTwo")
	entries := locks.GetChatAllowlist(f.chat.Id)
	if len(entries) != 3 {
		t.Fatalf("allowlist entries = %v, want 3", entries)
	}
	kinds := map[string]string{}
	for _, e := range entries {
		kinds[e.Value] = e.Kind
	}
	if kinds["example.com"] != "host" || kinds["+HashOne"] != "invite" || kinds["+HashTwo"] != "invite" {
		t.Fatalf("allowlist kinds = %v", kinds)
	}

	// A public username is not an invite: the entry is refused, not stored.
	process("/allowlist @SomeChat")
	last := f.client.callsFor("sendMessage")
	if len(last) == 0 {
		t.Fatal("no sendMessage for /allowlist @SomeChat")
	}
	if reply := fmt.Sprint(last[len(last)-1].Params["text"]); !strings.Contains(reply, "@SomeChat") {
		t.Errorf("/allowlist @SomeChat reply %q does not report it invalid", reply)
	}
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 3 {
		t.Fatalf("allowlist entries after username add = %d, want 3", got)
	}

	// A phone-number profile is not an invite either: t.me/+15551234567
	// names a user's phone link, so the entry is refused, not stored.
	process("/allowlist t.me/+15551234567")
	last = f.client.callsFor("sendMessage")
	if len(last) == 0 {
		t.Fatal("no sendMessage for /allowlist t.me/+15551234567")
	}
	if reply := fmt.Sprint(last[len(last)-1].Params["text"]); !strings.Contains(reply, "+15551234567") {
		t.Errorf("/allowlist phone profile reply %q does not report it invalid", reply)
	}
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 3 {
		t.Fatalf("allowlist entries after phone profile add = %d, want 3", got)
	}

	// A duplicate add reports already-listed and stores nothing new.
	process("/allowlist example.com")
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 3 {
		t.Fatalf("allowlist entries after duplicate add = %d, want 3", got)
	}

	// /allowlist with no arguments lists the entries.
	process("/allowlist")
	last = f.client.callsFor("sendMessage")
	if len(last) == 0 {
		t.Fatal("no sendMessage for /allowlist list")
	}
	text := fmt.Sprint(last[len(last)-1].Params["text"])
	for _, want := range []string{"example.com", "+HashOne", "+HashTwo"} {
		if !strings.Contains(text, want) {
			t.Errorf("/allowlist list reply %q missing %q", text, want)
		}
	}

	process("/rmallowlist example.com +HashOne")
	entries = locks.GetChatAllowlist(f.chat.Id)
	if len(entries) != 1 || entries[0].Value != "+HashTwo" {
		t.Fatalf("allowlist after rmallowlist = %v, want only +HashTwo", entries)
	}
}

// A malformed Telegram path or tg: link is refused rather than falling back
// to the host: "t.me/+", a bare username, a preview, a phone lookup, a
// phone-number profile and tg://resolve never become a t.me host
// exception, so the whole host stays unapproved for the url lock.
func TestAllowlistMalformedTelegramLinkIsNotHostException(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	if err := locks.UpdateLock(f.chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	process := func(text string) {
		t.Helper()
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text).Update, nil); err != nil {
			t.Fatal(err)
		}
	}

	// Every malformed or non-invite Telegram entry is refused; none is stored
	// as a host, so t.me itself is never excepted.
	process("/allowlist t.me t.me/+ tg://resolve?domain=somechat t.me/somechat t.me/s/somechat @somechat tg://join?invite= t.me/+15551234567")
	if entries := locks.GetChatAllowlist(f.chat.Id); len(entries) != 0 {
		t.Fatalf("allowlist entries after malformed adds = %v, want none", entries)
	}

	// A real private invite still adds, and only as an invite.
	process("/allowlist t.me/+GoodHash")
	entries := locks.GetChatAllowlist(f.chat.Id)
	if len(entries) != 1 || entries[0].Kind != "invite" || entries[0].Value != "+GoodHash" {
		t.Fatalf("allowlist entries = %v, want one invite +GoodHash", entries)
	}

	// With t.me not excepted, a url-locked message carrying any Telegram link
	// is still deleted — the refused entries never became host exceptions.
	text := "see https://t.me/somechat"
	ctx := newModuleMessageContext(f.bot, f.chat, f.member, text)
	ctx.EffectiveMessage.MessageId = 501
	ctx.EffectiveMessage.Entities = []gotgbot.MessageEntity{linkURLEntity(text, "https://t.me/somechat")}
	if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatal(err)
	}
	if deleted := deletedMessageIDs(f.client, f.chat.Id); !deleted["501"] {
		t.Fatalf("t.me link survived the url lock; deleted %v", deleted)
	}
}

// /rmallowlistall asks the owner, commits once, and refuses replays and
// non-owners.
func TestRmallallowlistAllOwnerOneUse(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	f.makeOwner()
	if err := locks.AddAllowlist(f.chat.Id, "host", "example.com"); err != nil {
		t.Fatalf("AddAllowlist setup error = %v", err)
	}
	if err := locks.AddAllowlist(f.chat.Id, "invite", "+HashOne"); err != nil {
		t.Fatalf("AddAllowlist setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	process := func(text string, from gotgbot.User) {
		t.Helper()
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, from, text).Update, nil); err != nil {
			t.Fatal(err)
		}
	}
	callback := func(from gotgbot.User, data string) {
		t.Helper()
		if err := d.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, f.chat, from, data).Update, nil); err != nil {
			t.Fatal(err)
		}
	}

	// A non-owner cannot even ask.
	process("/rmallowlistall", f.member)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 2 {
		t.Fatalf("allowlist after member /rmallowlistall = %d entries, want unchanged 2", got)
	}

	process("/rmallowlistall", f.admin)
	sent := f.client.callsFor("sendMessage")
	var yesData string
	for i := len(sent) - 1; i >= 0; i-- {
		markup, ok := sent[i].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData, "rmallowlist"); ok {
					if a, _ := decoded.Field("a"); a == "yes" {
						yesData = button.CallbackData
					}
				}
			}
		}
		if yesData != "" {
			break
		}
	}
	if yesData == "" {
		t.Fatal("no yes button on the /rmallowlistall prompt")
	}

	// A member pressing yes is refused and nothing is cleared.
	callback(f.member, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 2 {
		t.Fatalf("allowlist after member press = %d entries, want unchanged 2", got)
	}

	// The owner confirms: entries are gone.
	callback(f.admin, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 0 {
		t.Fatalf("allowlist after owner confirm = %d entries, want 0", got)
	}

	// Replaying the same button finds the spent nonce and commits nothing.
	callback(f.admin, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 0 {
		t.Fatalf("allowlist after replay = %d entries, want 0", got)
	}
}

// Telegram's documented private invite spellings — t.me/+HASH,
// t.me/joinchat/HASH, tg://join?invite=HASH and tg:join?invite=HASH — are
// one invite identity, so allowlisting any form exempts all of them;
// public usernames, previews, bot links, phone lookups and phone-number
// profiles are not invites.
func TestInviteLinkFormsShareOneIdentity(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	if err := locks.UpdateLock(f.chat.Id, "invitelink", true); err != nil {
		t.Fatalf("UpdateLock invitelink setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/allowlist t.me/+GoodHash").Update, nil); err != nil {
		t.Fatal(err)
	}

	send := func(id int64, text string, entity gotgbot.MessageEntity) {
		t.Helper()
		ctx := newModuleMessageContext(f.bot, f.chat, f.member, text)
		ctx.EffectiveMessage.MessageId = id
		ctx.EffectiveMessage.Entities = []gotgbot.MessageEntity{entity}
		if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatal(err)
		}
	}
	textLink := func(text, target string) gotgbot.MessageEntity {
		return gotgbot.MessageEntity{Type: "text_link", Offset: int64(strings.Index(text, target)), Length: int64(len(target)), Url: target}
	}

	send(401, "https://t.me/+GoodHash", linkURLEntity("https://t.me/+GoodHash", "https://t.me/+GoodHash"))
	send(402, "https://t.me/joinchat/GoodHash", linkURLEntity("https://t.me/joinchat/GoodHash", "https://t.me/joinchat/GoodHash"))
	send(413, "tg:join?invite=GoodHash", textLink("tg:join?invite=GoodHash", "tg:join?invite=GoodHash"))
	send(403, "tg://join?invite=GoodHash", textLink("tg://join?invite=GoodHash", "tg://join?invite=GoodHash"))
	send(404, "https://telegram.me/+GoodHash", linkURLEntity("https://telegram.me/+GoodHash", "https://telegram.me/+GoodHash"))
	// Not invites: a login link, a non-group tg:// link, a public preview, a
	// public username, a bot link and a phone lookup.
	send(405, "https://t.me/login/foo", linkURLEntity("https://t.me/login/foo", "https://t.me/login/foo"))
	send(406, "tg://socks?server=127.0.0.1&port=1080", textLink("tg://socks?server=127.0.0.1&port=1080", "tg://socks?server=127.0.0.1&port=1080"))
	send(409, "https://t.me/s/groupname", linkURLEntity("https://t.me/s/groupname", "https://t.me/s/groupname"))
	send(414, "https://t.me/+15551234567", linkURLEntity("https://t.me/+15551234567", "https://t.me/+15551234567"))
	send(410, "https://t.me/groupname", linkURLEntity("https://t.me/groupname", "https://t.me/groupname"))
	send(411, "https://t.me/AlitaTestBot", linkURLEntity("https://t.me/AlitaTestBot", "https://t.me/AlitaTestBot"))
	send(412, "tg://resolve?domain=15551234567", textLink("tg://resolve?domain=15551234567", "tg://resolve?domain=15551234567"))
	send(415, "tg:join?invite=OtherHash", textLink("tg:join?invite=OtherHash", "tg:join?invite=OtherHash"))
	// Unapproved invites still match, in every spelling.
	send(407, "tg://join?invite=OtherHash", textLink("tg://join?invite=OtherHash", "tg://join?invite=OtherHash"))
	send(408, "https://t.me/+OtherHash", linkURLEntity("https://t.me/+OtherHash", "https://t.me/+OtherHash"))

	deleted := deletedMessageIDs(f.client, f.chat.Id)
	for _, id := range []string{"401", "402", "403", "404", "405", "406", "409", "410", "411", "412", "413", "414"} {
		if deleted[id] {
			t.Errorf("message %s was deleted; deleted %v", id, deleted)
		}
	}
	for _, id := range []string{"407", "408", "415"} {
		if !deleted[id] {
			t.Errorf("message %s was not deleted; deleted %v", id, deleted)
		}
	}
}

// /allowlist and /rmallowlist sent from PM name the group they act on.
func TestAllowlistPMRepliesNameGroup(t *testing.T) {
	useEnglishLocale(t)
	withMiniredis(t)
	f := oneOffFixture(t)
	f.connectAdmin(t)
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	want := strings.NewReplacer("{title}", f.chat.Title, "{id}", fmt.Sprint(f.chat.Id)).
		Replace(translated(t, "connections_pm_group_confirm_target"))

	for _, text := range []string{"/allowlist", "/allowlist example.com", "/rmallowlist example.com"} {
		before := len(f.client.callsFor("sendMessage"))
		processPM(t, d, f, f.admin.Id, text)

		replies := f.client.callsFor("sendMessage")[before:]
		if len(replies) != 1 {
			t.Fatalf("%s from PM sent %d replies, want one; sent:\n%s", text, len(replies), sentText(f.client))
		}
		if got := fmt.Sprint(replies[0].Params["text"]); !strings.Contains(got, want) {
			t.Fatalf("%s from PM replied %q, want it to name the group %q", text, got, want)
		}
	}
}

// /rmallowlistall from PM binds its one-use confirmation to the asking owner:
// another owner or a member cannot spend it, a replay cannot either, and the
// button data stays inside Telegram's 64-byte limit for the largest signed
// group ID.
func TestRmallallowlistAllPMActorBindingAndCallbackLength(t *testing.T) {
	useEnglishLocale(t)
	withMiniredis(t)
	f := oneOffFixture(t)
	// The largest signed group ID makes the "c" field as long as it gets.
	f.chat.Id = math.MinInt64
	f.client.chatsByID[fmt.Sprint(f.chat.Id)] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, f.chat.Title))
	f.makeOwner()
	f.connectAdmin(t)
	if err := locks.AddAllowlist(f.chat.Id, "host", "example.com"); err != nil {
		t.Fatalf("AddAllowlist setup error = %v", err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	processPM(t, d, f, f.admin.Id, "/rmallowlistall")

	var yesData string
	for _, call := range f.client.callsFor("sendMessage") {
		markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData, "rmallowlist"); ok {
					if a, _ := decoded.Field("a"); a == "yes" {
						yesData = button.CallbackData
					}
				}
			}
		}
	}
	if yesData == "" {
		t.Fatal("no yes button on the /rmallowlistall prompt")
	}
	if len(yesData) > 64 {
		t.Fatalf("callback data is %d bytes, over Telegram's 64-byte limit: %q", len(yesData), yesData)
	}

	// A member's press is refused outright.
	pressConfirmInPM(t, d, f, f.member, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 1 {
		t.Fatalf("allowlist after member press = %d entries, want unchanged 1", got)
	}

	// Another owner passes the owner check but cannot spend the asker's nonce.
	coOwner := gotgbot.User{Id: 555, FirstName: "Co"}
	f.client.membersByUser[fmt.Sprint(coOwner.Id)] = json.RawMessage(
		`{"status":"creator","user":{"id":555,"is_bot":false,"first_name":"Co"}}`)
	pressConfirmInPM(t, d, f, coOwner, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 1 {
		t.Fatalf("allowlist after another owner's press = %d entries, want unchanged 1", got)
	}

	// The asker confirms: entries are gone.
	pressConfirmInPM(t, d, f, f.admin, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 0 {
		t.Fatalf("allowlist after owner confirm = %d entries, want 0", got)
	}

	// Replaying the same button finds the spent nonce and commits nothing.
	pressConfirmInPM(t, d, f, f.admin, yesData)
	if got := len(locks.GetChatAllowlist(f.chat.Id)); got != 0 {
		t.Fatalf("allowlist after replay = %d entries, want 0", got)
	}
}

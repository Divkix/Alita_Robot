package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamfilter"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/spamtext"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// spamHamNamespace is the callback namespace of the "not spam" button on a
// match's log entry: spamham|v1|c=<group>&s=<sample id>.
const spamHamNamespace = "spamham"

// spamFilterTag tags a match's log entry.
const spamFilterTag = "SPAMFILTER"

const (
	// spamCorpusTTL is how long this replica keeps a chat's decoded samples
	// and the shared corpus before reading them again; a write here clears
	// them at once, a write on another replica is seen within it.
	spamCorpusTTL = 10 * time.Second
	// spamCorpusChats bounds the chats whose samples this replica keeps;
	// past it, the expired ones are dropped.
	spamCorpusChats = 2048
	// spamVectorMemo bounds the n-gram vectors kept by text, so a sample is
	// hashed once rather than on every reload. Past it the memo starts over.
	spamVectorMemo = 4096
)

// spamCorpusEntry is one sample ready to compare.
type spamCorpusEntry struct {
	id     int64
	chatID int64
	vector spamtext.Vector
}

type spamCorpus struct {
	spam, ham []spamCorpusEntry
	loadedAt  time.Time
}

var spamCorpora = struct {
	mu      sync.Mutex
	chats   map[int64]spamCorpus
	shared  spamCorpus
	vectors map[string]spamtext.Vector
}{
	chats:   make(map[int64]spamCorpus),
	vectors: make(map[string]spamtext.Vector),
}

// spamCorpusForget drops this replica's copy of the chat's samples and of
// the shared corpus, after a write that changed them.
// Every sample mutation must call this or spamCorpusForgetAll after the repository has
// invalidated its own sample cache.
func spamCorpusForget(chatID int64) {
	spamCorpora.mu.Lock()
	delete(spamCorpora.chats, chatID)
	spamCorpora.shared = spamCorpus{}
	spamCorpora.mu.Unlock()
}

// spamCorpusForgetAll drops every chat's copy, after the retention sweep.
func spamCorpusForgetAll() {
	spamCorpora.mu.Lock()
	spamCorpora.chats = make(map[int64]spamCorpus)
	spamCorpora.shared = spamCorpus{}
	spamCorpora.mu.Unlock()
}

// spamVector returns the vector of a stored normalised text, hashing it only
// the first time. The caller holds spamCorpora.mu.
func spamVector(normalized string) spamtext.Vector {
	key := spamfilter.Hash(normalized)
	if v, ok := spamCorpora.vectors[key]; ok {
		return v
	}
	if len(spamCorpora.vectors) >= spamVectorMemo {
		spamCorpora.vectors = make(map[string]spamtext.Vector)
	}
	v := spamtext.Vectorize(normalized)
	spamCorpora.vectors[key] = v
	return v
}

// spamChatCorpus returns the chat's own samples, split by label.
func spamChatCorpus(ctx context.Context, chatID int64, now time.Time) (spamCorpus, error) {
	spamCorpora.mu.Lock()
	corpus, ok := spamCorpora.chats[chatID]
	spamCorpora.mu.Unlock()
	if ok && now.Sub(corpus.loadedAt) < spamCorpusTTL {
		return corpus, nil
	}

	samples, err := spamfilter.List(ctx, chatID)
	if err != nil {
		return spamCorpus{}, err
	}
	corpus = spamCorpus{loadedAt: now}
	spamCorpora.mu.Lock()
	defer spamCorpora.mu.Unlock()
	for _, s := range samples {
		entry := spamCorpusEntry{id: s.ID, chatID: s.ChatID, vector: spamVector(s.Normalized)}
		if s.Label == models.SpamLabelHam {
			corpus.ham = append(corpus.ham, entry)
		} else {
			corpus.spam = append(corpus.spam, entry)
		}
	}
	if len(spamCorpora.chats) >= spamCorpusChats {
		for id, c := range spamCorpora.chats {
			if now.Sub(c.loadedAt) >= spamCorpusTTL {
				delete(spamCorpora.chats, id)
			}
		}
	}
	spamCorpora.chats[chatID] = corpus
	return corpus, nil
}

// spamSharedCorpus returns texts flagged by two unrelated chats via spamfilter.Shared.
// Both owners must be known and different, with no shared human admin.
func spamSharedCorpus(ctx context.Context, now time.Time) (spamCorpus, error) {
	spamCorpora.mu.Lock()
	corpus := spamCorpora.shared
	spamCorpora.mu.Unlock()
	if !corpus.loadedAt.IsZero() && now.Sub(corpus.loadedAt) < spamCorpusTTL {
		return corpus, nil
	}

	shared, err := spamfilter.Shared(ctx)
	if err != nil {
		return spamCorpus{}, err
	}
	corpus = spamCorpus{loadedAt: now}
	spamCorpora.mu.Lock()
	defer spamCorpora.mu.Unlock()
	for _, s := range shared {
		corpus.spam = append(corpus.spam, spamCorpusEntry{id: s.ID, chatID: s.ChatID, vector: spamVector(s.Normalized)})
	}
	spamCorpora.shared = corpus
	return corpus, nil
}

// spamMatch is the sample a message matched.
type spamMatch struct {
	sampleID   int64
	sampleChat int64
	similarity float64
}

// bestSpamMatch returns the entry most similar to v among those that can
// reach floor, or a zero match.
func bestSpamMatch(entries []spamCorpusEntry, v spamtext.Vector, floor float64) spamMatch {
	var best spamMatch
	for _, e := range entries {
		if !spamtext.CanReach(v, e.vector, max(floor, best.similarity)) {
			continue
		}
		if s := spamtext.Similarity(v, e.vector); s > best.similarity {
			best = spamMatch{sampleID: e.id, sampleChat: e.chatID, similarity: s}
		}
	}
	return best
}

// spamFilterMatch compares v only with the chat's own spam and the shared corpus. A
// message matches when its closest spam text is at least threshold similar
// and closer than any text the chat called ham.
func spamFilterMatch(ctx context.Context, chatID int64, v spamtext.Vector, threshold float64, now time.Time) (spamMatch, bool, error) {
	own, err := spamChatCorpus(ctx, chatID, now)
	if err != nil {
		return spamMatch{}, false, err
	}
	shared, err := spamSharedCorpus(ctx, now)
	if err != nil {
		return spamMatch{}, false, err
	}
	best := bestSpamMatch(own.spam, v, threshold)
	if fromShared := bestSpamMatch(shared.spam, v, threshold); fromShared.similarity > best.similarity {
		best = fromShared
	}
	if best.sampleID == 0 || best.similarity < threshold {
		return spamMatch{}, false, nil
	}
	if ham := bestSpamMatch(own.ham, v, best.similarity); ham.sampleID != 0 && ham.similarity >= best.similarity {
		return spamMatch{}, false, nil
	}
	return best, true, nil
}

// spamFilterCheck runs in the aispam watcher before any Jev call and reports whether
// it deleted the message, in which case the AI check is skipped. The filter defaults
// to off, costing one cached settings read. In dryrun mode and for seven days after
// the chat enables the filter, a match is only logged and the message kept,
// so the AI check still runs.
func spamFilterCheck(b *gotgbot.Bot, ctx *ext.Context, text string) bool {
	chat, msg, user := ctx.EffectiveChat, ctx.EffectiveMessage, ctx.EffectiveUser
	reqCtx := tracing.UpdateContext(ctx)
	settings, err := spamfilter.GetSettings(reqCtx, chat.Id)
	if err != nil || settings.Mode == models.SpamFilterOff {
		return false
	}
	normalized := spamtext.Normalize(text)
	if spamtext.RuneCount(normalized) < spamtext.MinRunes {
		return false
	}
	if aispamCarvedOut(b, ctx, chat.Id, user.Id) {
		return false
	}

	now := spamFilterNow()
	match, ok, err := spamFilterMatch(reqCtx, chat.Id, spamtext.Vectorize(normalized), settings.Threshold, now)
	if err != nil {
		log.WithError(err).WithField("chat_id", chat.Id).Warn("[SpamFilter] could not read the samples")
		return false
	}
	if !ok {
		return false
	}

	acting := spamfilter.Acting(settings, now)
	if acting {
		if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId); err != nil {
			log.WithError(err).WithField("chat_id", chat.Id).Warn("[SpamFilter] could not delete a match")
			return true
		}
	}
	spamFilterLogMatch(b, ctx, text, match, acting)
	return acting
}

// spamFilterLogMatch logs a match to the automated category with a "not
// spam" button. Like the rest of the entry the field names are English; the
// button is in the group's language.
func spamFilterLogMatch(b *gotgbot.Bot, ctx *ext.Context, text string, match spamMatch, deleted bool) {
	chat, user := ctx.EffectiveChat, ctx.EffectiveUser
	source := "this chat"
	if match.sampleChat != chat.Id {
		source = "shared"
	}
	action := "dry run, message kept"
	if deleted {
		action = "message deleted"
	}

	var markup *gotgbot.InlineKeyboardMarkup
	data := encodeCallbackData(spamHamNamespace, map[string]string{
		"c": strconv.FormatInt(chat.Id, 10),
		"s": strconv.FormatInt(match.sampleID, 10),
	})
	if data != "" {
		tr := i18n.MustNewTranslator(lang.GetGroupLanguage(tracing.UpdateContext(ctx), chat.Id))
		label, _ := tr.GetString("spamfilter_not_spam_button")
		markup = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: label, CallbackData: data},
		}}}
	}
	actionlog.AutomatedWithMarkup(b, chat, markup, spamFilterTag, user.Id,
		formatting.GetFullName(user.FirstName, user.LastName),
		actionlog.Field{Name: "Similarity", Value: strconv.FormatFloat(match.similarity, 'f', 2, 64)},
		actionlog.Field{Name: "Sample", Value: fmt.Sprintf("#%d (%s)", match.sampleID, source)},
		actionlog.Field{Name: "Action", Value: action},
		// The message goes last: the button reads it back from the entry.
		actionlog.Field{Name: "Message", Value: aispamTruncate(text, spamtext.MaxRunes)},
	)
}

// spamLogMessageMarker opens the message in a match's log entry as Telegram
// returns it, without formatting.
const spamLogMessageMarker = "\nMessage: "

// spamHamButton handles "not spam" on a match's log entry. The entry lives
// in the log channel, but the presser is checked against the group, like a
// report card: an admin of the channel alone can do nothing. The group's own
// sample is removed and the message stored as the group's ham; a shared
// sample stays, and the ham keeps it from matching in this group.
func (moduleStruct) spamHamButton(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	decoded, decodedOK := decodeCallbackData(query.Data, spamHamNamespace)
	var chatID, sampleID int64
	if decodedOK {
		chatRaw, _ := decoded.Field("c")
		sampleRaw, _ := decoded.Field("s")
		chatID, _ = strconv.ParseInt(chatRaw, 10, 64)
		sampleID, _ = strconv.ParseInt(sampleRaw, 10, 64)
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chatID < 0 {
		tr = i18n.MustNewTranslator(lang.GetGroupLanguage(tracing.UpdateContext(ctx), chatID))
	}
	answer := func(key string) error {
		text, _ := tr.GetString(key)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	entry := spamLogEntry(query.Message)
	// Callback data can be forged: the entry must be this group's.
	if chatID >= 0 || sampleID <= 0 || entry == nil || !spamLogEntryOf(entry.Text, chatID) {
		log.Warnf("[SpamFilter] invalid not-spam callback: %s", query.Data)
		return answer("spamfilter_not_spam_invalid")
	}

	user := query.From
	source := &gotgbot.Chat{Id: chatID, Type: "supergroup"}
	responder := chat_status.NewPermissionResponder(b)
	if !chat_status.IsUserAdmin(b, source.Id, user.Id) {
		responder.Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithTranslator(tr))
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, source, user.Id) {
		responder.Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error", chat_status.WithTranslator(tr))
		return ext.EndGroups
	}

	text := ""
	if _, after, found := strings.Cut(entry.Text, spamLogMessageMarker); found {
		text = after
	}
	if err := spamfilter.MarkHam(chatID, sampleID, text, user.Id); err != nil {
		log.WithError(err).WithField("chat_id", chatID).Error("[SpamFilter] could not store a not-spam press")
		_ = answer("spamfilter_not_spam_failed")
		return err
	}
	spamCorpusForget(chatID)
	closeReportCard(b, query.Message, "not spam", user)
	return answer("spamfilter_not_spam_done")
}

// spamLogEntry returns the log entry a button was pressed on, or nil when
// Telegram no longer lets the bot read it.
func spamLogEntry(card gotgbot.MaybeInaccessibleMessage) *gotgbot.Message {
	switch m := card.(type) {
	case gotgbot.Message:
		return &m
	case *gotgbot.Message:
		return m
	}
	return nil
}

// spamLogEntryOf reports whether entry, a log entry's text, is the group's:
// actionlog opens every entry with the group's title and ID on one line.
func spamLogEntryOf(entry string, chatID int64) bool {
	header, _, _ := strings.Cut(entry, "\n")
	return strings.HasSuffix(header, fmt.Sprintf("(%d)", chatID)) &&
		strings.Contains(entry, "\n#"+spamFilterTag+"\n")
}

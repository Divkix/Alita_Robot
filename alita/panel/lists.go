package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// listKind names one of the three per-chat text lists the dashboard manages.
type listKind string

const (
	listFilters    listKind = "filters"
	listNotes      listKind = "notes"
	listBlacklists listKind = "blacklists"
)

var listKinds = []listKind{listFilters, listNotes, listBlacklists}

const (
	listPageDefault = 100
	listPageMax     = 500
	listImportMax   = 500

	filterKeywordMaxRunes = 100
	blacklistWordMaxRunes = 100
	listReplyMaxRunes     = 4096
	filterPerChatMax      = 150
)

const (
	contentTypeJSON  = "application/json; charset=utf-8"
	contentTypePlain = "text/plain; charset=utf-8"
)

// Import and delete result statuses.
const (
	statusCreated   = "created"
	statusDuplicate = "duplicate"
	statusInvalid   = "invalid"
	statusRemoved   = "removed"
	statusMissing   = "missing"
)

// listEntry is one dashboard list item: a filter keyword, a note name or a
// blacklist word, with the text the bot replies with (empty for blacklists).
// Panel-created entries are text-only; media replies stay with the commands.
type listEntry struct {
	Name  string `json:"name"`
	Reply string `json:"reply,omitempty"`
}

type listPageResponse struct {
	Entries    []listEntry `json:"entries"`
	NextCursor int         `json:"next_cursor,omitempty"`
	ReadOnly   bool        `json:"read_only"`
}

type listImportResult struct {
	Line   int    `json:"line"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type listImportResponse struct {
	Results []listImportResult `json:"results"`
}

type listImportRequest struct {
	Text    string          `json:"text"`
	Entries json.RawMessage `json:"entries"`
}

// handleListGet serves one page of a list, or the whole matching set as a
// download with ?format=text. Filters and notes export JSON so replies keep
// their newlines; blacklists export one word per line.
func (p *Panel) handleListGet(kind listKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, access, ok := p.chatRequest(w, r)
		if !ok {
			return
		}
		query, limit, offset, ok := parseListPage(w, r)
		if !ok {
			return
		}

		if r.URL.Query().Get("format") == "text" {
			body, contentType, filename, err := exportList(r.Context(), kind, access.ChatID, query)
			if err != nil {
				log.WithError(err).Error("[Panel] Failed to export list")
				writeJSONError(w, http.StatusInternalServerError, "list_failed", "could not read this list")
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(body); err != nil {
				log.WithError(err).Debug("[Panel] Failed to write list export")
			}
			return
		}

		entries, next, err := listEntries(r.Context(), kind, access.ChatID, query, limit, offset)
		if err != nil {
			log.WithError(err).Error("[Panel] Failed to read list")
			writeJSONError(w, http.StatusInternalServerError, "list_failed", "could not read this list")
			return
		}
		writeJSON(w, http.StatusOK, listPageResponse{
			Entries:    entries,
			NextCursor: next,
			ReadOnly:   access.Role != RoleAdmin || !access.BotAdmin,
		})
	}
}

// handleListImport adds a batch of entries through the same repositories the
// commands use, so cache invalidation is identical. A request may carry at
// most 500 entries and is rejected before any write above that. Imports are
// text-only: the body carries a name and a text reply, never media, so every
// created filter or note replies with text.
func (p *Panel) handleListImport(kind listKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, access, ok := p.chatRequest(w, r)
		if !ok {
			return
		}
		if !requireWritable(w, access) {
			return
		}

		entries, ok := readImportEntries(w, r, kind)
		if !ok {
			return
		}
		if len(entries) > listImportMax {
			writeJSONError(w, http.StatusBadRequest, "too_many_entries", "a request can import at most 500 entries")
			return
		}

		results, err := importList(r.Context(), kind, access.ChatID, entries)
		if err != nil {
			log.WithError(err).Error("[Panel] Failed to import list entries")
			writeJSONError(w, http.StatusInternalServerError, "save_failed", "could not save these entries")
			return
		}
		writeJSON(w, http.StatusOK, listImportResponse{Results: results})
	}
}

// handleListDelete removes entries by keyword, name or word through the same
// repository functions the commands use, so the bot stops acting on them as
// soon as the write invalidates the cache.
func (p *Panel) handleListDelete(kind listKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, access, ok := p.chatRequest(w, r)
		if !ok {
			return
		}
		if !requireWritable(w, access) {
			return
		}

		names, ok := readDeleteNames(w, r)
		if !ok {
			return
		}
		if len(names) > listImportMax {
			writeJSONError(w, http.StatusBadRequest, "too_many_entries", "a request can delete at most 500 entries")
			return
		}

		results := make([]listImportResult, 0, len(names))
		for index, raw := range names {
			name := strings.ToLower(strings.TrimSpace(raw))
			result := listImportResult{Line: index + 1, Value: name}
			if name == "" {
				result.Status, result.Error = statusInvalid, "value is required"
				results = append(results, result)
				continue
			}
			removed, err := removeListEntry(r.Context(), kind, access.ChatID, name)
			if err != nil {
				log.WithError(err).Error("[Panel] Failed to delete list entry")
				writeJSONError(w, http.StatusInternalServerError, "delete_failed", "could not delete this entry")
				return
			}
			if removed {
				result.Status = statusRemoved
			} else {
				result.Status = statusMissing
			}
			results = append(results, result)
		}
		writeJSON(w, http.StatusOK, listImportResponse{Results: results})
	}
}

// parseListPage reads ?q, ?limit and ?cursor (or ?offset) from a list request.
func parseListPage(w http.ResponseWriter, r *http.Request) (query string, limit, offset int, ok bool) {
	values := r.URL.Query()
	query = strings.ToLower(strings.TrimSpace(values.Get("q")))

	limit = listPageDefault
	if raw := values.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return "", 0, 0, false
		}
		limit = min(parsed, listPageMax)
	}

	rawCursor := values.Get("cursor")
	if rawCursor == "" {
		rawCursor = values.Get("offset")
	}
	if rawCursor != "" {
		parsed, err := strconv.Atoi(rawCursor)
		if err != nil || parsed < 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_cursor", "cursor must be a non-negative integer")
			return "", 0, 0, false
		}
		offset = parsed
	}
	return query, limit, offset, true
}

// listEntries returns one page and, when another page exists, the cursor to
// request it. limit<=0 returns every matching entry, which export uses.
func listEntries(ctx context.Context, kind listKind, chatID int64, query string, limit, offset int) ([]listEntry, int, error) {
	switch kind {
	case listFilters, listNotes:
		return repositoryPage(ctx, kind, chatID, query, limit, offset)
	case listBlacklists:
		entries := blacklistEntries(ctx, chatID, query)
		if offset >= len(entries) {
			return []listEntry{}, 0, nil
		}
		if limit <= 0 {
			return entries[offset:], 0, nil
		}
		end := min(offset+limit, len(entries))
		next := 0
		if end < len(entries) {
			next = end
		}
		return entries[offset:end], next, nil
	default:
		return nil, 0, fmt.Errorf("unknown list %q", kind)
	}
}

func repositoryPage(ctx context.Context, kind listKind, chatID int64, query string, limit, offset int) ([]listEntry, int, error) {
	fetchLimit := 0
	if limit > 0 {
		fetchLimit = limit + 1
	}

	entries := make([]listEntry, 0)
	switch kind {
	case listFilters:
		rows, err := filters.ListFiltersContext(ctx, chatID, query, fetchLimit, offset)
		if err != nil {
			return nil, 0, err
		}
		for _, row := range rows {
			entries = append(entries, listEntry{Name: row.KeyWord, Reply: row.FilterReply})
		}
	case listNotes:
		rows, err := notes.ListNotesContext(ctx, chatID, query, fetchLimit, offset)
		if err != nil {
			return nil, 0, err
		}
		for _, row := range rows {
			entries = append(entries, listEntry{Name: row.NoteName, Reply: row.NoteContent})
		}
	default:
		return nil, 0, fmt.Errorf("unknown repository list %q", kind)
	}

	next := 0
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
		next = offset + limit
	}
	return entries, next, nil
}

// blacklistEntries reads the chat's cached words and applies the same
// normalisation the bot uses when matching: the stored word and the query are
// both lower-cased. Words are sorted so pages are stable.
func blacklistEntries(ctx context.Context, chatID int64, query string) []listEntry {
	rows := blacklists.GetBlacklistSettingsContext(ctx, chatID)
	entries := make([]listEntry, 0, len(rows))
	for _, row := range rows {
		word := strings.ToLower(row.Word)
		if query != "" && !strings.Contains(word, query) {
			continue
		}
		entries = append(entries, listEntry{Name: word})
	}
	slices.SortFunc(entries, func(a, b listEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return entries
}

func exportList(ctx context.Context, kind listKind, chatID int64, query string) ([]byte, string, string, error) {
	entries, _, err := listEntries(ctx, kind, chatID, query, 0, 0)
	if err != nil {
		return nil, "", "", err
	}

	if kind == listBlacklists {
		var out strings.Builder
		for _, entry := range entries {
			out.WriteString(entry.Name)
			out.WriteByte('\n')
		}
		return []byte(out.String()), contentTypePlain, "blacklists.txt", nil
	}

	body, err := json.Marshal(entries)
	if err != nil {
		return nil, "", "", err
	}
	return body, contentTypeJSON, string(kind) + ".json", nil
}

// readImportBody reads a mutating list request. The body is either a JSON
// array of entries or an object with a line-based "text" field and an
// optional "entries" array.
func readImportBody(w http.ResponseWriter, r *http.Request) (string, json.RawMessage, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
		return "", nil, false
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty_import", `send {"text": "one entry per line"} or a JSON array`)
		return "", nil, false
	}
	if trimmed[0] == '[' {
		return "", json.RawMessage(trimmed), true
	}

	var request listImportRequest
	if err := json.Unmarshal(trimmed, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_import", "body must be a JSON object or array")
		return "", nil, false
	}
	return request.Text, request.Entries, true
}

func readImportEntries(w http.ResponseWriter, r *http.Request, kind listKind) ([]listEntry, bool) {
	text, rawEntries, ok := readImportBody(w, r)
	if !ok {
		return nil, false
	}

	entries := splitImportLines(text, kind)
	if len(rawEntries) > 0 {
		decoded, err := decodeListEntries(rawEntries)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_entries", "entries must be strings or {name, reply} objects")
			return nil, false
		}
		entries = append(entries, decoded...)
	}
	if len(entries) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty_import", `send {"text": "one entry per line"} or a JSON array`)
		return nil, false
	}
	return entries, true
}

func readDeleteNames(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	text, rawEntries, ok := readImportBody(w, r)
	if !ok {
		return nil, false
	}

	names := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	if len(rawEntries) > 0 {
		entries, err := decodeListEntries(rawEntries)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_entries", "entries must be strings or {name} objects")
			return nil, false
		}
		for _, entry := range entries {
			names = append(names, entry.Name)
		}
	}
	if len(names) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty_import", `send {"text": "one entry per line"} or a JSON array`)
		return nil, false
	}
	return names, true
}

// splitImportLines parses the text box: blacklists take the whole line as the
// word, filters and notes take keyword=reply.
func splitImportLines(text string, kind listKind) []listEntry {
	if text == "" {
		return nil
	}
	entries := make([]listEntry, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if kind == listBlacklists {
			entries = append(entries, listEntry{Name: line})
			continue
		}
		name, reply, found := strings.Cut(line, "=")
		if !found {
			entries = append(entries, listEntry{Name: line})
			continue
		}
		entries = append(entries, listEntry{Name: strings.TrimSpace(name), Reply: strings.TrimSpace(reply)})
	}
	return entries
}

func decodeListEntries(raw json.RawMessage) ([]listEntry, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	entries := make([]listEntry, 0, len(items))
	for _, item := range items {
		var name string
		if err := json.Unmarshal(item, &name); err == nil {
			entries = append(entries, listEntry{Name: name})
			continue
		}
		var entry listEntry
		if err := json.Unmarshal(item, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func importList(ctx context.Context, kind listKind, chatID int64, entries []listEntry) ([]listImportResult, error) {
	switch kind {
	case listFilters:
		return importFilters(chatID, entries)
	case listNotes:
		return importNotes(chatID, entries)
	case listBlacklists:
		return importBlacklists(ctx, chatID, entries)
	default:
		return nil, fmt.Errorf("unknown list %q", kind)
	}
}

func importFilters(chatID int64, entries []listEntry) ([]listImportResult, error) {
	count := int(filters.CountFilters(chatID))
	results := make([]listImportResult, 0, len(entries))
	for index, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		result := listImportResult{Line: index + 1, Value: name, Status: statusCreated}
		switch {
		case name == "":
			result.Status, result.Error = statusInvalid, "keyword is required"
		case utf8.RuneCountInString(name) > filterKeywordMaxRunes:
			result.Status, result.Error = statusInvalid, "keyword must be at most 100 characters"
		case strings.TrimSpace(entry.Reply) == "":
			result.Status, result.Error = statusInvalid, "reply is required"
		case utf8.RuneCountInString(entry.Reply) > listReplyMaxRunes:
			result.Status, result.Error = statusInvalid, "reply must be at most 4096 characters"
		case filters.DoesFilterExists(chatID, name):
			result.Status = statusDuplicate
		case count >= filterPerChatMax:
			result.Status, result.Error = statusInvalid, "this chat already has the maximum of 150 filters"
		}
		if result.Status == statusCreated {
			if err := filters.AddFilter(chatID, name, entry.Reply, "", nil, db.TEXT); err != nil {
				return nil, err
			}
			count++
		}
		results = append(results, result)
	}
	return results, nil
}

func importNotes(chatID int64, entries []listEntry) ([]listImportResult, error) {
	results := make([]listImportResult, 0, len(entries))
	for index, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		result := listImportResult{Line: index + 1, Value: name, Status: statusCreated}
		switch {
		case name == "":
			result.Status, result.Error = statusInvalid, "name is required"
		case strings.TrimSpace(entry.Reply) == "":
			result.Status, result.Error = statusInvalid, "reply is required"
		case utf8.RuneCountInString(entry.Reply) > listReplyMaxRunes:
			result.Status, result.Error = statusInvalid, "reply must be at most 4096 characters"
		case notes.DoesNoteExists(chatID, name):
			result.Status = statusDuplicate
		}
		if result.Status == statusCreated {
			if err := notes.AddNote(chatID, name, entry.Reply, "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
				return nil, err
			}
		}
		results = append(results, result)
	}
	return results, nil
}

func importBlacklists(ctx context.Context, chatID int64, entries []listEntry) ([]listImportResult, error) {
	present := make(map[string]struct{})
	for _, row := range blacklists.GetBlacklistSettingsContext(ctx, chatID) {
		present[strings.ToLower(row.Word)] = struct{}{}
	}

	results := make([]listImportResult, 0, len(entries))
	toAdd := make([]string, 0, len(entries))
	for index, entry := range entries {
		word := strings.ToLower(strings.TrimSpace(entry.Name))
		result := listImportResult{Line: index + 1, Value: word}
		switch {
		case word == "":
			result.Status, result.Error = statusInvalid, "word is required"
		case utf8.RuneCountInString(word) > blacklistWordMaxRunes:
			result.Status, result.Error = statusInvalid, "word must be at most 100 characters"
		default:
			if _, ok := present[word]; ok {
				result.Status = statusDuplicate
			} else {
				present[word] = struct{}{}
				result.Status = statusCreated
				toAdd = append(toAdd, word)
			}
		}
		results = append(results, result)
	}

	// One statement and one cache invalidation for the whole request.
	if len(toAdd) > 0 {
		if err := blacklists.AddBlacklistBulk(chatID, toAdd); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func removeListEntry(ctx context.Context, kind listKind, chatID int64, name string) (bool, error) {
	switch kind {
	case listFilters:
		if !filters.DoesFilterExists(chatID, name) {
			return false, nil
		}
		return true, filters.RemoveFilter(chatID, name)
	case listNotes:
		if !notes.DoesNoteExists(chatID, name) {
			return false, nil
		}
		return true, notes.RemoveNote(chatID, name)
	case listBlacklists:
		for _, row := range blacklists.GetBlacklistSettingsContext(ctx, chatID) {
			if strings.EqualFold(row.Word, name) {
				return true, blacklists.RemoveBlacklist(chatID, name)
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown list %q", kind)
	}
}

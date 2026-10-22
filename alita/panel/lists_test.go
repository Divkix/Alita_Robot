//go:build testtools

package panel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// panelTestUserID is the user in testdata/initdata_main.txt.
const panelTestUserID = 279058397

func newListFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, "Lists Chat", fixtureNow)
	fixture.client.admins[chatID] = adminListJSON(panelTestUserID, 999)
	return fixture, chatID
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON fixture: %v", err)
	}
	return string(body)
}

func listTarget(chatID int64, kind string) string {
	return "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10) + "/" + kind
}

func postList(t *testing.T, fixture panelFixture, chatID int64, kind, body string) *httptest.ResponseRecorder {
	t.Helper()
	return panelRequest(t, fixture.handler, http.MethodPost, listTarget(chatID, kind), readFixture(t, "initdata_main.txt"), body, "application/json")
}

func requireImportResults(t *testing.T, recorder *httptest.ResponseRecorder) []listImportResult {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body listImportResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode import response %q: %v", recorder.Body.String(), err)
	}
	return body.Results
}

func listPage(t *testing.T, fixture panelFixture, chatID int64, target string) listPageResponse {
	t.Helper()
	recorder := panelRequest(t, fixture.handler, http.MethodGet, target, readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200 (body %s)", target, recorder.Code, recorder.Body.String())
	}
	var body listPageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func listEntriesOf(t *testing.T, fixture panelFixture, chatID int64, kind, query string) []listEntry {
	t.Helper()
	target := listTarget(chatID, kind) + "?limit=500"
	if query != "" {
		target += "&q=" + url.QueryEscape(query)
	}
	return listPage(t, fixture, chatID, target).Entries
}

func listNamesOf(t *testing.T, fixture panelFixture, chatID int64, kind, query string) []string {
	t.Helper()
	entries := listEntriesOf(t, fixture, chatID, kind, query)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func TestBulkBlacklistImportWritesAllWordsAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newListFixture(t)

	// Seed the cache with the empty list the bot would have read before the
	// import; the bulk write must invalidate it, not leave a stale empty slice.
	if cached := blacklists.GetBlacklistSettings(chatID); len(cached) != 0 {
		t.Fatalf("seeded cache has %d words, want 0", len(cached))
	}

	words := make([]string, 0, 200)
	for i := range 200 {
		words = append(words, fmt.Sprintf("badword%03d", i))
	}
	results := requireImportResults(t, postList(t, fixture, chatID, "blacklists", `{"text":`+mustJSON(t, strings.Join(words, "\n"))+`}`))
	if len(results) != 200 {
		t.Fatalf("results = %d, want 200", len(results))
	}
	for _, result := range results {
		if result.Status != statusCreated {
			t.Fatalf("word %q status = %q (%s), want created", result.Value, result.Status, result.Error)
		}
	}

	// This is the bot's own read path: it goes through the cache, so a missing
	// invalidation would return the seeded empty slice.
	after := blacklists.GetBlacklistSettings(chatID)
	if len(after) != 200 {
		t.Fatalf("cached read after import = %d words, want 200", len(after))
	}
	stored := make(map[string]struct{}, len(after))
	for _, row := range after {
		stored[row.Word] = struct{}{}
	}
	for _, word := range words {
		if _, ok := stored[word]; !ok {
			t.Fatalf("word %q is missing after the bulk import", word)
		}
	}
}

func TestBulkBlacklistImportReportsDuplicates(t *testing.T) {
	fixture, chatID := newListFixture(t)

	results := requireImportResults(t, postList(t, fixture, chatID, "blacklists", `{"text":"badword\ngoodword\nbadword\nBadWord"}`))
	want := []string{statusCreated, statusCreated, statusDuplicate, statusDuplicate}
	if len(results) != len(want) {
		t.Fatalf("results = %d, want %d", len(results), len(want))
	}
	for i, status := range want {
		if results[i].Status != status {
			t.Fatalf("line %d (%q) status = %q, want %q", results[i].Line, results[i].Value, results[i].Status, status)
		}
	}
	if results[1].Value != "goodword" {
		t.Fatalf("line 2 value = %q, want goodword", results[1].Value)
	}

	if got := len(blacklists.GetBlacklistSettings(chatID)); got != 2 {
		t.Fatalf("stored words = %d, want 2", got)
	}
}

func TestBulkImportRejectsMoreThan500Entries(t *testing.T) {
	fixture, chatID := newListFixture(t)

	words := make([]string, 0, listImportMax+1)
	for i := range listImportMax + 1 {
		words = append(words, fmt.Sprintf("word%03d", i))
	}
	recorder := postList(t, fixture, chatID, "blacklists", `{"text":`+mustJSON(t, strings.Join(words, "\n"))+`}`)
	requireAPIError(t, recorder, http.StatusBadRequest, "too_many_entries")

	var blacklistRows int64
	if err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ?", chatID).Count(&blacklistRows).Error; err != nil {
		t.Fatalf("count blacklist rows: %v", err)
	}
	if blacklistRows != 0 {
		t.Fatalf("blacklist rows after rejected import = %d, want 0", blacklistRows)
	}

	entries := make([]listEntry, 0, listImportMax+1)
	for i := range listImportMax + 1 {
		entries = append(entries, listEntry{Name: fmt.Sprintf("key%03d", i), Reply: "reply"})
	}
	recorder = postList(t, fixture, chatID, "filters", mustJSON(t, entries))
	requireAPIError(t, recorder, http.StatusBadRequest, "too_many_entries")

	var filterRows int64
	if err := db.DB.Model(&models.ChatFilters{}).Where("chat_id = ?", chatID).Count(&filterRows).Error; err != nil {
		t.Fatalf("count filter rows: %v", err)
	}
	if filterRows != 0 {
		t.Fatalf("filter rows after rejected import = %d, want 0", filterRows)
	}
}

func TestDeleteBlacklistStopsBotImmediately(t *testing.T) {
	fixture, chatID := newListFixture(t)

	results := requireImportResults(t, postList(t, fixture, chatID, "blacklists", `{"text":"harvest"}`))
	if len(results) != 1 || results[0].Status != statusCreated {
		t.Fatalf("import results = %+v, want one created word", results)
	}
	if cached := blacklists.GetBlacklistSettings(chatID); len(cached) != 1 {
		t.Fatalf("cached words before delete = %d, want 1", len(cached))
	}

	recorder := panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "blacklists"), readFixture(t, "initdata_main.txt"), `{"entries":["harvest"]}`, "application/json")
	results = requireImportResults(t, recorder)
	if len(results) != 1 || results[0].Status != statusRemoved {
		t.Fatalf("delete results = %+v, want one removed word", results)
	}

	// The bot matches messages against this cached read; an empty result means
	// it stops acting on the word on the very next message.
	if cached := blacklists.GetBlacklistSettings(chatID); len(cached) != 0 {
		t.Fatalf("cached words after delete = %d, want 0", len(cached))
	}
}

func TestDeleteFilterStopsBotImmediately(t *testing.T) {
	fixture, chatID := newListFixture(t)

	requireImportResults(t, postList(t, fixture, chatID, "filters", `{"entries":[{"name":"ping","reply":"pong"}]}`))
	// The filter watcher matches against this cached keyword list.
	if cached := filters.GetFiltersList(chatID); !reflect.DeepEqual(cached, []string{"ping"}) {
		t.Fatalf("cached filters before delete = %v, want [ping]", cached)
	}

	recorder := panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "filters"), readFixture(t, "initdata_main.txt"), `{"entries":["ping"]}`, "application/json")
	results := requireImportResults(t, recorder)
	if len(results) != 1 || results[0].Status != statusRemoved {
		t.Fatalf("delete results = %+v, want one removed filter", results)
	}

	if cached := filters.GetFiltersList(chatID); len(cached) != 0 {
		t.Fatalf("cached filters after delete = %v, want none", cached)
	}
}

func TestFilterExportReimportsToTheSameSet(t *testing.T) {
	fixture, chatID := newListFixture(t)

	source := []listEntry{
		{Name: "alpha", Reply: "<b>bold</b>"},
		{Name: "beta", Reply: "line one\nline two"},
	}
	requireImportResults(t, postList(t, fixture, chatID, "filters", `{"entries":`+mustJSON(t, source)+`}`))

	recorder := panelRequest(t, fixture.handler, http.MethodGet, listTarget(chatID, "filters")+"?format=text", readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != contentTypeJSON {
		t.Fatalf("export Content-Type = %q, want %q", got, contentTypeJSON)
	}
	var exported []listEntry
	if err := json.Unmarshal(recorder.Body.Bytes(), &exported); err != nil {
		t.Fatalf("decode export %q: %v", recorder.Body.String(), err)
	}
	if !reflect.DeepEqual(exported, source) {
		t.Fatalf("exported = %+v, want %+v", exported, source)
	}

	otherID := uniquePanelChatID()
	seedPanelChat(t, otherID, "Other Lists Chat", fixtureNow)
	fixture.client.admins[otherID] = adminListJSON(panelTestUserID, 999)

	requireImportResults(t, postList(t, fixture, otherID, "filters", recorder.Body.String()))

	reimported := listEntriesOf(t, fixture, otherID, "filters", "")
	if !reflect.DeepEqual(reimported, exported) {
		t.Fatalf("re-imported = %+v, want the exported set %+v", reimported, exported)
	}
}

func TestBlacklistExportIsOneWordPerLine(t *testing.T) {
	fixture, chatID := newListFixture(t)

	requireImportResults(t, postList(t, fixture, chatID, "blacklists", `{"text":"beer\nwine\nrum"}`))

	recorder := panelRequest(t, fixture.handler, http.MethodGet, listTarget(chatID, "blacklists")+"?format=text", readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != contentTypePlain {
		t.Fatalf("export Content-Type = %q, want %q", got, contentTypePlain)
	}
	if got := recorder.Body.String(); got != "beer\nrum\nwine\n" {
		t.Fatalf("export = %q, want one sorted word per line", got)
	}

	otherID := uniquePanelChatID()
	seedPanelChat(t, otherID, "Other Lists Chat", fixtureNow)
	fixture.client.admins[otherID] = adminListJSON(panelTestUserID, 999)

	requireImportResults(t, postList(t, fixture, otherID, "blacklists", `{"text":`+mustJSON(t, recorder.Body.String())+`}`))

	if got, want := listNamesOf(t, fixture, otherID, "blacklists", ""), []string{"beer", "rum", "wine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("re-imported words = %v, want %v", got, want)
	}
}

func TestFilterValueWithHTMLAndNewlineRoundTrips(t *testing.T) {
	fixture, chatID := newListFixture(t)
	reply := "<script>alert('x')</script>\nsecond & line"

	requireImportResults(t, postList(t, fixture, chatID, "filters", `{"entries":[{"name":"markup","reply":`+mustJSON(t, reply)+`}]}`))

	recorder := panelRequest(t, fixture.handler, http.MethodGet, listTarget(chatID, "filters"), readFixture(t, "initdata_main.txt"), "", "")
	entries := listPage(t, fixture, chatID, listTarget(chatID, "filters")).Entries
	if len(entries) != 1 {
		t.Fatalf("filters = %d, want 1", len(entries))
	}
	if entries[0].Reply != reply {
		t.Fatalf("reply = %q, want %q", entries[0].Reply, reply)
	}
	if strings.Contains(recorder.Body.String(), "<script>") {
		t.Fatalf("raw JSON body contains unescaped HTML: %s", recorder.Body.String())
	}
}

func TestListSearchIsCaseInsensitive(t *testing.T) {
	fixture, chatID := newListFixture(t)

	requireImportResults(t, postList(t, fixture, chatID, "filters", `{"entries":[{"name":"HelloWorLD","reply":"NeedleBody"},{"name":"other","reply":"unrelated"}]}`))
	requireImportResults(t, postList(t, fixture, chatID, "notes", `{"entries":[{"name":"NoteOne","reply":"CamelBody"},{"name":"silent","reply":"zzz"}]}`))
	requireImportResults(t, postList(t, fixture, chatID, "blacklists", `{"text":"MiXeDWord\nplain"}`))

	tests := []struct {
		kind  string
		query string
		want  []string
	}{
		{kind: "filters", query: "hellow", want: []string{"helloworld"}},
		{kind: "filters", query: "NEEDLEBODY", want: []string{"helloworld"}},
		{kind: "notes", query: "noteone", want: []string{"noteone"}},
		{kind: "notes", query: "camelbody", want: []string{"noteone"}},
		{kind: "blacklists", query: "MIXEDWORD", want: []string{"mixedword"}},
	}
	for _, test := range tests {
		if got := listNamesOf(t, fixture, chatID, test.kind, test.query); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s search %q = %v, want %v", test.kind, test.query, got, test.want)
		}
	}
}

func TestListPaginationReturnsStablePages(t *testing.T) {
	fixture, chatID := newListFixture(t)

	requireImportResults(t, postList(t, fixture, chatID, "filters", `{"text":"k1=a\nk2=b\nk3=c\nk4=d\nk5=e"}`))

	first := listPage(t, fixture, chatID, listTarget(chatID, "filters")+"?limit=2")
	if got, want := entryNames(first.Entries), []string{"k1", "k2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 1 = %v, want %v", got, want)
	}
	if first.NextCursor != 2 {
		t.Fatalf("page 1 next_cursor = %d, want 2", first.NextCursor)
	}

	second := listPage(t, fixture, chatID, listTarget(chatID, "filters")+"?limit=2&cursor=2")
	if got, want := entryNames(second.Entries), []string{"k3", "k4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 2 = %v, want %v", got, want)
	}
	if second.NextCursor != 4 {
		t.Fatalf("page 2 next_cursor = %d, want 4", second.NextCursor)
	}

	third := listPage(t, fixture, chatID, listTarget(chatID, "filters")+"?limit=2&cursor=4")
	if got, want := entryNames(third.Entries), []string{"k5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 3 = %v, want %v", got, want)
	}
	if third.NextCursor != 0 {
		t.Fatalf("page 3 next_cursor = %d, want 0", third.NextCursor)
	}
}

func TestNonAdminCannotWriteLists(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, "Lists Chat", fixtureNow)
	fixture.client.admins[chatID] = adminListJSON(999)

	recorder := postList(t, fixture, chatID, "blacklists", `{"text":"forbidden"}`)
	requireAPIError(t, recorder, http.StatusForbidden, "not_admin")

	recorder = panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "blacklists"), readFixture(t, "initdata_main.txt"), `{"entries":["forbidden"]}`, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "not_admin")

	page := listPage(t, fixture, chatID, listTarget(chatID, "blacklists"))
	if !page.ReadOnly {
		t.Fatal("read_only = false for a non-admin, want true")
	}
	if len(page.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(page.Entries))
	}
}

func TestBotWithoutAdminRightsCannotWriteLists(t *testing.T) {
	fixture, chatID := newListFixture(t)
	// The caller stays an admin, but the bot is not in the admin list and
	// getChatMember reports a plain member, so writes must be disabled.
	fixture.client.admins[chatID] = adminListJSON(panelTestUserID)
	fixture.client.botAdmin = false

	recorder := postList(t, fixture, chatID, "blacklists", `{"text":"forbidden"}`)
	requireAPIError(t, recorder, http.StatusConflict, "bot_not_admin")

	page := listPage(t, fixture, chatID, listTarget(chatID, "blacklists"))
	if !page.ReadOnly {
		t.Fatal("read_only = false without bot admin rights, want true")
	}
}

func entryNames(entries []listEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

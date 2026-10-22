package panel

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	fixtureBotToken = "7342037359:AAHI25ES9xCOMPokpYoz-p8XVrZUdygo2J4"
	fixtureAuthUnix = int64(1733509682)
)

var fixtureNow = time.Unix(fixtureAuthUnix, 0)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return strings.TrimSpace(string(data))
}

func encodedWithout(t *testing.T, raw string, fields ...string) string {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, field := range fields {
		values.Del(field)
	}
	return values.Encode()
}

func requireAuthCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %q, got nil", wantCode)
	}
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("error type = %T (%v), want *AuthError", err, err)
	}
	if authErr.Code != wantCode {
		t.Fatalf("error code = %q, want %q", authErr.Code, wantCode)
	}
}

func TestVerifyInitDataAcceptsPublishedFixture(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	session, err := VerifyInitData(raw, fixtureBotToken, fixtureNow, 24*time.Hour)
	if err != nil {
		t.Fatalf("VerifyInitData() error = %v", err)
	}
	if session.User.ID != 279058397 {
		t.Fatalf("user ID = %d, want 279058397", session.User.ID)
	}
	if session.User.FirstName != "Vladislav + - ? /" {
		t.Fatalf("first name = %q, want the URL-decoded value", session.User.FirstName)
	}
	if session.User.Username != "vdkfrost" {
		t.Fatalf("username = %q, want vdkfrost", session.User.Username)
	}
	if session.User.LanguageCode != "ru" {
		t.Fatalf("language = %q, want ru", session.User.LanguageCode)
	}
	if session.AuthDate.Unix() != fixtureAuthUnix {
		t.Fatalf("auth date = %v, want unix %d", session.AuthDate, fixtureAuthUnix)
	}
}

func TestVerifyInitDataRequiresSignatureInCheckString(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	// The published fixture carries signature. Dropping it changes the
	// data-check-string, so HMAC validation must fail; dropping both hash and
	// signature would be the separate Ed25519 third-party procedure.
	_, err := VerifyInitData(encodedWithout(t, raw, "signature"), fixtureBotToken, fixtureNow, 24*time.Hour)
	requireAuthCode(t, err, CodeBadHash)
}

func TestVerifyInitDataAcceptsReorderedFields(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	// Values.Encode sorts the fields, which is a different order from the raw
	// client payload. The data-check-string is order-independent.
	reordered := encodedWithout(t, raw)
	if reordered == raw {
		t.Fatal("fixture was already in sorted order; test would not prove anything")
	}
	if _, err := VerifyInitData(reordered, fixtureBotToken, fixtureNow, 24*time.Hour); err != nil {
		t.Fatalf("VerifyInitData(reordered) error = %v", err)
	}
}

func TestVerifyInitDataSpecialCharactersInFirstName(t *testing.T) {
	raw := readFixture(t, "initdata_special_name.txt")

	session, err := VerifyInitData(raw, fixtureBotToken, fixtureNow, 24*time.Hour)
	if err != nil {
		t.Fatalf("VerifyInitData() error = %v", err)
	}
	if session.User.FirstName != "A&B=C" {
		t.Fatalf("first name = %q, want A&B=C after URL decoding", session.User.FirstName)
	}
	if session.StartParam != "chat_-1001234567890" {
		t.Fatalf("start_param = %q, want chat_-1001234567890", session.StartParam)
	}
	if chatID, ok := ParseStartParamChat(session.StartParam); !ok || chatID != -1001234567890 {
		t.Fatalf("ParseStartParamChat() = (%d, %v), want (-1001234567890, true)", chatID, ok)
	}
}

func TestVerifyInitDataRejectsInvalidPayloads(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	tampered := func() string {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		values.Set("user", strings.Replace(values.Get("user"), `"id":279058397`, `"id":279058398`, 1))
		return values.Encode()
	}()

	tests := []struct {
		name    string
		raw     string
		token   string
		now     time.Time
		maxAge  time.Duration
		wantErr string
	}{
		{
			name:    "missing header",
			raw:     "",
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeMissingInitData,
		},
		{
			name:    "malformed query",
			raw:     "%zz=1",
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeMalformed,
		},
		{
			name:    "missing hash",
			raw:     encodedWithout(t, raw, "hash"),
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeMissingHash,
		},
		{
			name:    "hash is not hex",
			raw:     encodedWithout(t, raw, "hash") + "&hash=not-hex",
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeBadHash,
		},
		{
			name:    "wrong bot token",
			raw:     raw,
			token:   "123456789:AAHnotTheSigningToken",
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeBadHash,
		},
		{
			name:    "tampered user field",
			raw:     tampered,
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeBadHash,
		},
		{
			name:    "stale auth date",
			raw:     raw,
			token:   fixtureBotToken,
			now:     fixtureNow.Add(25 * time.Hour),
			maxAge:  24 * time.Hour,
			wantErr: CodeStaleAuthDate,
		},
		{
			name:    "future auth date",
			raw:     raw,
			token:   fixtureBotToken,
			now:     fixtureNow.Add(-time.Minute),
			maxAge:  time.Hour,
			wantErr: CodeFutureAuthDate,
		},
		{
			name:    "missing auth date",
			raw:     encodedWithout(t, raw, "auth_date"),
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeMissingAuthDate,
		},
		{
			name:    "missing user",
			raw:     encodedWithout(t, raw, "user"),
			token:   fixtureBotToken,
			now:     fixtureNow,
			maxAge:  time.Hour,
			wantErr: CodeMissingUser,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerifyInitData(test.raw, test.token, test.now, test.maxAge)
			requireAuthCode(t, err, test.wantErr)
		})
	}
}

func TestPanelURLHelpers(t *testing.T) {
	if got := WebAppURL("https://panel.example.com/", -100123); got != "https://panel.example.com?chat=-100123" {
		t.Fatalf("WebAppURL() = %q", got)
	}
	if got := StartDeepLink("AlitaBot", -100123); got != "https://t.me/AlitaBot?start=panel_-100123" {
		t.Fatalf("StartDeepLink() = %q", got)
	}
	if got := EntryURL("AlitaBot", "", -100123); got != "https://t.me/AlitaBot?start=panel_-100123" {
		t.Fatalf("EntryURL(no short name) = %q", got)
	}
	if got := EntryURL("AlitaBot", "alita_panel", -100123); got != "https://t.me/AlitaBot/alita_panel?startapp=chat_-100123" {
		t.Fatalf("EntryURL(short name) = %q", got)
	}
	if _, ok := ParseStartParamChat("start"); ok {
		t.Fatal("ParseStartParamChat accepted a non-chat start_param")
	}
	if _, ok := ParseStartParamChat("chat_abc"); ok {
		t.Fatal("ParseStartParamChat accepted a non-numeric chat id")
	}
}

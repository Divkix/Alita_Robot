package panel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultSessionMaxAge bounds how old initData may be when the caller passes a
// non-positive maxAge to VerifyInitData.
const DefaultSessionMaxAge = 24 * time.Hour

// Stable error codes returned for a rejected initData payload. The panel API
// mirrors them in the JSON error body so clients can react without parsing
// messages.
const (
	CodeMissingInitData = "missing_init_data"
	CodeMalformed       = "malformed"
	CodeMissingHash     = "missing_hash"
	CodeBadHash         = "bad_hash"
	CodeMissingAuthDate = "missing_auth_date"
	CodeStaleAuthDate   = "stale_auth_date"
	CodeFutureAuthDate  = "future_auth_date"
	CodeMissingUser     = "missing_user"
)

// AuthError is a rejected initData payload with a stable code.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string { return e.Message }

func newAuthError(code, message string) *AuthError {
	return &AuthError{Code: code, Message: message}
}

// User is the verified Telegram user from initData.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
}

// Session is the trusted result of VerifyInitData.
type Session struct {
	User       User
	AuthDate   time.Time
	StartParam string
}

// VerifyInitData validates a raw Mini App initData payload exactly as Telegram
// documents for bots: URL-decode every value, build a data-check-string from
// every field except hash sorted by key, derive the secret with
// HMAC-SHA-256("WebAppData", botToken), then HMAC-SHA-256 that string and
// compare with hash in constant time.
//
// signature stays in the data-check-string: dropping hash and signature
// together is the separate Ed25519 third-party procedure and rejects real
// payloads from current clients.
func VerifyInitData(raw string, botToken string, now time.Time, maxAge time.Duration) (Session, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Session{}, newAuthError(CodeMissingInitData, "X-Telegram-Init-Data is required")
	}
	if maxAge <= 0 {
		maxAge = DefaultSessionMaxAge
	}

	values, received, err := parseInitData(raw)
	if err != nil {
		return Session{}, err
	}
	if err := checkInitDataHash(values, botToken, received); err != nil {
		return Session{}, err
	}
	authDate, err := checkAuthDate(values["auth_date"][0], now, maxAge)
	if err != nil {
		return Session{}, err
	}
	user, err := decodeUser(values["user"][0])
	if err != nil {
		return Session{}, err
	}

	return Session{
		User:       user,
		AuthDate:   authDate,
		StartParam: values.Get("start_param"),
	}, nil
}

// parseInitData parses raw, decodes its hash, and rejects payloads that are
// missing or repeat required fields.
func parseInitData(raw string) (url.Values, []byte, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, nil, newAuthError(CodeMalformed, "initData is not a valid query string")
	}

	received, err := decodeInitDataHash(values)
	if err != nil {
		return nil, nil, err
	}

	// Presence is structural, so report it before the signature check gives a
	// generic bad_hash for a payload that is missing a required field.
	authDateValues, ok := values["auth_date"]
	if !ok || len(authDateValues) == 0 || authDateValues[0] == "" {
		return nil, nil, newAuthError(CodeMissingAuthDate, "initData has no auth_date")
	}
	userValues, ok := values["user"]
	if !ok || len(userValues) == 0 || strings.TrimSpace(userValues[0]) == "" {
		return nil, nil, newAuthError(CodeMissingUser, "initData has no user")
	}

	for key, fieldValues := range values {
		if key == "hash" {
			continue
		}
		if len(fieldValues) != 1 {
			return nil, nil, newAuthError(CodeMalformed, fmt.Sprintf("initData repeats the %s field", key))
		}
	}
	return values, received, nil
}

// decodeInitDataHash decodes the hex hash field into a SHA-256 digest.
func decodeInitDataHash(values url.Values) ([]byte, error) {
	hashValues, ok := values["hash"]
	if !ok || len(hashValues) == 0 || strings.TrimSpace(hashValues[0]) == "" {
		return nil, newAuthError(CodeMissingHash, "initData has no hash")
	}
	if len(hashValues) > 1 {
		return nil, newAuthError(CodeMalformed, "initData repeats the hash field")
	}
	receivedHex := strings.ToLower(strings.TrimSpace(hashValues[0]))
	received, err := hex.DecodeString(receivedHex)
	if err != nil || len(received) != sha256.Size {
		return nil, newAuthError(CodeBadHash, "initData hash is not a SHA-256 hex digest")
	}
	return received, nil
}

// checkInitDataHash rebuilds the data-check-string and compares its HMAC with
// received in constant time.
func checkInitDataHash(values url.Values, botToken string, received []byte) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = key + "=" + values[key][0]
	}

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = mac.Write([]byte(strings.Join(pairs, "\n")))
	if !hmac.Equal(mac.Sum(nil), received) {
		return newAuthError(CodeBadHash, "initData hash does not match")
	}
	return nil
}

// checkAuthDate parses raw as a Unix timestamp and bounds it by now and maxAge.
func checkAuthDate(raw string, now time.Time, maxAge time.Duration) (time.Time, error) {
	authDateUnix, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, newAuthError(CodeMalformed, "initData auth_date is not a Unix timestamp")
	}
	authDate := time.Unix(authDateUnix, 0)
	if authDate.After(now) {
		return time.Time{}, newAuthError(CodeFutureAuthDate, "initData auth_date is in the future")
	}
	if now.Sub(authDate) > maxAge {
		return time.Time{}, newAuthError(CodeStaleAuthDate, "initData is older than the allowed session age")
	}
	return authDate, nil
}

// decodeUser parses the user field and requires a positive id.
func decodeUser(raw string) (User, error) {
	var user User
	if err := json.Unmarshal([]byte(raw), &user); err != nil {
		return User{}, newAuthError(CodeMalformed, "initData user is not valid JSON")
	}
	if user.ID <= 0 {
		return User{}, newAuthError(CodeMalformed, "initData user has no id")
	}
	return user, nil
}

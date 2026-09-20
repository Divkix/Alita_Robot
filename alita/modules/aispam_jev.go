package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// TypeSafe Jev returns typed decisions with calibrated probabilities instead
// of generated text, so every judgement this module acts on is a number in a
// named answer. Questions and criteria are code-owned constants: no chat
// content reaches them.
const (
	aispamJevModel = "jev-latest"

	// aispamJevTimeout bounds one HTTP round trip. The whole check runs off the
	// update path, so a slow provider costs a worker slot, not a message.
	aispamJevTimeout = 8 * time.Second

	// aispamJevRetryWait is the fallback wait before the single retry when the
	// provider sends no Retry-After.
	aispamJevRetryWait = 500 * time.Millisecond

	// aispamJevMaxRetryWait drops the retry rather than hold a worker on a long
	// provider backoff. A shed check is "not checked", which is the safe side.
	aispamJevMaxRetryWait = 2 * time.Second

	aispamJevMaxResponseBytes = 1 << 20
)

// Swappable so tests can point at an httptest server; nothing else sets them.
var (
	aispamJevEndpoint = "https://api.typesafe.ai/v1/systemone"
	aispamHTTPClient  = &http.Client{
		Timeout: aispamJevTimeout,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 90 * time.Second,
		},
	}
)

// aispamState is what the model is asked to judge. Every number is computed in
// code because Jev cannot count reliably and reads dates as text.
type aispamState struct {
	Chat             aispamChatState    `json:"chat"`
	Sender           aispamSenderState  `json:"sender"`
	Message          aispamMessageState `json:"message"`
	RecentFromSender []string           `json:"recent_from_sender,omitempty"`
}

type aispamChatState struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Rules       string `json:"rules,omitempty"`
	Language    string `json:"language"`
}

type aispamSenderState struct {
	FirstSeenDaysAgo int `json:"first_seen_days_ago"`
	MessagesLastHour int `json:"messages_last_hour"`
	WarnCount        int `json:"warn_count"`
}

type aispamMessageState struct {
	Text        string   `json:"text,omitempty"`
	HasLink     bool     `json:"has_link"`
	LinkDomains []string `json:"link_domains,omitempty"`
	IsForward   bool     `json:"is_forward"`
	IsReply     bool     `json:"is_reply"`
}

type aispamJevRequest struct {
	State     aispamState                  `json:"state"`
	Model     string                       `json:"model"`
	Questions map[string]aispamJevQuestion `json:"questions"`
}

type aispamJevQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

// aispamJevResponse mirrors the documented response envelope: model and usage
// at the top level, and one entry per requested question under answers. Fields
// the module does not act on are still decoded so a shape change is visible.
type aispamJevResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]aispamJevAnswer `json:"answers"`
	Usage   aispamJevUsage             `json:"usage"`
}

type aispamJevAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type aispamJevUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// aispamVerdict is the answered check after validation. Category never drives
// action; it exists only for the audit trail. Language picks the threshold,
// and an answer the module cannot read takes the higher one.
type aispamVerdict struct {
	DeleteProbability float64
	Category          string
	Language          string
	Model             string
	InputTokens       int
	OutputTokens      int
}

func aispamJevQuestions() map[string]aispamJevQuestion {
	return map[string]aispamJevQuestion{
		"decision": {
			Type:         "choice",
			Instructions: "Should this message be deleted from the group as spam?",
			Criteria: map[string]string{
				"delete": "Unsolicited promotion, scam, or mass-posted content the group's rules prohibit",
				"keep":   "Normal conversation, questions, or content the group's rules allow",
			},
		},
		"category": {
			Type:         "choice",
			Instructions: "What kind of spam is this, if any?",
			Criteria: map[string]string{
				"promotion":  "Advertising a product, service, channel or event the sender benefits from",
				"scam":       "Fraudulent or deceptive offer, including investment, crypto and job scams",
				"link_spam":  "A link posted mainly to drive traffic rather than to contribute to the conversation",
				"repetition": "The same or near-identical content posted repeatedly",
				"none":       "Not spam",
			},
		},
		"language": {
			Type:         "choice",
			Instructions: "Is the message text written in English?",
			Criteria: map[string]string{
				"english": "The message text is written in English",
				"other":   "The message text is written in another language, or is too short or mixed to tell",
			},
		},
	}
}

// aispamHTTPError is a non-2xx answer from the provider. 429 and 5xx are worth
// exactly one more attempt; every other status is treated as final.
type aispamHTTPError struct {
	status     int
	retryAfter time.Duration
}

func (e *aispamHTTPError) Error() string {
	return fmt.Sprintf("aispam: typesafe returned status %d", e.status)
}

func (e *aispamHTTPError) retryable() bool {
	return e.status == http.StatusTooManyRequests || e.status >= 500
}

// aispamJevDecide asks the model to judge one message. Any failure -- timeout,
// 429, 5xx, transport error, unparseable answer -- returns an error and no
// verdict, and the caller fails open.
func aispamJevDecide(ctx context.Context, apiKey string, state aispamState) (*aispamVerdict, error) {
	ctx, cancel := context.WithTimeout(ctx, aispamJevTimeout)
	defer cancel()

	verdict, err := aispamJevPost(ctx, apiKey, state)
	if err == nil {
		return verdict, nil
	}

	var httpErr *aispamHTTPError
	if !errors.As(err, &httpErr) || !httpErr.retryable() {
		return nil, err
	}

	wait := httpErr.retryAfter
	if wait <= 0 {
		wait = aispamJevRetryWait
	}
	if wait > aispamJevMaxRetryWait {
		return nil, err
	}
	if !aispamSleepContext(ctx, wait) {
		return nil, err
	}

	return aispamJevPost(ctx, apiKey, state)
}

func aispamJevPost(ctx context.Context, apiKey string, state aispamState) (*aispamVerdict, error) {
	payload, err := json.Marshal(aispamJevRequest{
		State:     state,
		Model:     aispamJevModel,
		Questions: aispamJevQuestions(),
	})
	if err != nil {
		return nil, fmt.Errorf("aispam: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aispamJevEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aispam: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := aispamHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aispam: typesafe request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, aispamJevMaxResponseBytes))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &aispamHTTPError{
			status:     resp.StatusCode,
			retryAfter: aispamParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	var parsed aispamJevResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, aispamJevMaxResponseBytes)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("aispam: decode response: %w", err)
	}

	return aispamVerdictFromResponse(&parsed)
}

func aispamVerdictFromResponse(parsed *aispamJevResponse) (*aispamVerdict, error) {
	decision, ok := parsed.Answers["decision"]
	if !ok {
		return nil, errors.New("aispam: response carried no decision answer")
	}
	probability, ok := decision.Probabilities["delete"]
	if !ok {
		return nil, errors.New("aispam: decision answer carried no delete probability")
	}
	if math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 {
		return nil, fmt.Errorf("aispam: delete probability %v is outside [0,1]", probability)
	}

	return &aispamVerdict{
		DeleteProbability: probability,
		Category:          parsed.Answers["category"].Choice,
		Language:          parsed.Answers["language"].Choice,
		Model:             parsed.Model,
		InputTokens:       parsed.Usage.InputTokens,
		OutputTokens:      parsed.Usage.OutputTokens,
	}, nil
}

// aispamParseRetryAfter reads the Retry-After header in either of its two
// documented forms; an unreadable value falls back to the default wait.
func aispamParseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if wait := time.Until(at); wait > 0 {
			return wait
		}
	}
	return 0
}

// aispamSleepContext waits, reporting false if the context ended first.
func aispamSleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

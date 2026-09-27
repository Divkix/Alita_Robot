package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Default configuration constants for the Jev client.
const (
	DefaultBaseURL = "https://api.typesafe.ai/v1/systemone"
	DefaultModel   = "jev-latest"
	DefaultTimeout = 15 * time.Second
)

// ErrNoAPIKey is returned when an API call is attempted without an API key.
var ErrNoAPIKey = errors.New("typesafe: API key is not configured")

// QuestionType represents the supported question types for the Jev model.
type QuestionType string

const (
	TypeNoul   QuestionType = "noul"
	TypeChoice QuestionType = "choice"
	TypeScore  QuestionType = "score"
)

// Question represents a structured question to ask Jev about the state.
type Question struct {
	Type         QuestionType      `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// NewNoulQuestion creates a boolean (noul) question with the given instructions.
func NewNoulQuestion(instructions string) Question {
	return Question{
		Type:         TypeNoul,
		Instructions: instructions,
	}
}

// NewChoiceQuestion creates a categorical (choice) question with criteria options.
func NewChoiceQuestion(instructions string, criteria map[string]string) Question {
	return Question{
		Type:         TypeChoice,
		Instructions: instructions,
		Criteria:     criteria,
	}
}

// NewScoreQuestion creates a numerical (score) question with criteria levels.
func NewScoreQuestion(instructions string, criteria map[string]string) Question {
	return Question{
		Type:         TypeScore,
		Instructions: instructions,
		Criteria:     criteria,
	}
}

// Answer represents Jev's answer to a question.
type Answer struct {
	Type          QuestionType       `json:"type"`
	Noul          float64            `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// Response contains the structured answers returned by Jev.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage,omitempty"`
}

// Usage contains token usage statistics.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// HTTPDoer is an interface satisfied by *http.Client or custom mocks.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client defines the interface for interacting with TypeSafe AI's Jev model.
type Client interface {
	// Decide sends the state and a map of questions to Jev for structured analysis.
	Decide(ctx context.Context, state string, questions map[string]Question) (*Response, error)
	// AskNoul is a convenience method for asking a single boolean question.
	AskNoul(ctx context.Context, state string, instructions string) (float64, error)
}

type client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient HTTPDoer
}

// Option configures a Jev client.
type Option func(*client)

// WithBaseURL sets a custom API endpoint URL.
func WithBaseURL(url string) Option {
	return func(c *client) {
		if url != "" {
			c.baseURL = url
		}
	}
}

// WithModel sets a custom model identifier.
func WithModel(model string) Option {
	return func(c *client) {
		if model != "" {
			c.model = model
		}
	}
}

// WithHTTPClient sets a custom HTTP client or mock transport.
func WithHTTPClient(httpClient HTTPDoer) Option {
	return func(c *client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// NewClient creates a new Jev API client with the given API key and options.
func NewClient(apiKey string, opts ...Option) Client {
	c := &client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		model:      DefaultModel,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type requestPayload struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Decide submits state and questions to the Jev model and returns the structured response.
func (c *client) Decide(ctx context.Context, state string, questions map[string]Question) (*Response, error) {
	if c.apiKey == "" {
		return nil, ErrNoAPIKey
	}
	if len(questions) == 0 {
		return nil, errors.New("typesafe: questions map cannot be empty")
	}

	payload := requestPayload{
		Model:     c.model,
		State:     state,
		Questions: questions,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("typesafe: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("typesafe: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AlitaBot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("typesafe: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("typesafe: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("typesafe API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var res Response
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("typesafe: unmarshal response: %w", err)
	}

	if res.Answers == nil {
		return nil, errors.New("typesafe: response missing answers map")
	}

	return &res, nil
}

// AskNoul asks a single boolean question and returns the resulting probability (0.0 to 1.0).
func (c *client) AskNoul(ctx context.Context, state string, instructions string) (float64, error) {
	resp, err := c.Decide(ctx, state, map[string]Question{
		"q": NewNoulQuestion(instructions),
	})
	if err != nil {
		return 0, err
	}

	ans, ok := resp.Answers["q"]
	if !ok {
		return 0, errors.New("typesafe: response missing question answer")
	}

	return ans.Noul, nil
}

package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockDoer func(*http.Request) (*http.Response, error)

func (m mockDoer) Do(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestNewClientOptions(t *testing.T) {
	c := NewClient(
		"test-key",
		WithBaseURL("https://custom.endpoint/v1"),
		WithModel("jev-custom"),
		WithHTTPClient(&http.Client{}),
	).(*client)

	if c.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want 'test-key'", c.apiKey)
	}
	if c.baseURL != "https://custom.endpoint/v1" {
		t.Errorf("baseURL = %q, want 'https://custom.endpoint/v1'", c.baseURL)
	}
	if c.model != "jev-custom" {
		t.Errorf("model = %q, want 'jev-custom'", c.model)
	}
}

func TestQuestionConstructors(t *testing.T) {
	noulQ := NewNoulQuestion("Is this urgent?")
	if noulQ.Type != TypeNoul || noulQ.Instructions != "Is this urgent?" {
		t.Errorf("NewNoulQuestion = %+v", noulQ)
	}

	choiceCriteria := map[string]string{"a": "option a", "b": "option b"}
	choiceQ := NewChoiceQuestion("Select one", choiceCriteria)
	if choiceQ.Type != TypeChoice || choiceQ.Instructions != "Select one" || len(choiceQ.Criteria) != 2 {
		t.Errorf("NewChoiceQuestion = %+v", choiceQ)
	}

	scoreCriteria := map[string]string{"1": "low", "5": "high"}
	scoreQ := NewScoreQuestion("Rate toxicity", scoreCriteria)
	if scoreQ.Type != TypeScore || scoreQ.Instructions != "Rate toxicity" || len(scoreQ.Criteria) != 2 {
		t.Errorf("NewScoreQuestion = %+v", scoreQ)
	}
}

func TestDecideMissingAPIKey(t *testing.T) {
	c := NewClient("")
	_, err := c.Decide(context.Background(), "hello", map[string]Question{
		"q": NewNoulQuestion("is it?"),
	})
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got: %v", err)
	}
}

func TestDecideEmptyQuestions(t *testing.T) {
	c := NewClient("valid-key")
	_, err := c.Decide(context.Background(), "hello", map[string]Question{})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty questions error, got: %v", err)
	}
}

func TestDecideSuccess(t *testing.T) {
	mockHTTP := mockDoer(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("HTTP method = %q, want POST", req.Method)
		}
		if req.Header.Get("Authorization") != "Bearer secret-api-key" {
			t.Errorf("Authorization = %q, want Bearer secret-api-key", req.Header.Get("Authorization"))
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", req.Header.Get("Content-Type"))
		}

		var payload requestPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if payload.Model != DefaultModel {
			t.Errorf("payload.Model = %q, want %q", payload.Model, DefaultModel)
		}
		if payload.State != "sample state text" {
			t.Errorf("payload.State = %q, want 'sample state text'", payload.State)
		}
		if len(payload.Questions) != 2 {
			t.Errorf("payload.Questions count = %d, want 2", len(payload.Questions))
		}

		respJSON := `{
			"model": "jev-latest",
			"answers": {
				"is_nsfw": {
					"type": "noul",
					"noul": 0.85
				},
				"category": {
					"type": "choice",
					"choice": "refund",
					"confidence": 0.92
				}
			},
			"usage": {
				"input_tokens": 42,
				"output_tokens": 0
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	c := NewClient("secret-api-key", WithHTTPClient(mockHTTP))
	resp, err := c.Decide(context.Background(), "sample state text", map[string]Question{
		"is_nsfw":  NewNoulQuestion("Is this NSFW?"),
		"category": NewChoiceQuestion("Select category", map[string]string{"refund": "Refund"}),
	})
	if err != nil {
		t.Fatalf("Decide() unexpected error: %v", err)
	}

	if resp.Model != "jev-latest" {
		t.Errorf("resp.Model = %q, want 'jev-latest'", resp.Model)
	}
	if resp.Answers["is_nsfw"].Noul != 0.85 {
		t.Errorf("is_nsfw.Noul = %f, want 0.85", resp.Answers["is_nsfw"].Noul)
	}
	if resp.Answers["category"].Choice != "refund" {
		t.Errorf("category.Choice = %q, want 'refund'", resp.Answers["category"].Choice)
	}
	if resp.Usage.InputTokens != 42 {
		t.Errorf("Usage.InputTokens = %d, want 42", resp.Usage.InputTokens)
	}
}

func TestDecideHTTPError(t *testing.T) {
	mockHTTP := mockDoer(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error": "Invalid request"}`)),
			Header:     make(http.Header),
		}, nil
	})

	c := NewClient("secret-key", WithHTTPClient(mockHTTP))
	_, err := c.Decide(context.Background(), "text", map[string]Question{
		"q": NewNoulQuestion("instructions"),
	})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected status 400 error, got: %v", err)
	}
}

func TestDecideMalformedJSON(t *testing.T) {
	mockHTTP := mockDoer(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{not valid json`)),
			Header:     make(http.Header),
		}, nil
	})

	c := NewClient("secret-key", WithHTTPClient(mockHTTP))
	_, err := c.Decide(context.Background(), "text", map[string]Question{
		"q": NewNoulQuestion("instructions"),
	})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestAskNoul(t *testing.T) {
	mockHTTP := mockDoer(func(req *http.Request) (*http.Response, error) {
		respJSON := `{
			"model": "jev-latest",
			"answers": {
				"q": {
					"type": "noul",
					"noul": 0.93
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	c := NewClient("secret-key", WithHTTPClient(mockHTTP))
	prob, err := c.AskNoul(context.Background(), "message text", "Is this message toxic?")
	if err != nil {
		t.Fatalf("AskNoul() unexpected error: %v", err)
	}
	if prob != 0.93 {
		t.Errorf("AskNoul() = %f, want 0.93", prob)
	}
}

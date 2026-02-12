package modules

import "testing"

func TestParseTranslateResponseValidPayload(t *testing.T) {
	t.Parallel()

	body := []byte(`[["hello","en","bonjour"],null]`)
	lang, translated, err := parseTranslateResponse(body)
	if err != nil {
		t.Fatalf("parseTranslateResponse() error = %v", err)
	}
	if lang != "en" {
		t.Fatalf("unexpected detected language: %q", lang)
	}
	if translated != "hello" {
		t.Fatalf("unexpected translated text: %q", translated)
	}
}

func TestParseTranslateResponseMalformedJSON(t *testing.T) {
	t.Parallel()

	if _, _, err := parseTranslateResponse([]byte(`{not-json`)); err == nil {
		t.Fatalf("expected parse error for malformed json")
	}
}

func TestParseTranslateResponseEmptyPayload(t *testing.T) {
	t.Parallel()

	if _, _, err := parseTranslateResponse([]byte(`[]`)); err == nil {
		t.Fatalf("expected parse error for empty payload")
	}
}

func TestParseTranslateResponseUnexpectedShape(t *testing.T) {
	t.Parallel()

	if _, _, err := parseTranslateResponse([]byte(`[{}]`)); err == nil {
		t.Fatalf("expected parse error for unexpected payload shape")
	}
}

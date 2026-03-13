package modules

import (
	"testing"
)

func TestFixedStringCaptchaDriverReturnsExactQuestion(t *testing.T) {
	const question = "12 + 34"

	driver := newMathImageCaptchaDriver(question)

	_, gotQuestion, gotAnswer := driver.GenerateIdQuestionAnswer()

	if gotQuestion != question {
		t.Fatalf("expected generated question %q, got %q", question, gotQuestion)
	}

	if gotAnswer != question {
		t.Fatalf("expected generated answer payload %q, got %q", question, gotAnswer)
	}
}

func TestFormatMathQuestionUsesASCIIForMultiplication(t *testing.T) {
	got := formatMathQuestion(10, 3, "*")
	want := "10 x 3"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMathCaptchaDriverDisablesLineNoise(t *testing.T) {
	const question = "10 x 3"

	driver := newMathImageCaptchaDriver(question)

	if driver.ShowLineOptions != 0 {
		t.Fatalf("expected show line options to be disabled, got %d", driver.ShowLineOptions)
	}
}

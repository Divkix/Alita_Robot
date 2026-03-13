package modules

import (
	"testing"

	"github.com/mojocn/base64Captcha"
)

func TestFixedStringCaptchaDriverReturnsExactQuestion(t *testing.T) {
	const question = "12 + 34"

	driver := &fixedStringCaptchaDriver{
		DriverString: base64Captcha.NewDriverString(
			80,
			240,
			0,
			2,
			len(question),
			question,
			nil,
			nil,
			[]string{},
		),
		content: question,
	}

	_, gotQuestion, gotAnswer := driver.GenerateIdQuestionAnswer()

	if gotQuestion != question {
		t.Fatalf("expected generated question %q, got %q", question, gotQuestion)
	}

	if gotAnswer != question {
		t.Fatalf("expected generated answer payload %q, got %q", question, gotAnswer)
	}
}

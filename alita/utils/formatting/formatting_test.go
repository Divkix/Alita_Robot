package formatting

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSendMessageOptionBuilders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		buildOpts func() string
		wantMode  string
	}{
		{
			name: "html options",
			buildOpts: func() string {
				opts := Shtml()
				if opts.LinkPreviewOptions == nil || !opts.LinkPreviewOptions.IsDisabled {
					t.Fatal("Shtml must disable link previews")
				}
				if opts.ReplyParameters == nil || !opts.ReplyParameters.AllowSendingWithoutReply {
					t.Fatal("Shtml must allow sending without reply")
				}
				return opts.ParseMode
			},
			wantMode: HTML,
		},
		{
			name: "markdown options",
			buildOpts: func() string {
				opts := Smarkdown()
				if opts.LinkPreviewOptions == nil || !opts.LinkPreviewOptions.IsDisabled {
					t.Fatal("Smarkdown must disable link previews")
				}
				if opts.ReplyParameters == nil || !opts.ReplyParameters.AllowSendingWithoutReply {
					t.Fatal("Smarkdown must allow sending without reply")
				}
				return opts.ParseMode
			},
			wantMode: Markdown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.buildOpts(); got != tc.wantMode {
				t.Fatalf("ParseMode = %q, want %q", got, tc.wantMode)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	t.Parallel()

	t.Run("short message is returned unchanged", func(t *testing.T) {
		t.Parallel()

		got := SplitMessage("hello")
		if len(got) != 1 || got[0] != "hello" {
			t.Fatalf("SplitMessage short = %#v, want [hello]", got)
		}
	})

	t.Run("splits long single line by rune limit", func(t *testing.T) {
		t.Parallel()

		msg := strings.Repeat("x", MaxMessageLength+7)
		got := SplitMessage(msg)
		if len(got) != 2 {
			t.Fatalf("chunk count = %d, want 2", len(got))
		}
		if utf8.RuneCountInString(got[0]) != MaxMessageLength {
			t.Fatalf("first chunk runes = %d, want %d", utf8.RuneCountInString(got[0]), MaxMessageLength)
		}
		if got[1] != strings.Repeat("x", 7) {
			t.Fatalf("second chunk = %q, want seven x characters", got[1])
		}
	})

	t.Run("groups newline separated lines without exceeding limit", func(t *testing.T) {
		t.Parallel()

		firstLine := strings.Repeat("a", MaxMessageLength-2)
		secondLine := "bb"
		got := SplitMessage(firstLine + "\n" + secondLine)
		if len(got) != 2 {
			t.Fatalf("chunk count = %d, want 2", len(got))
		}
		if got[0] != firstLine+"\n" {
			t.Fatalf("first chunk = %q, want first line with newline", got[0])
		}
		if got[1] != secondLine+"\n" {
			t.Fatalf("second chunk = %q, want second line with newline", got[1])
		}
	})
}

func TestHTMLHelpersEscapeUntrustedText(t *testing.T) {
	t.Parallel()

	if got := HtmlEscape(`<tag attr="value">&`); got != `&lt;tag attr=&#34;value&#34;&gt;&amp;` {
		t.Fatalf("HtmlEscape = %q", got)
	}

	gotURL := MentionUrl(`https://example.com/?q=<x>&n="1"`, `A&B <user>`)
	wantURL := `<a href="https://example.com/?q=&lt;x&gt;&amp;n=&#34;1&#34;">A&amp;B &lt;user&gt;</a>`
	if gotURL != wantURL {
		t.Fatalf("MentionUrl = %q, want %q", gotURL, wantURL)
	}

	gotMention := MentionHtml(12345, `A&B`)
	wantMention := `<a href="tg://user?id=12345">A&amp;B</a>`
	if gotMention != wantMention {
		t.Fatalf("MentionHtml = %q, want %q", gotMention, wantMention)
	}
}

func TestReverseHTML2MD(t *testing.T) {
	t.Parallel()

	input := `<b>bold</b> <i>italic</i> <u>under</u> <s>strike</s> <code>code</code> <a href="https://example.com">link</a>`
	want := `*bold* _italic_ __under__ ~strike~ ` + "`code`" + ` [link](https://example.com)`
	if got := ReverseHTML2MD(input); got != want {
		t.Fatalf("ReverseHTML2MD = %q, want %q", got, want)
	}
}

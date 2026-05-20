package formatting

import (
	"reflect"
	"strings"
	"testing"
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

	longSingleLine := strings.Repeat("x", MaxMessageLength+7)
	firstLine := strings.Repeat("a", MaxMessageLength-2)
	secondLine := "bb"

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "short message is returned unchanged",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "splits long single line by rune limit",
			input: longSingleLine,
			want:  []string{strings.Repeat("x", MaxMessageLength), strings.Repeat("x", 7)},
		},
		{
			name:  "groups newline separated lines without exceeding limit",
			input: firstLine + "\n" + secondLine,
			want:  []string{firstLine + "\n", secondLine + "\n"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := SplitMessage(tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitMessage() = %#v, want %#v", got, tc.want)
			}
		})
	}
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

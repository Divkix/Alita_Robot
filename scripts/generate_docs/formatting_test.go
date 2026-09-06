package main

import (
	"testing"
)

func TestConvertTelegramMarkdown_CrossBulletWithCommand(t *testing.T) {
	input := "× /flood: Get the current antiflood settings."
	want := "- `/flood`: Get the current antiflood settings."
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHelpText_SectionHeader(t *testing.T) {
	input := "*Admin commands*:"
	want := "### Admin commands"
	got := formatHelpText(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

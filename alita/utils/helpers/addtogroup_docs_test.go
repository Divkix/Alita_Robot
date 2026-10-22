package helpers_test

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// docsAddToGroupUsername is the public bot the docs tell admins to add.
const docsAddToGroupUsername = "Alita_Robot"

// docsAddToGroupPages hardcode the add-to-group URL because MDX cannot call Go.
// TestDocsAddToGroupLinksMatchHelper keeps those copies equal to the helper.
var docsAddToGroupPages = []string{
	"docs/src/content/docs/add-to-group.mdx",
	"docs/src/content/docs/getting-started/quick-start.mdx",
}

var addToGroupURLLink = regexp.MustCompile(`https://t\.me/[A-Za-z0-9_]+\?startgroup=botstart&admin=[^\s)"'>]+`)

func TestAddToGroupURLRequestsEveryBotAdminRight(t *testing.T) {
	t.Parallel()

	raw := helpers.AddToGroupURL("ExampleBot")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	if u.Scheme != "https" || u.Host != "t.me" || u.Path != "/ExampleBot" {
		t.Fatalf("URL %q is not a t.me link to the bot", raw)
	}
	if got := u.Query().Get("startgroup"); got != "botstart" {
		t.Fatalf("startgroup = %q, want botstart", got)
	}
	// Without admin= Telegram adds the bot as a plain member, so every
	// admin-only feature fails until someone promotes it by hand.
	want := strings.Split(helpers.BotAdminRights, "+")
	got := strings.Split(u.Query().Get("admin"), " ") // "+" decodes to a space
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("granted rights = %v, want %v", got, want)
	}
}

func TestDocsAddToGroupLinksMatchHelper(t *testing.T) {
	t.Parallel()

	want := helpers.AddToGroupURL(docsAddToGroupUsername)
	for _, rel := range docsAddToGroupPages {
		path := filepath.Join("..", "..", "..", filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		found := addToGroupURLLink.FindAllString(string(data), -1)
		if len(found) == 0 {
			t.Fatalf("%s has no add-to-group link; it must contain %q", rel, want)
		}
		for _, link := range found {
			if link != want {
				t.Errorf("%s links to %q, but helpers.AddToGroupURL builds %q; update the page", rel, link, want)
			}
		}
	}
}

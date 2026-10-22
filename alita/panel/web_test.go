package panel

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

var (
	assetReferencePattern = regexp.MustCompile(`(?:src|href)="([^"]+)"`)
	moduleImportPattern   = regexp.MustCompile(`(?:from|import)\s*\(?\s*"([^"]+)"`)
)

func isRemoteReference(reference string) bool {
	for _, prefix := range []string{"http://", "https://", "//", "#", "data:", "mailto:"} {
		if strings.HasPrefix(reference, prefix) {
			return true
		}
	}
	return false
}

func TestIndexReferencesAssetsInEmbeddedFS(t *testing.T) {
	index, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(index)
	if !strings.Contains(html, "https://telegram.org/js/telegram-web-app.js") {
		t.Fatal("index.html does not load telegram-web-app.js")
	}
	if !strings.Contains(html, `type="module"`) || !strings.Contains(html, "app.js") {
		t.Fatal("index.html does not load app.js as a module")
	}

	checked := make(map[string]bool)

	var checkFile func(name string)
	checkFile = func(name string) {
		if checked[name] {
			return
		}
		checked[name] = true

		data, err := fs.ReadFile(webFiles, name)
		if err != nil {
			t.Fatalf("%s is referenced but missing from the embedded filesystem: %v", name, err)
		}

		pattern := assetReferencePattern
		if strings.HasSuffix(name, ".js") {
			pattern = moduleImportPattern
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			reference := match[1]
			if isRemoteReference(reference) {
				continue
			}
			resolved := path.Join(path.Dir(name), reference)
			checkFile(resolved)
		}
	}

	for _, match := range assetReferencePattern.FindAllStringSubmatch(html, -1) {
		reference := match[1]
		if isRemoteReference(reference) {
			continue
		}
		checkFile(path.Join("web", reference))
	}

	if !checked["web/app.js"] || !checked["web/app.css"] {
		t.Fatalf("index.html should reference app.js and app.css; checked %v", checked)
	}
}

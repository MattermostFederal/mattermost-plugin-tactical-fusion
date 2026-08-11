package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The mark is drawn twice: once as an SVG asset for the plugin list, once as a
// React component for the channel header button. A reader sees both, and a
// colour changed in one and not the other is the kind of drift nothing else
// would report.
//
// Checked from here because reading a sibling file is natural in a Go test and
// this repo already guards the timezone tables the same way.
func TestHeaderIconMatchesThePluginIcon(t *testing.T) {
	icon := readRepoFile(t, filepath.Join("..", "assets", "icon.svg"))
	header := readRepoFile(t, filepath.Join("..", "webapp", "src", "HeaderIcon.tsx"))

	declared := regexp.MustCompile(`const PIN_COLOR = '(#[0-9A-Fa-f]{6})'`).FindStringSubmatch(header)
	if declared == nil {
		t.Fatal("HeaderIcon.tsx no longer declares PIN_COLOR; has the header icon stopped carrying the pin colour?")
	}

	if !strings.Contains(strings.ToUpper(icon), strings.ToUpper(declared[1])) {
		t.Fatalf("the header pin is %s but assets/icon.svg does not use it; the two marks have drifted apart", declared[1])
	}
}

// The plugin list renders the icon on a light card and on a dark one, so the
// mark has to bring its own background rather than borrowing the card's.
func TestPluginIconCarriesItsOwnPlate(t *testing.T) {
	icon := readRepoFile(t, filepath.Join("..", "assets", "icon.svg"))

	if !strings.Contains(icon, "<rect") {
		t.Fatal("assets/icon.svg has no plate, so it would vanish against one of the cards it is drawn on")
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	return string(raw)
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

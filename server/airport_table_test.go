package main

import (
	"net/url"
	"testing"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

func TestTheLargestAirfieldTableFitsTheFloor(t *testing.T) {
	const href = "/mattermost/some/long/subpath/plugins/com.mattermost.tactical-fusion/decorate/airport?v=KORD"

	d := &airport.Decorator{}
	largest, who := 0, ""
	for _, ident := range airport.Idents() {
		expanded := d.ExpandMessage(href, "//", url.Values{airport.ParamValue: {ident}})
		if expanded == "" {
			t.Fatalf("%s expands to nothing", ident)
		}
		if n := utf8.RuneCountInString(expanded); n > largest {
			largest, who = n, ident
		}
	}

	if largest > safePostRunes {
		t.Fatalf("%s expands to %d runes, past the %d floor, so its table would never be written", who, largest, safePostRunes)
	}
	t.Logf("the largest table is %s at %d runes", who, largest)
}

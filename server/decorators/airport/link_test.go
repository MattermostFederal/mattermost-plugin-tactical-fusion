package airport

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

// The link every airfield surface offers must be one the location page renders.
// A token that differs by a digit, or by a space, is a permanent dead end.
func TestEveryAirfieldLinksToAPageThatRenders(t *testing.T) {
	for _, ident := range []string{"KIND", "KLAX", "NZSP", "PHIK", "USCG", "YSSY", "EGLL"} {
		d, ok := Describe(ident)
		if !ok || !d.HasPosition {
			t.Fatalf("%s has no coordinate", ident)
		}

		rec := httptest.NewRecorder()
		(&location.Decorator{}).RenderPage(rec, url.Values{"f": {d.Format}, "v": {d.Token}})
		if rec.Code != 200 {
			t.Errorf("%s: the coordinate page answered %d for %s", ident, rec.Code, d.Token)
		}
	}
}

func TestEveryAirfieldTokenIsAcceptedByLocation(t *testing.T) {
	bad := 0
	for ident := range airfields {
		d, _ := Describe(ident)

		// A record with no usable position offers no link at all, which
		// api.go and the page both gate on. Converting its empty token would
		// count a row the product handles correctly as a dead link.
		if !d.HasPosition {
			continue
		}

		if _, ok := location.Convert(location.Format(d.Format), d.Token, ""); !ok {
			if bad < 3 {
				t.Errorf("%s: %q is not a coordinate", ident, d.Token)
			}
			bad++
		}
	}

	// The count as well as the samples: reporting three of eight thousand
	// understates the blast radius of what is one permanently dead link each.
	if bad > 0 {
		t.Errorf("%d of %d airfields have a dead link", bad, Count())
	}
}

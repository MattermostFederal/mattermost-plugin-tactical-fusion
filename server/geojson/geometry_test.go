package geojson

import "testing"

/*
 * A line needs two points, and says so when it has fewer.
 *
 * RFC 7946 section 3.1.4 requires two. Without the guard a one-position
 * LineString was counted as a line, drew nothing and measured nothing, with no
 * note to say why: the card read "1 line" over an empty map. That is the exact
 * outcome the empty-polygon guard beside it exists to prevent, so the two kinds
 * had come to disagree.
 */
func TestALineWithTooFewPointsIsNotedRatherThanCounted(t *testing.T) {
	for _, source := range []string{
		`{"type":"LineString","coordinates":[[1,2]]}`,
		`{"type":"LineString","coordinates":[]}`,
		`{"type":"MultiLineString","coordinates":[[[1,2],[3,4]],[[5,6]]]}`,
	} {
		document := parse(t, source)
		feature := document.Features[0]

		if feature.Note != LineShortNote {
			t.Errorf("%s\n note = %q, want the short-line note", source, feature.Note)
		}
		// Still counted as a line, and counted as undrawable beside it. That
		// is what the short-ring polygon does, and hiding the feature would
		// tell a reader the document holds less than it does.
		if counts := document.Counts(); counts.Lines != 1 || counts.Undrawable != 1 {
			t.Errorf("%s\n lines=%d undrawable=%d, want 1 and 1",
				source, counts.Lines, counts.Undrawable)
		}
	}

	// And a real one still draws.
	document := parse(t, `{"type":"LineString","coordinates":[[1,2],[3,4]]}`)
	if document.Features[0].Note != "" || document.Counts().Undrawable != 0 {
		t.Errorf("a two point line was refused: note=%q undrawable=%d",
			document.Features[0].Note, document.Counts().Undrawable)
	}
}

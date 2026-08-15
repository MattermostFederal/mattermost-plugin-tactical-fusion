package location

import "net/url"

// Conversion is the rows the panel cannot work out for itself, already
// rendered.
//
// Exactly those rows and no others. The webapp has its own renderer in
// format.ts, pinned against format.go by paired fixtures, and it uses it for
// every textual grammar; what it does not have is a projection. So the two grid
// rows are here because they need one, and the three coordinate rows are here
// because a grid token has no Coordinate for format.ts to render from and its
// resolution is linear rather than angular.
//
// The three area-reference rows are here for a different reason, and it is a
// judgment rather than a necessity: GEOREF, GARS and Plus Codes need no
// projection, so format.ts COULD render them. What it would need is an encoder
// and a decoder for each, six pieces of arithmetic held to the Go ones by
// fixtures, against a request the panel already makes unconditionally on every
// open. The seam is the expensive part here, not the round trip, and this
// phase's whole lesson from the band class is that a duplicated definition
// fails silently.
//
// What the panel still computes for an area token is its RESOLUTION, from the
// length of the code and nothing else, so that row is on screen immediately and
// stays there even if the request never lands.
//
// Everything else a panel shows, it computes: the resolution of a grid token
// from its digit count, the confidence digits from a verified USMTF token, the
// author's text from the link. Sending those too would put fields on the wire
// that nothing reads, which is how a payload starts drifting from its consumer.
type Conversion struct {
	MGRS string `json:"mgrs"`
	UTM  string `json:"utm"`

	Decimal string `json:"decimal"`
	DMS     string `json:"dms"`
	DDM     string `json:"ddm"`
	USMTF   string `json:"usmtf"`

	GEOREF   string `json:"georef"`
	GARS     string `json:"gars"`
	PlusCode string `json:"pluscode"`
}

// Convert derives every reading of a coordinate from its format and canonical
// token.
//
// The inputs go through validateParams, the same function the public page uses,
// rather than through a check written for this caller. A conversion endpoint
// that accepted a token the page rejects would be a second door with different
// locks on it, and the panel and the page would then disagree about the same
// link.
//
// raw is the author's own text, and passing it is what lets the panel find out
// something it cannot work out for itself.
//
// Two of the four checks on r need the token grammar, which lives in Go only,
// so the webapp can check its length and its alphabet and nothing more. That
// left a hand-written link able to put prose in the panel's "As written" row
// beside a position derived from an unrelated token, while the server-rendered
// page refused the identical link. Sending r here closes that: the panel is
// asking the same question the page asks, and getting the same answer.
//
// ok=false for anything that is not a coordinate this plugin would have
// written. A polar coordinate is NOT one of those: past 84 north and 80 south
// there is no UTM grid, so the two grid rows come back empty, but the position
// itself is perfectly good and the conversion succeeds.
func Convert(f Format, canonical, raw string) (Conversion, bool) {
	params := url.Values{
		paramFormat: {string(f)},
		paramValue:  {canonical},
	}
	if raw != "" {
		params.Set(paramRaw, raw)
	}

	page, ok := validateParams(params)
	if !ok {
		return Conversion{}, false
	}

	loc := page.loc

	// A position the projection cannot resolve is a refusal rather than a set
	// of empty rows. Unreachable in practice, since parseMGRS and parseUTM both
	// required this same computation to succeed before validateParams accepted
	// the token, but the alternative if it ever changes is a panel rendering
	// blanks it would read as "outside the grid".
	if _, _, ok := loc.Point(); !ok {
		return Conversion{}, false
	}

	return Conversion{
		MGRS:     loc.MGRSText(),
		UTM:      loc.UTMText(),
		Decimal:  loc.DecimalText(),
		DMS:      loc.DMSText(),
		DDM:      loc.DDMText(),
		USMTF:    loc.USMTFText(),
		GEOREF:   loc.GEOREFText(),
		GARS:     loc.GARSText(),
		PlusCode: loc.PlusCodeText(),
	}, true
}

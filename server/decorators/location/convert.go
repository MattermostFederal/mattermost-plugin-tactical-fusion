package location

import "net/url"

type Conversion struct {
	MGRS string `json:"mgrs"`
	UTM  string `json:"utm"`

	Decimal string `json:"decimal"`
	DMS     string `json:"dms"`
	DDM     string `json:"ddm"`
	USMTF   string `json:"usmtf"`
	Region  string `json:"region"`

	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Convert derives every reading of a coordinate from its format and canonical
// token.
//
// The inputs go through validateParams, the same function the page uses,
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
	lat, lon, ok := loc.Point()
	if !ok {
		return Conversion{}, false
	}

	return Conversion{
		MGRS:    loc.MGRSText(),
		UTM:     loc.UTMText(),
		Decimal: loc.DecimalText(),
		DMS:     loc.DMSText(),
		DDM:     loc.DDMText(),
		USMTF:   loc.USMTFText(),
		Region:  loc.RegionText(),
		Lat:     lat,
		Lon:     lon,
	}, true
}

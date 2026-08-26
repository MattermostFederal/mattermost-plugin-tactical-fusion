package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// detailExample is one line of the /tactical-fusion examples output: the text
// a user would type, and a short reason when the outcome is not self-evident.
type detailExample struct {
	text string
	note string

	inline bool
}

// detailGroup is one labeled block of examples that all share an outcome.
//
// The grouping is the point of this command rather than decoration on it. The
// list is long enough that an undifferentiated run of rows would be unreadable,
// and the headings are where the *rule* gets stated: a reader whose token was
// declined is looking for which rule caught it, and the group it sits under is
// that answer.
type detailGroup struct {
	heading string

	// decorates is what every row in this group must do, asserted per row and
	// in both directions. A group cannot quietly hold a row that does the
	// opposite of what its heading claims, which is the failure mode that once
	// let a whole format sit in the "recognized" list decorating nothing.
	decorates bool

	examples []detailExample

	// live generates rows from the moment the command runs, in place of a
	// fixed list. Only the date-time group decorator has anything that varies
	// with the clock.
	live func(ref time.Time) []detailExample
}

// liveDetailOffsets are measured from the moment the command runs.
//
// The pair either side of the flash threshold is deliberate: 30 minutes is
// where a countdown starts warning, and a reader comparing the two sees the
// behavior rather than reading about it. A negative one counts up instead of
// down, which is the other half of the behavior and easy to forget exists.
//
// Kept as data so the tests can check the same offsets the output uses.
var liveDetailOffsets = []struct {
	offset time.Duration
	note   string
}{
	{5 * time.Minute, "5 minutes from now, inside the flash threshold"},
	{45 * time.Minute, "45 minutes from now, outside it"},
	{48 * time.Hour, "2 days from now"},
	{-6 * time.Hour, "6 hours ago, counting up"},
}

// liveDetailExamples are built from the moment the command runs, so the panel opens
// on a countdown that is actually moving rather than a fixed date.
//
// Generated through the decorator's own formatter, so whatever appears here is
// by construction something the decorator recognizes.
func liveDetailExamples(ref time.Time) []detailExample {
	examples := make([]detailExample, 0, len(liveDetailOffsets))

	for _, live := range liveDetailOffsets {
		examples = append(examples, detailExample{
			text: dtg.FormatZulu(ref.Add(live.offset)),
			note: live.note,
		})
	}

	return examples
}

const inlineDetailHeading = "Drawn as a map in the channel"

// dtgDetailGroups is every date-time group example, in the order they are posted.
var dtgDetailGroups = []detailGroup{{
	heading:   "Live, from the moment you ran this",
	decorates: true,
	live:      liveDetailExamples,
}, {
	heading:   "Military date-time groups",
	decorates: true,
	examples: []detailExample{
		{text: "091630ZAUG2026", note: "four-digit year, normalized to two in the link"},
		{text: "091630ZAUG26", note: "two digits always mean 20NN"},
		{text: "091630Z", note: "the short form: month and year from the post date"},
		{text: "010001ZJAN00", note: "the earliest the two-digit year can name"},
		{text: "312359ZDEC99", note: "and the latest"},
		{text: "290000ZFEB24", note: "29 February in a leap year"},
	},
}, {
	heading:   "Zone letters",
	decorates: true,
	examples: []detailExample{
		{text: "091630AAUG26", note: "A is UTC+1"},
		{text: "091630MAUG26", note: "M is UTC+12, the furthest east"},
		{text: "091630YAUG26", note: "Y is UTC-12, the furthest west"},
		{text: "091630RAUG26", note: "R is UTC-5"},
		{text: "091630Zaug26", note: "the month is case-insensitive; the zone letter is not"},
	},
}, {
	heading:   "RFC 3339 timestamps",
	decorates: true,
	examples: []detailExample{
		{text: "2026-08-09T16:30:00Z"},
		{text: "2026-08-09T16:30Z", note: "seconds optional"},
		{text: "2026-08-09T16:30:00.500Z", note: "a fraction is parsed and discarded"},
		{text: "2026-08-09T20:30:00+04:00", note: "the same instant, written in +04:00"},
		{text: "2026-08-09T22:00:00+05:30", note: "a half-hour offset no zone letter can say"},
		{text: "2026-08-09T22:15:00+05:45", note: "and a quarter-hour one"},
		{text: "2026-08-09T11:30:00-05:00", note: "behind UTC"},
		{text: "2026-08-09t16:30:00z", note: "RFC 3339 allows lower case"},
		{text: "1970-01-01T00:00:00Z", note: "the earliest instant a link may carry"},
		{text: "2200-01-01T00:00:00Z", note: "and the latest"},
	},
}, {
	heading:   "The DTG: moniker",
	decorates: true,
	examples: []detailExample{
		{text: "DTG: 091630ZAUG26", note: "consumed, so only the token is linked"},
		{text: "DTG:091630ZAUG26", note: "spacing optional"},
		{text: "DTG : 091630ZAUG26", note: "either side of the colon"},
		{text: "dtg: 091630ZAUG26", note: "case-insensitive"},
		{text: "Dtg:2026-08-09T16:30:00Z", note: "either grammar"},
	},
}, {
	heading:   "In a sentence",
	decorates: true,
	examples: []detailExample{
		{text: "ARCT 091630ZAUG26 confirmed", note: "only the token is linked"},
		{text: "window opens 090600ZAUG26 and closes 091800ZAUG26", note: "two on one line"},
		{text: "[the plan](https://example.com) says 091630ZAUG26", note: "the link is untouched"},
		{text: "run `status --now` at 091630ZAUG26", note: "the code is untouched"},
		{text: "091630ZAUG26 at 21.3353, -157.9483", note: "two decorators on one line"},
	},
}, {
	heading:   "Declined: impossible dates and times",
	decorates: false,
	examples: []detailExample{
		{text: "311200ZFEB26", note: "no 31 February"},
		{text: "2026-02-30T16:30:00Z", note: "nor 30 February"},
		{text: "290000ZFEB25", note: "2025 is not a leap year"},
		{text: "001630ZAUG26", note: "day 00"},
		{text: "321630ZAUG26", note: "day 32"},
		{text: "092400ZAUG26", note: "hour 24"},
		{text: "091660ZAUG26", note: "minute 60"},
		{text: "2026-13-09T16:30:00Z", note: "month 13"},
		{text: "2026-08-09T16:30:00+24:00", note: "a 24-hour offset"},
	},
}, {
	heading:   "Declined: no single instant",
	decorates: false,
	examples: []detailExample{
		{text: "2026-08-09", note: "a date is not a time"},
		{text: "2026-08-09T16:30:00", note: "no zone, so no instant"},
		{text: "16:30Z", note: "no date"},
		{text: "4:30 PM", note: "a 12-hour clock"},
		{text: "16:30 CST", note: "CST is three different zones"},
		{text: "16:30 EST", note: "and EST collides with \"est.\""},
		{text: "091630JAUG26", note: "J is the reader's own local time, which differs per reader"},
		{text: "091630IAUG26", note: "I is skipped in the alphabet, it reads as a 1"},
		{text: "DTG: 091630R", note: "the moniker vouches for nothing"},
	},
}, {
	heading:   "Declined: outside the range a link may carry",
	decorates: false,
	examples: []detailExample{
		{text: "1918-11-11T11:00:00Z", note: "before 1970"},
		{text: "2201-01-01T00:00:00Z", note: "after 2200"},
	},
}, {
	heading:   "Declined: a false positive would rewrite the message",
	decorates: false,
	examples: []detailExample{
		{text: "1786293000", note: "epoch seconds are any ten-digit number"},
		{text: "20260809T163000Z", note: "the basic ISO form has no word boundary"},
		{text: "snapshot-20260809T163000Z.zip", note: "which is what it would match inside"},
		{text: "X091630ZAUG26", note: "inside a longer run"},
		{text: "part 091630ZAUG26X", note: "or with one after it"},
	},
}, {
	heading:   "Left exactly as written",
	decorates: false,
	examples: []detailExample{
		{text: "the format is `091630ZAUG26`", note: "inside code"},
		{text: "[091630ZAUG26](https://example.com/window)", note: "already a link"},
		{text: "see https://example.com/logs/091630Z for detail", note: "inside a URL"},
		{text: "www.example.com/091630ZAUG26", note: "a bare www URL counts too"},
		{text: "<https://example.com/091630ZAUG26>", note: "an autolink"},
		{text: "<span title=\"091630ZAUG26\">x</span>", note: "inside an HTML tag"},
		{text: "[ref]: https://example.com/091630ZAUG26", note: "a reference definition"},
	},
}}

// locationDetailGroups is every coordinate example, in the order they are posted.
var locationDetailGroups = []detailGroup{{
	heading:   "Decimal degrees",
	decorates: true,
	examples: []detailExample{
		{text: "21.3353, -157.9483", note: "signed, and four decimals on both is the minimum"},
		{text: "21.3353,-157.9483", note: "the space is optional"},
		{text: "-33.8688, 151.2093", note: "south and east"},
		{text: "21.3353 N, 157.9483 W", note: "hemisphere letters instead of signs"},
		{text: "21.3353N,157.9483W", note: "run together"},
		{text: "21.3353N 157.9W", note: "mixed precision: each axis renders at its own"},
		{text: "90.0000, 180.0000", note: "the pole and the antimeridian"},
	},
}, {
	heading:   "Degrees, minutes and seconds",
	decorates: true,
	examples: []detailExample{
		{text: "21°21'53\"N 157°57'00\"W", note: "the typewriter quote and double quote"},
		{text: "21°20′07″N 157°56′53″W", note: "and the typographic prime and double prime"},
		{text: "21 20 07 N 157 56 53 W", note: "spaces instead of symbols"},
		{text: "21°20.118'N 157°56.898'W", note: "degrees and decimal minutes"},
		{text: "21°20.118′N 157°56.898′W"},
	},
}, {
	heading:   "USMTF compact",
	decorates: true,
	examples: []detailExample{
		{text: "2120N15757W", note: "LATM, degrees and whole minutes"},
		{text: "212007N1575654W", note: "LATS, with seconds"},
		{text: "212007.1N1575653.9W", note: "LATDS, a fractional second"},
		{text: "2120N9-15757W7", note: "verified, and the confidence digits are kept"},
		{text: "LATM:2130N15730W", note: "the field label is kept, not consumed"},
		{text: "LATD:21N157W", note: "the shortest shape, which is reachable only behind a label"},
		{text: "GEOK:2120.118N15756.898W"},
		{text: "DMPID:133502.0N1445548.0E"},
		{text: "VLATM:2120N9-15757W7"},
	},
}, {
	heading:   "Grid references (MGRS)",
	decorates: true,
	examples: []detailExample{
		{text: "4Q FJ 09060 59620", note: "MGRS at 1 m, spaced into its parts"},
		{text: "4QFJ0906059620", note: "and run together, in upper case with five digits an axis"},
		{text: "55P BR 76 02", note: "a 1 km square rather than a point"},
		{text: "5Q KB 80970 82920"},
		{text: "MGRS:4QFJ0959", note: "coarser than 100 m needs its label"},
	},
}, {
	// Its own group because it is the only format that ships disabled, so on a
	// default install every row here is shown as typed with no link. Under the
	// grid heading that reads as a bug; under this one it reads as the setting
	// it is.
	heading:   "UTM (off by default: an admin turns on EnableLocationUTM)",
	decorates: true,
	examples: []detailExample{
		{text: "4Q 609060 2359620", note: "zone, band, easting, northing"},
		{text: "55P 276020E 1502660N", note: "the axis letters are optional, and may carry a meter unit"},
		{text: "11S 385000 3769000", note: "N and S are read as latitude bands, not as hemispheres"},
		{text: "UTMO:4Q609060 2359620", note: "the USMTF grid labels"},
		{text: "UTMT:4Q6090602359620", note: "run together, which is label-only for UTM"},
	},
}, {
	// The three notations that name a cell of the graticule rather than a
	// point. Grouped together because that is what they have in common and
	// because two of the three are label-only, which is the row a reader whose
	// bare code did nothing needs to find.
	heading:   "Area references (GEOREF, GARS, Plus Codes)",
	decorates: true,
	examples: []detailExample{
		{text: "GEOREF:XGKP5535", note: "longitude first: zone, band, degree, then minutes"},
		{text: "GEOREF:XGKP553350", note: "tenths of a minute"},
		{text: "GEOREF:XGKP55803503", note: "and hundredths"},
		{text: "GARS:045KG", note: "a 30 minute cell"},
		{text: "GARS:045KG1", note: "quadrant 1 of it, 15 minutes"},
		{text: "GARS:045KG14", note: "and keypad cell 4 of that, 5 minutes"},
		{text: "GARS:006AG39", note: "the same shape near the south pole"},
		{text: "73H483P2+4M", note: "a Plus Code, matched without a label"},
		{text: "73H483P2+4MG", note: "one character finer"},
		{text: "73H40000+", note: "padded, which is how the notation writes a coarse cell"},
		{text: "PLUSCODE:73h483p2+4m", note: "lower case needs the label"},
	},
}, {
	heading:   inlineDetailHeading,
	decorates: true,
	examples: []detailExample{
		{text: "21.3353, -157.9483", inline: true, note: "a message that is only a coordinate draws its map below it"},
		{text: "MGRS: 4QFJ0906059620", inline: true, note: "a field label in front of it still counts"},
		{text: "2120N15757W", inline: true, note: "any grammar, so long as it is the whole message"},
		{text: "target at 55P BR 76 02", note: "anything else in the message and it is the link alone"},
		{text: "4Q FJ 09060 59620 55P BR 76 02", note: "two coordinates, so neither one is the message"},
	},
}, {
	heading:   "In a sentence",
	decorates: true,
	examples: []detailExample{
		{text: "21.3353, -157.9483 21.3354, -157.9484", note: "two on one line, both linked"},
		{text: "4Q FJ 09060 59620 and 55P BR 76 02"},
		{text: "part 5 of 21.3353, -157.9483 and more"},
		{text: "a `code` span then 2120N15757W", note: "the span is protected, the rest is not"},
	},
}, {
	heading:   "Declined: not a coordinate",
	decorates: false,
	examples: []detailExample{
		{text: "21.34, -157.95", note: "fewer than four decimals"},
		{text: "21.3353 -157.9483", note: "no comma, so not a pair"},
		{text: "Load 12 N, 5 W", note: "newtons and watts"},
		{text: "0.1234, 0.5678, 0.9012", note: "part of a numeric list"},
		{text: "-157.9483..-157.9583", note: "a range, not a position"},
		{text: "0.0000, 0.0000", note: "Null Island: a pair of exact zeroes is an unset field"},
		{text: "0000N00000E", note: "the same pair written in USMTF"},
	},
}, {
	heading:   "Declined: out of range",
	decorates: false,
	examples: []detailExample{
		{text: "9999N99999W", note: "latitude 99"},
		{text: "LATM:9999N99999W", note: "the label vouches for nothing"},
		{text: "91.0000, 181.0000", note: "past the pole and the antimeridian"},
		{text: "4QJJ123123", note: "J is not a 100 km column letter in zone 4"},
		{text: "11N 385000 3769000", note: "that northing is 34 north, which is band S and not band N"},
		{text: "33C 291000 5628000", note: "nor is it in the band, the other way round"},
	},
}, {
	heading:   "Declined: needs a label or spacing",
	decorates: false,
	examples: []detailExample{
		{text: "21N157W", note: "seven characters for a 111 km square"},
		{text: "4QFJ0959", note: "coarser than 100 m, too short to detect unlabeled"},
		{text: "4qfj0906059620", note: "a lower-case run of that shape is a short commit hash"},
		{text: "XGKP5535", note: "GEOREF: four letters and four digits is also a part number"},
		{text: "045KG14", note: "GARS: seven alphanumerics with almost nothing to check"},
		{text: "73h483p2+4m", note: "a lower-case Plus Code is what a version string looks like"},
		{text: "9372+X2", note: "a short Plus Code needs a reference location we do not have"},
		{text: "LOC:2120N15757W", note: "LOC introduces an airfield code, so the coordinate behind it is not read as one"},
		{text: "21.3353, -157.9483.", note: "ending a sentence: at the guard this and a range look alike"},
	},
}, {
	heading:   "Left exactly as written",
	decorates: false,
	examples: []detailExample{
		{text: "the grid is `2120N15757W`", note: "inside code"},
		{text: "see https://example.com/g/2120N15757W", note: "inside a URL"},
		{text: "![map 2120N15757W](https://example.com/m.png)", note: "inside an image"},
		{text: "[ref]: https://example.com/2120N15757W", note: "a reference definition"},
	},
}}

// airportDetailGroups are the airfield examples.
//
// Label-only, and the declined group is where that rule gets stated, because
// for this grammar the rule IS the content: four letters is what ordinary prose
// is made of, and 343 of the idents this plugin holds are English dictionary
// words.
var airportDetailGroups = []detailGroup{{
	heading:   "Recognized behind a USMTF field label",
	decorates: true,
	examples: []detailExample{
		{text: "ICAO:PHNL", note: "Daniel K. Inouye International, Honolulu"},
		{text: "LOC:PGUM", note: "Antonio B. Won Pat International, Guam"},
		{text: "DEPLOC:PHTO", note: "the departure airfield of a USMTF air movement"},
		{text: "ARRLOC:PHIK", note: "and the arrival airfield"},
		{text: "DEPLOC:PGUA//", note: "a set line ends //, which is kept out of the link"},
		{text: "ICAO :NZSP", note: "a space before the colon is optional"},
	},
}, {
	heading:   "Declined: the label is required, in upper case",
	decorates: false,
	examples: []detailExample{
		{text: "KIND", note: "a bare code is indistinguishable from a word, and KIND is one"},
		{text: "USCG", note: "and this one is Chelyabinsk Shagol, in Russia"},
		{text: "FACT", note: "Cape Town International, which is why nothing bare is read"},
		{text: "icao:kind", note: "a lower-case label would reach words in ordinary prose"},
		{text: "ICAO:QZQZ", note: "four letters this plugin's airfield database does not hold"},
		{text: "ICAO:KIN", note: "an ICAO ident is four letters"},
		{text: "logs/ICAO:KIND", note: "a label inside a path is not a field label"},
		{text: "LOC: FAST", note: "nothing after the colon, or ordinary capitalized prose would be rewritten"},
	},
}}

// detailSet is one decorator's worth of examples.
type detailSet struct {
	name   string
	groups []detailGroup
}

// detailSets is one entry per decorator.
var detailSets = map[string]detailSet{
	dtg.Type:      {name: "date-time groups", groups: dtgDetailGroups},
	location.Type: {name: "coordinates", groups: locationDetailGroups},
	airport.Type:  {name: "airfields", groups: airportDetailGroups},
}

// detailSetOrder is the order the messages arrive in, since a map has none.
var detailSetOrder = []string{dtg.Type, location.Type, airport.Type}

// setDetailExamples flattens the groups whose outcome matches, which is how the
// tests ask for "everything that must decorate" without restating the lists.
func setDetailExamples(set detailSet, decorates bool, ref time.Time) []detailExample {
	var out []detailExample

	for _, group := range set.groups {
		if group.decorates != decorates {
			continue
		}
		out = append(out, groupDetailExamples(group, ref)...)
	}

	return out
}

// groupDetailExamples is a group's rows, generated for a live group and fixed
// otherwise.
func groupDetailExamples(group detailGroup, ref time.Time) []detailExample {
	if group.live != nil {
		return group.live(ref)
	}
	return group.examples
}

// exampleDetailsResponse posts the full example set to the channel, one
// top-level post per decorator.
//
// It cannot be one post: the two sets together run to roughly 17,500 runes and
// a Mattermost post caps at 16,383. Cutting the content is the option this
// command exists to avoid, since being exhaustive is the whole difference
// between it and `examples`.
//
// The decorator is the unit a reader thinks in, so it is the unit a post is.
// Each one is a reference somebody comes back to, which is why they are
// separate posts rather than a thread: a reply is filed under the post above it
// and read as a remark about it, and the coordinate examples are not a remark
// about the date-time examples.
//
// Within a decorator the split is driven by SIZE and nothing else, so a set too
// large for one post continues into another rather than being truncated. The
// atomic unit is a line, so the packing can always make progress however long
// the links get, and a set that split says so in its heading.
//
// The examples are decorated here rather than by the message hook, because the
// output is itself full of code and links, which is exactly what the tagger
// leaves alone. Running the tagger directly is also what keeps the declined
// rows honest: they are its real output, not hand-written.
func (p *Plugin) exampleDetailsResponse(args *model.CommandArgs) *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse(errcode.WithCode(errcode.CommandDetailsNotReady,
			"Decorators are not registered yet. Try again once the plugin has finished activating."))
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	ref := time.Now().UTC()

	return p.postDetails(args, func(budget int) []string {
		return p.packDetails(tagger, ref, budget)
	})
}

// postDetails writes the set to the channel, one top-level post per decorator.
//
// Separate posts rather than a thread, because each one is a reference somebody
// comes back to. A reply is filed under the post above it and is read as a
// remark about it; the coordinate examples are not a remark about the date-time
// examples, and burying them one level down makes them harder to find later for
// no gain.
//
// It tries the usual limit first and falls back to the floor, rather than
// trusting either. That is not defensive programming for its own sake: the
// limit cannot be read from the API, so it is a guess, and this command is the
// one place in the plugin that writes enough text to find out the guess was
// wrong. It already has once, as "Post Message property is longer than the
// maximum permitted length (TF-16006)".
//
// The FIRST post is the canary. Every message in a packing shares one budget,
// so if the first fits the rest do; and until it lands nothing has been written,
// so repacking smaller and starting again costs a reader nothing. Once anything
// has landed there is no going back, which is why the retry is here and not
// around the ones that follow.
func (p *Plugin) postDetails(args *model.CommandArgs, pack func(budget int) []string) *model.CommandResponse {
	budgets := []int{defaultPostRunes - headingBudget, safePostRunes - headingBudget}

	for i, budget := range budgets {
		messages := pack(budget)

		// Unreachable today: every set has rows and packChunks always flushes
		// at least one message. Guarded because pack is a function value the
		// compiler cannot see through, and a panic here is a 500 on somebody's
		// slash command rather than a failed post.
		if len(messages) == 0 {
			return ephemeralResponse(errcode.WithCode(errcode.CommandDetailsPostFailed,
				"There are no examples to post."))
		}

		_, appErr := p.API.CreatePost(detailPost(args, messages[0]))
		if appErr == nil {
			return p.postRemainingDetails(args, messages)
		}

		// Nothing is in the channel yet. A smaller packing is worth trying
		// before giving up, and the reason is logged either way so an operator
		// can see which budget the server actually took.
		p.API.LogWarn("tactical-fusion: could not post the example details",
			"error_code", errcode.CommandDetailsPostFailed,
			"budget", budget, "error", appErr.Error())

		if i == len(budgets)-1 {
			return ephemeralResponse(errcode.WithCode(errcode.CommandDetailsPostFailed,
				"Could not post the examples to this channel: "+appErr.Message))
		}
	}

	// Unreachable: the loop either posts or returns on its last budget.
	return &model.CommandResponse{}
}

// postRemainingDetails writes everything after the one that already landed.
//
// A post that fails costs its own examples and nothing else, so the rest are
// still attempted rather than abandoned: a set missing its fourth message is
// worth more than one missing everything after it.
func (p *Plugin) postRemainingDetails(args *model.CommandArgs, messages []string) *model.CommandResponse {
	failed := 0

	for _, message := range messages[1:] {
		if _, appErr := p.API.CreatePost(detailPost(args, message)); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post part of the example details",
				"error_code", errcode.CommandDetailsPostFailed, "error", appErr.Error())
		}
	}

	// Last, and on its own, because a card owns the whole post body: an event
	// sharing a post with anything else would render that as plain text.
	//
	// Counted like the rest. It is the likeliest of the set to be refused, since
	// it is the only one carrying a post type and a props map, and reporting
	// success having posted four of five is the one answer that helps nobody.
	total := len(messages)
	if p.cotEnabled() {
		total++
	}
	if !p.postCotExample(args) {
		failed++
	}

	if failed > 0 {
		return ephemeralResponse(errcode.WithCode(errcode.CommandDetailsPostFailed,
			fmt.Sprintf("%d of %d messages could not be posted, so the examples are incomplete.",
				failed, total)))
	}

	// Nothing to add: the posts are the output, and an ephemeral "posted" would
	// only be a second thing to read.
	return &model.CommandResponse{}
}

// detailPost is one message as a top-level post.
//
// No RootId, deliberately and in one place, so that neither caller can make one
// of these a reply by accident.
func detailPost(args *model.CommandArgs, message string) *model.Post {
	return &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		Message:   message,
	}
}

// headingBudget is what a message's heading is allowed to cost.
//
// Reserved from the limit while packing rather than measured, because the
// heading says "(2 of 3)" and nothing knows the 3 until that set is packed.
// "#### Date-time groups (10 of 10)" is 32 runes; this is ample.
const headingBudget = 64

// detailChunk is a heading and the rows under it, which the packer keeps
// together where it can.
//
// The heading is held as plain text and marked up on the way out, because the
// packer has to be able to build a "(continued)" version of it and because
// whether it needs a blank line above depends on where in a message it lands.
type detailChunk struct {
	heading string
	lines   []string
}

// headingMarkup renders a group heading.
//
// The blank line below it is not decoration: without it the rows underneath are
// a list immediately following a paragraph, and the leading one is what keeps
// the heading from being absorbed into the list item above.
func headingMarkup(text string, separate bool) string {
	markup := "**" + text + "**\n\n"
	if separate {
		return "\n" + markup
	}
	return markup
}

// continued is a heading repeated because the thing it names did not fit in the
// message before.
func continued(text string) string { return text + " (continued)" }

// packDetails renders every decorator, one message per decorator.
//
// A decorator is the unit a reader thinks in, so it is the unit a post is: one
// for date-time groups, one for coordinates. Both fit comfortably at the
// ordinary post limit, and a set that does not fit continues into another
// message rather than being packed in beside its neighbor. Sharing a post
// between two decorators saves a message and costs the thing that made the
// output readable.
//
// The budget is a parameter rather than a constant, so the splitting can be
// exercised and so postDetails can retry smaller when a server turns out
// to accept less than this build assumed.
func (p *Plugin) packDetails(tagger *decorators.Tagger, ref time.Time, budget int) []string {
	var all []string

	for _, set := range p.detailMessageSets(tagger, ref) {
		messages := packChunks(set.chunks, budget)

		// Numbered only when the set needed more than one message, since the
		// heading alone already says which set this is.
		for i := range messages {
			heading := capitalize(set.name)
			if len(messages) > 1 {
				heading = fmt.Sprintf("%s (%d of %d)", heading, i+1, len(messages))
			}
			messages[i] = "#### " + heading + "\n\n" + messages[i]
		}

		all = append(all, messages...)
	}

	return all
}

// detailMessageSet is one post's worth of examples and the name above them.
type detailMessageSet struct {
	name   string
	chunks []detailChunk
}

// detailMessageSets is every set this command posts, in order.
//
// The catalog, so a test asking "which post does this heading belong to" reads
// it rather than listing the decorators and silently ignoring anything that is
// not one. Cursor on Target is the reason that distinction exists: it earns a
// post here without being a decorator, since it stamps a post type rather than
// rewriting a token and has nothing to run through the tagger.
func (p *Plugin) detailMessageSets(tagger *decorators.Tagger, ref time.Time) []detailMessageSet {
	sets := make([]detailMessageSet, 0, len(detailSetOrder)+1)

	for _, typ := range detailSetOrder {
		set := detailSets[typ]
		sets = append(sets, detailMessageSet{name: set.name, chunks: p.detailChunks(set, tagger, ref)})
	}

	return append(sets, detailMessageSet{
		name:   cotDetailSetName,
		chunks: cotDetailChunks(p.cotEnabled(), p.cotFilesEnabled()),
	}, detailMessageSet{
		name:   cotExtensionSetName,
		chunks: cotExtensionChunks(),
	})
}

// packChunks fills messages up to budget runes with one decorator's chunks.
func packChunks(chunks []detailChunk, budget int) []string {
	var messages []string

	var cur strings.Builder
	curRunes := 0

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		messages = append(messages, cur.String())
		cur.Reset()
		curRunes = 0
	}

	write := func(text string) {
		cur.WriteString(text)
		curRunes += utf8.RuneCountInString(text)
	}

	fits := func(text string) bool {
		return curRunes+utf8.RuneCountInString(text) <= budget
	}

	// A chunk is kept whole when it fits, and split with its heading repeated
	// when it does not. A continuation that began with a bare list of rows
	// would have lost the one thing that says what they are, which for the
	// declined groups is the entire content of the row.
	for _, chunk := range chunks {
		text := chunk.heading

		for i := 0; i < len(chunk.lines); {
			heading := headingMarkup(text, cur.Len() > 0)

			if cur.Len() > 0 && !fits(heading+chunk.lines[i]) {
				flush()
				heading = headingMarkup(text, false)
			}
			write(heading)

			// At least one row always goes in, so packing cannot stall however
			// long a single link gets.
			write(chunk.lines[i])
			i++

			for i < len(chunk.lines) && fits(chunk.lines[i]) {
				write(chunk.lines[i])
				i++
			}

			if i < len(chunk.lines) {
				flush()
				text = continued(chunk.heading)
			}
		}
	}
	flush()

	return messages
}

// detailChunks is one decorator's output, as the units the packer works in.
func (p *Plugin) detailChunks(set detailSet, tagger *decorators.Tagger, ref time.Time) []detailChunk {
	chunks := make([]detailChunk, 0, len(set.groups)+1)

	for _, group := range set.groups {
		chunks = append(chunks, detailChunk{
			heading: group.heading,
			lines:   detailLines(tagger, ref, groupDetailExamples(group, ref)),
		})
	}

	// The page a link normally opens only on clients that do not run the webapp,
	// such as the mobile app. Checking it from a desktop browser otherwise means
	// faking a phone, so the framework reserves a parameter that tells the
	// webapp to stand aside.
	if link, ok := standalonePageLink(p.decorators, tagger, ref, setDetailExamples(set, true, ref)); ok {
		chunks = append(chunks, detailChunk{
			heading: "Standalone page",
			lines: []string{
				"- " + link + " (what the mobile app gets, via `" +
					decorators.ForcePageParam + "`)\n",
			},
		})
	}

	// Asked of the registry rather than of one decorator's switches. Keyed on
	// dtgFormats alone, this claimed decoration was off whenever date-time
	// groups were, even with another decorator still running.
	if !p.decorationEnabled() {
		chunks = append(chunks, detailChunk{
			heading: "Decoration is off",
			lines: []string{"- Decoration is currently **off**, so new messages are not " +
				"decorated and every row above is shown as typed.\n"},
		})
	}

	return chunks
}

// capitalize upper-cases a set name for its heading.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// detailLines renders one line per example: the typed text, and the stored
// result when decoration changed it.
func detailLines(tagger *decorators.Tagger, ref time.Time, examples []detailExample) []string {
	lines := make([]string, 0, len(examples))

	for _, example := range examples {
		var b strings.Builder
		b.WriteString("- " + inlineCode(example.text))

		if decorated := tagger.Decorate(example.text, ref); decorated != example.text {
			b.WriteString(" → " + decorated)
		}

		if example.note != "" {
			b.WriteString(" (" + example.note + ")")
		}

		b.WriteString("\n")
		lines = append(lines, b.String())
	}

	return lines
}

// inlineCode wraps text in the shortest backtick delimiter that can hold it.
//
// Several examples contain backticks of their own, and a single-backtick
// delimiter would end early and break the rest of the line.
func inlineCode(text string) string {
	longest, run := 0, 0
	for i := range len(text) {
		if text[i] != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}

	delimiter := strings.Repeat("`", longest+1)

	// A leading or trailing backtick would otherwise merge into the delimiter.
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		return delimiter + " " + text + " " + delimiter
	}
	return delimiter + text + delimiter
}

// standalonePageLink builds a demonstration link from whichever decorator can
// parse one of the recognized examples, so this stays correct if the first
// decorator in the registry changes.
//
// The page it points at is normally only reachable from a client that does not
// run the webapp, such as the mobile app. Checking it from a desktop browser
// otherwise means faking a phone, so the framework reserves a parameter that
// tells the webapp to stand aside.
func standalonePageLink(registry *decorators.Registry, tagger *decorators.Tagger, ref time.Time, from []detailExample) (string, bool) {
	for _, example := range from {
		for _, decorator := range registry.All() {
			for _, pattern := range decorator.Patterns() {
				// Guarded the way the tagger guards it. A decorator offering a
				// pattern with no expression should be skipped, not panic the
				// slash command.
				if pattern.Regexp == nil {
					continue
				}

				match := pattern.Regexp.FindStringSubmatch(example.text)
				if match == nil {
					continue
				}

				params, ok := decorator.Parse(pattern.Value(match), ref)
				if !ok {
					continue
				}

				params.Set(decorators.ForcePageParam, "1")
				return "[" + match[0] + " as a page](" + tagger.URLFor(decorator.Type(), params) + ")", true
			}
		}
	}

	return "", false
}

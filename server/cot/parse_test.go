package cot

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// parseOne is Parse for a source that carries exactly one event, which is what
// almost every fixture here is. It fails the test rather than the assertion if
// a fixture grows a second one by accident.
func parseOne(source []byte) (Event, error) {
	events, err := Parse(source)
	if err != nil {
		return Event{}, err
	}
	if len(events) != 1 {
		return Event{}, fmt.Errorf("fixture carries %d events, want 1", len(events))
	}
	return events[0], nil
}

const atakPLI = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<event version="2.0" uid="ANDROID-352214111" type="a-f-G-U-C" how="m-g"
       time="2026-08-23T11:43:38.070Z" start="2026-08-23T11:43:38.070Z"
       stale="2026-08-23T11:45:38.070Z">
  <point lat="30.009027" lon="-85.957874" hae="-42.6" ce="45.3" le="99.5"/>
  <detail>
    <contact callsign="DELTA1" endpoint="*:-1:stcp"/>
    <__group name="Cyan" role="Team Member"/>
    <status battery="100"/>
    <track speed="3.2" course="180.0"/>
    <remarks>holding at checkpoint</remarks>
  </detail>
</event>`

func TestParseReadsAnATAKPositionReport(t *testing.T) {
	event, err := parseOne([]byte(atakPLI))
	if err != nil {
		t.Fatalf("Parse refused a real ATAK PLI: %v", err)
	}

	checks := map[string]struct{ got, want string }{
		"uid":      {event.UID, "ANDROID-352214111"},
		"type":     {event.Type, "a-f-G-U-C"},
		"how":      {event.How, "m-g"},
		"time":     {event.Time, "2026-08-23T11:43:38.070Z"},
		"stale":    {event.Stale, "2026-08-23T11:45:38.070Z"},
		"lat":      {event.Point.Lat, "30.009027"},
		"lon":      {event.Point.Lon, "-85.957874"},
		"ce":       {event.Point.CE, "45.3"},
		"callsign": {event.Detail.Callsign, "DELTA1"},
		"group":    {event.Detail.Group, "Cyan"},
		"role":     {event.Detail.Role, "Team Member"},
		"speed":    {event.Detail.Speed, "3.2"},
		"remarks":  {event.Detail.Remarks, "holding at checkpoint"},
	}

	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestParseAcceptsTheEventKindsATAKEmits(t *testing.T) {
	cases := map[string]string{
		"geochat":   `<event uid="GeoChat.x" type="b-t-f" time="2026-08-23T11:43:38Z"><point lat="1.0" lon="2.0"/><detail><remarks>hello</remarks></detail></event>`,
		"spi":       `<event uid="spi1" type="b-m-p-s-p-i" time="2026-08-23T11:43:38Z"><point lat="1.0" lon="2.0" ce="9999999.0"/></event>`,
		"emergency": `<event uid="e1" type="b-a-o-tbl" time="2026-08-23T11:43:38Z"><point lat="1.0" lon="2.0"/></event>`,
		"casevac":   `<event uid="c1" type="b-r-f-h-c" time="2026-08-23T11:43:38Z"><point lat="1.0" lon="2.0"/></event>`,
		"no detail": `<event uid="u" type="a-h-A" time="2026-08-23T11:43:38Z"><point lat="1.0" lon="2.0"/></event>`,
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(source)); err != nil {
				t.Errorf("Parse refused a %s event: %v", name, err)
			}
		})
	}
}

func TestParseRefusesHostileInput(t *testing.T) {
	cases := map[string]struct {
		source string
		want   error
	}{
		"doctype": {
			`<?xml version="1.0"?><!DOCTYPE event [<!ENTITY x "y">]><event uid="u" type="a-f" time="t"/>`,
			ErrDirective,
		},
		"doctype behind a comment": {
			"<!-- innocent --><!DOCTYPE event []><event uid=\"u\" type=\"a-f\" time=\"t\"/>",
			ErrDirective,
		},
		"wrong root": {
			`<events><event uid="u" type="a-f" time="t"/></events>`,
			ErrNotEvent,
		},
		"empty event": {
			`<event/>`,
			ErrIncomplete,
		},
		"missing time": {
			`<event uid="u" type="a-f"/>`,
			ErrIncomplete,
		},
		"invalid utf8": {
			"<event uid=\"\xff\xfe\" type=\"a-f\" time=\"t\"/>",
			ErrNotUTF8,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.source))
			if !errors.Is(err, tc.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tc.source, err, tc.want)
			}
		})
	}
}

func TestParseRefusesAnUndefinedEntity(t *testing.T) {
	_, err := Parse([]byte(`<event uid="u" type="a-f" time="t"><detail><remarks>&boom;</remarks></detail></event>`))
	if err == nil {
		t.Fatal("Parse accepted an undefined entity")
	}
}

func TestParseRefusesANonUTF8EncodingDeclaration(t *testing.T) {
	_, err := Parse([]byte(`<?xml version="1.0" encoding="UTF-16"?><event uid="u" type="a-f" time="t"/>`))
	if err == nil {
		t.Fatal("Parse accepted a UTF-16 encoding declaration with no CharsetReader")
	}
}

func TestParseRefusesOversizedInput(t *testing.T) {
	source := make([]byte, maxCotBytes+1)
	for i := range source {
		source[i] = ' '
	}

	if _, err := Parse(source); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Parse error = %v, want %v", err, ErrTooLarge)
	}
}

func TestParseRefusesDeepNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<event uid="u" type="a-f" time="t"><detail>`)
	for range maxCotDepth + 5 {
		b.WriteString("<a>")
	}
	for range maxCotDepth + 5 {
		b.WriteString("</a>")
	}
	b.WriteString(`</detail></event>`)

	if _, err := Parse([]byte(b.String())); !errors.Is(err, ErrTooDeep) {
		t.Errorf("Parse error = %v, want %v", err, ErrTooDeep)
	}
}

func TestParseRefusesTooManyElements(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<event uid="u" type="a-f" time="t"><detail>`)
	for range maxCotElements + 10 {
		b.WriteString("<a/>")
	}
	b.WriteString(`</detail></event>`)

	if _, err := Parse([]byte(b.String())); !errors.Is(err, ErrTooMany) {
		t.Errorf("Parse error = %v, want %v", err, ErrTooMany)
	}
}

func TestParseKeepsDepthAccountingAcrossRemarks(t *testing.T) {
	source := `<event uid="u" type="a-f-G" time="t">
	  <point lat="1.0" lon="2.0"/>
	  <detail>
	    <remarks>one</remarks>
	    <contact callsign="AFTER"/>
	  </detail>
	</event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if event.Detail.Remarks != "one" {
		t.Errorf("remarks = %q", event.Detail.Remarks)
	}
	if event.Detail.Callsign != "AFTER" {
		t.Errorf("callsign after remarks = %q, want AFTER; depth accounting drifted", event.Detail.Callsign)
	}
}

func TestParseIgnoresUnknownDetailChildren(t *testing.T) {
	source := `<event uid="u" type="a-f-G" time="t"><point lat="1.0" lon="2.0"/>` +
		`<detail><takv platform="ATAK-CIV"/><precisionlocation geopointsrc="GPS"/>` +
		`<_flow-tags_ x="1"/><contact callsign="OK"/></detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("Parse refused ordinary open-ended detail: %v", err)
	}
	if event.Detail.Callsign != "OK" {
		t.Errorf("callsign = %q", event.Detail.Callsign)
	}
}

// The budgets have to reach inside remarks too. They did not: nesting and
// element counts were enforced only by the main decoder loop, so a payload
// buried in a remarks element escaped both entirely.
func TestParseBudgetsReachInsideRemarks(t *testing.T) {
	cases := map[string]struct {
		payload string
		want    error
	}{
		"deep nesting": {
			strings.Repeat("<a>", maxCotDepth+10) + strings.Repeat("</a>", maxCotDepth+10),
			ErrTooDeep,
		},
		"element flood": {
			strings.Repeat("<a/>", maxCotElements+10),
			ErrTooMany,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			source := `<event uid="u" type="a-f" time="t"><detail><remarks>` +
				tc.payload + `</remarks></detail></event>`

			if _, err := Parse([]byte(source)); !errors.Is(err, tc.want) {
				t.Errorf("Parse error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseRefusesContentAfterTheEvent(t *testing.T) {
	source := `<event uid="u" type="a-f-G" time="t"><point lat="1.0" lon="2.0"/></event>trailing`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrTrailing) {
		t.Errorf("Parse error = %v, want %v", err, ErrTrailing)
	}
}

func TestParseAllowsWhitespaceAroundTheEvent(t *testing.T) {
	source := "\n  <event uid=\"u\" type=\"a-f-G\" time=\"t\"><point lat=\"1.0\" lon=\"2.0\"/></event>\n\n"

	if _, err := Parse([]byte(source)); err != nil {
		t.Errorf("Parse refused ordinary surrounding whitespace: %v", err)
	}
}

func TestParseAllowsAnOrdinaryCommentInsideRemarks(t *testing.T) {
	source := `<event uid="u" type="a-f" time="t"><detail><remarks><!-- ok --></remarks></detail></event>`

	if _, err := Parse([]byte(source)); err != nil {
		t.Errorf("Parse refused an ordinary comment inside remarks: %v", err)
	}
}

// The refusals the top of the document gets have to hold inside remarks too, or
// the one element that reads arbitrary author text is the way past them.
func TestParseRefusesAProcessingInstructionInsideRemarks(t *testing.T) {
	source := `<event uid="u" type="a-f" time="t"><detail><remarks>ok<?xml-stylesheet href="x"?></remarks></detail></event>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrProcInst) {
		t.Errorf("Parse error = %v, want %v", err, ErrProcInst)
	}
}

func TestParseRefusesAProcessingInstructionAtTheTop(t *testing.T) {
	source := `<?xml-stylesheet href="x"?><event uid="u" type="a-f" time="t"/>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrProcInst) {
		t.Errorf("Parse error = %v, want %v", err, ErrProcInst)
	}
}

func TestParseRefusesContentBeforeTheEvent(t *testing.T) {
	source := `Not a real event, ignore me: <event uid="u" type="a-f-G" time="t"><point lat="1.0" lon="2.0"/></event>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrTrailing) {
		t.Errorf("Parse error = %v, want %v; prose before the root is as much a lie as prose after it", err, ErrTrailing)
	}
}

// A budget spent inside remarks must still be spent when the reader comes back
// out, or a document could nest to the limit repeatedly.
func TestParseKeepsCountingAfterRemarks(t *testing.T) {
	source := `<event uid="u" type="a-f" time="t"><detail>` +
		`<remarks>` + strings.Repeat("<a/>", maxCotElements/2) + `</remarks>` +
		strings.Repeat("<b/>", maxCotElements) +
		`</detail></event>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrTooMany) {
		t.Errorf("Parse error = %v, want %v; the remarks budget was refunded", err, ErrTooMany)
	}
}

// A source may carry several events: a batch of position reports, or a set of
// markers pasted together. They come back in the order they were written.
func TestParseReadsEveryEventInOrder(t *testing.T) {
	source := `<event uid="a" type="a-f-G" time="t"><point lat="1.0" lon="2.0"/></event>
	           <event uid="b" type="a-h-A" time="t"><point lat="3.0" lon="4.0"/></event>
	           <event uid="c" type="a-n-S" time="t"><point lat="5.0" lon="6.0"/></event>`

	events, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse refused a multi-event source: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("read %d events, want 3", len(events))
	}

	for i, want := range []string{"a", "b", "c"} {
		if events[i].UID != want {
			t.Errorf("event %d is %q, want %q", i, events[i].UID, want)
		}
	}
	if events[1].Point.Lat != "3.0" {
		t.Errorf("the second event took another's position: %q", events[1].Point.Lat)
	}
}

// One malformed event spoils the block. A source whose third event is missing a
// uid is a source somebody will read as three good ones.
func TestOneIncompleteEventRefusesTheWholeSource(t *testing.T) {
	source := `<event uid="a" type="a-f-G" time="t"/><event type="a-f-G" time="t"/>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrIncomplete) {
		t.Errorf("Parse error = %v, want %v", err, ErrIncomplete)
	}
}

func TestParseRefusesMoreEventsThanItReads(t *testing.T) {
	var b strings.Builder
	for range maxCotEvents + 1 {
		b.WriteString(`<event uid="u" type="a-f-G" time="t"/>`)
	}

	if _, err := Parse([]byte(b.String())); !errors.Is(err, ErrManyEvents) {
		t.Errorf("Parse error = %v, want %v", err, ErrManyEvents)
	}
}

func TestExactlyTheEventLimitIsRead(t *testing.T) {
	var b strings.Builder
	for range maxCotEvents {
		b.WriteString(`<event uid="u" type="a-f-G" time="t"/>`)
	}

	events, err := Parse([]byte(b.String()))
	if err != nil {
		t.Fatalf("Parse refused exactly the limit: %v", err)
	}
	if len(events) != maxCotEvents {
		t.Errorf("read %d events, want %d", len(events), maxCotEvents)
	}
}

// ATAK writes a p-p relation on almost every event naming the device that
// produced it, usually with the sending unit's callsign on it.
func TestParseReadsLinkRelations(t *testing.T) {
	source := `<event uid="u" type="a-f-G" time="t"><point lat="1.0" lon="2.0"/><detail>` +
		`<link uid="ANDROID-9" type="a-f-G-U-C" relation="p-p" parent_callsign="ALPHA"/>` +
		`<link uid="OTHER-1" relation="c"/>` +
		`</detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(event.Detail.Links) != 2 {
		t.Fatalf("read %d links, want 2", len(event.Detail.Links))
	}
	if event.Detail.Links[0].ParentCallsign != "ALPHA" {
		t.Errorf("parent callsign = %q", event.Detail.Links[0].ParentCallsign)
	}
	if event.Detail.Links[0].Relation != "p-p" || event.Detail.Links[1].UID != "OTHER-1" {
		t.Errorf("links read wrongly: %+v", event.Detail.Links)
	}
}

func TestLinksAreCapped(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<event uid="u" type="a-f-G" time="t"><detail>`)
	for range maxCotLinks + 10 {
		b.WriteString(`<link uid="x" relation="c"/>`)
	}
	b.WriteString(`</detail></event>`)

	event, err := parseOne([]byte(b.String()))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(event.Detail.Links) != maxCotLinks {
		t.Errorf("kept %d links, want the cap of %d", len(event.Detail.Links), maxCotLinks)
	}
}

// A source carrying no event at all is refused by the count, not by the name.
//
// Every other ErrNotEvent case reaches the depth-1 name check, so the final
// "did anything arrive" guard had no test: a source whose only tokens are a
// prolog, a comment or whitespace never opens a root element for that check to
// reject, and would otherwise have returned an empty slice as a success.
func TestParseRefusesASourceCarryingNoEvent(t *testing.T) {
	for name, source := range map[string]string{
		"empty":        "",
		"whitespace":   "   \n\t ",
		"comment only": "<!-- nothing here -->",
		"prolog only":  `<?xml version="1.0"?>`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(source)); !errors.Is(err, ErrNotEvent) {
				t.Errorf("Parse error = %v, want %v", err, ErrNotEvent)
			}
		})
	}
}

// The twin of TestParseRefusesAProcessingInstructionInsideRemarks.
//
// Remarks is the one place the parser reads character data in a loop of its
// own, so every refusal the top of the document gets has to hold inside it too.
// The processing-instruction half was covered and the directive half was not,
// which is the half that carries DOCTYPE and therefore entity expansion.
func TestParseRefusesADirectiveInsideRemarks(t *testing.T) {
	source := `<event uid="u" type="a-f" time="t"><detail><remarks>ok<!DOCTYPE x []></remarks></detail></event>`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrDirective) {
		t.Errorf("Parse error = %v, want %v", err, ErrDirective)
	}
}

package cot

import (
	"slices"
	"testing"
)

func classOf(t *testing.T, cotType, detail string) string {
	t.Helper()

	source := `<event version="2.0" uid="U1" type="` + cotType + `" how="m-g" ` +
		`time="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">` +
		`<point lat="34.056100" lon="-118.250000" hae="1" ce="2" le="3"/>` +
		`<detail>` + detail + `</detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	return classify(event)
}

func TestTheTypeCodeDecidesTheClass(t *testing.T) {
	cases := map[string]string{
		"b-t-f":      ClassChat,
		"b-t-f-r":    ClassChat,
		"b-r-f-h-c":  ClassMedevac,
		"b-l-p-c":    ClassSensor,
		"a-f-G-U-C":  "",
		"b-m-p-w":    "",
		"t-x-takp-v": "",
		"b-t-fake":   "",
		"b-t":        "",
		"not-a-type": "",
	}

	for cotType, want := range cases {
		if got := classOf(t, cotType, ""); got != want {
			t.Errorf("%q classified as %q, want %q", cotType, got, want)
		}
	}
}

// A single ordered table of "type matches OR block present" made a hostile
// contact carrying an empty <__chat/> render as a chat message. Ten bytes,
// chosen by the author, to re-shape somebody else's contact report.
func TestAClassNeverOverridesAnAtomType(t *testing.T) {
	spoofs := []string{
		`<__chat senderCallsign="ADMIN" chatroom="Operations"/>`,
		`<_medevac_ urgent="9"/>`,
		`<sensor fov="18"/>`,
		`<__video url="rtsp://attacker/x"/>`,
	}

	for _, detail := range spoofs {
		if got := classOf(t, "a-h-G-U-C-A", detail); got != "" {
			t.Errorf("a hostile contact carrying %s classified as %q; the type code decides", detail, got)
		}
	}
}

// Pass two exists so an event whose type says nothing still gets a layout.
func TestAnElementPromotesAnEventTheTypeLeftUnclassified(t *testing.T) {
	cases := []struct {
		detail string
		want   string
	}{
		{`<_medevac_ urgent="1"/>`, ClassMedevac},
		{`<__chat chatroom="Ops"/>`, ClassChat},
		{`<sensor fov="18"/>`, ClassSensor},
		{`<__video url="rtsp://x"/>`, ClassVideo},
	}

	for _, c := range cases {
		if got := classOf(t, "b-m-p-s-p-i", c.detail); got != c.want {
			t.Errorf("%s promoted to %q, want %q", c.detail, got, c.want)
		}
	}
}

// A MEDEVAC forwarded into a chat thread is a MEDEVAC first: the life-safety
// payload outranks the transport it arrived on.
func TestMedevacOutranksChatWhenBothElementsArePresent(t *testing.T) {
	got := classOf(t, "b-m-p-s-p-i", `<__chat chatroom="Ops"/><_medevac_ urgent="2"/>`)
	if got != ClassMedevac {
		t.Errorf("classified as %q, want %q", got, ClassMedevac)
	}
}

// Case is part of a CoT code everywhere else in this package, and a classify
// that folded it would disagree with the label rendered beside it.
func TestClassMatchingIsCaseSensitive(t *testing.T) {
	for _, cotType := range []string{"B-T-F", "b-T-f", "B-R-F-H-C"} {
		if got := classOf(t, cotType, ""); got != "" {
			t.Errorf("%q classified as %q; case is part of the code", cotType, got)
		}
	}
}

// A prefix match must not fire on a code that merely starts with the same
// letters, which is what the separator in the comparison is for.
func TestAPrefixClassNeedsTheSeparator(t *testing.T) {
	if got := classOf(t, "b-t-fx", ""); got != "" {
		t.Errorf("b-t-fx classified as %q; b-t-f is not a prefix of it", got)
	}
}

// The class is absent for an ordinary event, so a post stamped before this
// change and one stamped after it render the same default layout.
func TestNoClassIsWrittenForAnOrdinaryEvent(t *testing.T) {
	props := detailProps(t, `<contact callsign="DELTA1"/>`)

	if held, ok := props["class"]; ok {
		t.Errorf("class is %v; it is absent when it changes nothing", held)
	}
}

// Every class the server writes needs a layout in the webapp, which
// TestWebappCotClassesMatch holds from the other side.
func TestClassesAreTheOnesTheServerCanWrite(t *testing.T) {
	declared := Classes()

	for _, row := range typeClasses {
		if !slices.Contains(declared, row.Class) {
			t.Errorf("typeClasses can produce %q, which Classes() does not name", row.Class)
		}
	}
	for _, row := range blockClasses {
		if !slices.Contains(declared, row.Class) {
			t.Errorf("blockClasses can produce %q, which Classes() does not name", row.Class)
		}
	}
}

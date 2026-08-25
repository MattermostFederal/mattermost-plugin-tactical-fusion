package decorators

import (
	"strings"
	"testing"
)

func TestSoleFencedBlockReadsTheInfoStringAndBody(t *testing.T) {
	block, ok := SoleFencedBlock("```cot\n<event/>\n```")
	if !ok {
		t.Fatal("a lone closed fence was not recognized")
	}
	if block.Info != "cot" {
		t.Errorf("info = %q, want %q", block.Info, "cot")
	}
	if block.Body != "<event/>" {
		t.Errorf("body = %q, want %q", block.Body, "<event/>")
	}
	if block.Lead != "" || block.Trail != "" {
		t.Errorf("lead = %q, trail = %q, want both empty", block.Lead, block.Trail)
	}
}

func TestSoleFencedBlockSplitsLeadAndTrail(t *testing.T) {
	block, ok := SoleFencedBlock("latest PLI\n```cot\n<event/>\n```\nfrom ALPHA")
	if !ok {
		t.Fatal("a fence with prose around it was not recognized")
	}
	if block.Lead != "latest PLI\n" {
		t.Errorf("lead = %q", block.Lead)
	}
	if block.Trail != "\nfrom ALPHA" {
		t.Errorf("trail = %q", block.Trail)
	}
	if block.Body != "<event/>" {
		t.Errorf("body = %q", block.Body)
	}
}

func TestSoleFencedBlockRefusesAnythingButExactlyOneFence(t *testing.T) {
	cases := map[string]string{
		"no fence at all":    "just prose",
		"unterminated fence": "```cot\n<event/>",
		"two fences":         "```cot\n<a/>\n```\n\n```cot\n<b/>\n```",
		"indented fence":     "    ```cot\n    <event/>\n    ```",
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := SoleFencedBlock(message); ok {
				t.Errorf("SoleFencedBlock accepted %q", message)
			}
		})
	}
}

func TestSoleFencedBlockHandlesCRLFAndWiderFences(t *testing.T) {
	cases := map[string]struct {
		message string
		body    string
		info    string
	}{
		"crlf":           {"```cot\r\n<event/>\r\n```\r\n", "<event/>", "cot"},
		"four backticks": {"````xml\n<event/>\n````", "<event/>", "xml"},
		"tilde":          {"~~~cot\n<event/>\n~~~", "<event/>", "cot"},
		"empty body":     {"```cot\n```", "", "cot"},
		"no info string": {"```\n<event/>\n```", "<event/>", ""},
		"inner backticks": {
			"````cot\n```\n<event/>\n```\n````", "```\n<event/>\n```", "cot",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			block, ok := SoleFencedBlock(tc.message)
			if !ok {
				t.Fatalf("SoleFencedBlock refused %q", tc.message)
			}
			if block.Body != tc.body {
				t.Errorf("body = %q, want %q", block.Body, tc.body)
			}
			if block.Info != tc.info {
				t.Errorf("info = %q, want %q", block.Info, tc.info)
			}
		})
	}
}

func TestSoleFencedBlockReassemblesTheMessage(t *testing.T) {
	message := "before\n```cot\n<event/>\n```\nafter"

	block, ok := SoleFencedBlock(message)
	if !ok {
		t.Fatal("SoleFencedBlock refused a well-formed message")
	}

	if !strings.HasPrefix(message, block.Lead) || !strings.HasSuffix(message, block.Trail) {
		t.Fatalf("lead %q and trail %q do not bound %q", block.Lead, block.Trail, message)
	}
	if !strings.Contains(message, block.Body) {
		t.Errorf("body %q is not a substring of the message", block.Body)
	}
}

func TestSoleFencedBlockIsAlwaysProtected(t *testing.T) {
	messages := []string{
		"```cot\n<event uid=\"x\"/>\n```",
		"see MGRS: 18SUJ2347806483\n```cot\n<event/>\n```",
		"```xml\n34.0561,-118.2500\n```\ntrailing 091630ZAUG26",
		"~~~cot\n<event/>\n~~~",
		"````xml\n```\n<event/>\n```\n````",
	}

	for _, message := range messages {
		block, ok := SoleFencedBlock(message)
		if !ok {
			t.Fatalf("SoleFencedBlock refused %q", message)
		}

		bodyStart := len(block.Lead)
		bodyEnd := len(message) - len(block.Trail)

		protected := findProtectedRanges(message)
		covered := false
		for _, r := range protected {
			if r.start <= bodyStart && r.end >= bodyEnd {
				covered = true
				break
			}
		}

		if !covered {
			t.Errorf("the block in %q is not inside any protected range %v", message, protected)
		}
	}
}

func TestSoleElementSpanFindsABareElement(t *testing.T) {
	block, ok := SoleElementSpan("before <event uid=\"u\"/> after", "event")
	if !ok {
		t.Fatal("a bare element was not found")
	}

	if block.Body != `<event uid="u"/>` {
		t.Errorf("body = %q", block.Body)
	}
	if block.Lead != "before " || block.Trail != " after" {
		t.Errorf("lead = %q, trail = %q", block.Lead, block.Trail)
	}
}

func TestSoleElementSpanCoversSiblings(t *testing.T) {
	message := `x <event uid="a"></event> mid <event uid="b"></event> y`

	block, ok := SoleElementSpan(message, "event")
	if !ok {
		t.Fatal("siblings were not found")
	}

	if block.Lead != "x " || block.Trail != " y" {
		t.Errorf("lead = %q, trail = %q", block.Lead, block.Trail)
	}
	if !strings.Contains(block.Body, `uid="a"`) || !strings.Contains(block.Body, `uid="b"`) {
		t.Errorf("body did not cover both siblings: %q", block.Body)
	}
}

// The whole reason this lives beside the fence scanner. An author who fenced an
// element has said it is code, and reaching inside that is the corruption
// protected ranges exist to stop.
func TestSoleElementSpanRefusesWhatIsProtected(t *testing.T) {
	cases := map[string]string{
		"unlabelled fence": "```\n<event uid=\"u\"/>\n```",
		"labelled fence":   "```xml\n<event uid=\"u\"/>\n```",
		"tilde fence":      "~~~\n<event uid=\"u\"/>\n~~~",
		"inline code":      "`<event uid=\"u\"/>`",
		"indented code":    "    <event uid=\"u\"/>",
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := SoleElementSpan(message, "event"); ok {
				t.Errorf("SoleElementSpan reached inside %s", name)
			}
		})
	}
}

func TestSoleElementSpanIsNotFooledByALongerName(t *testing.T) {
	for _, message := range []string{"the <eventual> plan", "<events><a/></events>", "no element here"} {
		if _, ok := SoleElementSpan(message, "event"); ok {
			t.Errorf("SoleElementSpan matched %q", message)
		}
	}
}

func TestSoleElementSpanReassemblesTheMessage(t *testing.T) {
	message := `note <event uid="u"><point lat="1" lon="2"/></event> end`

	block, ok := SoleElementSpan(message, "event")
	if !ok {
		t.Fatal("SoleElementSpan refused a well formed message")
	}

	if block.Lead+block.Body+block.Trail != message {
		t.Errorf("the three parts do not reassemble: %q + %q + %q", block.Lead, block.Body, block.Trail)
	}
}

// An element that is opened and never closed is not a span.
//
// elementEnd's refusal had no test: every other declining case here is a name
// that does not match or a span sitting inside protected text, and both of
// those are refused before the end is looked for. An unterminated element is
// the one shape that gets all the way to the search, and answering it with a
// span running to the end of the message would hand the whole rest of the post
// to a parser as though the author had closed it.
func TestSoleElementSpanRefusesAnUnterminatedElement(t *testing.T) {
	for name, message := range map[string]string{
		"never closed":                 `see <event uid="u"`,
		"opened with text after":       `<event uid="u">no close`,
		"closed with a different name": `<event uid="u"></eventual>`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := SoleElementSpan(message, "event"); ok {
				t.Error("an unterminated element was read as a span")
			}
		})
	}
}

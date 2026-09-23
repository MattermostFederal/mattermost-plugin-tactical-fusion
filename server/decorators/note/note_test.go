package note

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

const table = "| Tail | Fuel (lb) |\n|:--|--:|\n| 101 | 12,000 |\n| 102 & #3 | ~ 9,800 * |\n\nNotes: café [ref](https://example.com/a_b)"

func TestParseAcceptsMultiLineMarkdown(t *testing.T) {
	params, ok := (&Decorator{}).Parse(table, time.Time{})
	if !ok || params.Get(ParamValue) != table {
		t.Fatalf("Parse(%q) = %v, %v", table, params, ok)
	}
}

func TestParseRefusesEmptyAndOverlongMarkdown(t *testing.T) {
	for name, value := range map[string]string{
		"empty":     "",
		"blank":     " \n\t ",
		"over cap":  strings.Repeat("é", MaxNoteRunes+1),
		"bad utf-8": "a\xffb",
	} {
		if _, ok := (&Decorator{}).Parse(value, time.Time{}); ok {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, ok := (&Decorator{}).Parse(strings.Repeat("é", MaxNoteRunes), time.Time{}); !ok {
		t.Error("a note at the cap was refused")
	}
}

func TestANoteNeverMatchesMessageText(t *testing.T) {
	if patterns := (&Decorator{}).Patterns(); len(patterns) != 0 {
		t.Errorf("a note has %d patterns", len(patterns))
	}
}

func TestTheLinkCarriesTheMarkdownByteForByte(t *testing.T) {
	params, _ := (&Decorator{}).Parse(table, time.Time{})
	tagger := &decorators.Tagger{URLPrefix: "/plugins/tf/decorate"}
	link := tagger.LinkFor(Type, "Fuel", params)

	href := strings.TrimSuffix(strings.TrimPrefix(link, "[Fuel]("), ")")
	if strings.ContainsAny(href, " \n()") {
		t.Fatalf("the destination would end the markdown link early: %q", href)
	}
	parsed, err := url.Parse(href)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get(ParamValue); got != table {
		t.Errorf("round trip:\n got %q\nwant %q", got, table)
	}
}

func TestThePageEscapesTheMarkdownSource(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"**hi** <script>alert(1)</script>"}})

	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "<script>alert") || !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("status %d, body:\n%s", rec.Code, body)
	}
}

func TestThePageRefusesALinkWithNoNote(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "TF-17006") {
		t.Errorf("status %d, body:\n%s", rec.Code, rec.Body.String())
	}
}

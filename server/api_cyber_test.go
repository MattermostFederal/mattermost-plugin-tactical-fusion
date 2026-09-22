package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const testTeamID = "yb7hrkmbcigxuk1w1xgxj3rrqe"

func cyberURL(kind, value string) string {
	return cyberPath + "?" + url.Values{"k": {kind}, "v": {value}}.Encode()
}

func mentionsURL(kind, value, team string) string {
	return cyberMentionsPath + "?" + url.Values{"k": {kind}, "v": {value}, "team": {team}}.Encode()
}

func TestCyberRequiresASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, path := range []string{
		cyberURL("cve", "CVE-2021-44228"),
		mentionsURL("cve", "CVE-2021-44228", testTeamID),
	} {
		rec := call(p, http.MethodGet, path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, rec.Code)
		}
		assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
	}
}

func TestCyberRefusesAnythingButGet(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := call(p, method, cyberURL("cve", "CVE-2021-44228"), testUserID, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
	}
}

func TestCyberAnswersWithTheIndicator(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("attack", "T1059.001"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	var got cyberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a cyber response: %v (%s)", err, rec.Body.String())
	}

	if got.Kind != "attack" || got.Value != "T1059.001" {
		t.Fatalf("kind = %q, value = %q", got.Kind, got.Value)
	}
	if got.Title != "PowerShell" {
		t.Errorf("title = %q", got.Title)
	}
	if len(got.Rows) == 0 || len(got.Related) == 0 || len(got.Datasets) == 0 {
		t.Errorf("rows = %d, related = %d, datasets = %d", len(got.Rows), len(got.Related), len(got.Datasets))
	}
}

func TestCyberListsAreNeverNull(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("hash", strings.Repeat("a", 64)), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	for _, field := range []string{`"rows":[`, `"related":[`, `"watchlist":[`, `"datasets":[`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("%s is not an array in %s", field, rec.Body.String())
		}
	}
}

func TestCyberRefusesAPairThatDisagreesWithItself(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, path := range []string{
		cyberURL("cve", "CWE-79"),
		cyberURL("nonsense", "CVE-2021-44228"),
		cyberURL("cve", "cve-2021-44228"),
		cyberURL("attack", "T9999"),
		cyberURL("", "CVE-2021-44228"),
		cyberURL("cve", ""),
		cyberURL("cve", "<script>"),
	} {
		rec := call(p, http.MethodGet, path, testUserID, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", path, rec.Code)
			continue
		}
		assertCode(t, rec.Body.String(), errcode.APICyberInvalid)
	}
}

func TestCyberCachesPrivatelyAndBriefly(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("cwe", "CWE-79"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestCyberRefusalIsNotCached(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("cve", "nonsense"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("a refusal carried Cache-Control %q", got)
	}
}

func TestCyberAnswersWhileTheDecoratorIsOff(t *testing.T) {
	p, _ := newAPIPlugin(t)
	p.setConfiguration(&configuration{})

	rec := call(p, http.MethodGet, cyberURL("cwe", "CWE-79"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with everything switched off", rec.Code)
	}
}

func TestCyberMentionsRequireATeam(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, team := range []string{"", "short", "not a team id at all here ok"} {
		rec := call(p, http.MethodGet, mentionsURL("cve", "CVE-2021-44228", team), testUserID, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("team %q: status = %d, want 400", team, rec.Code)
			continue
		}
		assertCode(t, rec.Body.String(), errcode.APICyberTeamInvalid)
	}
}

func TestCyberMentionsRefuseAnIndicatorTheyDidNotIssue(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, mentionsURL("cve", "CWE-79", testTeamID), testUserID, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APICyberInvalid)
}

func TestCyberMentionsSearchAsTheReader(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.teams = map[string]*model.Team{testTeamID: {Id: testTeamID, Name: "ops"}}
	api.channels = map[string]*model.Channel{"chan1": {Id: "chan1", DisplayName: "Incident 4821"}}
	api.searchResults = &model.PostSearchResults{
		PostList: &model.PostList{
			Order: []string{"post1"},
			Posts: map[string]*model.Post{
				"post1": {
					Id:        "post1",
					ChannelId: "chan1",
					CreateAt:  1767225600000,
					Message:   "patching [CVE-2021-44228](/plugins/tf/decorate/cyber?k=cve&v=CVE-2021-44228) tonight",
				},
			},
		},
	}

	rec := call(p, http.MethodGet, mentionsURL("cve", "CVE-2021-44228", testTeamID), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	if len(api.searchAsked) != 1 {
		t.Fatalf("%d searches ran", len(api.searchAsked))
	}
	asked := api.searchAsked[0]
	if asked.userID != testUserID {
		t.Errorf("searched as %q, want the reader %q", asked.userID, testUserID)
	}
	if asked.teamID != testTeamID {
		t.Errorf("searched team %q", asked.teamID)
	}
	if asked.terms != `"CVE-2021-44228"` {
		t.Errorf("terms = %q, want the value as a quoted phrase", asked.terms)
	}

	var got cyberMentionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a mentions response: %v", err)
	}
	if len(got.Mentions) != 1 {
		t.Fatalf("%d mentions", len(got.Mentions))
	}

	mention := got.Mentions[0]
	if mention.Channel != "Incident 4821" {
		t.Errorf("channel = %q", mention.Channel)
	}
	if mention.Permalink != "/ops/pl/post1" {
		t.Errorf("permalink = %q", mention.Permalink)
	}

	if mention.Snippet != "patching CVE-2021-44228 tonight" {
		t.Errorf("snippet = %q", mention.Snippet)
	}
}

func TestCyberMentionsAreNeverCached(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.searchResults = &model.PostSearchResults{PostList: &model.PostList{}}

	rec := call(p, http.MethodGet, mentionsURL("cve", "CVE-2021-44228", testTeamID), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store for a per-reader answer", got)
	}
}

func TestCyberMentionsReportAFailedSearch(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.searchErr = model.NewAppError("search", "failed", nil, "", 500)

	rec := call(p, http.MethodGet, mentionsURL("cve", "CVE-2021-44228", testTeamID), testUserID, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APICyberSearchFailed)
}

func TestCyberMentionsAnswerEmptyRatherThanNull(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.searchResults = nil

	rec := call(p, http.MethodGet, mentionsURL("cve", "CVE-2021-44228", testTeamID), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"mentions":[]`) {
		t.Fatalf("mentions is not an empty array: %s", rec.Body.String())
	}
}

func TestMentionSnippetCollapsesLinksAndIsBounded(t *testing.T) {
	cases := map[string]string{
		"[CVE-2021-44228](/x?k=cve) patched": "CVE-2021-44228 patched",
		"see   spaced\n\nlines":              "see spaced lines",
		strings.Repeat("word ", 200):         "",
		"no links here":                      "no links here",
		"[a](/1) and [b](/2)":                "a and b",
	}

	for message, want := range cases {
		got := mentionSnippet(message)
		if want != "" && got != want {
			t.Errorf("mentionSnippet(%q) = %q, want %q", message, got, want)
		}
		if len([]rune(got)) > mentionSnippetRune+3 {
			t.Errorf("a snippet of %d runes escaped the bound", len([]rune(got)))
		}
	}
}

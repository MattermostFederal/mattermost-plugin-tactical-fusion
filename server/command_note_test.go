package main

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/note"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func runNoteCommand(t *testing.T, p *Plugin, command string) *model.CommandResponse {
	t.Helper()

	p.API.(*fakeAPI).created = nil
	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   command,
		UserId:    "user1",
		ChannelId: "channel1",
		RootId:    "root1",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	return response
}

var noteLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*/decorate/note\?[^)]*)\)`)

func noteMarkdownIn(t *testing.T, message string) (string, string) {
	t.Helper()

	m := noteLinkPattern.FindStringSubmatch(message)
	if m == nil {
		t.Fatalf("no note link in %q", message)
	}
	parsed, err := url.Parse(m[2])
	if err != nil {
		t.Fatal(err)
	}
	return m[1], parsed.Query().Get(note.ParamValue)
}

func TestTheNoteCommandPostsALinkCarryingTheMarkdown(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	markdown := "| Tail | Fuel |\n|:--|--:|\n| 101 | 12,000 lb |"

	response := runNoteCommand(t, p, "/tactical-fusion note Fuel state | "+markdown)
	created := p.API.(*fakeAPI).created
	if response.Text != "" || len(created) != 1 {
		t.Fatalf("response %q, %d posts", response.Text, len(created))
	}

	post := created[0]
	if post.UserId != "user1" || post.ChannelId != "channel1" || post.RootId != "root1" {
		t.Errorf("posted as %q in %q under %q", post.UserId, post.ChannelId, post.RootId)
	}
	label, got := noteMarkdownIn(t, post.Message)
	if label != "Fuel state" || got != markdown {
		t.Errorf("label %q, markdown %q", label, got)
	}
}

func TestTheNoteCommandRefusesWhatItCannotPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for command, code := range map[string]int{
		"/tactical-fusion note":                                                    errcode.CommandNoteUsage,
		"/tactical-fusion note Fuel":                                               errcode.CommandNoteUsage,
		"/tactical-fusion note | **bold**":                                         errcode.CommandNoteUsage,
		"/tactical-fusion note Fuel |   ":                                          errcode.CommandNoteUsage,
		"/tactical-fusion note Fu\nel | x":                                         errcode.CommandNoteUsage,
		"/tactical-fusion note Fuel | " + strings.Repeat("x", note.MaxNoteRunes+1): errcode.CommandNoteInvalid,
		"/tactical-fusion note Fuel | " + strings.Repeat("é", note.MaxNoteRunes):   errcode.CommandNoteTooLong,
	} {
		response := runNoteCommand(t, p, command)
		if !strings.Contains(response.Text, "(TF-"+strconv.Itoa(code)+")") {
			t.Errorf("%.60q replied %q, want %v", command, response.Text, code)
		}
		if n := len(p.API.(*fakeAPI).created); n != 0 {
			t.Errorf("%.60q posted %d messages", command, n)
		}
	}
}

func TestEveryNoteExampleCarriesItsMarkdownWhole(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	messages := p.noteExampleMessages()

	for i, example := range noteExamples {
		message := messages[i]
		if n := utf8.RuneCountInString(message); n > safePostRunes {
			t.Errorf("%q is %d runes, past the safe post size", example.heading, n)
		}
		if strings.Contains(message, "{") {
			t.Errorf("%q has a placeholder left over:\n%s", example.heading, message)
		}
		for _, entry := range example.notes {
			if _, ok := (&note.Decorator{}).Parse(entry.markdown, time.Time{}); !ok {
				t.Errorf("the %q note does not parse", entry.label)
			}
		}

		links := noteLinkPattern.FindAllStringSubmatch(message, -1)
		if len(links) != len(example.notes) {
			t.Fatalf("%q has %d note links for %d notes", example.heading, len(links), len(example.notes))
		}
		for j, entry := range example.notes {
			label, markdown := noteMarkdownIn(t, links[j][0])
			if label != entry.label || markdown != entry.markdown {
				t.Errorf("link %d of %q is %q with %q", j, example.heading, label, markdown)
			}
		}
	}
}

func TestTheGlossaryExampleExpandsEveryAcronym(t *testing.T) {
	glossary := strings.Join(noteExampleMessages(t), "\n")
	for _, acronym := range []string{"DCA", "OCA", "ISR", "ACO", "ATO", "COP", "ROE", "BDA", "CAS", "TST"} {
		if !strings.Contains(glossary, "["+acronym+"](") {
			t.Errorf("%s is not a note link", acronym)
		}
	}
}

func noteExampleMessages(t *testing.T) []string {
	t.Helper()
	return newTestPlugin(t, "https://example.com", true).noteExampleMessages()
}

func TestCommandsThatPostRefuseAUserWhoCannotPost(t *testing.T) {
	for _, command := range []string{"/tactical-fusion note Fuel | **12,400 lb**", "/tactical-fusion examples"} {
		p := newTestPlugin(t, "https://example.com", true)
		api := p.API.(*fakeAPI)
		api.channelDenied = map[string]bool{model.PermissionCreatePost.Id: true}

		response := runNoteCommand(t, p, command)
		if !strings.Contains(response.Text, "(TF-"+strconv.Itoa(errcode.CommandPostNotPermitted)+")") {
			t.Errorf("%q replied %q", command, response.Text)
		}
		if len(api.created) != 0 {
			t.Errorf("%q posted %d messages", command, len(api.created))
		}
	}
}

func TestANoteFromAUserWithoutChannelMentionsCannotNotifyTheChannel(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	runNoteCommand(t, p, "/tactical-fusion note @channel | read me")
	if api.created[0].GetProp(model.PostPropsMentionHighlightDisabled) != nil {
		t.Error("a user allowed channel mentions had them disabled")
	}

	api.channelDenied = map[string]bool{model.PermissionUseChannelMentions.Id: true}
	runNoteCommand(t, p, "/tactical-fusion note @channel | read me")
	if api.created[0].GetProp(model.PostPropsMentionHighlightDisabled) != true {
		t.Error("a user without channel mentions can notify the channel through a note")
	}
}

func TestTheNoteCommandSaysWhenItCouldNotPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.createPostErr = model.NewAppError("CreatePost", "archived", nil, "", 403)

	response := runNoteCommand(t, p, "/tactical-fusion note Fuel | **12,400 lb**")
	if !strings.Contains(response.Text, "(TF-"+strconv.Itoa(errcode.CommandNotePostFailed)+")") {
		t.Errorf("replied %q", response.Text)
	}
}

func TestNotesPassThroughThePostHookUnchanged(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	messages := append(p.noteExampleMessages(), noteLink(&decorators.Tagger{URLPrefix: p.decorateURLPrefix()},
		"tokens", "141200ZSEP26 at 21.3353, -157.9483, ICAO:PHNL on FREQ:121.5\n\n```\n18S UJ 23478 06483\n```"))
	for _, message := range messages {
		if changed := p.decoratePost(&model.Post{Message: message}, time.Now()); changed != nil {
			t.Errorf("the post hook rewrote a note:\n%s\n=>\n%s", message, changed.Message)
		}
	}
}

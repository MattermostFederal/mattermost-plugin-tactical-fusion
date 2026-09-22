package main

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	cyberPath = apiPath + "/cyber"

	cyberMentionsPath = apiPath + "/cyber/mentions"
)

const (
	mentionsPerPage    = 20
	mentionSnippetRune = 200
)

type cyberRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type cyberLink struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Label string `json:"label"`
}

type cyberWatchEntry struct {
	Verdict string `json:"verdict"`
	Source  string `json:"source"`
	Note    string `json:"note"`
	Updated string `json:"updated"`
	Known   bool   `json:"known"`
}

type cyberDataset struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Present   bool   `json:"present"`
	Generated string `json:"generated"`
}

type cyberResponse struct {
	Kind      string            `json:"kind"`
	Value     string            `json:"value"`
	Title     string            `json:"title"`
	Headline  string            `json:"headline"`
	Summary   string            `json:"summary"`
	Status    string            `json:"status"`
	Rows      []cyberRow        `json:"rows"`
	Related   []cyberLink       `json:"related"`
	Watchlist []cyberWatchEntry `json:"watchlist"`
	Datasets  []cyberDataset    `json:"datasets"`
}

type cyberMention struct {
	PostID    string `json:"post_id"`
	ChannelID string `json:"channel_id"`
	Channel   string `json:"channel"`
	CreateAt  int64  `json:"create_at"`
	Snippet   string `json:"snippet"`
	Permalink string `json:"permalink"`
}

type cyberMentionsResponse struct {
	Value     string         `json:"value"`
	Mentions  []cyberMention `json:"mentions"`
	Truncated bool           `json:"truncated"`
}

func (p *Plugin) serveCyber(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	kind, value, ok := cyberParams(r)
	if !ok {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APICyberInvalid, "That is not an indicator this plugin issued."))
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=60")

	writeAPIJSON(w, http.StatusOK, cyberBody(cyber.Describe(kind, value, p.cyberIntel())))
}

func cyberParams(r *http.Request) (cyber.Kind, string, bool) {
	query := r.URL.Query()
	kind := cyber.Kind(query.Get(cyber.ParamKind))
	value := query.Get(cyber.ParamValue)

	return kind, value, cyber.RecognizeAs(kind, value)
}

func cyberBody(details cyber.Details) cyberResponse {
	body := cyberResponse{
		Kind:      string(details.Kind),
		Value:     details.Value,
		Title:     details.Title,
		Headline:  details.Headline,
		Summary:   details.Summary,
		Status:    details.Status,
		Rows:      []cyberRow{},
		Related:   []cyberLink{},
		Watchlist: []cyberWatchEntry{},
		Datasets:  []cyberDataset{},
	}

	for _, row := range details.Rows {
		body.Rows = append(body.Rows, cyberRow{Label: row.Label, Value: row.Value})
	}
	for _, link := range details.Related {
		body.Related = append(body.Related, cyberLink{
			Kind: string(link.Kind), Value: link.Value, Label: link.Label,
		})
	}
	for _, entry := range details.Watchlist {
		body.Watchlist = append(body.Watchlist, cyberWatchEntry{
			Verdict: entry.Verdict, Source: entry.Source, Note: entry.Note,
			Updated: entry.Updated, Known: entry.Known,
		})
	}
	for _, dataset := range details.Datasets {
		body.Datasets = append(body.Datasets, cyberDataset{
			Name: dataset.Name, Label: dataset.Label,
			Present: dataset.Present, Generated: dataset.Generated,
		})
	}

	return body
}

func (p *Plugin) serveCyberMentions(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	_, value, ok := cyberParams(r)
	if !ok {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APICyberInvalid, "That is not an indicator this plugin issued."))
		return
	}

	teamID := r.URL.Query().Get("team")
	if !model.IsValidId(teamID) {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APICyberTeamInvalid, "That is not a team."))
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	results, appErr := p.API.SearchPostsInTeamForUser(teamID, userID, model.SearchParameter{
		Terms:      stringPointer(`"` + value + `"`),
		IsOrSearch: boolPointer(false),
		Page:       intPointer(0),
		PerPage:    intPointer(mentionsPerPage),
	})
	if appErr != nil {
		p.API.LogWarn("the prior-mentions search failed",
			"error_code", errcode.APICyberSearchFailed, "team_id", teamID, "error", appErr.Error())
		writeAPIError(w, http.StatusBadGateway,
			errcode.WithCode(errcode.APICyberSearchFailed, "Could not search for earlier mentions."))
		return
	}

	writeAPIJSON(w, http.StatusOK, p.mentionsBody(value, teamID, results))
}

func (p *Plugin) mentionsBody(value, teamID string, results *model.PostSearchResults) cyberMentionsResponse {
	body := cyberMentionsResponse{Value: value, Mentions: []cyberMention{}}
	if results == nil || results.PostList == nil {
		return body
	}

	teamName := ""
	if team, appErr := p.API.GetTeam(teamID); appErr == nil && team != nil {
		teamName = team.Name
	}

	for _, id := range results.Order {
		post := results.PostList.Posts[id]
		if post == nil {
			continue
		}

		mention := cyberMention{
			PostID:    post.Id,
			ChannelID: post.ChannelId,
			CreateAt:  post.CreateAt,
			Snippet:   mentionSnippet(post.Message),
		}

		if channel, appErr := p.API.GetChannel(post.ChannelId); appErr == nil && channel != nil {
			mention.Channel = channel.DisplayName
		}
		if teamName != "" {
			mention.Permalink = "/" + teamName + "/pl/" + post.Id
		}

		body.Mentions = append(body.Mentions, mention)
	}

	body.Truncated = len(body.Mentions) >= mentionsPerPage

	return body
}

func mentionSnippet(message string) string {
	collapsed := markdownLinkRe.ReplaceAllString(message, "$1")
	collapsed = strings.Join(strings.Fields(collapsed), " ")

	runes := []rune(collapsed)
	if len(runes) <= mentionSnippetRune {
		return collapsed
	}

	return strings.TrimSpace(string(runes[:mentionSnippetRune])) + "..."
}

func stringPointer(v string) *string { return &v }

func boolPointer(v bool) *bool { return &v }

func intPointer(v int) *int { return &v }

var markdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

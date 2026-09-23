package main

import (
	"net/http"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const cyberPath = apiPath + "/cyber"

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

	Affected       []string         `json:"affected"`
	Configurations []string         `json:"configurations"`
	References     []cyberReference `json:"references"`
}

type cyberReference struct {
	URL  string `json:"url"`
	Tags string `json:"tags"`
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

		Affected:       append([]string{}, details.Affected...),
		Configurations: append([]string{}, details.Configurations...),
		References:     []cyberReference{},
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
	for _, ref := range details.References {
		body.References = append(body.References, cyberReference{URL: ref.URL, Tags: ref.Tags})
	}

	return body
}

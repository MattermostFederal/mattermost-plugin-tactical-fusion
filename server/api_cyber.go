package main

import (
	"net/http"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const cyberPath = apiPath + "/cyber"

type cyberRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Query string `json:"query"`
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

	Score     string              `json:"score"`
	Severity  string              `json:"severity"`
	Exploited bool                `json:"exploited"`
	Vector    []cyberVectorMetric `json:"vector"`

	Affected       []string         `json:"affected"`
	Configurations []string         `json:"configurations"`
	References     []cyberReference `json:"references"`

	Sections []cyberSection `json:"sections"`
	Credits  []cyberCredit  `json:"credits"`
	Glance   cyberGlance    `json:"glance"`
	Reports  []cyberReport  `json:"reports"`
}

type cyberReport struct {
	Source    string `json:"source"`
	Malicious bool   `json:"malicious"`
	Threat    string `json:"threat"`
	Detail    string `json:"detail"`
	URL       string `json:"url"`
}

type cyberGlance struct {
	Subtitle string   `json:"subtitle"`
	Summary  string   `json:"summary"`
	Tags     []string `json:"tags"`
	Facts    []string `json:"facts"`
	Status   string   `json:"status"`
}

type cyberCredit struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type cyberSection struct {
	Title string      `json:"title"`
	Items []cyberItem `json:"items"`
}

type cyberItem struct {
	Head  string `json:"head"`
	Text  string `json:"text"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
	URL   string `json:"url"`
}

type cyberVectorMetric struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
	Severe bool   `json:"severe"`
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

		Score:     details.Score,
		Severity:  details.Severity,
		Exploited: details.Exploited,
		Vector:    []cyberVectorMetric{},

		Affected:       append([]string{}, details.Affected...),
		Configurations: append([]string{}, details.Configurations...),
		References:     []cyberReference{},

		Sections: []cyberSection{},
		Credits:  []cyberCredit{},
		Reports:  []cyberReport{},
		Glance: cyberGlance{
			Subtitle: details.Glance.Subtitle,
			Summary:  details.Glance.Summary,
			Tags:     append([]string{}, details.Glance.Tags...),
			Facts:    append([]string{}, details.Glance.Facts...),
			Status:   details.Glance.Status,
		},
	}

	for _, row := range details.Rows {
		body.Rows = append(body.Rows, cyberRow{Label: row.Label, Value: row.Value, Query: dtg.QueryForZulu(row.At)})
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
	for _, metric := range details.Vector {
		body.Vector = append(body.Vector, cyberVectorMetric{Metric: metric.Metric, Value: metric.Value, Severe: metric.Severe})
	}
	for _, report := range details.Reports {
		body.Reports = append(body.Reports, cyberReport{Source: report.Source, Malicious: report.Malicious, Threat: report.Threat, Detail: report.Detail, URL: report.URL})
	}
	for _, credit := range details.Credits {
		body.Credits = append(body.Credits, cyberCredit{Text: credit.Text, URL: credit.URL})
	}
	for _, section := range details.Sections {
		body.Sections = append(body.Sections, cyberSectionOf(section))
	}
	for _, ref := range details.References {
		body.References = append(body.References, cyberReference{URL: ref.URL, Tags: ref.Tags})
	}

	return body
}

func cyberSectionOf(section cyber.Section) cyberSection {
	items := make([]cyberItem, 0, len(section.Items))
	for _, item := range section.Items {
		wire := cyberItem{Head: item.Head, Text: item.Text, URL: item.URL}
		if item.Link != nil {
			wire.Kind = string(item.Link.Kind)
			wire.Value = item.Link.Value
		}
		items = append(items, wire)
	}
	return cyberSection{Title: section.Title, Items: items}
}

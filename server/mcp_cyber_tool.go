package main

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	maxCyberToolIndicators = 25
	maxCyberToolItems      = 10

	notAnIndicator = "not a CVE, CWE, MITRE ATT&CK id, IP address or MD5, SHA-1 or SHA-256 hash"
)

type LookupCyberIndicatorArgs struct {
	Indicators []string `json:"indicators" jsonschema:"1 to 25 indicators as a person writes them: CVE ids such as CVE-2021-44228, CWE ids such as CWE-79, MITRE ATT&CK technique or tactic ids such as T1059.001 or TA0002, IPv4 or IPv6 addresses, and MD5, SHA-1 or SHA-256 file hashes"`
}

type CyberFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CyberVectorEntry struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
	Severe bool   `json:"severe"`
}

type CyberRelated struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Label string `json:"label"`
}

type CyberVerdict struct {
	Verdict string `json:"verdict"`
	Source  string `json:"source"`
	Note    string `json:"note,omitempty"`
	Updated string `json:"updated,omitempty"`
}

type CyberThreat struct {
	Source    string `json:"source"`
	Malicious bool   `json:"malicious"`
	Threat    string `json:"threat"`
	Detail    string `json:"detail,omitempty"`
	URL       string `json:"url,omitempty"`
}

type CyberCredit struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

type CyberSectionItem struct {
	Head string `json:"head,omitempty"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type CyberToolSection struct {
	Title     string             `json:"title"`
	Total     int                `json:"total"`
	Truncated bool               `json:"truncated"`
	Items     []CyberSectionItem `json:"items"`
}

type CyberIndicatorResult struct {
	Input      string `json:"input"`
	Recognized bool   `json:"recognized"`
	Reason     string `json:"reason,omitempty"`

	Kind      string `json:"kind,omitempty"`
	Value     string `json:"value,omitempty"`
	Title     string `json:"title,omitempty"`
	Headline  string `json:"headline,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
	Link      string `json:"link,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Score     string `json:"score,omitempty"`
	Exploited bool   `json:"exploited"`

	DataSources []CyberDataSource `json:"data_sources,omitempty" jsonschema:"every dataset file behind this answer and when it was compiled"`

	Facts     []CyberFact        `json:"facts,omitempty"`
	Vector    []CyberVectorEntry `json:"vector,omitempty"`
	Related   []CyberRelated     `json:"related,omitempty"`
	Watchlist []CyberVerdict     `json:"watchlist,omitempty"`
	Reports   []CyberThreat      `json:"reports,omitempty"`
	Credits   []CyberCredit      `json:"credits,omitempty"`
	Sections  []CyberToolSection `json:"sections,omitempty"`
}

type CyberDataSource struct {
	Label    string `json:"label"`
	File     string `json:"file"`
	Compiled string `json:"compiled" jsonschema:"when the file was compiled, RFC 3339"`
}

type CyberIndicatorLookup struct {
	Results         []CyberIndicatorResult `json:"results"`
	DatasetsMissing []string               `json:"datasets_missing"`
}

func (p *Plugin) lookupCyberIndicatorTool(_ context.Context, _ *mcp.CallToolRequest, in LookupCyberIndicatorArgs) (*mcp.CallToolResult, CyberIndicatorLookup, error) {
	if len(in.Indicators) == 0 || len(in.Indicators) > maxCyberToolIndicators {
		return toolRefusal(errcode.MCPCyberInvalid,
			"Give between 1 and 25 indicators."), CyberIndicatorLookup{}, nil
	}

	set := p.cyberIntel()
	lookup := CyberIndicatorLookup{Results: []CyberIndicatorResult{}, DatasetsMissing: missingDatasets(set)}
	answered := map[string]bool{}

	for _, input := range in.Indicators {
		kind, canonical, ok := cyber.Recognize(input)
		if !ok {
			lookup.Results = append(lookup.Results, CyberIndicatorResult{Input: input, Reason: notAnIndicator})
			continue
		}
		if answered[string(kind)+":"+canonical] {
			continue
		}
		answered[string(kind)+":"+canonical] = true

		result := cyberIndicatorResult(input, cyber.Describe(kind, canonical, set))
		result.Link = p.cyberLink(canonical)
		lookup.Results = append(lookup.Results, result)
	}

	return nil, lookup, nil
}

func (p *Plugin) cyberLink(indicator string) string {
	link, refusal := p.buildLink(bridgeclient.LinkRequest{Type: cyber.Type, Token: indicator})
	if refusal != nil {
		return ""
	}
	return link.Markdown
}

func missingDatasets(set *intel.Set) []string {
	missing := []string{}
	for _, status := range set.Statuses() {
		if !status.Present {
			missing = append(missing, cyber.DatasetLabel(status.Name))
		}
	}
	return missing
}

func capped[T any](items []T) []T {
	if len(items) > maxCyberToolItems {
		return items[:maxCyberToolItems]
	}
	return items
}

func cyberIndicatorResult(input string, d cyber.Details) CyberIndicatorResult {
	result := CyberIndicatorResult{
		Input:      input,
		Recognized: true,
		Kind:       string(d.Kind),
		Value:      d.Value,
		Title:      d.Title,
		Headline:   d.Headline,
		Summary:    d.Summary,
		Status:     d.Status,
		Severity:   d.Severity,
		Score:      d.Score,
		Exploited:  d.Exploited,
	}

	for _, row := range d.Rows {
		result.Facts = append(result.Facts, CyberFact{Label: row.Label, Value: row.Value})
	}
	for _, metric := range d.Vector {
		result.Vector = append(result.Vector, CyberVectorEntry{Metric: metric.Metric, Value: metric.Value, Severe: metric.Severe})
	}
	for _, link := range capped(d.Related) {
		result.Related = append(result.Related, CyberRelated{Kind: string(link.Kind), Value: link.Value, Label: link.Label})
	}
	for _, entry := range d.Watchlist {
		result.Watchlist = append(result.Watchlist, CyberVerdict{Verdict: entry.Verdict, Source: entry.Source, Note: entry.Note, Updated: entry.Updated})
	}
	for _, report := range d.Reports {
		result.Reports = append(result.Reports, CyberThreat{Source: report.Source, Malicious: report.Malicious, Threat: report.Threat, Detail: report.Detail, URL: report.URL})
	}
	for _, credit := range d.Credits {
		result.Credits = append(result.Credits, CyberCredit{Text: credit.Text, URL: credit.URL})
	}

	for _, source := range d.Freshness {
		result.DataSources = append(result.DataSources, CyberDataSource{Label: source.Label, File: source.File, Compiled: source.Compiled.Format(time.RFC3339)})
	}

	result.Sections = cyberToolSections(d)
	return result
}

func toolSection(title string, items []CyberSectionItem) CyberToolSection {
	return CyberToolSection{
		Title:     title,
		Total:     len(items),
		Truncated: len(items) > maxCyberToolItems,
		Items:     capped(items),
	}
}

func lineItems(lines []string) []CyberSectionItem {
	items := make([]CyberSectionItem, 0, len(lines))
	for _, line := range lines {
		items = append(items, CyberSectionItem{Text: line})
	}
	return items
}

func cyberToolSections(d cyber.Details) []CyberToolSection {
	var sections []CyberToolSection

	if len(d.Affected) > 0 {
		sections = append(sections, toolSection("Affected, as reported", lineItems(d.Affected)))
	}
	if len(d.Configurations) > 0 {
		sections = append(sections, toolSection("Affected, per NVD", lineItems(d.Configurations)))
	}
	if len(d.References) > 0 {
		items := make([]CyberSectionItem, 0, len(d.References))
		for _, ref := range d.References {
			items = append(items, CyberSectionItem{Text: ref.Tags, URL: ref.URL})
		}
		sections = append(sections, toolSection("References", items))
	}
	for _, section := range d.Sections {
		items := make([]CyberSectionItem, 0, len(section.Items))
		for _, item := range section.Items {
			items = append(items, CyberSectionItem{Head: item.Head, Text: item.Text, URL: item.URL})
		}
		sections = append(sections, toolSection(section.Title, items))
	}

	return sections
}

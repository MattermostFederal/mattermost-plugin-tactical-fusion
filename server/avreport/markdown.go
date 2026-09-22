package avreport

import (
	"net/url"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

const (
	summaryRowLabel      = "Summary"
	tableFallbackHeading = "Aviation report"

	timestampLayout = "2006-01-02T15:04:05Z"
)

func issuedLabel(kind string) string {
	if kind == KindNOTAM {
		return "Effective"
	}
	return "Issued"
}

func reportTable(href string, report Report) string {
	var b strings.Builder
	links := &decorators.Tagger{URLPrefix: href[:strings.LastIndex(href, "/"+Type+"?")]}

	b.WriteString("| " + decorators.TableCell(report.Kind) + " | " + tableHeadingDetail(links, report) + " |\n")
	b.WriteString("|:--|:--|\n")

	if report.Summary != "" {
		writeTableRow(&b, summaryRowLabel, decorators.TableCell(report.Summary))
	}
	if !report.IssuedAt.IsZero() {
		writeTableRow(&b, issuedLabel(report.Kind), timeCell(links, report.IssuedAt))
	}
	if len(report.Flags) > 0 {
		writeTableRow(&b, "Flags", decorators.TableCell(strings.Join(report.Flags, ", ")))
	}
	for _, row := range report.Rows {
		if row.Label == "Effective" && report.Kind == KindNOTAM {
			continue
		}
		writeTableRow(&b, decorators.TableCell(row.Label), rowCell(links, row))
	}
	for _, period := range report.Periods {
		writeTableRow(&b, decorators.TableCell(period.Period), joinedRows(period.Rows))
	}
	if len(report.Remarks) > 0 {
		writeTableRow(&b, "Remarks", joinedRows(report.Remarks))
	}
	if len(report.Unknown) > 0 {
		writeTableRow(&b, "Not decoded", decorators.TableCell(strings.Join(report.Unknown, " ")))
	}
	b.WriteString(decorators.TableDetailsRow(href))

	return b.String()
}

func tableHeadingDetail(links *decorators.Tagger, report Report) string {
	switch {
	case report.Station != "" && report.StationName != "":
		label := decorators.TableCell(report.Station + " - " + report.StationName)
		return "[" + label + "](" + links.URLFor(airfieldPath, url.Values{"v": {report.Station}}) + ")"
	case report.Station != "":
		return decorators.TableCell(report.Station)
	}
	return tableFallbackHeading
}

func rowCell(links *decorators.Tagger, row Row) string {
	if row.At.IsZero() {
		return decorators.TableCell(row.Value)
	}
	return timeCell(links, row.At) + decorators.TableCell(strings.TrimPrefix(row.Value, zuluText(row.At)))
}

func timeCell(links *decorators.Tagger, at time.Time) string {
	label := decorators.TableCell(zuluText(at))
	params, ok := (&dtg.Decorator{}).Parse(at.UTC().Format(timestampLayout), at)
	if !ok {
		return label
	}
	return "[" + label + "](" + links.URLFor(dtg.Type, params) + ")"
}

func joinedRows(rows []Row) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, decorators.TableCell(row.Label+": "+row.Value))
	}
	return strings.Join(parts, "; ")
}

func writeTableRow(b *strings.Builder, label, value string) {
	b.WriteString("| " + label + " | " + value + " |\n")
}

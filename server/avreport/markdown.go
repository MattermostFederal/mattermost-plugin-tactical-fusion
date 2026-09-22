package avreport

import (
	"net/url"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

const (
	reportRowLabel       = "Report"
	summaryRowLabel      = "Summary"
	tableFallbackHeading = "Aviation report"

	inferredDateNote = " (month and year taken from the post date)"
)

func issuedLabel(kind string) string {
	if kind == KindNOTAM {
		return "Effective"
	}
	return "Issued"
}

func reportTable(href, trail string, report Report, withReportRow bool) string {
	var b strings.Builder

	b.WriteString("| " + decorators.TableCell(report.Kind) + " | " + tableHeadingDetail(href, report) + " |\n")
	b.WriteString("|:--|:--|\n")

	if withReportRow {
		writeTableRow(&b, reportRowLabel, reportCell(report.Raw)+decorators.TableCell(trail))
	}
	if report.Summary != "" {
		writeTableRow(&b, summaryRowLabel, decorators.TableCell(report.Summary))
	}
	if issued := zuluText(report.IssuedAt); issued != "" {
		if report.Inferred {
			issued += inferredDateNote
		}
		writeTableRow(&b, issuedLabel(report.Kind), decorators.TableCell(issued))
	}
	if len(report.Flags) > 0 {
		writeTableRow(&b, "Flags", decorators.TableCell(strings.Join(report.Flags, ", ")))
	}
	for _, row := range report.Rows {
		if row.Label == "Effective" && report.Kind == KindNOTAM {
			continue
		}
		writeTableRow(&b, decorators.TableCell(row.Label), decorators.TableCell(row.Value))
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

func tableHeadingDetail(href string, report Report) string {
	switch {
	case report.Station != "" && report.StationName != "":
		label := decorators.TableCell(report.Station + " - " + report.StationName)
		return "[" + label + "](" + airfieldHref(href, report.Station) + ")"
	case report.Station != "":
		return decorators.TableCell(report.Station)
	}
	return tableFallbackHeading
}

func airfieldHref(reportHref, station string) string {
	prefix := reportHref[:strings.LastIndex(reportHref, "/"+Type+"?")]
	return prefix + "/" + airfieldPath + "?" + url.Values{"v": {station}}.Encode()
}

func reportCell(raw string) string {
	delimiter := "`"
	for strings.Contains(raw, delimiter) {
		delimiter += "`"
	}

	body := strings.ReplaceAll(raw, "|", `\|`)
	if strings.HasPrefix(raw, "`") || strings.HasSuffix(raw, "`") {
		body = " " + body + " "
	}

	return delimiter + body + delimiter
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

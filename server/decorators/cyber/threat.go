package cyber

import (
	"errors"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Report struct {
	Source    string
	Malicious bool
	Threat    string
	Detail    string
	URL       string
}

func seenText(first, last string) string {
	switch {
	case first != "" && last != "" && first != last:
		return "seen " + first + " to " + last
	case last != "":
		return "last seen " + last
	case first != "":
		return "first seen " + first
	}
	return ""
}

func reportDetail(report intel.ThreatReport) string {
	parts := []string{report.Malware}
	if report.File != "" {
		parts = append(parts, "file "+report.File)
	}
	if report.Ports != "" {
		label := "port "
		if strings.Contains(report.Ports, ",") {
			label = "ports "
		}
		parts = append(parts, label+strings.ReplaceAll(report.Ports, ",", ", "))
	}
	if report.Confidence != "" {
		parts = append(parts, "confidence "+report.Confidence+"%")
	}
	parts = append(parts, seenText(report.FirstSeen, report.LastSeen), report.Status)
	return joinNonEmpty(glanceSeparator, parts...)
}

func describeThreatReports(d *Details, set *intel.Set) {
	found, err := set.ThreatReports(d.Value)
	switch {
	case errors.Is(err, intel.ErrNoDataset), errors.Is(err, intel.ErrNotFound):
		return
	case err != nil:
		addRow(d, "Threat reports", datasetSentence(set, intel.NameAdvisory, err))
		return
	}

	malicious := 0
	for _, report := range found {
		isMalicious := report.Category == intel.CategoryMalicious
		if isMalicious {
			malicious++
		}
		d.Reports = append(d.Reports, Report{
			Source:    report.Source,
			Malicious: isMalicious,
			Threat:    report.Threat,
			Detail:    reportDetail(report),
			URL:       webURL(report.URL),
		})
	}

	if malicious > 0 {
		d.Headline = joinSentence(d.Headline, "reported malicious")
	}
}

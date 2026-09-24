package intel

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

const (
	CategoryMalicious = "malicious"
	CategoryContext   = "context"
)

type ThreatReport struct {
	Source     string `json:"source"`
	Category   string `json:"category"`
	Threat     string `json:"threat"`
	Malware    string `json:"malware"`
	Confidence string `json:"confidence"`
	Ports      string `json:"ports"`
	Status     string `json:"status"`
	File       string `json:"file"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
	URL        string `json:"url"`
}

func IndicatorKey(value string) string {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap().String()
	}
	return strings.ToLower(value)
}

var reportDatasets = []string{NameAdvisory}

func (s *Set) ThreatReports(value string) ([]ThreatReport, error) {
	key := IndicatorKey(value)

	var reports []ThreatReport
	installed := false
	var firstErr error

	for _, name := range reportDatasets {
		row, err := s.lookup(name, key)
		switch {
		case errors.Is(err, ErrNoDataset):
			continue
		case errors.Is(err, ErrNotFound):
			installed = true
			continue
		case err != nil:
			installed = true
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		installed = true

		var found []ThreatReport
		if err := decodeJSONFields(row[0], []jsonField{{"threat reports", row[1], &found}}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		reports = append(reports, found...)
	}

	switch {
	case len(reports) > 0:
		return reports, nil
	case firstErr != nil:
		return nil, fmt.Errorf("reading threat reports: %w", firstErr)
	case !installed:
		return nil, ErrNoDataset
	}
	return nil, ErrNotFound
}

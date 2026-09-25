package intel

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
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

var reportDatasets = []string{NameAdvisory, NameHashLists}

const netListFields = 3

func (s *Set) reportsField(name, key string) (string, error) {
	if name != NameNetLists {
		row, err := s.lookup(name, key)
		if err != nil {
			return "", err
		}
		return row[1], nil
	}

	addr, err := netip.ParseAddr(key)
	if err != nil {
		return "", ErrNotFound
	}
	dataset, ok := s.datasets[NameNetLists]
	if !ok {
		return "", ErrNoDataset
	}
	rangeKey := IPKey(addr)
	row, err := dataset.file.LookupRange(rangeKey)
	switch {
	case err != nil:
		return "", err
	case len(row) != netListFields || row[0] > rangeKey || rangeKey > row[1]:
		return "", ErrNotFound
	}
	return row[2], nil
}

func (s *Set) ThreatReports(value string) ([]ThreatReport, error) {
	key := IndicatorKey(value)
	if s == nil {
		return nil, ErrNoDataset
	}

	var reports []ThreatReport
	installed := false
	var firstErr error

	for _, name := range append(slices.Clone(reportDatasets), NameNetLists) {
		field, err := s.reportsField(name, key)
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
		if err := decodeJSONFields(key, []jsonField{{"threat reports", field, &found}}); err != nil {
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

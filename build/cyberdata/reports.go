package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	categoryMalicious = "malicious"
	categoryContext   = "context"

	torExitList = "tor-exits.txt"
	torSource   = "Tor Project"

	torMetrics     = "https://metrics.torproject.org/rs.html#search/"
	cisaAdvisories = "https://www.cisa.gov/news-events/cybersecurity-advisories/"
)

type threatReport struct {
	Source     string `json:"source"`
	Category   string `json:"category"`
	Threat     string `json:"threat"`
	Malware    string `json:"malware,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Ports      string `json:"ports,omitempty"`
	Status     string `json:"status,omitempty"`
	FirstSeen  string `json:"firstSeen,omitempty"`
	LastSeen   string `json:"lastSeen,omitempty"`
	URL        string `json:"url,omitempty"`
}

var hexDigest = regexp.MustCompile(`^(?:[0-9a-f]{32}|[0-9a-f]{40}|[0-9a-f]{64})$`)

func indicatorKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap().String(), true
	}
	lowered := strings.ToLower(value)
	if hexDigest.MatchString(lowered) {
		return lowered, true
	}
	return "", false
}

func day(timestamp string) string {
	timestamp = clean(timestamp)
	if len(timestamp) >= len("2006-01-02") {
		return timestamp[:len("2006-01-02")]
	}
	return timestamp
}

type reportIndex struct {
	byKey map[string][]threatReport
}

func (r *reportIndex) add(key string, report threatReport) {
	if r.byKey == nil {
		r.byKey = map[string][]threatReport{}
	}
	reports := r.byKey[key]
	for i := range reports {
		existing := &reports[i]
		if existing.Source != report.Source || existing.Threat != report.Threat || existing.Malware != report.Malware {
			continue
		}
		existing.Ports = mergeList(existing.Ports, report.Ports)
		existing.FirstSeen = earliest(existing.FirstSeen, report.FirstSeen)
		existing.LastSeen = latest(existing.LastSeen, report.LastSeen)
		existing.Confidence = highest(existing.Confidence, report.Confidence)
		if report.Status != "" {
			existing.Status = report.Status
		}
		return
	}
	r.byKey[key] = append(reports, report)
}

func (r *reportIndex) rows() ([][]string, error) {
	rows := make([][]string, 0, len(r.byKey))
	for key, reports := range r.byKey {
		slices.SortStableFunc(reports, func(a, b threatReport) int {
			if c := strings.Compare(a.Source, b.Source); c != 0 {
				return c
			}
			return strings.Compare(b.LastSeen, a.LastSeen)
		})
		field, err := compactJSON(reports)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []string{key, field})
	}
	return rows, nil
}

func mergeList(a, b string) string {
	var values []string
	for _, list := range []string{a, b} {
		for value := range strings.SplitSeq(list, ",") {
			if value = strings.TrimSpace(value); value != "" && !slices.Contains(values, value) {
				values = append(values, value)
			}
		}
	}
	slices.SortFunc(values, func(x, y string) int {
		xi, xerr := strconv.Atoi(x)
		yi, yerr := strconv.Atoi(y)
		if xerr == nil && yerr == nil {
			return xi - yi
		}
		return strings.Compare(x, y)
	})
	return strings.Join(values, ",")
}

func earliest(a, b string) string {
	if a == "" || (b != "" && b < a) {
		return b
	}
	return a
}

func latest(a, b string) string {
	if b > a {
		return b
	}
	return a
}

func highest(a, b string) string {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	switch {
	case aerr != nil:
		return b
	case berr != nil:
		return a
	case bi > ai:
		return b
	}
	return a
}

func loadTorExits(path string, index *reportIndex) error {
	handle, err := os.Open(path) // #nosec G304 -- a feed file under the directory the operator names with -source
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()

	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		key, ok := indicatorKey(scanner.Text())
		if !ok {
			continue
		}
		index.add(key, threatReport{
			Source:   torSource,
			Category: categoryContext,
			Threat:   "Tor exit node",
			URL:      torMetrics + key,
		})
	}
	return scanner.Err()
}

type stixAdvisory struct {
	Objects []struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		Pattern   string `json:"pattern"`
		ValidFrom string `json:"valid_from"`
		Published string `json:"published"`
	} `json:"objects"`
}

var (
	advisoryID    = regexp.MustCompile(`(?i)\b(AA\d{2}-\d{3}[A-Z])\b`)
	stixIPv4      = regexp.MustCompile(`ipv4-addr:value\s*=\s*'([^']+)'`)
	stixIPv6      = regexp.MustCompile(`ipv6-addr:value\s*=\s*'([^']+)'`)
	stixFileHash  = regexp.MustCompile(`file:hashes\.'?(?:MD5|SHA-1|SHA-256)'?\s*=\s*'([^']+)'`)
	advisoryTitle = regexp.MustCompile(`^(?i:AA\d{2}-\d{3}[A-Z])\s*`)
)

func publishedText(published string) string {
	if published == "" {
		return ""
	}
	return "Advisory published " + published
}

func buildAdvisory(dir string) ([][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var index reportIndex
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if err := loadAdvisory(filepath.Join(dir, entry.Name()), &index); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
	}
	if len(index.byKey) == 0 {
		return nil, fmt.Errorf("no advisory indicators in %s; run 'make cyber-advisories' to fetch them", dir)
	}

	return index.rows()
}

func loadAdvisory(path string, index *reportIndex) error {
	raw, err := os.ReadFile(path) // #nosec G304 -- an advisory file under the directory the operator names with -source
	if err != nil {
		return err
	}

	var bundle stixAdvisory
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return err
	}

	id, title, published := "", "", ""
	for _, o := range bundle.Objects {
		if o.Type == "report" {
			if match := advisoryID.FindStringSubmatch(o.Name); match != nil {
				id = strings.ToUpper(match[1])
			}
			title = clean(advisoryTitle.ReplaceAllString(o.Name, ""))
			published = day(o.Published)
		}
	}
	if id == "" {
		return fmt.Errorf("names no advisory id in its report object")
	}

	for _, o := range bundle.Objects {
		if o.Type != "indicator" {
			continue
		}
		for _, pattern := range []*regexp.Regexp{stixIPv4, stixIPv6, stixFileHash} {
			for _, match := range pattern.FindAllStringSubmatch(o.Pattern, -1) {
				key, ok := indicatorKey(match[1])
				if !ok {
					continue
				}
				index.add(key, threatReport{
					Source:    "CISA " + id,
					Category:  categoryMalicious,
					Threat:    title,
					Status:    publishedText(published),
					FirstSeen: day(o.ValidFrom),
					URL:       cisaAdvisories + strings.ToLower(id),
				})
			}
		}
	}
	return nil
}

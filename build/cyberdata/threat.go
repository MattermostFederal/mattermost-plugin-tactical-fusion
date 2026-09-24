package main

import (
	"archive/zip"
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
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

	threatFoxExport = "threatfox-full.zip"
	feodoBlocklist  = "feodo-ipblocklist.csv"
	torExitList     = "tor-exits.txt"

	threatFoxSource = "abuse.ch ThreatFox"
	feodoSource     = "abuse.ch Feodo Tracker"
	torSource       = "Tor Project"

	threatFoxSearch = "https://threatfox.abuse.ch/browse.php?search=ioc%3A"
	feodoBrowse     = "https://feodotracker.abuse.ch/browse/host/"
	torMetrics      = "https://metrics.torproject.org/rs.html#search/"
	cisaAdvisories  = "https://www.cisa.gov/news-events/cybersecurity-advisories/"
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

var threatNames = map[string]string{
	"botnet_cc":        "Botnet C2",
	"payload_delivery": "Payload delivery",
	"payload":          "Malware payload",
	"cc_skimming":      "Card skimming",
}

func threatName(raw string) string {
	if name, ok := threatNames[raw]; ok {
		return name
	}
	return clean(strings.ReplaceAll(raw, "_", " "))
}

func day(timestamp string) string {
	timestamp = clean(timestamp)
	if len(timestamp) >= len("2006-01-02") {
		return timestamp[:len("2006-01-02")]
	}
	return timestamp
}

var saysNothing = []string{"none", "unknown malware"}

func noneAsEmpty(value string) string {
	value = clean(value)
	if slices.ContainsFunc(saysNothing, func(empty string) bool { return strings.EqualFold(value, empty) }) {
		return ""
	}
	return value
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

func buildThreat(dir string) ([][]string, error) {
	var index reportIndex
	read := 0

	for _, feed := range []struct {
		file string
		load func(string, *reportIndex) error
	}{
		{threatFoxExport, loadThreatFox},
		{feodoBlocklist, loadFeodo},
		{torExitList, loadTorExits},
	} {
		path := filepath.Join(dir, feed.file)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := feed.load(path, &index); err != nil {
			return nil, fmt.Errorf("%s: %w", feed.file, err)
		}
		read++
	}

	if read == 0 {
		return nil, fmt.Errorf("no feed in %s; run 'make cyber-threat' to fetch them", dir)
	}

	return index.rows()
}

func csvBody(r io.Reader) *csv.Reader {
	reader := csv.NewReader(commentStripper(r))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true
	return reader
}

func commentStripper(r io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "#") {
				continue
			}
			if _, err := io.WriteString(pw, line+"\n"); err != nil {
				return
			}
		}
		_ = pw.CloseWithError(scanner.Err())
	}()
	return pr
}

func loadThreatFox(path string, index *reportIndex) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()

	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".csv") {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return err
		}
		err = readThreatFox(handle, index)
		_ = handle.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func readThreatFox(r io.Reader, index *reportIndex) error {
	reader := csvBody(r)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(record) < 12 {
			continue
		}

		value, port := record[2], ""
		if record[3] == "ip:port" {
			host, p, err := net.SplitHostPort(record[2])
			if err != nil {
				continue
			}
			value, port = host, p
		}
		key, ok := indicatorKey(value)
		if !ok {
			continue
		}

		index.add(key, threatReport{
			Source:     threatFoxSource,
			Category:   categoryMalicious,
			Threat:     threatName(record[4]),
			Malware:    noneAsEmpty(record[7]),
			Confidence: clean(record[9]),
			Ports:      port,
			FirstSeen:  day(record[0]),
			LastSeen:   day(record[8]),
			URL:        threatFoxSearch + url.QueryEscape(key),
		})
	}
}

func loadFeodo(path string, index *reportIndex) error {
	handle, err := os.Open(path) // #nosec G304 -- a feed file under the directory the operator names with -source
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()

	reader := csvBody(handle)
	header := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header {
			header = false
			continue
		}
		if len(record) < 6 {
			continue
		}
		key, ok := indicatorKey(record[1])
		if !ok {
			continue
		}
		index.add(key, threatReport{
			Source:    feodoSource,
			Category:  categoryMalicious,
			Threat:    "Botnet C2",
			Malware:   clean(record[5]),
			Ports:     clean(record[2]),
			Status:    clean(record[3]),
			FirstSeen: day(record[0]),
			LastSeen:  day(record[4]),
			URL:       feodoBrowse + key + "/",
		})
	}
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

const (
	malwareBazaarExport = "malwarebazaar-full.zip"
	notAvailable        = "n/a"
)

func knownOrEmpty(value string) string {
	value = clean(value)
	if strings.EqualFold(value, notAvailable) {
		return ""
	}
	return value
}

func buildMalware(dir string) ([][]string, error) {
	path := filepath.Join(dir, malwareBazaarExport)
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("%w; run 'make cyber-threat' to fetch it", err)
	}
	defer func() { _ = archive.Close() }()

	var rows [][]string
	seen := map[string]bool{}

	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".csv") {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		rows, err = readMalwareBazaar(handle, rows, seen)
		_ = handle.Close()
		if err != nil {
			return nil, err
		}
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("%s holds no samples", malwareBazaarExport)
	}
	return rows, nil
}

func readMalwareBazaar(r io.Reader, rows [][]string, seen map[string]bool) ([][]string, error) {
	reader := csvBody(r)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 9 {
			continue
		}

		sha256, ok := indicatorKey(record[1])
		if !ok || len(sha256) != 64 || seen[sha256] {
			continue
		}
		seen[sha256] = true

		name := knownOrEmpty(record[5])
		if strings.EqualFold(name, sha256) {
			name = ""
		}
		rows = append(rows, []string{sha256, "", day(record[0]), name, knownOrEmpty(record[6]), knownOrEmpty(record[8])})

		for _, alias := range []string{record[2], record[3]} {
			if key, ok := indicatorKey(alias); ok && !seen[key] {
				seen[key] = true
				rows = append(rows, []string{key, sha256, "", "", "", ""})
			}
		}
	}
}

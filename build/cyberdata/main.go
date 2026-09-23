package main

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	schemaPrefix  = "#tactical-fusion-cyber/"
	schemaVersion = 1

	summaryRunes = 300
)

var (
	sourceDir = flag.String("source", filepath.Join("build", "cyberdata", "source"), "where the fetched upstream files are")
	outDir    = flag.String("out", filepath.Join("build", "cyberdata", "out"), "where the release-asset datasets are written")
	treeDir   = flag.String("tree", ".", "the repository root, for the committed outputs")
	only      = flag.String("only", "", "build one dataset by name rather than all of them")
	label     = flag.String("label", "", "what the stamp names as the source, in place of the source file")
)

type builder struct {
	name   string
	source string
	build  func(source string) ([][]string, error)
	target func() string
	stamp  bool
}

func main() {
	flag.Parse()

	if strings.ContainsAny(*label, "\t\r\n") {
		fmt.Fprintln(os.Stderr, "-label may not contain a tab or a line break: the stamp is one tab-separated line")
		os.Exit(2)
	}

	builders := []builder{
		{
			name:   "attack",
			source: "enterprise-attack.json",
			build:  buildAttack,
			target: func() string { return filepath.Join(*treeDir, "server", "decorators", "cyber", "data", "attack.tsv") },
		},
		{
			name:   "cwe",
			source: "cwe-1000.csv",
			build:  buildCWE,
			target: func() string { return filepath.Join(*treeDir, "server", "decorators", "cyber", "data", "cwe.tsv") },
		},
		{
			name:   "kev",
			source: "known_exploited_vulnerabilities.json",
			build:  buildKEV,
			target: func() string { return filepath.Join(*treeDir, "assets", "cyber", "kev.tsv") },
			stamp:  true,
		},
		{
			name:   "cve",
			source: "nvd",
			build:  buildCVE,
			target: func() string { return filepath.Join(*outDir, "cve.tsv") },
			stamp:  true,
		},
		{
			name:   "epss",
			source: "epss_scores-current.csv",
			build:  buildEPSS,
			target: func() string { return filepath.Join(*outDir, "epss.tsv") },
			stamp:  true,
		},
		{
			name:   "ip",
			source: "ip2asn-combined.tsv",
			build:  buildIP,
			target: func() string { return filepath.Join(*outDir, "ip.tsv") },
			stamp:  true,
		},
	}

	for _, b := range builders {
		if *only != "" && *only != b.name {
			continue
		}

		if err := run(b); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", b.name, err)
			os.Exit(1)
		}
	}
}

func run(b builder) error {
	source := filepath.Join(*sourceDir, b.source)
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("%w; run 'make cyber-sources' first", err)
	}

	rows, err := b.build(source)
	if err != nil {
		return err
	}

	if err := checkRows(rows); err != nil {
		return err
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	target := b.target()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	stampSource := b.source
	if *label != "" {
		stampSource = *label
	}

	var b2 strings.Builder
	if b.stamp {
		fmt.Fprintf(&b2, "%s%d\t%s\t%s\t%s\n",
			schemaPrefix, schemaVersion, b.name, time.Now().UTC().Format(time.RFC3339), stampSource)
	} else {
		b2.WriteString(strings.Join(headerFor(b.name), "\t") + "\n")
	}
	for _, row := range rows {
		b2.WriteString(strings.Join(row, "\t") + "\n")
	}

	if err := os.WriteFile(target, []byte(b2.String()), 0o644); err != nil {
		return err
	}

	fmt.Printf("wrote %d %s rows to %s\n", len(rows), b.name, target)

	return nil
}

var headers = map[string][]string{
	"attack": {"id", "name", "kind", "tactics", "parent", "platforms", "summary", "status"},
	"cwe":    {"id", "name", "abstraction", "status", "summary", "parents"},
}

func headerFor(name string) []string { return headers[name] }

func checkRows(rows [][]string) error {
	if len(rows) == 0 {
		return fmt.Errorf("produced no rows")
	}

	width := len(rows[0])
	seen := map[string]bool{}

	for _, row := range rows {
		if len(row) != width {
			return fmt.Errorf("a row has %d fields and the first has %d", len(row), width)
		}
		for _, field := range row {
			if strings.ContainsAny(field, "\t\n\r") {
				return fmt.Errorf("a field carries a separator: %q", field)
			}
		}
		if seen[row[0]] {
			return fmt.Errorf("duplicate key %q", row[0])
		}
		seen[row[0]] = true
	}

	return nil
}

func clean(text string) string {
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.Join(strings.Fields(text), " ")

	return text
}

func firstSentence(text string) string {
	text = clean(text)

	if end := strings.Index(text, ". "); end >= 0 {
		text = text[:end+1]
	}

	runes := []rune(text)
	if len(runes) <= summaryRunes {
		return text
	}

	cut := string(runes[:summaryRunes])
	if space := strings.LastIndexByte(cut, ' '); space > 0 {
		cut = cut[:space]
	}

	return cut + "..."
}

type stixBundle struct {
	Objects []stixObject `json:"objects"`
}

type stixObject struct {
	Type               string          `json:"type"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Revoked            bool            `json:"revoked"`
	Deprecated         bool            `json:"x_mitre_deprecated"`
	IsSubtechnique     bool            `json:"x_mitre_is_subtechnique"`
	Platforms          []string        `json:"x_mitre_platforms"`
	ShortName          string          `json:"x_mitre_shortname"`
	KillChainPhases    []stixKillChain `json:"kill_chain_phases"`
	ExternalReferences []stixReference `json:"external_references"`
}

type stixKillChain struct {
	KillChainName string `json:"kill_chain_name"`
	PhaseName     string `json:"phase_name"`
}

type stixReference struct {
	SourceName string `json:"source_name"`
	ExternalID string `json:"external_id"`
}

func attackID(o stixObject) string {
	for _, ref := range o.ExternalReferences {
		if ref.SourceName == "mitre-attack" {
			return ref.ExternalID
		}
	}

	return ""
}

func buildAttack(source string) ([][]string, error) {
	raw, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}

	var bundle stixBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, err
	}

	tacticByShortName := map[string]string{}
	for _, o := range bundle.Objects {
		if o.Type == "x-mitre-tactic" && !o.Revoked && !o.Deprecated {
			tacticByShortName[o.ShortName] = attackID(o)
		}
	}

	var rows [][]string

	for _, o := range bundle.Objects {
		if o.Revoked || o.Deprecated {
			continue
		}

		id := attackID(o)
		if id == "" {
			continue
		}

		switch o.Type {
		case "x-mitre-tactic":
			rows = append(rows, []string{
				id, clean(o.Name), "tactic", "", "", "", firstSentence(o.Description), "active",
			})

		case "attack-pattern":
			var tactics []string
			for _, phase := range o.KillChainPhases {
				if phase.KillChainName != "mitre-attack" {
					continue
				}
				if tactic, ok := tacticByShortName[phase.PhaseName]; ok {
					tactics = append(tactics, tactic)
				}
			}

			kind, parent := "technique", ""
			if o.IsSubtechnique {
				kind = "subtechnique"
				parent, _, _ = strings.Cut(id, ".")
			}

			rows = append(rows, []string{
				id, clean(o.Name), kind, strings.Join(tactics, ","), parent,
				strings.Join(o.Platforms, ","), firstSentence(o.Description), "active",
			})
		}
	}

	return rows, nil
}

func buildCWE(source string) ([][]string, error) {
	handle, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	reader := csv.NewReader(handle)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("the CWE export is empty")
	}

	index := map[string]int{}
	for i, name := range records[0] {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}

	field := func(record []string, name string) string {
		i, ok := index[name]
		if !ok || i >= len(record) {
			return ""
		}
		return clean(record[i])
	}

	var rows [][]string
	for _, record := range records[1:] {
		id := field(record, "cwe-id")
		if id == "" {
			continue
		}

		rows = append(rows, []string{
			"CWE-" + id,
			field(record, "name"),
			field(record, "weakness abstraction"),
			field(record, "status"),
			firstSentence(field(record, "description")),
			parentsOf(field(record, "related weaknesses")),
		})
	}

	return rows, nil
}

func parentsOf(related string) string {
	var parents []string

	for _, part := range strings.Split(related, "::") {
		if !strings.HasPrefix(part, "NATURE:ChildOf:") {
			continue
		}

		fields := strings.Split(part, ":")
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "CWE ID" {
				parents = append(parents, "CWE-"+fields[i+1])
				break
			}
		}
	}

	return strings.Join(parents, ",")
}

type kevCatalog struct {
	Vulnerabilities []kevEntry `json:"vulnerabilities"`
}

type kevEntry struct {
	CveID             string `json:"cveID"`
	VendorProject     string `json:"vendorProject"`
	Product           string `json:"product"`
	DateAdded         string `json:"dateAdded"`
	DueDate           string `json:"dueDate"`
	RequiredAction    string `json:"requiredAction"`
	RansomwareUse     string `json:"knownRansomwareCampaignUse"`
	ShortDescription  string `json:"shortDescription"`
	VulnerabilityName string `json:"vulnerabilityName"`
}

func buildKEV(source string) ([][]string, error) {
	raw, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}

	var catalog kevCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, err
	}

	var rows [][]string
	for _, entry := range catalog.Vulnerabilities {
		if entry.CveID == "" {
			continue
		}

		product := clean(strings.TrimSpace(entry.VendorProject + " " + entry.Product))

		rows = append(rows, []string{
			strings.ToUpper(entry.CveID),
			clean(entry.DateAdded),
			clean(entry.DueDate),
			clean(entry.RansomwareUse),
			product,
			firstSentence(entry.RequiredAction),
		})
	}

	return rows, nil
}

type nvdFeed struct {
	Vulnerabilities []struct {
		CVE struct {
			ID           string `json:"id"`
			Published    string `json:"published"`
			LastModified string `json:"lastModified"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics struct {
				V31 []nvdMetric `json:"cvssMetricV31"`
				V30 []nvdMetric `json:"cvssMetricV30"`
				V40 []nvdMetric `json:"cvssMetricV40"`
				V2  []nvdMetric `json:"cvssMetricV2"`
			} `json:"metrics"`
			Weaknesses []struct {
				Description []struct {
					Lang  string `json:"lang"`
					Value string `json:"value"`
				} `json:"description"`
			} `json:"weaknesses"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdMetric struct {
	CvssData struct {
		BaseScore    float64 `json:"baseScore"`
		BaseSeverity string  `json:"baseSeverity"`
		VectorString string  `json:"vectorString"`
	} `json:"cvssData"`
	BaseSeverity string `json:"baseSeverity"`
}

func buildCVE(source string) ([][]string, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}

	var rows [][]string
	seen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			return nil, err
		}

		var feed nvdFeed
		if err := json.Unmarshal(raw, &feed); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}

		for _, item := range feed.Vulnerabilities {
			cve := item.CVE
			if cve.ID == "" || seen[cve.ID] {
				continue
			}
			seen[cve.ID] = true

			summary := ""
			for _, description := range cve.Descriptions {
				if description.Lang == "en" {
					summary = clean(description.Value)
					break
				}
			}

			score, severity, vector := "", "", ""
			for _, metrics := range [][]nvdMetric{cve.Metrics.V31, cve.Metrics.V30, cve.Metrics.V40, cve.Metrics.V2} {
				if len(metrics) == 0 {
					continue
				}
				metric := metrics[0]
				score = strconv.FormatFloat(metric.CvssData.BaseScore, 'f', 1, 64)
				severity = metric.CvssData.BaseSeverity
				if severity == "" {
					severity = metric.BaseSeverity
				}
				severity = title(strings.ToLower(severity))
				vector = metric.CvssData.VectorString
				break
			}

			var weaknesses []string
			for _, weakness := range cve.Weaknesses {
				for _, description := range weakness.Description {
					if strings.HasPrefix(description.Value, "CWE-") {
						weaknesses = appendOnce(weaknesses, description.Value)
					}
				}
			}

			rows = append(rows, []string{
				cve.ID,
				clean(cve.Published),
				clean(cve.LastModified),
				score,
				severity,
				clean(vector),
				strings.Join(weaknesses, ","),
				summary,
			})
		}
	}

	return rows, nil
}

func title(text string) string {
	if text == "" {
		return ""
	}

	runes := []rune(text)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 32
	}

	return string(runes)
}

func appendOnce(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func buildEPSS(source string) ([][]string, error) {
	handle, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	reader := csv.NewReader(handle)
	reader.FieldsPerRecord = -1
	reader.Comment = '#'

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("the EPSS export is empty")
	}

	modelDate := time.Now().UTC().Format("2006-01-02")

	var rows [][]string
	for _, record := range records[1:] {
		if len(record) < 3 || !strings.HasPrefix(record[0], "CVE-") {
			continue
		}

		rows = append(rows, []string{
			strings.ToUpper(clean(record[0])),
			clean(record[1]),
			clean(record[2]),
			modelDate,
		})
	}

	return rows, nil
}

func buildIP(source string) ([][]string, error) {
	raw, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}

	var rows [][]string

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}

		start, err := netip.ParseAddr(fields[0])
		if err != nil {
			continue
		}
		end, err := netip.ParseAddr(fields[1])
		if err != nil {
			continue
		}

		asn := clean(fields[2])
		if asn == "0" {
			continue
		}

		country := clean(fields[3])
		name := clean(fields[4])
		if name == "Not routed" {
			continue
		}

		rows = append(rows, []string{
			ipKey(start), ipKey(end), "AS" + asn, name, country, "", "",
		})
	}

	return rows, nil
}

func ipKey(addr netip.Addr) string {
	as16 := addr.Unmap().As16()
	return hex.EncodeToString(as16[:])
}

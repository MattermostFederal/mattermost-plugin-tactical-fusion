package main

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
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
	only      = flag.String("only", "", "build only the named datasets, comma separated, rather than all of them")
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
			name:   "cvedetail",
			source: "nvd",
			build:  buildCVEDetail,
			target: func() string { return filepath.Join(*outDir, "cvedetail.tsv") },
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
		if *only != "" && !slices.Contains(strings.Split(*only, ","), b.name) {
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
				if parent := "CWE-" + fields[i+1]; !slices.Contains(parents, parent) {
					parents = append(parents, parent)
				}
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

type nvdText struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdCPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

type nvdAffected struct {
	Vendor        string `json:"vendor"`
	Product       string `json:"product"`
	DefaultStatus string `json:"defaultStatus"`
	Versions      []struct {
		Version         string `json:"version"`
		Status          string `json:"status"`
		LessThan        string `json:"lessThan"`
		LessThanOrEqual string `json:"lessThanOrEqual"`
		Changes         []struct {
			At     string `json:"at"`
			Status string `json:"status"`
		} `json:"changes"`
	} `json:"versions"`
}

type nvdCVE struct {
	ID           string    `json:"id"`
	Published    string    `json:"published"`
	LastModified string    `json:"lastModified"`
	Descriptions []nvdText `json:"descriptions"`
	Metrics      struct {
		V31 []nvdMetric `json:"cvssMetricV31"`
		V30 []nvdMetric `json:"cvssMetricV30"`
		V40 []nvdMetric `json:"cvssMetricV40"`
		V2  []nvdMetric `json:"cvssMetricV2"`
	} `json:"metrics"`
	Weaknesses []struct {
		Source      string    `json:"source"`
		Description []nvdText `json:"description"`
	} `json:"weaknesses"`
	Configurations []struct {
		Nodes []struct {
			CPEMatch []nvdCPEMatch `json:"cpeMatch"`
		} `json:"nodes"`
	} `json:"configurations"`
	Affected []struct {
		AffectedData []nvdAffected `json:"affectedData"`
	} `json:"affected"`
	References []struct {
		URL  string   `json:"url"`
		Tags []string `json:"tags"`
	} `json:"references"`
}

type nvdFeed struct {
	Vulnerabilities []struct {
		CVE nvdCVE `json:"cve"`
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

func eachCVE(source string, visit func(cve nvdCVE) error) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	seen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			return err
		}

		var feed nvdFeed
		if err := json.Unmarshal(raw, &feed); err != nil {
			return fmt.Errorf("%s: %w", entry.Name(), err)
		}

		for _, item := range feed.Vulnerabilities {
			if item.CVE.ID == "" || seen[item.CVE.ID] {
				continue
			}
			seen[item.CVE.ID] = true

			if err := visit(item.CVE); err != nil {
				return fmt.Errorf("%s: %w", item.CVE.ID, err)
			}
		}
	}

	return nil
}

func buildCVE(source string) ([][]string, error) {
	var rows [][]string

	err := eachCVE(source, func(cve nvdCVE) error {
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

		return nil
	})

	return rows, err
}

type weaknessSource struct {
	Source string   `json:"source"`
	CWE    []string `json:"cwe"`
}

type cpeMatch struct {
	CPE     string `json:"cpe"`
	From    string `json:"from,omitempty"`
	After   string `json:"after,omitempty"`
	Through string `json:"through,omitempty"`
	Before  string `json:"before,omitempty"`
}

type configuration struct {
	Vulnerable []cpeMatch `json:"vulnerable"`
	On         []cpeMatch `json:"on,omitempty"`
}

type versionChange struct {
	At     string `json:"at"`
	Status string `json:"status"`
}

type affectedVersion struct {
	Version         string          `json:"version,omitempty"`
	Status          string          `json:"status,omitempty"`
	LessThan        string          `json:"lessThan,omitempty"`
	LessThanOrEqual string          `json:"lessThanOrEqual,omitempty"`
	Changes         []versionChange `json:"changes,omitempty"`
}

type affectedProduct struct {
	Vendor        string            `json:"vendor,omitempty"`
	Product       string            `json:"product,omitempty"`
	DefaultStatus string            `json:"defaultStatus,omitempty"`
	Versions      []affectedVersion `json:"versions,omitempty"`
}

type reference struct {
	URL  string   `json:"url"`
	Tags []string `json:"tags,omitempty"`
}

func buildCVEDetail(source string) ([][]string, error) {
	var rows [][]string

	err := eachCVE(source, func(cve nvdCVE) error {
		weaknesses, err := compactJSON(weaknessSources(cve))
		if err != nil {
			return err
		}
		configured, err := compactJSON(configurations(cve))
		if err != nil {
			return err
		}
		affected, err := compactJSON(affectedProducts(cve))
		if err != nil {
			return err
		}
		refs, err := compactJSON(references(cve))
		if err != nil {
			return err
		}

		rows = append(rows, []string{cve.ID, weaknesses, configured, affected, refs})
		return nil
	})

	return rows, err
}

func compactJSON[T any](values []T) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(values); err != nil {
		return "", err
	}

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func weaknessSources(cve nvdCVE) []weaknessSource {
	var sources []weaknessSource
	index := map[string]int{}

	for _, weakness := range cve.Weaknesses {
		for _, description := range weakness.Description {
			if !strings.HasPrefix(description.Value, "CWE-") {
				continue
			}
			source := clean(weakness.Source)
			at, ok := index[source]
			if !ok {
				at = len(sources)
				index[source] = at
				sources = append(sources, weaknessSource{Source: source})
			}
			sources[at].CWE = appendOnce(sources[at].CWE, description.Value)
		}
	}

	return sources
}

func shortCPE(criteria string) string {
	criteria = strings.TrimPrefix(clean(criteria), "cpe:2.3:")
	for strings.HasSuffix(criteria, ":*") {
		criteria = strings.TrimSuffix(criteria, ":*")
	}
	return criteria
}

func toCPEMatch(match nvdCPEMatch) cpeMatch {
	return cpeMatch{
		CPE:     shortCPE(match.Criteria),
		From:    clean(match.VersionStartIncluding),
		After:   clean(match.VersionStartExcluding),
		Through: clean(match.VersionEndIncluding),
		Before:  clean(match.VersionEndExcluding),
	}
}

func configurations(cve nvdCVE) []configuration {
	var groups []configuration

	for _, config := range cve.Configurations {
		var group configuration
		for _, node := range config.Nodes {
			for _, match := range node.CPEMatch {
				if match.Vulnerable {
					group.Vulnerable = append(group.Vulnerable, toCPEMatch(match))
				} else {
					group.On = append(group.On, toCPEMatch(match))
				}
			}
		}
		if len(group.Vulnerable) > 0 {
			groups = append(groups, group)
		}
	}

	return groups
}

func isPlaceholder(value string) bool {
	switch strings.ToLower(clean(value)) {
	case "", "n/a", "na", "unknown", "-":
		return true
	}
	return false
}

func affectedProducts(cve nvdCVE) []affectedProduct {
	var products []affectedProduct

	for _, affected := range cve.Affected {
		for _, data := range affected.AffectedData {
			if isPlaceholder(data.Vendor) && isPlaceholder(data.Product) {
				continue
			}

			product := affectedProduct{
				Vendor:        clean(data.Vendor),
				Product:       clean(data.Product),
				DefaultStatus: clean(data.DefaultStatus),
			}

			for _, version := range data.Versions {
				entry := affectedVersion{
					Version:         clean(version.Version),
					Status:          clean(version.Status),
					LessThan:        clean(version.LessThan),
					LessThanOrEqual: clean(version.LessThanOrEqual),
				}
				for _, change := range version.Changes {
					entry.Changes = append(entry.Changes, versionChange{At: clean(change.At), Status: clean(change.Status)})
				}
				if isPlaceholder(entry.Version) && entry.LessThan == "" && entry.LessThanOrEqual == "" && len(entry.Changes) == 0 {
					continue
				}
				product.Versions = append(product.Versions, entry)
			}

			products = append(products, product)
		}
	}

	return products
}

func references(cve nvdCVE) []reference {
	refs := make([]reference, 0, len(cve.References))
	index := map[string]int{}

	for _, ref := range cve.References {
		url := clean(ref.URL)
		if url == "" {
			continue
		}

		at, seen := index[url]
		if !seen {
			at = len(refs)
			index[url] = at
			refs = append(refs, reference{URL: url})
		}
		for _, tag := range ref.Tags {
			refs[at].Tags = appendOnce(refs[at].Tags, clean(tag))
		}
	}

	return refs
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

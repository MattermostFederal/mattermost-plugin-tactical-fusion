package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
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
			name:   "advisory",
			source: "advisories",
			build:  buildAdvisory,
			target: func() string { return filepath.Join(*treeDir, "assets", "cyber", "advisory.tsv") },
		},
		{
			name:   "threat",
			source: "threat",
			build:  buildThreat,
			target: func() string { return filepath.Join(*outDir, "threat.tsv") },
		},
		{
			name:   "malware",
			source: "threat",
			build:  buildMalware,
			target: func() string { return filepath.Join(*outDir, "malware.tsv") },
		},
		{
			name:   "attackdetail",
			source: "enterprise-attack.json",
			build:  buildAttackDetail,
			target: func() string { return filepath.Join(*treeDir, "assets", "cyber", "attackdetail.tsv") },
		},
		{
			name:   "cwedetail",
			source: "cwe-1000.csv",
			build:  buildCWEDetail,
			target: func() string { return filepath.Join(*treeDir, "assets", "cyber", "cwedetail.tsv") },
		},
		{
			name:   "kev",
			source: "known_exploited_vulnerabilities.json",
			build:  buildKEV,
			target: func() string { return filepath.Join(*treeDir, "assets", "cyber", "kev.tsv") },
		},
		{
			name:   "cve",
			source: "nvd",
			build:  buildCVE,
			target: func() string { return filepath.Join(*outDir, "cve.tsv") },
		},
		{
			name:   "cvedetail",
			source: "nvd",
			build:  buildCVEDetail,
			target: func() string { return filepath.Join(*outDir, "cvedetail.tsv") },
		},
		{
			name:   "epss",
			source: "epss_scores-current.csv",
			build:  buildEPSS,
			target: func() string { return filepath.Join(*outDir, "epss.tsv") },
		},
		{
			name:   "ip",
			source: "ip2asn-combined.tsv",
			build:  buildIP,
			target: func() string { return filepath.Join(*outDir, "ip.tsv") },
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
	fmt.Fprintf(&b2, "%s%d\t%s\t%s\t%s\n",
		schemaPrefix, schemaVersion, b.name, time.Now().UTC().Format(time.RFC3339), stampSource)
	if header := headerFor(b.name); header != nil {
		b2.WriteString(strings.Join(header, "\t") + "\n")
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
	"attack": {"id", "name", "kind", "tactics", "parent", "platforms", "summary", "status", "replaced_by"},
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
	ID                 string          `json:"id"`
	AnalyticRefs       []string        `json:"x_mitre_analytic_refs"`
	LogSources         []stixLogSource `json:"x_mitre_log_source_references"`
	Tunables           []stixTunable   `json:"x_mitre_mutable_elements"`
	Type               string          `json:"type"`
	RelationshipType   string          `json:"relationship_type"`
	SourceRef          string          `json:"source_ref"`
	TargetRef          string          `json:"target_ref"`
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
	SourceName  string `json:"source_name"`
	ExternalID  string `json:"external_id"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type stixLogSource struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

type stixTunable struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

func readSTIX(source string) (stixBundle, error) {
	raw, err := os.ReadFile(source) // #nosec G304 -- a source file under the directory the operator names with -source
	if err != nil {
		return stixBundle{}, err
	}

	var bundle stixBundle
	err = json.Unmarshal(raw, &bundle)
	return bundle, err
}

func attackID(o stixObject) string {
	for _, ref := range o.ExternalReferences {
		if ref.SourceName == "mitre-attack" {
			return ref.ExternalID
		}
	}

	return ""
}

const (
	attackActive     = "active"
	attackRevoked    = "revoked"
	attackDeprecated = "deprecated"
)

func attackStatus(o stixObject) string {
	switch {
	case o.Revoked:
		return attackRevoked
	case o.Deprecated:
		return attackDeprecated
	}
	return attackActive
}

var (
	attackCitation     = regexp.MustCompile(`\(Citation:[^)]*\)`)
	attackMarkdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	attackMarkup       = regexp.MustCompile("</?code>|`")
)

var typographicPunctuation = strings.NewReplacer("\u2018", "'", "\u2019", "'", "\u201c", `"`, "\u201d", `"`, "\u2013", "-", "\u2014", " - ")

func attackText(text string) string {
	plain := attackCitation.ReplaceAllString(text, "")
	plain = typographicPunctuation.Replace(plain)
	plain = attackMarkdownLink.ReplaceAllString(plain, "$1")
	plain = attackMarkup.ReplaceAllString(plain, "")
	return clean(plain)
}

func attackName(name string) string {
	return clean(typographicPunctuation.Replace(name))
}

func attackSummary(description string) string {
	return firstSentence(attackText(description))
}

func buildAttack(source string) ([][]string, error) {
	bundle, err := readSTIX(source)
	if err != nil {
		return nil, err
	}

	idOf := map[string]string{}
	tacticByShortName := map[string]string{}
	for _, o := range bundle.Objects {
		id := attackID(o)
		if id == "" {
			continue
		}
		idOf[o.ID] = id
		if o.Type == "x-mitre-tactic" && attackStatus(o) == attackActive {
			tacticByShortName[o.ShortName] = id
		}
	}

	replacedBy := map[string]string{}
	for _, o := range bundle.Objects {
		if o.Type == "relationship" && o.RelationshipType == "revoked-by" {
			replacedBy[o.SourceRef] = idOf[o.TargetRef]
		}
	}

	var rows [][]string

	for _, o := range bundle.Objects {
		id := attackID(o)
		if id == "" {
			continue
		}
		status := attackStatus(o)

		switch o.Type {
		case "x-mitre-tactic":
			rows = append(rows, []string{
				id, attackName(o.Name), "tactic", "", "", "", attackSummary(o.Description), status, replacedBy[o.ID],
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
			slices.Sort(tactics)

			kind, parent := "technique", ""
			if o.IsSubtechnique {
				kind = "subtechnique"
				parent, _, _ = strings.Cut(id, ".")
			}

			rows = append(rows, []string{
				id, attackName(o.Name), kind, strings.Join(tactics, ","), parent,
				strings.Join(o.Platforms, ","), attackSummary(o.Description), status, replacedBy[o.ID],
			})
		}
	}

	return keepResolvableParents(rows), nil
}

type attackReference struct {
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type attackMitigation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type attackLogSource struct {
	Name    string `json:"name"`
	Channel string `json:"channel,omitempty"`
}

type attackTunable struct {
	Field       string `json:"field"`
	Description string `json:"description,omitempty"`
}

type attackAnalytic struct {
	ID          string            `json:"id"`
	Platforms   []string          `json:"platforms,omitempty"`
	Description string            `json:"description,omitempty"`
	LogSources  []attackLogSource `json:"logSources,omitempty"`
	Tunables    []attackTunable   `json:"tunables,omitempty"`
}

type attackDetection struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	URL       string           `json:"url,omitempty"`
	Analytics []attackAnalytic `json:"analytics,omitempty"`
}

type attackProcedure struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

var procedureKinds = map[string]string{
	"intrusion-set": "group",
	"malware":       "malware",
	"tool":          "tool",
	"campaign":      "campaign",
}

func attackURL(o stixObject) string {
	for _, ref := range o.ExternalReferences {
		if ref.SourceName == "mitre-attack" {
			return ref.URL
		}
	}
	return ""
}

func attackReferences(o stixObject) []attackReference {
	var refs []attackReference
	for _, ref := range o.ExternalReferences {
		if ref.SourceName == "mitre-attack" {
			continue
		}
		description := attackText(ref.Description)
		if ref.URL == "" && description == "" {
			continue
		}
		refs = append(refs, attackReference{Source: attackName(ref.SourceName), URL: ref.URL, Description: description})
	}
	return refs
}

func attackAnalytics(strategy stixObject, byID map[string]stixObject) []attackAnalytic {
	var analytics []attackAnalytic
	for _, ref := range strategy.AnalyticRefs {
		analytic, ok := byID[ref]
		if !ok {
			continue
		}
		entry := attackAnalytic{ID: attackID(analytic), Platforms: analytic.Platforms, Description: attackText(analytic.Description)}
		for _, source := range analytic.LogSources {
			entry.LogSources = append(entry.LogSources, attackLogSource{Name: attackName(source.Name), Channel: attackText(source.Channel)})
		}
		for _, tunable := range analytic.Tunables {
			entry.Tunables = append(entry.Tunables, attackTunable{Field: attackName(tunable.Field), Description: attackText(tunable.Description)})
		}
		analytics = append(analytics, entry)
	}
	slices.SortStableFunc(analytics, func(a, b attackAnalytic) int { return strings.Compare(a.ID, b.ID) })
	return analytics
}

func buildAttackDetail(source string) ([][]string, error) {
	bundle, err := readSTIX(source)
	if err != nil {
		return nil, err
	}

	byID := map[string]stixObject{}
	for _, o := range bundle.Objects {
		byID[o.ID] = o
	}

	mitigations := map[string][]attackMitigation{}
	detections := map[string][]attackDetection{}
	procedures := map[string][]attackProcedure{}

	for _, r := range bundle.Objects {
		if r.Type != "relationship" || attackStatus(r) != attackActive {
			continue
		}
		from, ok := byID[r.SourceRef]
		if !ok || attackStatus(from) != attackActive || attackID(from) == "" {
			continue
		}

		switch r.RelationshipType {
		case "mitigates":
			mitigations[r.TargetRef] = append(mitigations[r.TargetRef], attackMitigation{
				ID: attackID(from), Name: attackName(from.Name), URL: attackURL(from), Description: attackText(r.Description),
			})
		case "detects":
			detections[r.TargetRef] = append(detections[r.TargetRef], attackDetection{
				ID: attackID(from), Name: attackName(from.Name), URL: attackURL(from), Analytics: attackAnalytics(from, byID),
			})
		case "uses":
			kind, known := procedureKinds[from.Type]
			if !known {
				continue
			}
			procedures[r.TargetRef] = append(procedures[r.TargetRef], attackProcedure{
				ID: attackID(from), Name: attackName(from.Name), Kind: kind, URL: attackURL(from), Description: attackText(r.Description),
			})
		}
	}

	var rows [][]string
	for _, o := range bundle.Objects {
		id := attackID(o)
		if id == "" || (o.Type != "attack-pattern" && o.Type != "x-mitre-tactic") {
			continue
		}

		slices.SortStableFunc(mitigations[o.ID], func(a, b attackMitigation) int { return strings.Compare(a.ID, b.ID) })
		slices.SortStableFunc(detections[o.ID], func(a, b attackDetection) int { return strings.Compare(a.ID, b.ID) })
		slices.SortStableFunc(procedures[o.ID], func(a, b attackProcedure) int { return strings.Compare(a.ID, b.ID) })

		references, err := compactJSON(attackReferences(o))
		if err != nil {
			return nil, err
		}
		mitigationField, err := compactJSON(mitigations[o.ID])
		if err != nil {
			return nil, err
		}
		detectionField, err := compactJSON(detections[o.ID])
		if err != nil {
			return nil, err
		}
		procedureField, err := compactJSON(procedures[o.ID])
		if err != nil {
			return nil, err
		}

		rows = append(rows, []string{id, attackText(o.Description), references, mitigationField, detectionField, procedureField})
	}

	return rows, nil
}

func keepResolvableParents(rows [][]string) [][]string {
	known := map[string]bool{}
	for _, row := range rows {
		known[row[0]] = true
	}
	for _, row := range rows {
		if row[4] != "" && !known[row[4]] {
			row[4] = ""
		}
		if row[8] != "" && !known[row[8]] {
			row[8] = ""
		}
	}
	return rows
}

func readCWE(source string) ([]map[string]string, error) {
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

	header := make([]string, len(records[0]))
	for i, name := range records[0] {
		header[i] = strings.ToLower(strings.TrimSpace(name))
	}

	var weaknesses []map[string]string
	for _, record := range records[1:] {
		fields := map[string]string{}
		for i, value := range record {
			if i < len(header) {
				fields[header[i]] = value
			}
		}
		if clean(fields["cwe-id"]) == "" {
			continue
		}
		weaknesses = append(weaknesses, fields)
	}

	return weaknesses, nil
}

func buildCWE(source string) ([][]string, error) {
	weaknesses, err := readCWE(source)
	if err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(weaknesses))
	for _, fields := range weaknesses {
		rows = append(rows, []string{
			"CWE-" + clean(fields["cwe-id"]),
			clean(fields["name"]),
			clean(fields["weakness abstraction"]),
			clean(fields["status"]),
			firstSentence(fields["description"]),
			parentsOf(clean(fields["related weaknesses"])),
		})
	}

	return rows, nil
}

type cweConsequence struct {
	Scopes     []string `json:"scopes"`
	Impacts    []string `json:"impacts"`
	Likelihood string   `json:"likelihood,omitempty"`
	Note       string   `json:"note,omitempty"`
}

type cweMitigation struct {
	Phase         string `json:"phase,omitempty"`
	Strategy      string `json:"strategy,omitempty"`
	Description   string `json:"description"`
	Effectiveness string `json:"effectiveness,omitempty"`
}

type cweDetection struct {
	Method        string `json:"method"`
	Description   string `json:"description"`
	Effectiveness string `json:"effectiveness,omitempty"`
}

type cweExample struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type packedPair struct {
	key   string
	value string
}

func packedEntries(field string, keys ...string) [][]packedPair {
	alternatives := strings.Join(keys, "|")
	entryStart := regexp.MustCompile(`::(?:` + alternatives + `):`)
	keyStart := regexp.MustCompile(`(?:^|:)(` + alternatives + `):`)

	starts := entryStart.FindAllStringIndex(field, -1)
	entries := make([][]packedPair, 0, len(starts))

	for i, start := range starts {
		end := len(field)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		body := strings.TrimRight(field[start[0]+2:end], ":")

		matches := keyStart.FindAllStringSubmatchIndex(body, -1)
		var pairs []packedPair
		for j, match := range matches {
			valueEnd := len(body)
			if j+1 < len(matches) {
				valueEnd = matches[j+1][0]
			}
			if value := clean(body[match[1]:valueEnd]); value != "" {
				pairs = append(pairs, packedPair{key: body[match[2]:match[3]], value: value})
			}
		}
		if len(pairs) > 0 {
			entries = append(entries, pairs)
		}
	}

	return entries
}

func consequencesOf(field string) []cweConsequence {
	var out []cweConsequence
	for _, entry := range packedEntries(field, "SCOPE", "IMPACT", "LIKELIHOOD", "NOTE") {
		var c cweConsequence
		for _, pair := range entry {
			switch pair.key {
			case "SCOPE":
				c.Scopes = append(c.Scopes, pair.value)
			case "IMPACT":
				c.Impacts = append(c.Impacts, pair.value)
			case "LIKELIHOOD":
				c.Likelihood = pair.value
			case "NOTE":
				c.Note = pair.value
			}
		}
		out = append(out, c)
	}
	return out
}

func mitigationsOf(field string) []cweMitigation {
	var out []cweMitigation
	for _, entry := range packedEntries(field, "PHASE", "STRATEGY", "DESCRIPTION", "EFFECTIVENESS") {
		var m cweMitigation
		for _, pair := range entry {
			switch pair.key {
			case "PHASE":
				m.Phase = pair.value
			case "STRATEGY":
				m.Strategy = pair.value
			case "DESCRIPTION":
				m.Description = pair.value
			case "EFFECTIVENESS":
				m.Effectiveness = pair.value
			}
		}
		if m.Description != "" {
			out = append(out, m)
		}
	}
	return out
}

func detectionsOf(field string) []cweDetection {
	var out []cweDetection
	for _, entry := range packedEntries(field, "METHOD", "DESCRIPTION", "EFFECTIVENESS") {
		var d cweDetection
		for _, pair := range entry {
			switch pair.key {
			case "METHOD":
				d.Method = pair.value
			case "DESCRIPTION":
				d.Description = pair.value
			case "EFFECTIVENESS":
				d.Effectiveness = pair.value
			}
		}
		if d.Method != "" || d.Description != "" {
			out = append(out, d)
		}
	}
	return out
}

func examplesOf(field string) []cweExample {
	var out []cweExample
	for _, entry := range packedEntries(field, "REFERENCE", "DESCRIPTION", "LINK") {
		var e cweExample
		for _, pair := range entry {
			switch pair.key {
			case "REFERENCE":
				e.ID = pair.value
			case "DESCRIPTION":
				e.Description = pair.value
			}
		}
		if e.ID != "" {
			out = append(out, e)
		}
	}
	return out
}

func buildCWEDetail(source string) ([][]string, error) {
	weaknesses, err := readCWE(source)
	if err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(weaknesses))
	for _, fields := range weaknesses {
		consequences, err := compactJSON(consequencesOf(fields["common consequences"]))
		if err != nil {
			return nil, err
		}
		mitigations, err := compactJSON(mitigationsOf(fields["potential mitigations"]))
		if err != nil {
			return nil, err
		}
		detections, err := compactJSON(detectionsOf(fields["detection methods"]))
		if err != nil {
			return nil, err
		}
		examples, err := compactJSON(examplesOf(fields["observed examples"]))
		if err != nil {
			return nil, err
		}

		rows = append(rows, []string{
			"CWE-" + clean(fields["cwe-id"]),
			clean(fields["description"]),
			clean(fields["extended description"]),
			consequences,
			mitigations,
			detections,
			examples,
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

func epssScoreDate(header string) (string, error) {
	for field := range strings.SplitSeq(strings.TrimPrefix(strings.TrimSpace(header), "#"), ",") {
		key, value, ok := strings.Cut(field, ":")
		if !ok || key != "score_date" {
			continue
		}
		scored, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return "", fmt.Errorf("the EPSS score date %q is not a timestamp: %w", value, err)
		}
		return scored.UTC().Format("2006-01-02"), nil
	}
	return "", fmt.Errorf("the EPSS export names no score_date in its first line %q", header)
}

func plainDecimal(value string) string {
	value = clean(value)
	if !strings.ContainsAny(value, "eE") {
		return value
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	return strconv.FormatFloat(parsed, 'f', -1, 64)
}

func buildEPSS(source string) ([][]string, error) {
	handle, err := os.Open(source) // #nosec G304 -- a source file under the directory the operator names with -source
	if err != nil {
		return nil, err
	}
	defer func() { _ = handle.Close() }()

	buffered := bufio.NewReader(handle)
	header, err := buffered.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading the EPSS header: %w", err)
	}
	scoreDate, err := epssScoreDate(header)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(buffered)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("the EPSS export is empty")
	}

	var rows [][]string
	for _, record := range records[1:] {
		if len(record) < 3 || !strings.HasPrefix(record[0], "CVE-") {
			continue
		}

		rows = append(rows, []string{
			strings.ToUpper(clean(record[0])),
			plainDecimal(record[1]),
			plainDecimal(record[2]),
			scoreDate,
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

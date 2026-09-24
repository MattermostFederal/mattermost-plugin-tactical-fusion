package cyber

import (
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type Row struct {
	Label string
	Value string
	At    time.Time
}

type Link struct {
	Kind  Kind
	Value string
	Label string
}

type WatchEntry struct {
	Verdict string
	Source  string
	Note    string
	Updated string
	Known   bool
}

type DatasetStatus struct {
	Name      string
	Label     string
	Present   bool
	Generated string
}

type Details struct {
	Kind      Kind
	Value     string
	Title     string
	Headline  string
	Summary   string
	Rows      []Row
	Related   []Link
	Watchlist []WatchEntry
	Status    string
	Datasets  []DatasetStatus

	Score     string
	Severity  string
	Exploited bool
	Vector    []VectorMetric

	Affected       []string
	Configurations []string
	References     []Reference

	Sections []Section
	Credits  []Credit
	Glance   Glance
	Reports  []Report
}

type Credit struct {
	Text string
	URL  string
}

func DatasetLabel(name string) string {
	return datasetLabels[name]
}

var datasetLabels = map[string]string{
	intel.NameCVE:          "vulnerability",
	intel.NameCVEDetail:    "vulnerability detail",
	intel.NameCWEDetail:    "weakness detail",
	intel.NameAttackDetail: "ATT&CK detail",
	intel.NameAdvisory:     "CISA advisory",
	intel.NameThreat:       "threat feed",
	intel.NameEPSS:         "exploit prediction",
	intel.NameKEV:          "known exploited vulnerabilities",
	intel.NameIP:           "IP address",
	intel.NameWatchlist:    "watchlist",
}

const attackBaseURL = "https://attack.mitre.org"

func Describe(kind Kind, value string, set *intel.Set) Details {
	d := Details{Kind: kind, Value: value, Title: value}

	switch kind {
	case KindCVE:
		describeCVE(&d, set)
	case KindCWE:
		describeCWE(&d, set)
	case KindAttack:
		describeAttack(&d, set)
	case KindIP:
		describeIP(&d, set)
	case KindHash:
		describeHash(&d)
	}

	if kind == KindIP || kind == KindHash {
		describeThreatReports(&d, set)
	}

	d.Watchlist = watchlistFor(value, set)
	if kind == KindHash {
		algorithm, bytes := hashAlgorithm(value)
		d.Glance = hashGlance(algorithm, bytes, set, len(d.Watchlist) > 0)
	}
	if len(d.Watchlist) > 0 {
		d.Headline = joinSentence(d.Headline, watchlistHeadline(d.Watchlist))
	}
	if d.Headline == "" {
		d.Headline = d.Status
	}
	if d.Headline == "" {
		d.Headline = kind.Label()
	}

	d.Datasets = datasetStatuses(set)

	return d
}

func datasetStatuses(set *intel.Set) []DatasetStatus {
	statuses := set.Statuses()

	all := make([]DatasetStatus, 0, len(statuses))
	for _, status := range statuses {
		all = append(all, DatasetStatus{
			Name:      status.Name,
			Label:     datasetLabels[status.Name],
			Present:   status.Present,
			Generated: status.Generated,
		})
	}

	return all
}

func datasetSentence(set *intel.Set, name string, err error) string {
	label := datasetLabels[name]

	switch {
	case errors.Is(err, intel.ErrNoDataset) || !set.Has(name):
		return "No " + label + " dataset is installed."

	case !errors.Is(err, intel.ErrNotFound):
		return errcode.WithCode(errcode.CyberDataLookupFailed,
			"The "+label+" dataset is installed and could not be read.")
	}

	generated := set.Generated(name)
	if generated == "" {
		return "Not in the " + label + " dataset."
	}

	return "Not in the " + label + " dataset generated " + generated + "."
}

func addRow(d *Details, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	d.Rows = append(d.Rows, Row{Label: label, Value: value})
}

func addTimeRow(d *Details, label, raw string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	text, at := nvdTimestamp(raw)
	d.Rows = append(d.Rows, Row{Label: label, Value: text, At: at})
}

func joinSentence(first, second string) string {
	switch {
	case first == "":
		return second
	case second == "":
		return first
	}
	return first + ", " + second
}

func describeCVE(d *Details, set *intel.Set) {
	record, err := set.CVE(d.Value)
	recordFound := err == nil

	if err != nil {
		d.Status = datasetSentence(set, intel.NameCVE, err)
	} else {
		d.Summary = record.Summary
		addTimeRow(d, "Published", record.Published)
		addTimeRow(d, "Last modified", record.Modified)
		addRow(d, "CVSS", severityText(record.Score, record.Severity))
		addRow(d, "Vector", record.Vector)

		for _, weakness := range record.Weaknesses {
			if _, known := LookupWeakness(weakness); known {
				d.Related = append(d.Related, weaknessLink(weakness))
			}
		}

		d.Headline = severityText(record.Score, record.Severity)
		d.Score = record.Score
		d.Severity = severityLevel(record.Severity)
		d.Vector = DescribeVector(record.Vector)
	}

	if epss, err := set.EPSS(d.Value); err == nil {
		addRow(d, "EPSS", epssText(epss))
	}

	if kev, err := set.KEV(d.Value); err == nil {
		addRow(d, "Known exploited", kevText(kev))
		addRow(d, "Action due", kev.DueDate)
		addRow(d, "Affected product", kev.Product)
		addRow(d, "Required action", kev.Action)
		d.Headline = joinSentence(d.Headline, "in KEV")
		d.Exploited = true
	}

	describeCVEDetail(d, set, recordFound)
}

func severityText(score, severity string) string {
	switch {
	case score == "" && severity == "":
		return ""
	case score == "":
		return severity
	case severity == "":
		return score
	}

	return score + " " + severity
}

var SeverityLevels = []string{"critical", "high", "medium", "low", "none"}

func severityLevel(severity string) string {
	level := strings.ToLower(strings.TrimSpace(severity))
	if slices.Contains(SeverityLevels, level) {
		return level
	}
	return ""
}

const nvdTimestampLayout = "2006-01-02T15:04:05.999999999"

func nvdTimestamp(value string) (string, time.Time) {
	parsed, err := time.Parse(nvdTimestampLayout, value)
	if err != nil {
		return value, time.Time{}
	}
	minute := parsed.Round(time.Minute)
	return minute.Format("2006-01-02 15:04") + " UTC", minute
}

var unitFraction = regexp.MustCompile(`^(0|1)\.(\d+)$`)

func asPercent(fraction string) (string, bool) {
	parts := unitFraction.FindStringSubmatch(fraction)
	if parts == nil || (parts[1] == "1" && strings.Trim(parts[2], "0") != "") {
		return "", false
	}
	digits := parts[2] + strings.Repeat("0", max(0, 2-len(parts[2])))
	whole := strings.TrimLeft(parts[1]+digits[:2], "0")
	if whole == "" {
		whole = "0"
	}
	if rest := strings.TrimRight(digits[2:], "0"); rest != "" {
		return whole + "." + rest + "%", true
	}
	return whole + "%", true
}

func epssText(epss intel.EPSSRecord) string {
	probability, ok := asPercent(epss.Score)
	if !ok {
		probability = epss.Score
	}
	text := probability + " chance of exploitation in the next 30 days"
	if percentile, ok := asPercent(epss.Percentile); ok {
		text += ", higher than " + percentile + " of scored CVEs"
	} else if epss.Percentile != "" {
		text += " (percentile " + epss.Percentile + ")"
	}
	return text
}

func kevText(kev intel.KEVRecord) string {
	text := "Listed"
	if kev.DateAdded != "" {
		text += " " + kev.DateAdded
	}
	if kev.Ransomware != "" {
		text += ", ransomware use: " + kev.Ransomware
	}

	return text
}

func describeCWE(d *Details, set *intel.Set) {
	weakness, known := LookupWeakness(d.Value)
	if !known {
		return
	}

	d.Title = weakness.Name
	d.Summary = weakness.Summary
	d.Headline = weakness.Name
	d.Glance = weaknessGlance(weakness)

	addRow(d, "Identifier", weakness.ID)
	addRow(d, "Abstraction", weakness.Abstraction)
	addRow(d, "Status", weakness.Status)

	for _, parent := range weakness.Parents {
		if _, ok := LookupWeakness(parent); ok {
			d.Related = append(d.Related, weaknessLink(parent))
		}
	}

	describeCWEDetail(d, set)
}

func weaknessLink(id string) Link {
	label := id
	if weakness, ok := LookupWeakness(id); ok {
		label = id + " " + weakness.Name
	}

	return Link{Kind: KindCWE, Value: id, Label: label}
}

func describeAttack(d *Details, set *intel.Set) {
	technique, known := LookupTechnique(d.Value)
	if !known {
		return
	}

	d.Title = technique.Name
	d.Summary = technique.Summary
	d.Headline = joinSentence(attackHeadline(technique), attackRetirement(technique))
	d.Glance = techniqueGlance(technique)

	addRow(d, "Identifier", technique.ID)
	addRow(d, "Kind", attackKindText(technique.Kind))

	var tacticNames []string
	for _, tactic := range technique.Tactics {
		if parent, ok := LookupTechnique(tactic); ok {
			tacticNames = append(tacticNames, parent.Name)
			d.Related = append(d.Related, techniqueLink(tactic))
		}
	}
	addRow(d, "Tactics", strings.Join(tacticNames, ", "))

	if technique.Parent != "" {
		if parent, ok := LookupTechnique(technique.Parent); ok {
			addRow(d, "Parent technique", parent.ID+" "+parent.Name)
			d.Related = append(d.Related, techniqueLink(technique.Parent))
		}
	}

	for _, child := range SubTechniquesOf(technique.ID) {
		if child.Status == StatusActive {
			d.Related = append(d.Related, techniqueLink(child.ID))
		}
	}

	addRow(d, "Platforms", strings.Join(technique.Platforms, ", "))
	addRow(d, "Status", attackStatusText(technique.Status))
	if replacement, ok := LookupTechnique(technique.ReplacedBy); ok {
		addRow(d, "Replaced by", replacement.ID+" "+replacement.Name)
		d.Related = append([]Link{techniqueLink(replacement.ID)}, d.Related...)
	}
	addRow(d, "Reference", AttackURL(technique))

	if technique.Kind == techniqueKindTactic {
		addSection(d, "Techniques", tacticTechniqueItems(technique.ID))
	}
	describeAttackDetail(d, set)
}

func attackHeadline(technique Technique) string {
	if technique.Kind == techniqueKindSubTechnique {
		if parent, ok := LookupTechnique(technique.Parent); ok {
			return parent.Name + ": " + technique.Name
		}
	}

	return technique.Name
}

func attackStatusText(status string) string {
	switch status {
	case StatusActive:
		return "Active"
	case StatusRevoked:
		return "Revoked by MITRE"
	case StatusDeprecated:
		return "Deprecated by MITRE"
	}
	return status
}

func attackRetirement(technique Technique) string {
	if technique.Status == StatusActive || technique.Status == "" {
		return ""
	}
	if technique.ReplacedBy != "" {
		return technique.Status + ", see " + technique.ReplacedBy
	}
	return technique.Status
}

func attackKindText(kind string) string {
	switch kind {
	case techniqueKindTactic:
		return "Tactic"
	case techniqueKindTechnique:
		return "Technique"
	case techniqueKindSubTechnique:
		return "Sub-technique"
	}

	return kind
}

func AttackURL(technique Technique) string {
	if technique.Kind == techniqueKindTactic {
		return attackBaseURL + "/tactics/" + technique.ID + "/"
	}

	base, sub, found := strings.Cut(technique.ID, ".")
	if !found {
		return attackBaseURL + "/techniques/" + base + "/"
	}

	return attackBaseURL + "/techniques/" + base + "/" + sub + "/"
}

func techniqueLink(id string) Link {
	label := id
	if technique, ok := LookupTechnique(id); ok {
		label = id + " " + technique.Name
	}

	return Link{Kind: KindAttack, Value: id, Label: label}
}

func describeIP(d *Details, set *intel.Set) {
	addr, err := netip.ParseAddr(d.Value)
	if err != nil {
		return
	}

	addRow(d, "Version", addressVersion(addr))
	addRow(d, "Scope", AddressScope(addr))

	record, lookupErr := set.IP(addr)
	addRow(d, "Autonomous system", joinFields(record.ASN, record.ASName))
	addRow(d, "Country", record.Country)
	addRow(d, "Region", record.Region)
	addRow(d, "City", record.City)
	addRow(d, "Source", strings.Join(record.Sources, ", "))
	d.Glance = addressGlance(AddressScope(addr), record)
	for _, attribution := range record.Attributions {
		d.Credits = append(d.Credits, Credit{Text: attribution.Text, URL: webURL(attribution.URL)})
	}

	if record.Empty() {
		if scope := AddressScope(addr); scope != scopeGlobal {
			d.Status = "A " + strings.ToLower(scope) + " address, which no dataset describes."
			d.Headline = scope
			return
		}
		d.Status = datasetSentence(set, intel.NameIP, lookupErr)
		return
	}

	d.Headline = joinSentence(joinFields(record.ASN, record.ASName), record.Country)
}

func joinFields(values ...string) string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}

	return strings.Join(kept, " ")
}

func addressVersion(addr netip.Addr) string {
	if addr.Is4() {
		return "IPv4"
	}
	return "IPv6"
}

const scopeGlobal = "Global"

var documentationPrefixes = []netip.Prefix{
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}

var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

func AddressScope(addr netip.Addr) string {
	unmapped := addr.Unmap()

	switch {
	case unmapped.IsLoopback():
		return "Loopback"
	case unmapped.IsUnspecified():
		return "Unspecified"
	case unmapped.IsMulticast():
		return "Multicast"
	case unmapped.IsLinkLocalUnicast():
		return "Link-local"
	case unmapped.IsPrivate():
		return "Private"
	case sharedAddressSpace.Contains(unmapped):
		return "Carrier-grade NAT"
	}

	for _, prefix := range documentationPrefixes {
		if prefix.Contains(unmapped) {
			return "Documentation"
		}
	}

	return scopeGlobal
}

func describeHash(d *Details) {
	algorithm, bytes := hashAlgorithm(d.Value)

	addRow(d, "Algorithm", algorithm)
	addRow(d, "Length", strconv.Itoa(bytes)+" bytes")

	d.Headline = algorithm
	d.Status = "A hash is an identity rather than a record. What is known about one comes from the watchlist."
}

func hashAlgorithm(value string) (string, int) {
	switch len(value) {
	case 32:
		return "MD5", 16
	case 40:
		return "SHA-1, which is also the shape of a Git object id", 20
	case 64:
		return "SHA-256", 32
	}

	return "", 0
}

func watchlistFor(value string, set *intel.Set) []WatchEntry {
	rows := set.Watchlist(value)
	if len(rows) == 0 {
		return nil
	}

	entries := make([]WatchEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, WatchEntry{
			Verdict: row.Verdict,
			Source:  row.Source,
			Note:    row.Note,
			Updated: row.Updated,
			Known:   intel.KnownVerdict(row.Verdict),
		})
	}

	return entries
}

func watchlistHeadline(entries []WatchEntry) string {
	if len(entries) == 1 {
		return "watchlist: " + entries[0].Verdict
	}

	return "watchlist: " + strconv.Itoa(len(entries)) + " entries"
}

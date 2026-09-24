package cyber

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func datasets(t *testing.T, files map[string][]string) *intel.Set {
	t.Helper()

	dir := t.TempDir()
	for name, rows := range files {
		var body strings.Builder
		body.WriteString(fmt.Sprintf("%s%d\t%s\t2026-09-01T00:00:00Z\ttest\n", intel.SchemaPrefix, intel.SchemaVersion, name))
		for _, row := range rows {
			body.WriteString(row + "\n")
		}
		if err := os.WriteFile(filepath.Join(dir, name+intel.Suffix), []byte(body.String()), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	set, problems := intel.Open([]string{dir})
	for _, problem := range problems {
		t.Fatalf("unexpected problem: %v", problem)
	}
	t.Cleanup(set.Close)

	return set
}

func rowValue(d Details, label string) string {
	for _, row := range d.Rows {
		if row.Label == label {
			return row.Value
		}
	}
	return ""
}

func TestDescribeSaysWhenNoDatasetIsInstalled(t *testing.T) {
	d := Describe(KindCVE, "CVE-2021-44228", nil)

	if d.Status != "No vulnerability dataset is installed." {
		t.Fatalf("status %q", d.Status)
	}
	if d.Headline != d.Status {
		t.Fatalf("the hover and the panel disagree: %q and %q", d.Headline, d.Status)
	}

	for _, dataset := range d.Datasets {
		if dataset.Present {
			t.Fatalf("%s reported as installed with no datasets at all", dataset.Name)
		}
		if dataset.Label == "" {
			t.Fatalf("%s has no label", dataset.Name)
		}
	}
}

func TestDescribeSaysWhenADatasetDoesNotHoldTheIndicator(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2021-0001", "2021-01-01", "2021-02-01", "5.0", "Medium", "AV:N", "", "something else",
		}, "\t")},
	})

	d := Describe(KindCVE, "CVE-2021-44228", set)

	if d.Status != "Not in the vulnerability dataset generated 2026-09-01T00:00:00Z." {
		t.Fatalf("status %q", d.Status)
	}
}

func TestDescribeRendersAVulnerability(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2021-44228", "2021-12-10", "2022-01-01", "10.0", "Critical",
			"CVSS:3.1/AV:N/AC:L", "CWE-502,CWE-99999", "Remote code execution in a logging library.",
		}, "\t")},
		intel.NameEPSS: {strings.Join([]string{"CVE-2021-44228", "0.975", "99.9th"}, "\t") + "\t2026-09-01"},
		intel.NameKEV: {strings.Join([]string{
			"CVE-2021-44228", "2021-12-10", "2021-12-24", "Known", "Apache Log4j2", "Apply updates.",
		}, "\t")},
	})

	d := Describe(KindCVE, "CVE-2021-44228", set)

	if d.Status != "" {
		t.Fatalf("a found record still carried a status: %q", d.Status)
	}
	if d.Summary != "Remote code execution in a logging library." {
		t.Fatalf("summary %q", d.Summary)
	}
	if got := rowValue(d, "CVSS"); got != "10.0 Critical" {
		t.Fatalf("CVSS %q", got)
	}
	if got := rowValue(d, "EPSS"); got != "0.975 (99.9th percentile)" {
		t.Fatalf("EPSS %q", got)
	}
	if got := rowValue(d, "Known exploited"); !strings.Contains(got, "Listed 2021-12-10") {
		t.Fatalf("KEV %q", got)
	}
	if !strings.Contains(d.Headline, "10.0 Critical") || !strings.Contains(d.Headline, "in KEV") {
		t.Fatalf("headline %q", d.Headline)
	}
	if d.Score != "10.0" || d.Severity != "critical" || !d.Exploited {
		t.Fatalf("score %q, severity %q, exploited %v", d.Score, d.Severity, d.Exploited)
	}

	if len(d.Related) != 1 || d.Related[0].Value != "CWE-502" {
		t.Fatalf("related %+v", d.Related)
	}
	if !RecognizeAs(d.Related[0].Kind, d.Related[0].Value) {
		t.Fatalf("a related link does not agree with itself")
	}
}

func TestEveryRelatedLinkResolves(t *testing.T) {
	for _, value := range []string{"T1059.001", "T1059", "TA0002", "CWE-79"} {
		kind := KindAttack
		if strings.HasPrefix(value, "CWE") {
			kind = KindCWE
		}

		for _, link := range Describe(kind, value, nil).Related {
			if !RecognizeAs(link.Kind, link.Value) {
				t.Errorf("%s offers %s/%s, which does not agree with itself", value, link.Kind, link.Value)
			}
			if link.Label == "" {
				t.Errorf("%s offers a link with no label", value)
			}
		}
	}
}

func TestDescribeRendersATechnique(t *testing.T) {
	d := Describe(KindAttack, "T1059.001", nil)

	if d.Title != "PowerShell" {
		t.Fatalf("title %q", d.Title)
	}
	if d.Headline != "Command and Scripting Interpreter: PowerShell" {
		t.Fatalf("headline %q", d.Headline)
	}
	if got := rowValue(d, "Kind"); got != "Sub-technique" {
		t.Fatalf("kind %q", got)
	}
	if got := rowValue(d, "Tactics"); got != "Execution" {
		t.Fatalf("tactics %q", got)
	}
	if got := rowValue(d, "Parent technique"); !strings.HasPrefix(got, "T1059 ") {
		t.Fatalf("parent %q", got)
	}
	if got := rowValue(d, "Reference"); got != "https://attack.mitre.org/techniques/T1059/001/" {
		t.Fatalf("reference %q", got)
	}
}

func TestDescribeRendersAWeakness(t *testing.T) {
	d := Describe(KindCWE, "CWE-79", nil)

	if !strings.Contains(d.Title, "Cross-site Scripting") {
		t.Fatalf("title %q", d.Title)
	}
	if d.Summary == "" {
		t.Fatalf("no summary")
	}
	if rowValue(d, "Identifier") != "CWE-79" {
		t.Fatalf("no identifier row")
	}
}

func TestDescribeRendersAnAddressAndItsScope(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":  "Documentation",
		"2001:db8::1":  "Documentation",
		"10.0.0.5":     "Private",
		"192.168.1.1":  "Private",
		"169.254.1.1":  "Link-local",
		"fe80::1":      "Link-local",
		"224.0.0.1":    "Multicast",
		"100.64.0.1":   "Carrier-grade NAT",
		"8.8.8.8":      "Global",
		"2606:4700::1": "Global",
	}

	for addr, want := range cases {
		t.Run(addr, func(t *testing.T) {
			if got := AddressScope(netip.MustParseAddr(addr)); got != want {
				t.Fatalf("scope %q, want %q", got, want)
			}

			d := Describe(KindIP, addr, nil)
			if rowValue(d, "Scope") != want {
				t.Fatalf("the row says %q", rowValue(d, "Scope"))
			}
		})
	}
}

func TestAnAddressNoDatasetDescribesSaysSo(t *testing.T) {
	global := Describe(KindIP, "8.8.8.8", nil)
	if global.Status != "No IP address dataset is installed." {
		t.Fatalf("status %q", global.Status)
	}

	private := Describe(KindIP, "10.0.0.5", nil)
	if private.Status != "A private address, which no dataset describes." {
		t.Fatalf("status %q", private.Status)
	}
	if private.Headline != "Private" {
		t.Fatalf("headline %q", private.Headline)
	}
}

func TestDescribeRendersAHash(t *testing.T) {
	cases := map[string]string{
		"44d88612fea8a8f36de82e1278abb02f":         "MD5",
		"da39a3ee5e6b4b0d3255bfef95601890afd80709": "SHA-1",
		strings.Repeat("a", 64):                    "SHA-256",
	}

	for value, algorithm := range cases {
		d := Describe(KindHash, value, nil)
		if !strings.HasPrefix(rowValue(d, "Algorithm"), algorithm) {
			t.Errorf("%s reported %q", value, rowValue(d, "Algorithm"))
		}
	}

	sha1 := Describe(KindHash, "da39a3ee5e6b4b0d3255bfef95601890afd80709", nil)
	if !strings.Contains(rowValue(sha1, "Algorithm"), "Git object id") {
		t.Errorf("SHA-1 does not say it is also a Git object id: %q", rowValue(sha1, "Algorithm"))
	}
}

func TestTheWatchlistReachesEveryKind(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameWatchlist: {
			strings.Join([]string{"CVE-2021-44228", "cve", "malicious", "internal", "exploited here", "2026-08-01"}, "\t"),
		},
	})

	d := Describe(KindCVE, "CVE-2021-44228", set)

	if len(d.Watchlist) != 1 {
		t.Fatalf("%d watchlist entries", len(d.Watchlist))
	}
	if !d.Watchlist[0].Known {
		t.Fatalf("a recognized verdict was not marked as one")
	}
	if !strings.Contains(d.Headline, "watchlist: malicious") {
		t.Fatalf("headline %q", d.Headline)
	}
}

func TestAHostileDatasetRowCannotBecomeMarkup(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2021-44228", "<script>alert(1)</script>", "](https://evil.example)",
			"10.0", "\"onmouseover=x", "AV:N", "", "<img src=x onerror=alert(1)>",
		}, "\t")},
		intel.NameWatchlist: {strings.Join([]string{
			"CVE-2021-44228", "cve", "<b>malicious</b>", "<script>", "'\"><svg onload=1>", "2026-08-01",
		}, "\t")},
	})

	body := renderBody(Describe(KindCVE, "CVE-2021-44228", set))

	allowed := map[string]bool{
		"p": true, "table": true, "tbody": true, "tr": true, "td": true,
		"h2": true, "ul": true, "li": true, "span": true, "a": true, "/": true,
	}

	for _, tag := range elementNames(body) {
		if !allowed[tag] {
			t.Fatalf("a dataset row opened a <%s> element:\n%s", tag, body)
		}
	}

	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("the value was dropped rather than escaped:\n%s", body)
	}
}

func elementNames(body string) []string {
	var names []string

	for index := strings.IndexByte(body, '<'); index >= 0; index = strings.IndexByte(body, '<') {
		body = body[index+1:]
		end := strings.IndexAny(body, " >/")
		if end < 0 {
			end = len(body)
		}
		name := strings.TrimPrefix(body[:end], "/")
		if name == "" {
			name = "/"
		}
		names = append(names, name)
	}

	return names
}

func TestRelatedLinksOnThePagePointAtSiblingPages(t *testing.T) {
	body := renderBody(Describe(KindAttack, "T1059.001", nil))

	if !strings.Contains(body, `href="cyber?k=attack&amp;v=T1059"`) {
		t.Fatalf("no relative link to the parent technique:\n%s", body)
	}
	if strings.Contains(body, `href="/`) {
		t.Fatalf("a page link is rooted, which breaks a subpath install")
	}
}

func TestTheStatusSentenceTellsTheThreeFailuresApart(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2021-44228", "2021-12-10", "2021-12-14", "10.0", "Critical", "AV:N", "CWE-502", "Log4Shell",
		}, "\t")},
	})

	cases := []struct {
		name string
		set  *intel.Set
		key  string
		err  error
		want string
	}{
		{"no dataset", set, intel.NameKEV, intel.ErrNoDataset, "No known exploited vulnerabilities dataset is installed."},
		{"no row", set, intel.NameCVE, intel.ErrNotFound, "Not in the vulnerability dataset generated 2026-09-01T00:00:00Z."},
		{"unreadable", set, intel.NameCVE, errors.New("input/output error"), "The vulnerability dataset is installed and could not be read. (TF-21005)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := datasetSentence(tc.set, tc.key, tc.err); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAVulnerabilityOutsideKEVIsNotMarkedExploited(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2021-0001", "2021-01-01", "2021-02-01", "5.0", "MEDIUM", "AV:N", "", "something",
		}, "\t")},
	})

	d := Describe(KindCVE, "CVE-2021-0001", set)

	if d.Exploited {
		t.Fatalf("marked exploited with no KEV dataset")
	}
	if d.Severity != "medium" {
		t.Fatalf("severity %q, want the level in lower case", d.Severity)
	}
}

func TestSeverityLevelNamesOnlyALevelItKnows(t *testing.T) {
	cases := map[string]string{
		"Critical":  "critical",
		"HIGH":      "high",
		"none":      "none",
		"":          "",
		"Important": "",
		"red":       "",
	}

	for severity, want := range cases {
		if got := severityLevel(severity); got != want {
			t.Errorf("severityLevel(%q) = %q, want %q", severity, got, want)
		}
	}
}

func TestNVDTimestampRoundsToTheMinuteAndNamesTheZone(t *testing.T) {
	cases := map[string]string{
		"2021-12-10T10:15:09.143": "2021-12-10 10:15 UTC",
		"2021-12-10T10:15:45":     "2021-12-10 10:16 UTC",
		"2021-12-31T23:59:30":     "2022-01-01 00:00 UTC",
		"2021-12-10":              "2021-12-10",
		"":                        "",
		"not a time":              "not a time",
	}

	for value, want := range cases {
		if got, _ := nvdTimestamp(value); got != want {
			t.Errorf("nvdTimestamp(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestATimestampRowCarriesTheInstantItShows(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2025-55182", "2025-12-03T16:15:56.463", "2025-12-10", "10.0", "Critical", "AV:N", "", "React2Shell",
		}, "\t")},
	})

	d := Describe(KindCVE, "CVE-2025-55182", set)

	for _, row := range d.Rows {
		switch row.Label {
		case "Published":
			want := time.Date(2025, 12, 3, 16, 16, 0, 0, time.UTC)
			if !row.At.Equal(want) || row.Value != "2025-12-03 16:16 UTC" {
				t.Fatalf("published %q at %v, want %v", row.Value, row.At, want)
			}
		case "Last modified":
			if !row.At.IsZero() {
				t.Fatalf("a date with no time was given the instant %v, a time it never carried", row.At)
			}
		default:
			if !row.At.IsZero() {
				t.Fatalf("%s carries an instant", row.Label)
			}
		}
	}
}

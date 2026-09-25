package cyber

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func goldenDetailRow(t *testing.T, id string) string {
	t.Helper()

	raw, err := os.ReadFile("intel/testdata/cvedetail.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.HasPrefix(line, id+"\t") {
			return line
		}
	}
	t.Fatalf("%s is not in the golden file", id)
	return ""
}

func detailRow(id string, weaknesses, configurations, affected, references string) string {
	return strings.Join([]string{id, weaknesses, configurations, affected, references}, "\t")
}

func cveOnly(id string) string {
	return strings.Join([]string{id, "2026-09-20", "2026-09-21", "9.8", "Critical", "CVSS:3.1/AV:N", "", "A summary."}, "\t")
}

func TestTheGoldenDetailRendersEveryBlockReadably(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0006")},
		intel.NameCVEDetail: {goldenDetailRow(t, "CVE-2026-0006")},
	})

	d := Describe(KindCVE, "CVE-2026-0006", set)

	if got := rowValue(d, "Weakness sources"); got != "security@example.com: CWE-20, CWE-502; NVD: CWE-917" {
		t.Errorf("weakness sources: %q", got)
	}

	wantAffected := []string{
		"Example Corp Log Lib: from 2.0 before 2.3.1 (unaffected from 2.1, affected from 2.2); unaffected: 2.15.0",
	}
	if !reflect.DeepEqual(d.Affected, wantAffected) {
		t.Errorf("affected:\n got %q\nwant %q", d.Affected, wantAffected)
	}

	wantConfigurations := []string{
		"example router firmware: before 2.7.0 (on example router)",
		"example log:lib: from 2.0 before 2.3.1, 2.13.0 beta1",
	}
	if !reflect.DeepEqual(d.Configurations, wantConfigurations) {
		t.Errorf("configurations:\n got %q\nwant %q", d.Configurations, wantConfigurations)
	}

	wantReferences := []Reference{
		{URL: "https://example.com/advisory?id=1&lang=en", Tags: "Vendor Advisory, Patch"},
		{URL: "http://example.org/exploit", Tags: "Exploit, Third Party Advisory"},
		{URL: "https://example.net/untagged"},
	}
	if !reflect.DeepEqual(d.References, wantReferences) {
		t.Errorf("references:\n got %+v\nwant %+v", d.References, wantReferences)
	}
}

var referenceLinkCases = []string{
	"javascript:alert(1)",
	"data:text/html,x",
	"ftp://example.com/f",
	"//no-scheme.example/x",
	"not a url",
	"HTTPS://Example.com/Upper",
	"https://example.com/a",
	"http://example.org/b",
}

var referenceLinksKept = []string{"HTTPS://Example.com/Upper", "https://example.com/a", "http://example.org/b"}

func TestEachProductInAConfigurationGetsItsOwnReadableLine(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0007")},
		intel.NameCVEDetail: {goldenDetailRow(t, "CVE-2026-0007")},
	})

	d := Describe(KindCVE, "CVE-2026-0007", set)

	want := []string{
		"cisco unified communications manager: before 11.5(1), 11.5(1)",
		"cisco dna spaces: connector: before 2.5",
	}
	if !reflect.DeepEqual(d.Configurations, want) {
		t.Fatalf("configurations:\n got %q\nwant %q", d.Configurations, want)
	}
}

func TestACPEThatNamesNoProductIsLeftOut(t *testing.T) {
	config := `[{"vulnerable":[{"cpe":"o:-:-:-"}]},` +
		`{"vulnerable":[{"cpe":"a:example:tool","before":"2.0"}],"on":[{"cpe":"h:-:-:1.0"},{"cpe":"h:example:box"}]}]`
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0001")},
		intel.NameCVEDetail: {detailRow("CVE-2026-0001", "", config, "", "")},
	})

	d := Describe(KindCVE, "CVE-2026-0001", set)

	if want := []string{"example tool: before 2.0 (on example box)"}; !reflect.DeepEqual(d.Configurations, want) {
		t.Fatalf("configurations %q, want %q", d.Configurations, want)
	}
}

func TestOnlyWebLinksSurviveAsReferences(t *testing.T) {
	var kept []string
	for _, candidate := range referenceLinkCases {
		if isWebURL(candidate) {
			kept = append(kept, candidate)
		}
	}

	if !reflect.DeepEqual(kept, referenceLinksKept) {
		t.Fatalf("kept %q, want %q; cyber.spec.ts holds the same table for the webapp's gate", kept, referenceLinksKept)
	}
}

func TestAReferenceListedTwiceIsShownOnce(t *testing.T) {
	refs := `[{"url":"javascript:alert(1)"},{"url":"https://example.com/a"},{"url":"https://example.com/a"}]`
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0001")},
		intel.NameCVEDetail: {detailRow("CVE-2026-0001", "", "", "", refs)},
	})

	d := Describe(KindCVE, "CVE-2026-0001", set)

	if want := []Reference{{URL: "https://example.com/a"}}; !reflect.DeepEqual(d.References, want) {
		t.Fatalf("references %+v, want the web link once", d.References)
	}
}

func TestPlatformsBeyondFiveAreCountedRatherThanListed(t *testing.T) {
	var on []string
	for _, model := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		on = append(on, `{"cpe":"h:example:model_`+model+`:-"}`)
	}
	config := `[{"vulnerable":[{"cpe":"o:example:firmware","before":"3.0"}],"on":[` + strings.Join(on, ",") + `]}]`
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0001")},
		intel.NameCVEDetail: {detailRow("CVE-2026-0001", "", config, "", "")},
	})

	d := Describe(KindCVE, "CVE-2026-0001", set)

	want := "example firmware: before 3.0 (on example model a, example model b, example model c, example model d, example model e and 2 more)"
	if len(d.Configurations) != 1 || d.Configurations[0] != want {
		t.Fatalf("configurations %q", d.Configurations)
	}
}

func TestAMatchWithNoVersionOrBoundSaysAllVersions(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0001")},
		intel.NameCVEDetail: {detailRow("CVE-2026-0001", "", `[{"vulnerable":[{"cpe":"a:example:tool"}]}]`, "", "")},
	})

	d := Describe(KindCVE, "CVE-2026-0001", set)

	if len(d.Configurations) != 1 || d.Configurations[0] != "example tool: all versions" {
		t.Fatalf("configurations %q", d.Configurations)
	}
}

func TestADetailRowTellsItsThreeFailuresApart(t *testing.T) {
	t.Run("no detail dataset says nothing extra", func(t *testing.T) {
		set := datasets(t, map[string][]string{intel.NameCVE: {cveOnly("CVE-2026-0001")}})
		if got := rowValue(Describe(KindCVE, "CVE-2026-0001", set), "Details"); got != "" {
			t.Fatalf("details row %q with no detail dataset installed", got)
		}
	})

	t.Run("a row the detail dataset lacks is named", func(t *testing.T) {
		set := datasets(t, map[string][]string{
			intel.NameCVE:       {cveOnly("CVE-2026-0001")},
			intel.NameCVEDetail: {detailRow("CVE-2026-0002", "", "", "", "")},
		})
		got := rowValue(Describe(KindCVE, "CVE-2026-0001", set), "Details")
		if got != "Not in the vulnerability detail dataset generated 2026-09-01T00:00:00Z." {
			t.Fatalf("details row %q", got)
		}
	})

	t.Run("no details row when the CVE itself is unknown", func(t *testing.T) {
		set := datasets(t, map[string][]string{
			intel.NameCVE:       {cveOnly("CVE-2026-0003")},
			intel.NameCVEDetail: {detailRow("CVE-2026-0003", "", "", "", "")},
		})
		if got := rowValue(Describe(KindCVE, "CVE-2026-0001", set), "Details"); got != "" {
			t.Fatalf("details row %q repeats what the status already says", got)
		}
	})

	t.Run("a malformed row is a read failure", func(t *testing.T) {
		set := datasets(t, map[string][]string{
			intel.NameCVE:       {cveOnly("CVE-2026-0001")},
			intel.NameCVEDetail: {detailRow("CVE-2026-0001", "", "not json", "", "")},
		})
		got := rowValue(Describe(KindCVE, "CVE-2026-0001", set), "Details")
		if got != "The vulnerability detail dataset is installed and could not be read. (TF-21005)" {
			t.Fatalf("details row %q", got)
		}
	})
}

func TestAHostileDetailRowCannotBecomeMarkupOrAScriptLink(t *testing.T) {
	affected := `[{"vendor":"<script>alert(1)</script>","product":"<img src=x onerror=alert(1)>",` +
		`"versions":[{"version":"<b>1</b>","status":"affected"}]}]`
	config := `[{"vulnerable":[{"cpe":"a:<svg onload=1>:x","before":"<i>2</i>"}]}]`
	refs := `[{"url":"javascript:alert(1)"},{"url":"https://evil.example/\"><script>alert(1)</script>",` +
		`"tags":["<b>Exploit</b>"]}]`
	set := datasets(t, map[string][]string{
		intel.NameCVE:       {cveOnly("CVE-2026-0001")},
		intel.NameCVEDetail: {detailRow("CVE-2026-0001", `[{"source":"<script>","cwe":["CWE-79"]}]`, config, affected, refs)},
	})

	body := renderBody(Describe(KindCVE, "CVE-2026-0001", set))

	allowed := map[string]bool{
		"p": true, "table": true, "tbody": true, "tr": true, "td": true, "h2": true, "ul": true,
		"li": true, "span": true, "a": true, "details": true, "summary": true, "/": true,
	}
	for _, tag := range elementNames(body) {
		if !allowed[tag] {
			t.Fatalf("a detail row opened a <%s> element:\n%s", tag, body)
		}
	}

	if strings.Contains(body, "javascript:") {
		t.Fatalf("a script link reached the page:\n%s", body)
	}
	if strings.Contains(body, `/"><`) {
		t.Fatalf("a URL broke out of its attribute:\n%s", body)
	}
	if !strings.Contains(body, "https://evil.example/&#34;&gt;&lt;script&gt;") {
		t.Fatalf("the hostile web link was dropped rather than escaped:\n%s", body)
	}
}

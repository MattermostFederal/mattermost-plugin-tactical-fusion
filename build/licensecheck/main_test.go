package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func testPolicy() policy {
	return policy{
		Allowed: []string{"MIT", "Apache-2.0", "BSD-3-Clause", "ISC"},
		Denied:  []string{"GPL", "LGPL", "AGPL", "MPL", "SSPL", "CC-BY-SA"},
		Exceptions: []exception{
			{Component: "github.com/hashicorp/go-plugin", License: "MPL-2.0", Reason: "the plugin transport"},
			{Component: "github.com/tinylib/msgp", License: "UNDETECTED", Reason: "MIT, below the detector's threshold"},
		},
	}
}

func declared(name, id string) component {
	c := component{Name: name, Version: "v1.0.0"}
	if id != "" {
		var e entry
		e.License.ID = id
		c.Licenses = []entry{e}
	}
	return c
}

func TestAPermissiveLicensePasses(t *testing.T) {
	for _, id := range []string{"MIT", "Apache-2.0", "BSD-3-Clause", "ISC"} {
		if got := judge(testPolicy(), declared("lib", id), "npm").Status; got != "allowed" {
			t.Errorf("%s judged %q, want allowed", id, got)
		}
	}
}

func TestEveryCopyleftFamilyIsDeniedWholeAndNotJustItsBareStem(t *testing.T) {
	// The SPDX ids for one family share a stem but never the whole string, so a
	// policy matched by equality would pass GPL-3.0-or-later while denying GPL.
	for _, id := range []string{
		"GPL-2.0", "GPL-3.0-only", "GPL-3.0-or-later",
		"LGPL-2.1-only", "AGPL-3.0-only", "MPL-2.0", "SSPL-1.0", "CC-BY-SA-4.0",
	} {
		if got := judge(testPolicy(), declared("lib", id), "npm").Status; got != "denied" {
			t.Errorf("%s judged %q, want denied", id, got)
		}
	}
}

func TestAnUnstatedLicenseIsForbiddenRatherThanAssumedPermissive(t *testing.T) {
	v := judge(testPolicy(), declared("mystery", ""), "npm")

	if v.Status != "unstated" {
		t.Errorf("a component with no license judged %q, want unstated", v.Status)
	}
	if len(v.Licenses) != 1 || v.Licenses[0] != undetected {
		t.Errorf("licenses = %v, want [%s]", v.Licenses, undetected)
	}
}

func TestALicenseThePolicyNeverNamedIsNotSilentlyAllowed(t *testing.T) {
	if got := judge(testPolicy(), declared("odd", "WTFPL"), "npm").Status; got != "unrecognized" {
		t.Errorf("WTFPL judged %q, want unrecognized", got)
	}
}

func TestAChoiceOfLicensesPassesOnAnyArmAndAConjunctionOnNone(t *testing.T) {
	// "(MIT OR Apache-2.0)" is a choice the consumer makes, so one good arm is
	// enough. "MIT AND GPL-3.0" binds you to both, so the GPL arm decides it.
	choice := component{Name: "choice", Version: "v1", Licenses: []entry{{Expression: "(MIT OR Apache-2.0)"}}}
	if got := judge(testPolicy(), choice, "npm").Status; got != "allowed" {
		t.Errorf("a disjunction judged %q, want allowed", got)
	}

	both := component{Name: "both", Version: "v1", Licenses: []entry{{Expression: "MIT AND GPL-3.0"}}}
	if got := judge(testPolicy(), both, "npm").Status; got != "denied" {
		t.Errorf("a conjunction carrying GPL judged %q, want denied", got)
	}

	noGood := component{Name: "neither", Version: "v1", Licenses: []entry{{Expression: "(GPL-3.0 OR AGPL-3.0)"}}}
	if got := judge(testPolicy(), noGood, "npm").Status; got != "denied" {
		t.Errorf("a disjunction of copyleft judged %q, want denied", got)
	}
}

func TestAnExceptionExcusesOneComponentUnderOneLicense(t *testing.T) {
	excused := judge(testPolicy(), declared("github.com/hashicorp/go-plugin", "MPL-2.0"), "go")
	if excused.Status != "excepted" {
		t.Fatalf("the excepted module judged %q, want excepted", excused.Status)
	}
	if excused.Reason == "" {
		t.Error("an excepted verdict carries no reason; the report would not say why")
	}

	// The same module under a worse license is a new decision, not a covered one.
	relicensed := judge(testPolicy(), declared("github.com/hashicorp/go-plugin", "AGPL-3.0-only"), "go")
	if relicensed.Status != "denied" {
		t.Errorf("the excepted module relicensed to AGPL judged %q, want denied", relicensed.Status)
	}

	// And the exception does not spread to its neighbors.
	other := judge(testPolicy(), declared("github.com/hashicorp/yamux", "MPL-2.0"), "go")
	if other.Status != "denied" {
		t.Errorf("an unexcepted MPL module judged %q, want denied", other.Status)
	}
}

func TestAnUndetectedLicenseCanBeExcusedOnce(t *testing.T) {
	v := judge(testPolicy(), declared("github.com/tinylib/msgp", ""), "go")
	if v.Status != "excepted" {
		t.Errorf("the excepted undetected module judged %q, want excepted", v.Status)
	}
}

func TestADetectedLicenseIsReadFromEvidenceAsWellAsFromTheDeclaration(t *testing.T) {
	// cyclonedx-gomod reports a detected license as evidence unless asked to
	// assert it. Reading only the declaration judged the whole Go tree unstated.
	var e entry
	e.License.ID = "MIT"
	c := component{Name: "lib", Version: "v1", Evidence: &evidence{Licenses: []entry{e}}}

	if got := judge(testPolicy(), c, "go").Status; got != "allowed" {
		t.Errorf("a license carried only as evidence judged %q, want allowed", got)
	}
}

func TestAnExceptionThatMatchesNothingFailsRatherThanOutlivingItsReason(t *testing.T) {
	pol := policy{
		Allowed:    []string{"MIT"},
		Denied:     []string{"GPL"},
		Exceptions: []exception{{Component: "github.com/gone/away", License: "MPL-2.0", Reason: "stale"}},
	}

	var out bytes.Buffer
	problems := report(&out, pol, []verdict{{Component: "lib", Status: "allowed"}})

	if problems != 1 {
		t.Fatalf("a stale exception produced %d problem(s), want 1", problems)
	}
	if !strings.Contains(out.String(), "github.com/gone/away") {
		t.Errorf("the failure never names the stale exception:\n%s", out.String())
	}
}

func TestAPassingRunSaysHowManyRodeOnAnException(t *testing.T) {
	var out bytes.Buffer
	problems := report(&out, testPolicy(), []verdict{
		{Component: "a", Status: "allowed"},
		{Component: "github.com/hashicorp/go-plugin", Status: "excepted"},
		{Component: "github.com/tinylib/msgp", Status: "excepted"},
	})

	if problems != 0 {
		t.Fatalf("a clean run produced %d problem(s):\n%s", problems, out.String())
	}
	if !strings.Contains(out.String(), "2 under a recorded exception") {
		t.Errorf("the summary hides the exception count:\n%s", out.String())
	}
}

func TestTheFailureNamesTheComponentAndItsLicense(t *testing.T) {
	var out bytes.Buffer
	report(&out, policy{Allowed: []string{"MIT"}, Denied: []string{"GPL"}}, []verdict{
		{Component: "copyleft-lib", Version: "v2.0.0", Licenses: []string{"GPL-3.0-or-later"}, Status: "denied"},
	})

	for _, want := range []string{"copyleft-lib", "v2.0.0", "GPL-3.0-or-later", ".licenses.json"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the failure never mentions %q:\n%s", want, out.String())
		}
	}
}

func TestAScopedPackageIsNamedByItsGroup(t *testing.T) {
	// The npm SBOM splits "@maplibre/mlt" into a group and a name, so an
	// exception written the way a human types it would never match.
	got := qualify(component{Group: "@maplibre", Name: "mlt"})
	if got != "@maplibre/mlt" {
		t.Errorf("qualify = %q, want @maplibre/mlt", got)
	}
}

func TestTheShippedListNarrowsTheModuleGraph(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/shipped.txt"
	if err := os.WriteFile(path, []byte("github.com/a/one\n\ngithub.com/b/two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	shipped, err := readShipped(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(shipped) != 2 || !shipped["github.com/a/one"] || !shipped["github.com/b/two"] {
		t.Errorf("shipped = %v, want the two named modules and nothing from the blank line", shipped)
	}

	if _, err := readShipped(""); err != nil {
		t.Errorf("an absent list should judge everything, got %v", err)
	}
}

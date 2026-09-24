package cyber

import (
	"strings"
	"testing"
)

func TestEveryCatalogIdIsWellFormedAndReachable(t *testing.T) {
	if TechniqueCount() == 0 || WeaknessCount() == 0 {
		t.Fatalf("a catalog is empty: %d techniques, %d weaknesses", TechniqueCount(), WeaknessCount())
	}

	for id := range techniques {
		kind, canonical, ok := Recognize(id)
		if !ok || kind != KindAttack || canonical != id {
			t.Errorf("%q does not reproduce itself through Recognize", id)
		}
	}

	for id := range weaknesses {
		kind, canonical, ok := Recognize(id)
		if !ok || kind != KindCWE || canonical != id {
			t.Errorf("%q does not reproduce itself through Recognize", id)
		}
	}
}

func TestEveryCatalogReferenceResolves(t *testing.T) {
	for id, technique := range techniques {
		if technique.Parent != "" {
			if _, ok := LookupTechnique(technique.Parent); !ok {
				t.Errorf("%s names parent %s, which is absent", id, technique.Parent)
			}
		}
		for _, tactic := range technique.Tactics {
			parent, ok := LookupTechnique(tactic)
			if !ok {
				t.Errorf("%s names tactic %s, which is absent", id, tactic)
				continue
			}
			if parent.Kind != techniqueKindTactic {
				t.Errorf("%s names %s as a tactic and it is a %s", id, tactic, parent.Kind)
			}
		}
	}

	for id, weakness := range weaknesses {
		for _, parent := range weakness.Parents {
			if _, ok := LookupWeakness(parent); !ok {
				t.Errorf("%s names parent %s, which is absent", id, parent)
			}
		}
	}
}

func TestEveryCatalogFieldPassesTheWhitelist(t *testing.T) {
	for id, technique := range techniques {
		for _, field := range []string{technique.Name, technique.Summary, technique.Status, technique.Kind} {
			if !validText(field) {
				t.Errorf("%s carries a refused character: %q", id, field)
			}
		}
	}

	for id, weakness := range weaknesses {
		for _, field := range []string{weakness.Name, weakness.Summary, weakness.Abstraction} {
			if !validText(field) {
				t.Errorf("%s carries a refused character: %q", id, field)
			}
		}
	}
}

func TestEverySubTechniqueHasItsParentsTactics(t *testing.T) {
	for id, technique := range techniques {
		if technique.Kind != techniqueKindSubTechnique {
			continue
		}

		parent, ok := LookupTechnique(technique.Parent)
		if !ok {
			continue
		}
		if strings.Join(technique.Tactics, ",") != strings.Join(parent.Tactics, ",") {
			t.Errorf("%s and its parent %s name different tactics", id, parent.ID)
		}
	}
}

func TestPinnedCatalogEntries(t *testing.T) {
	technique, ok := LookupTechnique("T1059.001")
	if !ok {
		t.Fatalf("T1059.001 is absent")
	}
	if technique.Name != "PowerShell" {
		t.Errorf("T1059.001 is named %q", technique.Name)
	}
	if technique.Parent != "T1059" {
		t.Errorf("T1059.001 has parent %q", technique.Parent)
	}
	if technique.Kind != techniqueKindSubTechnique {
		t.Errorf("T1059.001 is a %q", technique.Kind)
	}

	tactic, ok := LookupTechnique("TA0002")
	if !ok {
		t.Fatalf("TA0002 is absent")
	}
	if tactic.Name != "Execution" || tactic.Kind != techniqueKindTactic {
		t.Errorf("TA0002 is %q, a %q", tactic.Name, tactic.Kind)
	}

	weakness, ok := LookupWeakness("CWE-79")
	if !ok {
		t.Fatalf("CWE-79 is absent")
	}
	if !strings.Contains(weakness.Name, "Cross-site Scripting") {
		t.Errorf("CWE-79 is named %q", weakness.Name)
	}
}

func TestSubTechniquesAreListedUnderTheirParent(t *testing.T) {
	children := SubTechniquesOf("T1059")
	if len(children) == 0 {
		t.Fatalf("T1059 lists no sub-techniques")
	}

	previous := ""
	for _, child := range children {
		if child.Parent != "T1059" {
			t.Errorf("%s is listed under T1059 and names parent %s", child.ID, child.Parent)
		}
		if child.ID <= previous {
			t.Errorf("sub-techniques are out of order at %s", child.ID)
		}
		previous = child.ID
	}
}

func TestBadCatalogDataIsRefused(t *testing.T) {
	cases := map[string]string{
		"no rows":              "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\n",
		"wrong field count":    "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tShell\n",
		"unknown kind":         "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tShell\twidget\t\t\t\ts\tactive\t\n",
		"malformed id":         "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nX1059\tShell\ttechnique\t\t\t\ts\tactive\t\n",
		"missing name":         "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\t\ttechnique\t\t\t\ts\tactive\t\n",
		"dangling parent":      "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059.001\tPS\tsubtechnique\t\tT9999\t\ts\tactive\t\n",
		"dangling tactic":      "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tShell\ttechnique\tTA9999\t\t\ts\tactive\t\n",
		"refused character":    "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tShell\ttechnique\t\t\t\thttps://x\tactive\t\n",
		"duplicate id":         "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tA\ttechnique\t\t\t\ts\tactive\t\nT1059\tB\ttechnique\t\t\t\ts\tactive\t\n",
		"dangling replacement": "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nT1059\tShell\ttechnique\t\t\t\ts\trevoked\tT9999\n",
		"a tactic that isnt":   "id\tname\tkind\ttactics\tparent\tplatforms\tsummary\tstatus\treplaced_by\nTA0002\tExec\ttactic\t\t\t\ts\tactive\t\nT1059\tShell\ttechnique\tT1059\t\t\ts\tactive\t\n",
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTechniques(source); err == nil {
				t.Fatalf("accepted %q", source)
			}
		})
	}
}

func TestBadEmbeddedDataPanicsAtInit(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("bad data did not panic")
		}
	}()

	mustParseTechniques("id\tname\n")
}

func TestAttackURLIsDerivedFromTheId(t *testing.T) {
	cases := map[string]string{
		"T1059":     "https://attack.mitre.org/techniques/T1059/",
		"T1059.001": "https://attack.mitre.org/techniques/T1059/001/",
		"TA0002":    "https://attack.mitre.org/tactics/TA0002/",
	}

	for id, want := range cases {
		technique, ok := LookupTechnique(id)
		if !ok {
			t.Fatalf("%s is absent", id)
		}
		if got := AttackURL(technique); got != want {
			t.Errorf("AttackURL(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestTheCatalogAcceptsTheOperatorsMITRETextWrites(t *testing.T) {
	for _, field := range []string{
		"Path Equivalence: 'filedir*' (Wildcard)",
		"such as using == when the .equals() method should be used",
		"special characters such as <, >",
	} {
		if !validText(field) {
			t.Errorf("refused %q", field)
		}
	}
}

func TestTheCatalogStillRefusesAutolinksAndControlCharacters(t *testing.T) {
	for _, field := range []string{"see https://cwe.mitre.org", "www.example.com", "tab\there", "bell\a", "back`tick", "pipe|"} {
		if validText(field) {
			t.Errorf("accepted %q", field)
		}
	}
}

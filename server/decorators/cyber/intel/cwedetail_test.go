package intel

import (
	"reflect"
	"testing"
)

func TestTheReaderParsesTheCWEGoldenFileTheGeneratorWrites(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.CWEDetail("CWE-9001")
	if err != nil {
		t.Fatal(err)
	}

	want := CWEDetail{
		ID:          "CWE-9001",
		Description: "The product mishandles an invented input. A second sentence follows.",
		Extended:    "Background about the invented weakness, with a colon: here.",
		Consequences: []Consequence{
			{Scopes: []string{"Confidentiality", "Integrity"}, Impacts: []string{"Read Application Data", "Modify Memory"}, Likelihood: "High", Note: "An invented note."},
			{Scopes: []string{"Availability"}, Impacts: []string{"DoS: Crash, Exit, or Restart"}},
		},
		Mitigations: []Mitigation{
			{Phase: "Implementation", Strategy: "Input Validation", Description: "Hold every value in a smart pointer class such as std::auto_ptr before use.", Effectiveness: "High"},
			{Phase: "Architecture and Design", Description: "Pick a design that avoids it."},
		},
		Detections: []Detection{
			{Method: "Automated Static Analysis", Description: "Run an invented analyzer over the code.", Effectiveness: "Moderate"},
		},
		Examples: []Example{
			{ID: "CVE-2099-0001", Description: "Invented product mishandles the invented input."},
			{ID: "[REF-1]", Description: "An invented citation."},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the golden row read back as\n%+v\nwant\n%+v", got, want)
	}
}

func TestACWEDetailRowWithNoSectionsReadsAsEmpty(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.CWEDetail("CWE-9002")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extended != "" || got.Consequences != nil || got.Mitigations != nil || got.Detections != nil || got.Examples != nil {
		t.Fatalf("a row with no sections read back with some: %+v", got)
	}
	if got.Description == "" {
		t.Fatalf("the description was lost")
	}
}

package intel

import (
	"reflect"
	"testing"
)

func TestTheReaderParsesTheAttackGoldenFileTheGeneratorWrites(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.AttackDetail("T9001")
	if err != nil {
		t.Fatal(err)
	}

	want := AttackDetail{
		ID:          "T9001",
		Description: "Adversaries may invent things. See Invented Tool.",
		References: []AttackReference{
			{Source: "Invented Report", URL: "https://example.org/report", Description: "Invented, A. (2026). A report. Retrieved September 1, 2026."},
		},
		Mitigations: []AttackMitigation{
			{ID: "M9001", Name: "Invented Mitigation", URL: "https://attack.example/mitigations/M9001", Description: "Turn the invention off."},
		},
		Detections: []AttackDetection{{
			ID:   "DET9001",
			Name: "Invented Detection",
			URL:  "https://attack.example/detectionstrategies/DET9001",
			Analytics: []Analytic{{
				ID:          "AN9001",
				Platforms:   []string{"Linux"},
				Description: "Watch for invented events.",
				LogSources:  []LogSource{{Name: "auditd:SYSCALL", Channel: "execve"}},
				Tunables:    []Tunable{{Field: "Threshold", Description: "How many."}},
			}},
		}},
		Procedures: []Procedure{
			{ID: "G9001", Name: "Invented Group", Kind: "group", URL: "https://attack.example/groups/G9001", Description: "Invented Group has invented things."},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the golden row read back as\n%+v\nwant\n%+v", got, want)
	}
}

func TestAnAttackDetailRowWithOnlyADescriptionReadsAsEmpty(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.AttackDetail("TA9001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "The adversary is inventing." || got.References != nil || got.Mitigations != nil || got.Detections != nil || got.Procedures != nil {
		t.Fatalf("got %+v", got)
	}
}

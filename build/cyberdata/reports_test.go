package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const torFixture = "198.51.100.5\nnot an address\n"

const advisoryFixture = `{"type":"bundle","objects":[
{"type":"report","name":"AA99-001A Invented Ransomware Campaign","published":"2026-09-01T00:00:00Z"},
{"type":"indicator","name":"IPv4 Indicator","pattern":"[ipv4-addr:value = '203.0.113.10']","valid_from":"2026-08-15T00:00:00Z"},
{"type":"indicator","name":"File Indicator","pattern":"[(file:name = 'x.ps1') AND file:hashes.'SHA-256' = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA']","valid_from":"2026-08-15T00:00:00Z"},
{"type":"indicator","name":"Domain Indicator","pattern":"[domain-name:value = 'invented.example']","valid_from":"2026-08-15T00:00:00Z"}
]}`

func TestTheAdvisoryRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aa99-001a.json"), []byte(advisoryFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := buildAdvisory(dir)
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "advisory", rows)
}

func TestOneThreatIsOneReportWhateverPortItWasSeenOn(t *testing.T) {
	var index reportIndex
	index.add("203.0.113.10", threatReport{Source: "s", Threat: "Botnet C2", Malware: "M", Ports: "8443", Confidence: "75", FirstSeen: "2026-09-01", LastSeen: "2026-09-20"})
	index.add("203.0.113.10", threatReport{Source: "s", Threat: "Botnet C2", Malware: "M", Ports: "443", Confidence: "100", FirstSeen: "2026-08-01", LastSeen: "2026-09-10"})

	got := index.byKey["203.0.113.10"]
	want := []threatReport{{Source: "s", Threat: "Botnet C2", Malware: "M", Ports: "443,8443", Confidence: "100", FirstSeen: "2026-08-01", LastSeen: "2026-09-20"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestAnIndicatorKeyIsTheFormThePluginLooksUp(t *testing.T) {
	cases := map[string]string{
		"203.0.113.10":        "203.0.113.10",
		"::ffff:203.0.113.10": "203.0.113.10",
		"2001:DB8::7":         "2001:db8::7",
		strings.ToUpper(strings.Repeat("ab", 16)): strings.Repeat("ab", 16),
	}
	for value, want := range cases {
		if got, ok := indicatorKey(value); !ok || got != want {
			t.Errorf("indicatorKey(%q) = %q, %v, want %q", value, got, ok, want)
		}
	}
	for _, value := range []string{"invented.example", "abc", ""} {
		if _, ok := indicatorKey(value); ok {
			t.Errorf("indicatorKey accepted %q", value)
		}
	}
}

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const threatFoxFixture = `################################################################
# ThreatFox IOCs: full dump - CSV format                       #
################################################################
#
# "first_seen_utc","ioc_id","ioc_value","ioc_type","threat_type","fk_malware","malware_alias","malware_printable","last_seen_utc","confidence_level","is_compromised","reference","tags","anonymous","reporter"
"2026-09-01 10:00:00", "1", "203.0.113.10:443", "ip:port", "botnet_cc", "win.invented", "None", "InventedBot", "2026-09-20 11:00:00", "75", "False", "None", "c2", "0", "tester"
"2026-08-01 10:00:00", "2", "203.0.113.10:8443", "ip:port", "botnet_cc", "win.invented", "None", "InventedBot", "2026-09-10 11:00:00", "100", "False", "None", "c2", "0", "tester"
"2026-09-02 10:00:00", "3", "[2001:db8::7]:80", "ip:port", "payload_delivery", "unknown", "None", "Unknown malware", "", "50", "False", "None", "", "1", "anonymous"
"2026-09-03 10:00:00", "4", "ABCDEF0123456789ABCDEF0123456789", "md5_hash", "payload", "win.invented", "None", "InventedBot", "", "100", "False", "None", "", "0", "tester"
"2026-09-04 10:00:00", "5", "invented.example", "domain", "botnet_cc", "win.invented", "None", "InventedBot", "", "100", "False", "None", "", "0", "tester"
`

const feodoFixture = `################################################################
# abuse.ch Feodo Tracker Botnet C2 IP Blocklist (CSV)          #
################################################################
#
"first_seen_utc","dst_ip","dst_port","c2_status","last_online","malware"
"2026-01-01 00:00:00","::ffff:203.0.113.10","443","online","2026-09-21","InventedBot"
`

const torFixture = "198.51.100.5\nnot an address\n"

const advisoryFixture = `{"type":"bundle","objects":[
{"type":"report","name":"AA99-001A Invented Ransomware Campaign","published":"2026-09-01T00:00:00Z"},
{"type":"indicator","name":"IPv4 Indicator","pattern":"[ipv4-addr:value = '203.0.113.10']","valid_from":"2026-08-15T00:00:00Z"},
{"type":"indicator","name":"File Indicator","pattern":"[(file:name = 'x.ps1') AND file:hashes.'SHA-256' = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA']","valid_from":"2026-08-15T00:00:00Z"},
{"type":"indicator","name":"Domain Indicator","pattern":"[domain-name:value = 'invented.example']","valid_from":"2026-08-15T00:00:00Z"}
]}`

func writeThreatFixtures(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	archive, err := os.Create(filepath.Join(dir, threatFoxExport)) // #nosec G304 -- a test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("full.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(threatFoxFixture)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{feodoBlocklist: feodoFixture, torExitList: torFixture} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTheThreatRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	rows, err := buildThreat(writeThreatFixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "threat", rows)
}

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

func TestAThreatBuildWithNoFeedsIsRefused(t *testing.T) {
	if _, err := buildThreat(t.TempDir()); err == nil {
		t.Fatal("built a threat dataset from nothing")
	}
}

package main

import (
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func keyOf(t *testing.T, value string) string {
	t.Helper()
	key := netip.MustParseAddr(value).Unmap().As16()
	return hex.EncodeToString(key[:])
}

func writeWarninglistFixtures(t *testing.T) string {
	t.Helper()

	source := t.TempDir()
	misp := filepath.Join(source, "misp-warninglists")
	if err := os.MkdirAll(misp, 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(source, torExitList): "198.51.100.5\nnot an address\n2001:db8::5\n",
		filepath.Join(misp, "invented-cloud.json"): `{"name":"List of known Invented Cloud IP ranges","type":"cidr",` +
			`"list":["198.51.100.0/24","::ffff:203.0.113.0/120","2001:db8::/32","not a range"]}`,
		filepath.Join(misp, "invented-dns.json"):    `{"name":"List of known public DNS resolvers","type":"cidr","list":["198.51.100.53"]}`,
		filepath.Join(misp, "invented-hashes.json"): `{"name":"List of known hashes for empty files","type":"string","list":["D41D8CD98F00B204E9800998ECF8427E","3::"]}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return misp
}

func TestTheNetListRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	rows, err := buildNetLists(writeWarninglistFixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "netlists", rows)
}

func TestTheHashListRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	rows, err := buildHashLists(writeWarninglistFixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("rows %q", rows)
	}
	matchGolden(t, "hashlists", rows)
}

func TestOverlappingRangesSplitIntoSegmentsCarryingEveryListThatCoversThem(t *testing.T) {
	rows, err := buildNetLists(writeWarninglistFixtures(t))
	if err != nil {
		t.Fatal(err)
	}

	threats := func(value string) string {
		key := keyOf(t, value)
		for _, row := range rows {
			if row[0] <= key && key <= row[1] {
				return row[2]
			}
		}
		return ""
	}

	if got := threats("198.51.100.53"); !strings.Contains(got, "Invented Cloud IP ranges") || !strings.Contains(got, "Public DNS resolvers") {
		t.Errorf("198.51.100.53, inside both lists, carries %s", got)
	}
	if got := threats("198.51.100.5"); !strings.Contains(got, "Tor exit node") || !strings.Contains(got, "Invented Cloud") {
		t.Errorf("198.51.100.5, a Tor exit inside the cloud range, carries %s", got)
	}
	if got := threats("198.51.100.54"); strings.Contains(got, "Public DNS") || !strings.Contains(got, "Invented Cloud") {
		t.Errorf("198.51.100.54, just past the resolver, carries %s", got)
	}
	if got := threats("203.0.113.200"); !strings.Contains(got, "Invented Cloud") {
		t.Errorf("an IPv4-mapped prefix was not read as its IPv4 range: %s", got)
	}
	if got := threats("2001:db8:ffff::1"); !strings.Contains(got, "Invented Cloud") {
		t.Errorf("an IPv6 range was not kept: %s", got)
	}
	if got := threats("192.0.2.1"); got != "" {
		t.Errorf("an address no list names carries %s", got)
	}
}

func TestAdjacentSegmentsWithTheSameListsAreOneRow(t *testing.T) {
	report := threatReport{Source: "s", Category: categoryContext, Threat: "t"}
	var ranges []addressRange
	for _, cidr := range []string{"198.51.100.0/25", "198.51.100.128/25"} {
		r, ok := rangeOf(cidr)
		if !ok {
			t.Fatalf("%s did not parse", cidr)
		}
		r.report = report
		ranges = append(ranges, r)
	}

	rows, err := flattenRanges(ranges)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != keyOf(t, "198.51.100.0") || rows[0][1] != keyOf(t, "198.51.100.255") {
		t.Fatalf("rows %q", rows)
	}
}

func TestAListNameLosesItsListOfKnownPrefix(t *testing.T) {
	if got := mispReport(mispList{ID: "x", Name: "List of known Amazon AWS IP address ranges"}).Threat; got != "Amazon AWS IP address ranges" {
		t.Errorf("threat %q", got)
	}
}

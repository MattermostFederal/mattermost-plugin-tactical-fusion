package intel

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const mmdbMetadataMarker = "\xab\xcd\xefMaxMind.com"

func mmdbControl(t *testing.T, kind, size int) []byte {
	t.Helper()

	if size > 28+255 {
		t.Fatalf("the fixture builder only writes short values, asked for %d", size)
	}

	marker, extra := size, []byte(nil)
	if size >= 29 {
		marker, extra = 29, []byte{byte(size - 29)}
	}

	if kind <= 7 {
		return append([]byte{byte(kind<<5) | byte(marker)}, extra...)
	}

	return append([]byte{byte(marker), byte(kind - 7)}, extra...)
}

func mmdbString(t *testing.T, value string) []byte {
	return append(mmdbControl(t, 2, len(value)), value...)
}

func mmdbUint(t *testing.T, kind int, value uint64) []byte {
	var payload []byte
	for shift := 56; shift >= 0; shift -= 8 {
		b := byte(value >> uint(shift))
		if len(payload) == 0 && b == 0 {
			continue
		}
		payload = append(payload, b)
	}

	return append(mmdbControl(t, kind, len(payload)), payload...)
}

func mmdbMap(t *testing.T, pairs ...[]byte) []byte {
	if len(pairs)%2 != 0 {
		t.Fatalf("a map takes key and value pairs, got %d entries", len(pairs))
	}

	out := mmdbControl(t, 7, len(pairs)/2)
	for _, part := range pairs {
		out = append(out, part...)
	}

	return out
}

func mmdbArray(t *testing.T, entries ...[]byte) []byte {
	out := mmdbControl(t, 11, len(entries))
	for _, entry := range entries {
		out = append(out, entry...)
	}

	return out
}

func writeMMDB(t *testing.T, path, databaseType string, record []byte) {
	t.Helper()

	const nodeCount = 1

	tree := make([]byte, 6)
	pointer := make([]byte, 4)
	binary.BigEndian.PutUint32(pointer, nodeCount+16)
	copy(tree[0:3], pointer[1:])
	copy(tree[3:6], pointer[1:])

	file := append([]byte(nil), tree...)
	file = append(file, make([]byte, 16)...)
	file = append(file, record...)
	file = append(file, mmdbMetadataMarker...)
	file = append(file, mmdbMap(t,
		mmdbString(t, "binary_format_major_version"), mmdbUint(t, 5, 2),
		mmdbString(t, "binary_format_minor_version"), mmdbUint(t, 5, 0),
		mmdbString(t, "build_epoch"), mmdbUint(t, 9, 1758499200),
		mmdbString(t, "database_type"), mmdbString(t, databaseType),
		mmdbString(t, "description"), mmdbMap(t, mmdbString(t, "en"), mmdbString(t, "a fixture")),
		mmdbString(t, "ip_version"), mmdbUint(t, 5, 6),
		mmdbString(t, "languages"), mmdbArray(t, mmdbString(t, "en")),
		mmdbString(t, "node_count"), mmdbUint(t, 6, nodeCount),
		mmdbString(t, "record_size"), mmdbUint(t, 5, 24),
	)...)

	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func asnRecord(t *testing.T, number uint64, organization string) []byte {
	return mmdbMap(t,
		mmdbString(t, "autonomous_system_number"), mmdbUint(t, 6, number),
		mmdbString(t, "autonomous_system_organization"), mmdbString(t, organization),
	)
}

func cityRecord(t *testing.T, country, region, city string) []byte {
	return mmdbMap(t,
		mmdbString(t, "city"), mmdbMap(t,
			mmdbString(t, "names"), mmdbMap(t, mmdbString(t, "en"), mmdbString(t, city))),
		mmdbString(t, "country"), mmdbMap(t, mmdbString(t, "iso_code"), mmdbString(t, country)),
		mmdbString(t, "subdivisions"), mmdbArray(t, mmdbMap(t,
			mmdbString(t, "names"), mmdbMap(t, mmdbString(t, "en"), mmdbString(t, region)))),
	)
}

func TestTheDatabaseTypeDecidesWhatIsRead(t *testing.T) {
	cases := map[string]string{
		"GeoLite2-ASN":                        mmdbKindASN,
		"GeoIP2-ISP":                          "",
		"DBIP-ASN-Lite (compat=GeoLite2-ASN)": mmdbKindASN,
		"GeoLite2-City":                       mmdbKindCity,
		"DBIP-City-Lite":                      mmdbKindCity,
		"GeoLite2-Country":                    mmdbKindCountry,
		"dbip-country-lite":                   mmdbKindCountry,
		"GeoIP2-Domain":                       "",
		"":                                    "",
	}

	for databaseType, want := range cases {
		t.Run(databaseType, func(t *testing.T) {
			if got := classifyMMDB(databaseType); got != want {
				t.Fatalf("%q classified as %q, want %q", databaseType, got, want)
			}
		})
	}
}

func TestAVendorDatabaseDescribesAnAddress(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))

	set := openIn(t, dir)

	for _, addr := range []string{"8.8.8.8", "2001:db8::1"} {
		t.Run(addr, func(t *testing.T) {
			record, err := set.IP(netip.MustParseAddr(addr))
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if record.ASN != "AS15169" || record.ASName != "Google LLC" {
				t.Fatalf("read back as %+v", record)
			}
			if len(record.Sources) != 1 || !strings.HasPrefix(record.Sources[0], "GeoLite2-ASN") {
				t.Fatalf("sources %v", record.Sources)
			}
		})
	}
}

func TestTwoVendorDatabasesFillDifferentFields(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))
	writeMMDB(t, filepath.Join(dir, "GeoLite2-City"+MMDBSuffix), "GeoLite2-City", cityRecord(t, "US", "California", "Mountain View"))

	set := openIn(t, dir)

	record, err := set.IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if record.ASN != "AS15169" || record.Country != "US" || record.Region != "California" || record.City != "Mountain View" {
		t.Fatalf("read back as %+v", record)
	}
	if len(record.Sources) != 2 {
		t.Fatalf("sources %v, want both databases named", record.Sources)
	}
}

func TestTheRangeFileFillsOnlyWhatTheVendorLeftEmpty(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))
	writeDataset(t, dir, NameIP,
		ipRow(netip.MustParseAddr("8.8.8.0"), netip.MustParseAddr("8.8.8.255"),
			"AS64496", "Somebody Else", "US", "California", "Mountain View"),
	)

	set := openIn(t, dir)

	record, err := set.IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if record.ASN != "AS15169" || record.ASName != "Google LLC" {
		t.Fatalf("the range file overwrote the vendor database: %+v", record)
	}
	if record.City != "Mountain View" {
		t.Fatalf("the range file did not fill the empty fields: %+v", record)
	}
	if len(record.Sources) != 2 {
		t.Fatalf("sources %v, want both named", record.Sources)
	}
}

func TestAVendorDatabaseThisBuildCannotUseIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GeoIP2-Domain"+MMDBSuffix)
	writeMMDB(t, path, "GeoIP2-Domain", mmdbMap(t, mmdbString(t, "domain"), mmdbString(t, "example.com")))

	_, problems := Open([]string{dir})

	if len(problems) != 1 || problems[0].Class != ErrorMMDB {
		t.Fatalf("problems %v", problems)
	}

	var unknown *UnknownMMDBError
	if !errors.As(problems[0].Err, &unknown) {
		t.Fatalf("the failure did not name the declared type: %v", problems[0].Err)
	}
	if unknown.DatabaseType != "GeoIP2-Domain" {
		t.Fatalf("named %q", unknown.DatabaseType)
	}
	if !strings.Contains(unknown.Error(), "GeoIP2-Domain") {
		t.Fatalf("the message does not name the type: %s", unknown.Error())
	}
}

func TestAFileThatIsNotADatabaseIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), []byte("not a database"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if len(problems) != 1 || problems[0].Class != ErrorMMDB {
		t.Fatalf("problems %v", problems)
	}
	if record, _ := set.IP(netip.MustParseAddr("8.8.8.8")); !record.Empty() {
		t.Fatalf("a refused database still described an address: %+v", record)
	}
}

func TestADBIPDatabaseCreditsDBIPWhenItAnswers(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "dbip-city-lite"+MMDBSuffix), "DBIP-City-Lite", cityRecord(t, "US", "California", "Mountain View"))

	record, err := openIn(t, dir).IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	want := []Attribution{{Text: "IP Geolocation by DB-IP", URL: "https://db-ip.com"}}
	if !reflect.DeepEqual(record.Attributions, want) {
		t.Fatalf("attributions %+v, want %+v", record.Attributions, want)
	}
}

func TestEachVendorIsCreditedOnceAndARangeFileIsNotCredited(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))
	writeMMDB(t, filepath.Join(dir, "GeoLite2-City"+MMDBSuffix), "GeoLite2-City", cityRecord(t, "US", "California", "Mountain View"))
	writeDataset(t, dir, NameIP,
		ipRow(netip.MustParseAddr("8.8.8.0"), netip.MustParseAddr("8.8.8.255"), "AS15169", "GOOGLE", "US", "", ""),
	)

	record, err := openIn(t, dir).IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	if len(record.Attributions) != 1 || record.Attributions[0].URL != "https://www.maxmind.com" {
		t.Fatalf("attributions %+v, want MaxMind once", record.Attributions)
	}
}

func TestAVendorThatAnsweredNothingIsNotCredited(t *testing.T) {
	dir := t.TempDir()
	writeMMDB(t, filepath.Join(dir, "dbip-city-lite"+MMDBSuffix), "DBIP-City-Lite", cityRecord(t, "", "", ""))
	writeDataset(t, dir, NameIP,
		ipRow(netip.MustParseAddr("8.8.8.0"), netip.MustParseAddr("8.8.8.255"), "AS15169", "GOOGLE", "US", "", ""),
	)

	record, err := openIn(t, dir).IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(record.Attributions) != 0 {
		t.Fatalf("a database that filled nothing was credited: %+v", record.Attributions)
	}
}

func TestAnUnknownVendorGetsNoInventedCredit(t *testing.T) {
	if got := attributionFor("Acme-City"); got != nil {
		t.Fatalf("attributionFor invented %+v", got)
	}
	if got := attributionFor("GeoIP2-City"); got != nil {
		t.Fatalf("a paid MaxMind database was given GeoLite2's credit: %+v", got)
	}
}

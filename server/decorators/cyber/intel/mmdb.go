package intel

import (
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

const MMDBSuffix = ".mmdb"

type mmdbReader struct {
	path   string
	kind   string
	reader *maxminddb.Reader
}

const (
	mmdbKindASN     = "asn"
	mmdbKindCity    = "city"
	mmdbKindCountry = "country"
)

func classifyMMDB(databaseType string) string {
	lowered := strings.ToLower(databaseType)

	switch {
	case strings.Contains(lowered, "asn"):
		return mmdbKindASN
	case strings.Contains(lowered, "city"):
		return mmdbKindCity
	case strings.Contains(lowered, "country"):
		return mmdbKindCountry
	}

	return ""
}

func openMMDB(path string) (*mmdbReader, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}

	kind := classifyMMDB(reader.Metadata.DatabaseType)
	if kind == "" {
		_ = reader.Close()
		return nil, &UnknownMMDBError{Path: path, DatabaseType: reader.Metadata.DatabaseType}
	}

	return &mmdbReader{path: path, kind: kind, reader: reader}, nil
}

type UnknownMMDBError struct {
	Path         string
	DatabaseType string
}

func (e *UnknownMMDBError) Error() string {
	return "cyber: " + filepath.Base(e.Path) + " declares database type " + strconv.Quote(e.DatabaseType) + ", which this build does not read"
}

func (m *mmdbReader) close() {
	if m.reader != nil {
		_ = m.reader.Close()
	}
}

func (m *mmdbReader) enrich(addr netip.Addr, into *IPRecord) {
	result := m.reader.Lookup(addr)
	if !result.Found() || result.Err() != nil {
		return
	}

	switch m.kind {
	case mmdbKindASN:
		var number uint
		if err := result.DecodePath(&number, "autonomous_system_number"); err == nil && number != 0 && into.ASN == "" {
			into.ASN = "AS" + strconv.FormatUint(uint64(number), 10)
		}

		var organization string
		if err := result.DecodePath(&organization, "autonomous_system_organization"); err == nil && into.ASName == "" {
			into.ASName = organization
		}

	case mmdbKindCity, mmdbKindCountry:
		var country string
		if err := result.DecodePath(&country, "country", "iso_code"); err == nil && into.Country == "" {
			into.Country = country
		}

		var region string
		if err := result.DecodePath(&region, "subdivisions", 0, "names", "en"); err == nil && into.Region == "" {
			into.Region = region
		}

		var city string
		if err := result.DecodePath(&city, "city", "names", "en"); err == nil && into.City == "" {
			into.City = city
		}
	}
}

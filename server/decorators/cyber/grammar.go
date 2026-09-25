package cyber

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

const (
	trailingPunctuation = `[.,;:!?]?`

	cveBody    = `(?i:CVE)-\d{4}-\d{4,7}`
	cweBody    = `(?i:CWE)-\d{1,5}`
	attackBody = `TA\d{4}|T\d{4}(?:\.\d{3})?`

	ipv4Body = `\d{1,3}(?:\.\d{1,3}){3}`
	ipv4Port = `(?::\d{1,5})?`

	ipv6Body = `[0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7}`
	ipv6Zone = `(?:%[\w.\-]+)?`

	md5Body    = `[0-9A-Fa-f]{32}`
	sha1Body   = `[0-9A-Fa-f]{40}`
	sha256Body = `[0-9A-Fa-f]{64}`
	hashBody   = sha256Body + `|` + sha1Body + `|` + md5Body

	labelSeparator = `[ \t]*[:=][ \t]*`
)

const minimumCVEYear = 1999

var (
	cveScan    = regexp.MustCompile(`(` + cveBody + `)` + trailingPunctuation)
	cweScan    = regexp.MustCompile(`(` + cweBody + `)` + trailingPunctuation)
	attackScan = regexp.MustCompile(`(` + attackBody + `)` + trailingPunctuation)

	ipv4Scan = regexp.MustCompile(`(` + ipv4Body + `)` + ipv4Port + trailingPunctuation)
	ipv6Scan = regexp.MustCompile(`(` + ipv6Body + `)` + ipv6Zone + trailingPunctuation)

	hashScan        = regexp.MustCompile(`(` + hashBody + `)` + trailingPunctuation)
	md5LabelScan    = regexp.MustCompile(`(?i:md5)` + labelSeparator + `(` + md5Body + `)` + trailingPunctuation)
	sha1LabelScan   = regexp.MustCompile(`(?i:sha-?1)` + labelSeparator + `(` + sha1Body + `)` + trailingPunctuation)
	sha256LabelScan = regexp.MustCompile(`(?i:sha-?256)` + labelSeparator + `(` + sha256Body + `)` + trailingPunctuation)
)

var (
	cveToken    = regexp.MustCompile(`^(?:` + cveBody + `)$`)
	cweToken    = regexp.MustCompile(`^(?:` + cweBody + `)$`)
	attackToken = regexp.MustCompile(`^(?:` + attackBody + `)$`)
	ipToken     = regexp.MustCompile(`^(?:` + ipv4Body + `|` + ipv6Body + `)$`)
	hashToken   = regexp.MustCompile(`^(?:` + hashBody + `)$`)
)

const (
	cveShape    = `CVE-\d{4}-\d{4,7}`
	cweShape    = `CWE-[1-9]\d{0,4}`
	attackShape = `TA\d{4}|T\d{4}(?:\.\d{3})?`
	ipShape     = `[0-9a-f.:]{2,45}`
	hashShape   = `[0-9a-f]{32}|[0-9a-f]{40}|[0-9a-f]{64}`
)

var shapeExprs = map[Kind]string{
	KindCVE:    cveShape,
	KindCWE:    cweShape,
	KindAttack: attackShape,
	KindIP:     ipShape,
	KindHash:   hashShape,
}

var shapes = func() map[Kind]*regexp.Regexp {
	compiled := make(map[Kind]*regexp.Regexp, len(shapeExprs))
	for kind, expr := range shapeExprs {
		compiled[kind] = regexp.MustCompile(`^(?:` + expr + `)$`)
	}
	return compiled
}()

func ShapeExpr(kind Kind) string { return shapeExprs[kind] }

func MatchesShape(kind Kind, value string) bool {
	shape, ok := shapes[kind]
	return ok && shape.MatchString(value)
}

func Recognize(value string) (Kind, string, bool) {
	switch {
	case cveToken.MatchString(value):
		canonical, ok := canonicalCVE(value)
		return KindCVE, canonical, ok

	case cweToken.MatchString(value):
		canonical, ok := canonicalCWE(value)
		return KindCWE, canonical, ok

	case attackToken.MatchString(value):
		canonical, ok := canonicalAttack(value)
		return KindAttack, canonical, ok

	case ipToken.MatchString(value):
		canonical, ok := canonicalIP(value)
		return KindIP, canonical, ok

	case hashToken.MatchString(value):
		canonical, ok := canonicalHash(value)
		return KindHash, canonical, ok
	}

	return "", "", false
}

func RecognizeAs(kind Kind, value string) bool {
	if !kind.Known() || !MatchesShape(kind, value) {
		return false
	}

	derivedKind, canonical, ok := Recognize(value)

	return ok && derivedKind == kind && canonical == value
}

func canonicalCVE(value string) (string, bool) {
	upper := strings.ToUpper(value)

	year, err := strconv.Atoi(upper[4:8])
	if err != nil || year < minimumCVEYear {
		return "", false
	}

	return upper, true
}

func canonicalCWE(value string) (string, bool) {
	number, err := strconv.Atoi(value[4:])
	if err != nil || number < 1 {
		return "", false
	}

	id := "CWE-" + strconv.Itoa(number)
	if _, known := LookupWeakness(id); !known {
		return "", false
	}

	return id, true
}

func canonicalAttack(value string) (string, bool) {
	if _, known := LookupTechnique(value); !known {
		return "", false
	}

	return value, true
}

func canonicalIP(value string) (string, bool) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", false
	}
	if addr.Zone() != "" || addr.Is4In6() || addr.IsUnspecified() || addr.IsLoopback() {
		return "", false
	}

	return addr.String(), true
}

func canonicalHash(value string) (string, bool) {
	return strings.ToLower(value), true
}

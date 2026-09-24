package cyber

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

//go:embed data/attack.tsv
var attackTSV string

//go:embed data/cwe.tsv
var cweTSV string

type Technique struct {
	ID         string
	Name       string
	Kind       string
	Tactics    []string
	Parent     string
	Platforms  []string
	Summary    string
	Status     string
	ReplacedBy string
}

type Weakness struct {
	ID          string
	Name        string
	Abstraction string
	Status      string
	Summary     string
	Parents     []string
}

const attackColumns = 9

const (
	StatusActive     = "active"
	StatusRevoked    = "revoked"
	StatusDeprecated = "deprecated"

	techniqueKindTactic       = "tactic"
	techniqueKindTechnique    = "technique"
	techniqueKindSubTechnique = "subtechnique"
)

var (
	techniques = mustParseTechniques(attackTSV)
	weaknesses = mustParseWeaknesses(cweTSV)
	children   = indexChildren(techniques)

	attackCompiled = catalogCompiled(attackTSV)
	cweCompiled    = catalogCompiled(cweTSV)
)

func catalogCompiled(source string) time.Time {
	stamp, _, stamped := catalogStamp(source)
	if !stamped {
		return time.Time{}
	}
	fields := strings.Split(stamp, "\t")
	if len(fields) < 3 {
		return time.Time{}
	}
	compiled, err := time.Parse(time.RFC3339, fields[2])
	if err != nil {
		return time.Time{}
	}
	return compiled.UTC()
}

func catalogStamp(source string) (stamp, rest string, stamped bool) {
	if !strings.HasPrefix(source, intel.SchemaPrefix) {
		return "", source, false
	}
	stamp, rest, _ = strings.Cut(source, "\n")
	return stamp, rest, true
}

func LookupTechnique(id string) (Technique, bool) {
	t, ok := techniques[id]
	return t, ok
}

func LookupWeakness(id string) (Weakness, bool) {
	w, ok := weaknesses[id]
	return w, ok
}

func SubTechniquesOf(id string) []Technique {
	found := make([]Technique, 0, len(children[id]))
	for _, child := range children[id] {
		found = append(found, techniques[child])
	}

	return found
}

func TechniqueCount() int { return len(techniques) }

func WeaknessCount() int { return len(weaknesses) }

const allowedPunctuation = " _-,.'\"()[]/&+:;*=<>\\~$"

var autolinkTriggers = []string{"www.", "://"}

func validText(field string) bool {
	lowered := strings.ToLower(field)
	for _, trigger := range autolinkTriggers {
		if strings.Contains(lowered, trigger) {
			return false
		}
	}

	for _, r := range field {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			continue
		}
		if !strings.ContainsRune(allowedPunctuation, r) {
			return false
		}
	}

	return true
}

func mustParseTechniques(source string) map[string]Technique {
	parsed, err := parseTechniques(source)
	if err != nil {
		panic("cyber: " + err.Error())
	}
	return parsed
}

func mustParseWeaknesses(source string) map[string]Weakness {
	parsed, err := parseWeaknesses(source)
	if err != nil {
		panic("cyber: " + err.Error())
	}
	return parsed
}

func catalogRows(source string, want int) ([][]string, error) {
	_, body, _ := catalogStamp(source)
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("the catalog is empty")
	}

	rows := make([][]string, 0, len(lines)-1)
	for number, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != want {
			return nil, fmt.Errorf("line %d has %d fields, want %d", number+2, len(fields), want)
		}
		for _, field := range fields {
			if !validText(field) {
				return nil, fmt.Errorf("line %d carries a character the whitelist refuses: %q", number+2, field)
			}
		}
		rows = append(rows, fields)
	}

	return rows, nil
}

func splitList(field string) []string {
	if field == "" {
		return nil
	}
	return strings.Split(field, ",")
}

func parseTechniques(source string) (map[string]Technique, error) {
	rows, err := catalogRows(source, attackColumns)
	if err != nil {
		return nil, fmt.Errorf("reading the ATT&CK catalog: %w", err)
	}

	out := make(map[string]Technique, len(rows))
	for _, row := range rows {
		t := Technique{
			ID:         row[0],
			Name:       row[1],
			Kind:       row[2],
			Tactics:    splitList(row[3]),
			Parent:     row[4],
			Platforms:  splitList(row[5]),
			Summary:    row[6],
			Status:     row[7],
			ReplacedBy: row[8],
		}

		switch t.Kind {
		case techniqueKindTactic, techniqueKindTechnique, techniqueKindSubTechnique:
		default:
			return nil, fmt.Errorf("%s: unknown kind %q", t.ID, t.Kind)
		}

		if !attackToken.MatchString(t.ID) {
			return nil, fmt.Errorf("%q is not a well-formed ATT&CK id", t.ID)
		}
		if t.Name == "" {
			return nil, fmt.Errorf("%s: no name", t.ID)
		}
		if _, dup := out[t.ID]; dup {
			return nil, fmt.Errorf("duplicate id %q", t.ID)
		}

		out[t.ID] = t
	}

	for _, t := range out {
		if t.ReplacedBy != "" {
			if _, ok := out[t.ReplacedBy]; !ok {
				return nil, fmt.Errorf("%s is replaced by %s, which the catalog does not hold", t.ID, t.ReplacedBy)
			}
		}
		if t.Parent != "" {
			if _, ok := out[t.Parent]; !ok {
				return nil, fmt.Errorf("%s names parent %s, which the catalog does not hold", t.ID, t.Parent)
			}
		}
		for _, tactic := range t.Tactics {
			parent, ok := out[tactic]
			if !ok {
				return nil, fmt.Errorf("%s names tactic %s, which the catalog does not hold", t.ID, tactic)
			}
			if parent.Kind != techniqueKindTactic {
				return nil, fmt.Errorf("%s names %s as a tactic, and it is a %s", t.ID, tactic, parent.Kind)
			}
		}
	}

	return out, nil
}

func parseWeaknesses(source string) (map[string]Weakness, error) {
	rows, err := catalogRows(source, 6)
	if err != nil {
		return nil, fmt.Errorf("reading the CWE catalog: %w", err)
	}

	out := make(map[string]Weakness, len(rows))
	for _, row := range rows {
		w := Weakness{
			ID:          row[0],
			Name:        row[1],
			Abstraction: row[2],
			Status:      row[3],
			Summary:     row[4],
			Parents:     splitList(row[5]),
		}

		if !MatchesShape(KindCWE, w.ID) {
			return nil, fmt.Errorf("%q is not a well-formed CWE id", w.ID)
		}
		if w.Name == "" {
			return nil, fmt.Errorf("%s: no name", w.ID)
		}
		if _, dup := out[w.ID]; dup {
			return nil, fmt.Errorf("duplicate id %q", w.ID)
		}

		out[w.ID] = w
	}

	for _, w := range out {
		for _, parent := range w.Parents {
			if _, ok := out[parent]; !ok {
				return nil, fmt.Errorf("%s names parent %s, which the catalog does not hold", w.ID, parent)
			}
		}
	}

	return out, nil
}

func indexChildren(all map[string]Technique) map[string][]string {
	index := map[string][]string{}
	for id, t := range all {
		if t.Parent != "" {
			index[t.Parent] = append(index[t.Parent], id)
		}
	}
	for _, ids := range index {
		sort.Strings(ids)
	}

	return index
}

package cyber

import (
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Freshness struct {
	Label    string
	Compiled time.Time
}

const (
	attackCatalogLabel = "MITRE ATT&CK catalog"
	cweCatalogLabel    = "MITRE CWE catalog"
)

var freshnessDatasets = map[Kind][]string{
	KindCVE:    {intel.NameCVE, intel.NameCVEDetail, intel.NameEPSS, intel.NameKEV},
	KindCWE:    {intel.NameCWEDetail},
	KindAttack: {intel.NameAttackDetail},
	KindIP:     {intel.NameIP, intel.NameAdvisory, intel.NameThreat},
	KindHash:   {intel.NameMalware, intel.NameAdvisory, intel.NameThreat},
}

func freshnessFor(kind Kind, set *intel.Set) []Freshness {
	var sources []Freshness
	addSource := func(label string, compiled time.Time) {
		if !compiled.IsZero() {
			sources = append(sources, Freshness{Label: label, Compiled: compiled})
		}
	}

	switch kind {
	case KindCWE:
		addSource(cweCatalogLabel, cweCompiled)
	case KindAttack:
		addSource(attackCatalogLabel, attackCompiled)
	}

	for _, name := range append(freshnessDatasets[kind], intel.NameWatchlist) {
		if !set.Has(name) {
			continue
		}
		compiled, err := time.Parse(time.RFC3339, set.Generated(name))
		if err != nil {
			continue
		}
		addSource(datasetLabels[name], compiled.UTC())
	}

	if kind == KindIP {
		for _, database := range set.DatabaseBuilds() {
			addSource(database.Name, database.Built)
		}
	}

	return sources
}

func OldestCompiled(sources []Freshness) (time.Time, bool) {
	var oldest time.Time
	for _, source := range sources {
		if oldest.IsZero() || source.Compiled.Before(oldest) {
			oldest = source.Compiled
		}
	}
	return oldest, !oldest.IsZero()
}

func CompiledText(at time.Time) string {
	return at.UTC().Round(time.Minute).Format("2006-01-02 15:04") + " UTC"
}

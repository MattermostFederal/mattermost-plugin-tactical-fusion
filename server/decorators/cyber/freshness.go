package cyber

import (
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Freshness struct {
	Label    string
	File     string
	Compiled time.Time
}

const (
	attackCatalogLabel = "MITRE ATT&CK catalog"
	cweCatalogLabel    = "MITRE CWE catalog"
	vendorIPLabel      = "vendor IP database"
	builtInSuffix      = " (built in)"
)

var freshnessDatasets = map[Kind][]string{
	KindCVE:    {intel.NameCVE, intel.NameCVEDetail, intel.NameEPSS, intel.NameKEV, intel.NameCVEAttack},
	KindCWE:    {intel.NameCWEDetail, intel.NameCAPEC},
	KindAttack: {intel.NameAttackDetail, intel.NameCAPEC, intel.NameCVEAttack},
	KindIP:     {intel.NameIP, intel.NameAdvisory},
	KindHash:   {intel.NameAdvisory},
}

func freshnessFor(kind Kind, set *intel.Set) []Freshness {
	var sources []Freshness
	addSource := func(label, file string, compiled time.Time) {
		if !compiled.IsZero() {
			sources = append(sources, Freshness{Label: label, File: file, Compiled: compiled})
		}
	}

	switch kind {
	case KindCWE:
		addSource(cweCatalogLabel, "cwe.tsv"+builtInSuffix, cweCompiled)
	case KindAttack:
		addSource(attackCatalogLabel, "attack.tsv"+builtInSuffix, attackCompiled)
	}

	for _, name := range append(freshnessDatasets[kind], intel.NameWatchlist) {
		if !set.Has(name) {
			continue
		}
		compiled, err := time.Parse(time.RFC3339, set.Generated(name))
		if err != nil {
			continue
		}
		addSource(datasetLabels[name], set.FileName(name), compiled.UTC())
	}

	if kind == KindIP {
		for _, database := range set.DatabaseBuilds() {
			addSource(vendorIPLabel, database.Name, database.Built)
		}
	}

	return sources
}

func CompiledText(at time.Time) string {
	return at.UTC().Round(time.Minute).Format("2006-01-02 15:04") + " UTC"
}

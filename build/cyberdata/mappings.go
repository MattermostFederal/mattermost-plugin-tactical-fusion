package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type capecPattern struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Abstraction string `json:"abstraction,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Likelihood  string `json:"likelihood,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

var retiredCAPECStatuses = map[string]bool{"Deprecated": true, "Obsolete": true}

const capecAttackTaxonomy = "TAXONOMY NAME:ATTACK:ENTRY ID:"

func buildCAPEC(source string) ([][]string, error) {
	records, err := readCSVFields(source, "the CAPEC export")
	if err != nil {
		return nil, err
	}

	byKey := map[string][]capecPattern{}
	order := map[string]int{}
	for _, fields := range records {
		number := clean(fields["id"])
		if number == "" || retiredCAPECStatuses[clean(fields["status"])] {
			continue
		}
		numeric, err := strconv.Atoi(number)
		if err != nil {
			return nil, fmt.Errorf("CAPEC id %q is not a number", number)
		}

		pattern := capecPattern{
			ID:          "CAPEC-" + number,
			Name:        clean(typographicPunctuation.Replace(fields["name"])),
			Abstraction: clean(fields["abstraction"]),
			Severity:    clean(fields["typical severity"]),
			Likelihood:  clean(fields["likelihood of attack"]),
			Summary:     firstSentence(typographicPunctuation.Replace(fields["description"])),
		}
		order[pattern.ID] = numeric

		for _, key := range append(capecWeaknesses(fields["related weaknesses"]), capecTechniques(fields["taxonomy mappings"])...) {
			if !slices.ContainsFunc(byKey[key], func(p capecPattern) bool { return p.ID == pattern.ID }) {
				byKey[key] = append(byKey[key], pattern)
			}
		}
	}

	rows := make([][]string, 0, len(byKey))
	for key, patterns := range byKey {
		slices.SortFunc(patterns, func(a, b capecPattern) int { return order[a.ID] - order[b.ID] })
		packed, err := compactJSON(patterns)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []string{key, packed})
	}
	return rows, nil
}

func capecWeaknesses(field string) []string {
	var ids []string
	for _, part := range strings.Split(field, "::") {
		if part = strings.TrimSpace(part); part != "" {
			if _, err := strconv.Atoi(part); err == nil {
				ids = append(ids, "CWE-"+part)
			}
		}
	}
	return ids
}

func capecTechniques(field string) []string {
	var ids []string
	for _, entry := range strings.Split(field, "::") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(entry), capecAttackTaxonomy)
		if !ok {
			continue
		}
		id, _, _ := strings.Cut(rest, ":")
		id = strings.TrimPrefix(strings.TrimSpace(id), "T")
		if id != "" {
			ids = append(ids, "T"+id)
		}
	}
	return ids
}

type mappingFile struct {
	MappingObjects []struct {
		CapabilityID   string `json:"capability_id"`
		MappingType    string `json:"mapping_type"`
		AttackObjectID string `json:"attack_object_id"`
	} `json:"mapping_objects"`
}

type mappedEntry struct {
	ID    string   `json:"id"`
	Types []string `json:"types"`
}

var mappingDomainFiles = []string{"kev-attack-mobile.json"}

func buildCVEAttack(enterprise string) ([][]string, error) {
	byKey := map[string][]mappedEntry{}
	add := func(key, id, kind string) {
		for i := range byKey[key] {
			if byKey[key][i].ID == id {
				if !slices.Contains(byKey[key][i].Types, kind) {
					byKey[key][i].Types = append(byKey[key][i].Types, kind)
				}
				return
			}
		}
		byKey[key] = append(byKey[key], mappedEntry{ID: id, Types: []string{kind}})
	}

	for _, path := range append([]string{enterprise}, joinEach(filepath.Dir(enterprise), mappingDomainFiles)...) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var file mappingFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}

		for _, m := range file.MappingObjects {
			cve, technique := strings.TrimSpace(m.CapabilityID), strings.TrimSpace(m.AttackObjectID)
			if !strings.HasPrefix(cve, "CVE-") || !strings.HasPrefix(technique, "T") {
				continue
			}
			kind := strings.ReplaceAll(strings.TrimSpace(m.MappingType), "_", " ")
			add(cve, technique, kind)
			add(technique, cve, kind)
		}
	}

	rows := make([][]string, 0, len(byKey))
	for key, entries := range byKey {
		for i := range entries {
			slices.SortFunc(entries[i].Types, compareMappingTypes)
		}
		slices.SortFunc(entries, compareMapped)
		packed, err := compactJSON(entries)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []string{key, packed})
	}
	return rows, nil
}

func joinEach(dir string, names []string) []string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

var mappingTypeOrder = []string{"exploitation technique", "primary impact", "secondary impact"}

func compareMappingTypes(a, b string) int {
	ai, bi := slices.Index(mappingTypeOrder, a), slices.Index(mappingTypeOrder, b)
	if ai != bi {
		return ai - bi
	}
	return strings.Compare(a, b)
}

func compareMapped(a, b mappedEntry) int {
	if newer := compareCVEDescending(a.ID, b.ID); newer != 0 {
		return newer
	}
	if first := compareMappingTypes(a.Types[0], b.Types[0]); first != 0 {
		return first
	}
	return strings.Compare(a.ID, b.ID)
}

func compareCVEDescending(a, b string) int {
	ay, an, aok := cveParts(a)
	by, bn, bok := cveParts(b)
	if !aok || !bok {
		return 0
	}
	if ay != by {
		return by - ay
	}
	return bn - an
}

func cveParts(id string) (int, int, bool) {
	parts := strings.Split(id, "-")
	if len(parts) != 3 || parts[0] != "CVE" {
		return 0, 0, false
	}
	year, err1 := strconv.Atoi(parts[1])
	number, err2 := strconv.Atoi(parts[2])
	return year, number, err1 == nil && err2 == nil
}

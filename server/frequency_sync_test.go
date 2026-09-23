package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/frequency"
)

func TestWebappFrequencyTokenShapeMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "frequency", "index.ts")

	found := regexp.MustCompile(`export const TOKEN = /\^(.+)\$/;`).FindStringSubmatch(source)
	if found == nil {
		t.Fatal("no `export const TOKEN = /^...$/;` in the webapp's frequency/index.ts")
	}
	if want := frequency.ShapeExpr(); found[1] != want {
		t.Errorf("the token shape is %q in the webapp and %q here", found[1], want)
	}
	if !strings.Contains(source, "type: '"+frequency.Type+"'") {
		t.Errorf("the webapp decorator's type is not %q", frequency.Type)
	}
	for name, value := range map[string]int{"MIN_KHZ": frequency.MinKHz, "MAX_KHZ": frequency.MaxKHz} {
		if !strings.Contains(source, "export const "+name+" = "+strconv.Itoa(value)+";") {
			t.Errorf("the webapp's %s is not %d", name, value)
		}
	}
}

func TestWebappFrequencyBandsMatch(t *testing.T) {
	source := readWebappFile(t, "decorators", "frequency", "bands.ts")

	named := map[string]int{"VHF_AIR_LOW": frequency.VHFAirLowKHz, "VHF_AIR_HIGH": frequency.VHFAirHighKHz}
	bound := func(raw string) int {
		if n, ok := named[raw]; ok {
			return n
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("band bound %q is neither a number nor a named constant", raw)
		}
		return n
	}

	var bands []frequency.Band
	for _, m := range regexp.MustCompile(`\{low: (\w+), high: (\w+), name: '([^']+)'\}`).FindAllStringSubmatch(source, -1) {
		bands = append(bands, frequency.Band{LowKHz: bound(m[1]), HighKHz: bound(m[2]), Name: m[3]})
	}
	if len(bands) != len(frequency.Bands) {
		t.Fatalf("the webapp lists %d bands and Go %d", len(bands), len(frequency.Bands))
	}
	for i, band := range frequency.Bands {
		if bands[i] != band {
			t.Errorf("band %d is %+v in the webapp and %+v in Go", i, bands[i], band)
		}
	}

	var allocations []frequency.Allocation
	for _, m := range regexp.MustCompile(`\{khz: (\d+), use: '([^']+)'\}`).FindAllStringSubmatch(source, -1) {
		khz, _ := strconv.Atoi(m[1])
		allocations = append(allocations, frequency.Allocation{KHz: khz, Use: m[2]})
	}
	if len(allocations) != len(frequency.Allocations) {
		t.Fatalf("the webapp lists %d allocations and Go %d", len(allocations), len(frequency.Allocations))
	}
	for i, allocation := range frequency.Allocations {
		if allocations[i] != allocation {
			t.Errorf("allocation %d is %+v in the webapp and %+v in Go", i, allocations[i], allocation)
		}
	}

	for name, want := range map[string]string{
		"OUTSIDE_BANDS": "'" + frequency.OutsideBands + "'",
		"CHANNEL_25":    "'" + frequency.Channel25 + "'",
		"CHANNEL_833":   "'" + frequency.Channel833 + "'",
		"VHF_AIR_LOW":   strconv.Itoa(frequency.VHFAirLowKHz),
		"VHF_AIR_HIGH":  strconv.Itoa(frequency.VHFAirHighKHz),
	} {
		if !strings.Contains(source, "export const "+name+" = "+want+";") {
			t.Errorf("the webapp's %s is not %s", name, want)
		}
	}
}

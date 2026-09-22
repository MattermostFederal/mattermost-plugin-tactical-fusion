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
	want := `(\d{1,4}\.\d{1,3}|\d{4,5})(?:[ \t]*(MHZ|KHZ))?`
	if found[1] != want {
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

	var bands []frequency.Band
	for _, m := range regexp.MustCompile(`\{low: (\d+), high: (\d+), name: '([^']+)'\}`).FindAllStringSubmatch(source, -1) {
		low, _ := strconv.Atoi(m[1])
		high, _ := strconv.Atoi(m[2])
		bands = append(bands, frequency.Band{LowKHz: low, HighKHz: high, Name: m[3]})
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

	if !strings.Contains(source, "export const OUTSIDE_BANDS = '"+frequency.OutsideBands+"';") {
		t.Errorf("the webapp's OUTSIDE_BANDS is not %q", frequency.OutsideBands)
	}
}

func TestWebappFrequencyRendersLikeGo(t *testing.T) {
	source := readWebappFile(t, "decorators", "frequency", "index.spec.ts")

	for _, token := range []string{"121.5", "118.305", "8992 KHZ", "1090.0", "243.0"} {
		f, ok := frequency.ParseToken(token)
		if !ok {
			t.Fatalf("%q was refused", token)
		}
		d := frequency.Describe(f)
		want := "band: '" + d.Band + "'"
		if d.Band == frequency.OutsideBands {
			want = "band: OUTSIDE_BANDS"
		}
		if !strings.Contains(source, want) {
			t.Errorf("the webapp spec does not assert %s for %q", want, token)
		}
	}
}

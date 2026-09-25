package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	torExitList = "tor-exits.txt"
	torSource   = "Tor Project"
	torMetrics  = "https://metrics.torproject.org/rs.html#search/"

	mispSource      = "MISP warninglist"
	mispListBrowse  = "https://github.com/MISP/misp-warninglists/tree/main/lists/"
	mispNamePrefix  = "List of known "
	mispTypeCIDR    = "cidr"
	mispTypeString  = "string"
	mispListPattern = "*.json"
)

type mispList struct {
	ID          string
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	List        []string `json:"list"`
}

func readMISPLists(dir string) ([]mispList, error) {
	paths, err := filepath.Glob(filepath.Join(dir, mispListPattern))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no warninglist in %s; run 'make cyber-sources' first", dir)
	}
	sort.Strings(paths)

	lists := make([]mispList, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path) // #nosec G304 -- a list file under the directory the operator names with -source
		if err != nil {
			return nil, err
		}
		var list mispList
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		list.ID = strings.TrimSuffix(filepath.Base(path), ".json")
		lists = append(lists, list)
	}
	return lists, nil
}

func mispReport(list mispList) threatReport {
	name := clean(list.Name)
	if trimmed, ok := strings.CutPrefix(name, mispNamePrefix); ok && trimmed != "" {
		name = strings.ToUpper(trimmed[:1]) + trimmed[1:]
	}
	return threatReport{
		Source:   mispSource,
		Category: categoryContext,
		Threat:   name,
		URL:      mispListBrowse + list.ID,
	}
}

type addressRange struct {
	start, end [16]byte
	report     threatReport
}

func rangeOf(entry string) (addressRange, bool) {
	entry = strings.TrimSpace(entry)
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		addr, bits := prefix.Addr(), prefix.Bits()
		if addr.Is4In6() {
			addr, bits = addr.Unmap(), bits-96
		}
		if bits < 0 {
			return addressRange{}, false
		}
		first := netip.PrefixFrom(addr, bits).Masked().Addr()
		return addressRange{start: first.As16(), end: lastOf(first, bits)}, true
	}
	if addr, err := netip.ParseAddr(entry); err == nil {
		key := addr.Unmap().As16()
		return addressRange{start: key, end: key}, true
	}
	return addressRange{}, false
}

func lastOf(first netip.Addr, bits int) [16]byte {
	key := first.As16()
	hostBits := 128 - bits
	if first.Is4() {
		hostBits = 32 - bits
	}
	for i := 15; i >= 0 && hostBits > 0; i-- {
		take := min(hostBits, 8)
		key[i] |= byte(0xff >> (8 - take))
		hostBits -= take
	}
	return key
}

func readTorExits(path string) ([]addressRange, error) {
	handle, err := os.Open(path) // #nosec G304 -- a feed file under the directory the operator names with -source
	if err != nil {
		return nil, err
	}
	defer func() { _ = handle.Close() }()

	var ranges []addressRange
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		addr, err := netip.ParseAddr(strings.TrimSpace(scanner.Text()))
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		key := addr.As16()
		ranges = append(ranges, addressRange{start: key, end: key, report: threatReport{
			Source:   torSource,
			Category: categoryContext,
			Threat:   "Tor exit node",
			URL:      torMetrics + addr.String(),
		}})
	}
	return ranges, scanner.Err()
}

func buildNetLists(mispDir string) ([][]string, error) {
	ranges, err := readTorExits(filepath.Join(filepath.Dir(mispDir), torExitList))
	if err != nil {
		return nil, err
	}

	lists, err := readMISPLists(mispDir)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		if list.Type != mispTypeCIDR {
			continue
		}
		report := mispReport(list)
		for _, entry := range list.List {
			if r, ok := rangeOf(entry); ok {
				r.report = report
				ranges = append(ranges, r)
			}
		}
	}

	return flattenRanges(ranges)
}

func compareKeys(a, b [16]byte) int { return bytes.Compare(a[:], b[:]) }

func nextKey(key [16]byte) ([16]byte, bool) {
	for i := 15; i >= 0; i-- {
		key[i]++
		if key[i] != 0 {
			return key, true
		}
	}
	return key, false
}

func previousKey(key [16]byte) [16]byte {
	for i := 15; i >= 0; i-- {
		key[i]--
		if key[i] != 0xff {
			return key
		}
	}
	return key
}

func flattenRanges(ranges []addressRange) ([][]string, error) {
	var points [][16]byte
	for _, r := range ranges {
		points = append(points, r.start)
		if after, ok := nextKey(r.end); ok {
			points = append(points, after)
		}
	}
	slices.SortFunc(points, compareKeys)
	points = slices.CompactFunc(points, func(a, b [16]byte) bool { return a == b })

	byStart := slices.Clone(ranges)
	slices.SortStableFunc(byStart, func(a, b addressRange) int { return compareKeys(a.start, b.start) })

	var rows [][]string
	var active []addressRange
	next := 0
	for i, point := range points {
		kept := active[:0]
		for _, r := range active {
			if compareKeys(r.end, point) >= 0 {
				kept = append(kept, r)
			}
		}
		active = kept
		for next < len(byStart) && byStart[next].start == point {
			active = append(active, byStart[next])
			next++
		}
		if len(active) == 0 {
			continue
		}

		end := [16]byte{}
		for j := range end {
			end[j] = 0xff
		}
		if i+1 < len(points) {
			end = previousKey(points[i+1])
		}

		field, err := segmentReports(active)
		if err != nil {
			return nil, err
		}
		if last := len(rows) - 1; last >= 0 && rows[last][2] == field && adjacent(rows[last][1], point) {
			rows[last][1] = hex.EncodeToString(end[:])
			continue
		}
		rows = append(rows, []string{hex.EncodeToString(point[:]), hex.EncodeToString(end[:]), field})
	}
	return rows, nil
}

func adjacent(previousEnd string, start [16]byte) bool {
	raw, err := hex.DecodeString(previousEnd)
	if err != nil || len(raw) != 16 {
		return false
	}
	after, ok := nextKey([16]byte(raw))
	return ok && after == start
}

func segmentReports(active []addressRange) (string, error) {
	var reports []threatReport
	for _, r := range active {
		if !slices.ContainsFunc(reports, func(existing threatReport) bool {
			return existing.Source == r.report.Source && existing.Threat == r.report.Threat
		}) {
			reports = append(reports, r.report)
		}
	}
	slices.SortFunc(reports, func(a, b threatReport) int {
		if c := strings.Compare(a.Source, b.Source); c != 0 {
			return c
		}
		return strings.Compare(a.Threat, b.Threat)
	})
	return compactJSON(reports)
}

func buildHashLists(mispDir string) ([][]string, error) {
	lists, err := readMISPLists(mispDir)
	if err != nil {
		return nil, err
	}

	var index reportIndex
	for _, list := range lists {
		if list.Type != mispTypeString {
			continue
		}
		report := mispReport(list)
		for _, entry := range list.List {
			if digest := strings.ToLower(strings.TrimSpace(entry)); hexDigest.MatchString(digest) {
				index.add(digest, report)
			}
		}
	}
	if len(index.byKey) == 0 {
		return nil, fmt.Errorf("no hash list in %s", mispDir)
	}
	return index.rows()
}

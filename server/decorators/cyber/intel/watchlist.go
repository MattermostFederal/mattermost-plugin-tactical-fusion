package intel

import (
	"strings"
)

type WatchEntry struct {
	Value   string
	Kind    string
	Verdict string
	Source  string
	Note    string
	Updated string
}

const (
	VerdictMalicious  = "malicious"
	VerdictSuspicious = "suspicious"
	VerdictBenign     = "benign"
)

func KnownVerdict(verdict string) bool {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case VerdictMalicious, VerdictSuspicious, VerdictBenign:
		return true
	}
	return false
}

func loadWatchlist(d *Dataset) error {
	entries := map[string][]WatchEntry{}

	at := d.file.bodyStart
	for at < d.file.size {
		line, next, err := d.file.readLine(at)
		if err != nil {
			return err
		}
		at = next

		if strings.TrimSpace(line) == "" {
			continue
		}

		fields, err := d.file.row(line)
		if err != nil {
			return err
		}

		entry := WatchEntry{
			Value:   fields[0],
			Kind:    fields[1],
			Verdict: fields[2],
			Source:  fields[3],
			Note:    fields[4],
			Updated: fields[5],
		}
		entries[entry.Value] = append(entries[entry.Value], entry)
	}

	d.watchlist = entries

	return nil
}

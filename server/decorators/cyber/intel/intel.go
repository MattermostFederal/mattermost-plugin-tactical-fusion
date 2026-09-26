package intel

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type CVERecord struct {
	ID         string
	Published  string
	Modified   string
	Score      string
	Severity   string
	Vector     string
	Weaknesses []string
	Summary    string
}

type EPSSRecord struct {
	ID         string
	Score      string
	Percentile string
	ModelDate  string
}

type KEVRecord struct {
	ID         string
	DateAdded  string
	DueDate    string
	Ransomware string
	Product    string
	Action     string
}

type IPRecord struct {
	ASN          string
	ASName       string
	Country      string
	Region       string
	City         string
	Sources      []string
	Attributions []Attribution
}

func (r IPRecord) Empty() bool {
	return r.fields() == ipFields{}
}

type ipFields struct {
	asn     string
	asName  string
	country string
	region  string
	city    string
}

func (r IPRecord) fields() ipFields {
	return ipFields{asn: r.ASN, asName: r.ASName, country: r.Country, region: r.Region, city: r.City}
}

type ErrorClass int

const (
	ErrorUnreadable ErrorClass = iota
	ErrorSchema
	ErrorName
	ErrorMMDB
	ErrorUnpack
	ErrorTooLarge
)

type FileError struct {
	Path  string
	Class ErrorClass
	Err   error
}

func (e *FileError) Error() string { return e.Path + ": " + e.Err.Error() }

func (e *FileError) Unwrap() error { return e.Err }

var (
	ErrNoDataset = errors.New("cyber: no such dataset is installed")
	ErrRetired   = errors.New("cyber: the datasets were replaced and closed")
)

type Set struct {
	dirs       []string
	datasets   map[string]*Dataset
	mmdbs      []*mmdbReader
	candidates map[string]fingerprint
	replaced   []string
	problems   []*FileError

	guard  sync.RWMutex
	closed bool
}

func (s *Set) reading(read func() error) error {
	s.guard.RLock()
	defer s.guard.RUnlock()

	if s.closed {
		return ErrRetired
	}
	return read()
}

type fingerprint struct {
	size    int64
	modTime time.Time
}

func scanCandidates(dirs []string) map[string]fingerprint {
	found := map[string]fingerprint{}

	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !isCandidate(name) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}
			found[filepath.Join(dir, name)] = fingerprint{size: info.Size(), modTime: info.ModTime()}
		}
	}

	return found
}

func Open(dirs []string) (*Set, []*FileError) {
	problems := unpackArchives(dirs)

	set := &Set{
		dirs:       append([]string(nil), dirs...),
		datasets:   map[string]*Dataset{},
		candidates: scanCandidates(dirs),
	}

	tabular := map[string][]string{}
	vendor := map[string]string{}

	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				problems = append(problems, &FileError{Path: dir, Class: ErrorUnreadable, Err: err})
			}
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := filepath.Join(dir, entry.Name())

			switch {
			case strings.HasSuffix(entry.Name(), Suffix):
				name := strings.TrimSuffix(entry.Name(), Suffix)
				if !NamePattern.MatchString(name) {
					problems = append(problems, &FileError{Path: path, Class: ErrorName, Err: errors.New("the name is not one this build reads")})
					continue
				}
				if _, known := specs[name]; !known {
					problems = append(problems, &FileError{Path: path, Class: ErrorName, Err: errors.New("the name is not one this build reads")})
					continue
				}
				tabular[name] = append(tabular[name], path)

			case strings.HasSuffix(entry.Name(), MMDBSuffix):
				vendor[entry.Name()] = path
			}
		}
	}

	for name, paths := range tabular {
		for i := len(paths) - 1; i >= 0; i-- {
			dataset, err := openDataset(paths[i])
			if err != nil {
				class := ErrorUnreadable
				switch {
				case errors.Is(err, ErrSchema):
					class = ErrorSchema
				case errors.Is(err, ErrWatchlistTooLarge):
					class = ErrorTooLarge
				}
				problems = append(problems, &FileError{Path: paths[i], Class: class, Err: err})
				continue
			}
			set.datasets[name] = dataset
			set.replaced = append(set.replaced, paths[:i]...)
			break
		}
	}
	sort.Strings(set.replaced)

	for _, path := range sortedValues(vendor) {
		reader, err := openMMDB(path)
		if err != nil {
			problems = append(problems, &FileError{Path: path, Class: ErrorMMDB, Err: err})
			continue
		}
		set.mmdbs = append(set.mmdbs, reader)
	}

	set.problems = problems
	return set, problems
}

func sortedValues(byName map[string]string) []string {
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, byName[name])
	}

	return paths
}

func (s *Set) Close() {
	if s == nil {
		return
	}

	s.guard.Lock()
	defer s.guard.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	for _, dataset := range s.datasets {
		dataset.close()
	}
	for _, reader := range s.mmdbs {
		reader.close()
	}
}

func (s *Set) Changed(dirs []string) bool {
	if s == nil {
		return true
	}
	if !equalStrings(s.dirs, dirs) {
		return true
	}

	current := scanCandidates(dirs)
	if len(current) != len(s.candidates) {
		return true
	}
	for path, now := range current {
		was, known := s.candidates[path]
		if !known || now.size != was.size || !now.modTime.Equal(was.modTime) {
			return true
		}
	}

	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Set) Statuses() []Status {
	all := make([]Status, 0, len(Names))
	for _, name := range Names {
		if s != nil {
			if dataset, ok := s.datasets[name]; ok {
				all = append(all, dataset.status())
				continue
			}
		}
		all = append(all, Status{Name: name})
	}

	return all
}

func (s *Set) Has(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.datasets[name]
	return ok
}

func (s *Set) Generated(name string) string {
	if s == nil {
		return ""
	}
	if dataset, ok := s.datasets[name]; ok {
		return dataset.Generated
	}
	return ""
}

const KEVSlicePrefix = "KEV entries only, from "

func (s *Set) IsKEVSlice(name string) bool {
	if s == nil {
		return false
	}
	dataset, ok := s.datasets[name]
	return ok && strings.HasPrefix(dataset.Source, KEVSlicePrefix)
}

func (s *Set) FileName(name string) string {
	if s == nil {
		return ""
	}
	if dataset, ok := s.datasets[name]; ok {
		return filepath.Base(dataset.Path)
	}
	return ""
}

func (s *Set) lookup(name, key string) ([]string, error) {
	if s == nil {
		return nil, ErrNoDataset
	}
	dataset, ok := s.datasets[name]
	if !ok {
		return nil, ErrNoDataset
	}

	var row []string
	err := s.reading(func() error {
		var err error
		row, err = dataset.file.Lookup(key)
		return err
	})
	return row, err
}

func (s *Set) CVE(id string) (CVERecord, error) {
	row, err := s.lookup(NameCVE, id)
	if err != nil {
		return CVERecord{}, err
	}

	return CVERecord{
		ID:         row[0],
		Published:  row[1],
		Modified:   row[2],
		Score:      row[3],
		Severity:   row[4],
		Vector:     row[5],
		Weaknesses: splitList(row[6]),
		Summary:    row[7],
	}, nil
}

func (s *Set) EPSS(id string) (EPSSRecord, error) {
	row, err := s.lookup(NameEPSS, id)
	if err != nil {
		return EPSSRecord{}, err
	}

	return EPSSRecord{ID: row[0], Score: row[1], Percentile: row[2], ModelDate: row[3]}, nil
}

func (s *Set) KEV(id string) (KEVRecord, error) {
	row, err := s.lookup(NameKEV, id)
	if err != nil {
		return KEVRecord{}, err
	}

	return KEVRecord{
		ID:         row[0],
		DateAdded:  row[1],
		DueDate:    row[2],
		Ransomware: row[3],
		Product:    row[4],
		Action:     row[5],
	}, nil
}

func splitList(field string) []string {
	if strings.TrimSpace(field) == "" {
		return nil
	}
	return strings.Split(field, ",")
}

func IPKey(addr netip.Addr) string {
	as16 := addr.Unmap().As16()
	return hex.EncodeToString(as16[:])
}

func (s *Set) IP(addr netip.Addr) (IPRecord, error) {
	var record IPRecord
	if s == nil || !addr.IsValid() {
		return record, ErrNoDataset
	}

	if err := s.reading(func() error {
		s.enrichFromDatabases(addr, &record)
		return nil
	}); err != nil {
		return record, err
	}

	err := s.fillFromRanges(addr, &record)
	if !record.Empty() {
		return record, nil
	}

	return record, err
}

func (s *Set) enrichFromDatabases(addr netip.Addr, record *IPRecord) {
	for _, reader := range s.mmdbs {
		before := record.fields()
		reader.enrich(addr, record)
		if record.fields() != before {
			record.Sources = appendOnce(record.Sources, filepath.Base(reader.path))
			if reader.attribution != nil && !slices.Contains(record.Attributions, *reader.attribution) {
				record.Attributions = append(record.Attributions, *reader.attribution)
			}
		}
	}
}

func (s *Set) fillFromRanges(addr netip.Addr, record *IPRecord) error {
	dataset, ok := s.datasets[NameIP]
	if !ok {
		return ErrNoDataset
	}

	key := IPKey(addr)
	row, err := s.lookupRange(dataset, key)
	switch {
	case err != nil:
		return err
	case len(row) != 7 || row[0] > key || key > row[1]:
		return ErrNotFound
	}

	fill(&record.ASN, row[2])
	fill(&record.ASName, row[3])
	fill(&record.Country, row[4])
	fill(&record.Region, row[5])
	fill(&record.City, row[6])
	record.Sources = appendOnce(record.Sources, NameIP)

	return nil
}

func (s *Set) lookupRange(dataset *Dataset, key string) ([]string, error) {
	var row []string
	err := s.reading(func() error {
		var err error
		row, err = dataset.file.LookupRange(key)
		return err
	})
	return row, err
}

func fill(into *string, value string) {
	if *into == "" {
		*into = value
	}
}

func appendOnce(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func (s *Set) Watchlist(value string) []WatchEntry {
	if s == nil {
		return nil
	}
	dataset, ok := s.datasets[NameWatchlist]
	if !ok {
		return nil
	}

	return dataset.watchlist[value]
}

type DatabaseBuild struct {
	Name  string
	Built time.Time
}

func (s *Set) DatabaseBuilds() []DatabaseBuild {
	if s == nil {
		return nil
	}
	builds := make([]DatabaseBuild, 0, len(s.mmdbs))
	for _, reader := range s.mmdbs {
		epoch := reader.reader.Metadata.BuildEpoch
		if epoch == 0 {
			continue
		}
		builds = append(builds, DatabaseBuild{
			Name:  filepath.Base(reader.path),
			Built: time.Unix(int64(epoch), 0).UTC(), // #nosec G115 -- a build time in seconds, far inside int64
		})
	}
	return builds
}

package intel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	SchemaPrefix  = "#tactical-fusion-cyber/"
	SchemaVersion = 1

	Suffix = ".tsv"

	stampFields = 4
	maxStamp    = 1024
)

var ErrSchema = errors.New("cyber: the file does not carry this reader's schema stamp")

var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

const (
	NameCVE       = "cve"
	NameEPSS      = "epss"
	NameKEV       = "kev"
	NameIP        = "ip"
	NameWatchlist = "watchlist"
)

type spec struct {
	fields  int
	inspect func(*Dataset) error
}

var specs = map[string]spec{
	NameCVE:       {fields: 8},
	NameEPSS:      {fields: 4},
	NameKEV:       {fields: 6},
	NameIP:        {fields: 7},
	NameWatchlist: {fields: 6, inspect: loadWatchlist},
}

var Names = []string{NameCVE, NameEPSS, NameKEV, NameIP, NameWatchlist}

type Dataset struct {
	Name      string
	Path      string
	Generated string
	Source    string

	handle *os.File
	file   *sortedFile

	size    int64
	modTime time.Time

	watchlist map[string][]WatchEntry
}

type Status struct {
	Name      string
	Present   bool
	Generated string
	Source    string
}

func openDataset(path string) (*Dataset, error) {
	name := strings.TrimSuffix(filepath.Base(path), Suffix)
	spec, known := specs[name]
	if !known {
		return nil, fmt.Errorf("%q is not a dataset this build reads", name)
	}

	handle, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := handle.Stat()
	if err != nil {
		handle.Close()
		return nil, err
	}

	d := &Dataset{
		Name:    name,
		Path:    path,
		handle:  handle,
		size:    info.Size(),
		modTime: info.ModTime(),
	}

	bodyStart, err := d.readStamp()
	if err != nil {
		handle.Close()
		return nil, err
	}

	d.file = &sortedFile{
		source:    handle,
		bodyStart: bodyStart,
		size:      d.size,
		fields:    spec.fields,
	}

	if spec.inspect != nil {
		if err := spec.inspect(d); err != nil {
			handle.Close()
			return nil, err
		}
	}

	return d, nil
}

func (d *Dataset) readStamp() (int64, error) {
	head := make([]byte, maxStamp)
	n, err := d.handle.ReadAt(head, 0)
	if n == 0 && err != nil {
		return 0, err
	}

	line := string(head[:n])
	end := strings.IndexByte(line, '\n')
	if end < 0 {
		return 0, ErrSchema
	}
	line = line[:end]

	if !strings.HasPrefix(line, SchemaPrefix) {
		return 0, ErrSchema
	}

	fields := strings.Split(line, "\t")
	if len(fields) != stampFields {
		return 0, ErrSchema
	}
	if fields[0] != fmt.Sprintf("%s%d", SchemaPrefix, SchemaVersion) {
		return 0, ErrSchema
	}
	if fields[1] != d.Name {
		return 0, fmt.Errorf("%w: the stamp names %q and the file is %q", ErrSchema, fields[1], d.Name)
	}

	d.Generated = fields[2]
	d.Source = fields[3]

	return int64(end) + 1, nil
}

func (d *Dataset) close() {
	if d.handle != nil {
		d.handle.Close()
	}
}

func (d *Dataset) changed() bool {
	info, err := os.Stat(d.Path)
	if err != nil {
		return true
	}

	return info.Size() != d.size || !info.ModTime().Equal(d.modTime)
}

func (d *Dataset) status() Status {
	return Status{Name: d.Name, Present: true, Generated: d.Generated, Source: d.Source}
}

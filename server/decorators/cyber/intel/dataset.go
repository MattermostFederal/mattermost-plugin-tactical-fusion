package intel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	NameCVEDetail = "cvedetail"
	NameCWEDetail = "cwedetail"
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
	NameCVEDetail: {fields: 5},
	NameCWEDetail: {fields: 7},
	NameEPSS:      {fields: 4},
	NameKEV:       {fields: 6},
	NameIP:        {fields: 7},
	NameWatchlist: {fields: 6, inspect: loadWatchlist},
}

var Names = []string{NameCVE, NameCVEDetail, NameCWEDetail, NameEPSS, NameKEV, NameIP, NameWatchlist}

type Dataset struct {
	Name      string
	Path      string
	Generated string
	Source    string

	handle *os.File
	file   *sortedFile

	size int64

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

	handle, err := os.Open(path) // #nosec G304 -- a path built from a whitelisted name under an operator-configured directory
	if err != nil {
		return nil, err
	}

	info, err := handle.Stat()
	if err != nil {
		_ = handle.Close()
		return nil, err
	}

	d := &Dataset{
		Name:   name,
		Path:   path,
		handle: handle,
		size:   info.Size(),
	}

	bodyStart, err := d.readStamp()
	if err != nil {
		_ = handle.Close()
		return nil, err
	}

	body, err := searchableEnd(handle, bodyStart, d.size)
	if err != nil {
		_ = handle.Close()
		return nil, err
	}

	d.file = &sortedFile{
		source:    handle,
		bodyStart: bodyStart,
		size:      body,
		fields:    spec.fields,
	}

	if spec.inspect != nil {
		if err := spec.inspect(d); err != nil {
			_ = handle.Close()
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

	generated, source, err := parseStamp(line[:end], d.Name)
	if err != nil {
		return 0, err
	}

	d.Generated = generated
	d.Source = source

	return int64(end) + 1, nil
}

func parseStamp(line, name string) (generated, source string, err error) {
	if !strings.HasPrefix(line, SchemaPrefix) {
		return "", "", ErrSchema
	}

	fields := strings.Split(line, "\t")
	if len(fields) != stampFields {
		return "", "", ErrSchema
	}
	if fields[0] != fmt.Sprintf("%s%d", SchemaPrefix, SchemaVersion) {
		return "", "", ErrSchema
	}
	if fields[1] != name {
		return "", "", fmt.Errorf("%w: the stamp names %q and the file is %q", ErrSchema, fields[1], name)
	}

	return fields[2], fields[3], nil
}

func searchableEnd(source readerAt, bodyStart, size int64) (int64, error) {
	end := size

	for end-1 > bodyStart {
		last, err := byteAt(source, end-1)
		if err != nil {
			return 0, err
		}

		previous, err := byteAt(source, end-2)
		if err != nil {
			return 0, err
		}

		if last != '\n' || previous != '\n' {
			break
		}
		end--
	}

	if end <= bodyStart {
		return bodyStart, nil
	}

	return end, nil
}

func byteAt(source readerAt, off int64) (byte, error) {
	var one [1]byte
	if _, err := source.ReadAt(one[:], off); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}

	return one[0], nil
}

func (d *Dataset) close() {
	if d.handle != nil {
		_ = d.handle.Close()
	}
}

func (d *Dataset) status() Status {
	return Status{Name: d.Name, Present: true, Generated: d.Generated, Source: d.Source}
}

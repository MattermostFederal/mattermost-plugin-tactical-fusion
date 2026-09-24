package intel

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"time"
)

const countChunk = 1 << 20

type DatasetEntry struct {
	Name      string
	Path      string
	Size      int64
	Generated string
	Source    string
	Records   int
	CountErr  error
}

type DatabaseEntry struct {
	Path      string
	Type      string
	Built     time.Time
	Size      int64
	IPVersion uint
}

type Inventory struct {
	Datasets  []DatasetEntry
	Databases []DatabaseEntry
	Replaced  []string
	Problems  []*FileError
}

func (d *Dataset) Records() (int, error) {
	d.countOnce.Do(func() {
		if d.watchlist != nil {
			for _, entries := range d.watchlist {
				d.records += len(entries)
			}
			return
		}
		d.records, d.countErr = countLines(d.handle, d.file.bodyStart, d.file.size)
	})
	return d.records, d.countErr
}

func countLines(source io.ReaderAt, from, to int64) (int, error) {
	buffer := make([]byte, countChunk)
	lines := 0
	lastByte := byte('\n')

	for at := from; at < to; {
		want := min(int64(len(buffer)), to-at)
		n, err := source.ReadAt(buffer[:want], at)
		if n > 0 {
			lines += bytes.Count(buffer[:n], []byte{'\n'})
			lastByte = buffer[n-1]
			at += int64(n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		if n == 0 {
			break
		}
	}

	if lastByte != '\n' {
		lines++
	}
	return lines, nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (s *Set) Inventory() Inventory {
	var inv Inventory
	if s == nil {
		return inv
	}

	for _, name := range Names {
		dataset, ok := s.datasets[name]
		if !ok {
			continue
		}
		records, err := dataset.Records()
		inv.Datasets = append(inv.Datasets, DatasetEntry{
			Name:      dataset.Name,
			Path:      dataset.Path,
			Size:      dataset.size,
			Generated: dataset.Generated,
			Source:    dataset.Source,
			Records:   records,
			CountErr:  err,
		})
	}

	for _, reader := range s.mmdbs {
		meta := reader.reader.Metadata
		inv.Databases = append(inv.Databases, DatabaseEntry{
			Path:      reader.path,
			Type:      meta.DatabaseType,
			Built:     time.Unix(int64(meta.BuildEpoch), 0).UTC(), // #nosec G115 -- a build time in seconds, far inside int64
			Size:      fileSize(reader.path),
			IPVersion: meta.IPVersion,
		})
	}

	inv.Replaced = slices.Clone(s.replaced)
	inv.Problems = slices.Clone(s.problems)

	return inv
}

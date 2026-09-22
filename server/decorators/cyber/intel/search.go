package intel

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

const blockBytes = 4096

var ErrNotFound = errors.New("cyber: no row for that key")

type readerAt interface {
	ReadAt(p []byte, off int64) (int, error)
}

type sortedFile struct {
	source    readerAt
	bodyStart int64
	size      int64
	fields    int
}

func (f *sortedFile) readAt(p []byte, off int64) (int, error) {
	n, err := f.source.ReadAt(p, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	return n, nil
}

func (f *sortedFile) firstLineStartAtOrAfter(pos int64) (int64, error) {
	if pos <= f.bodyStart {
		return f.bodyStart, nil
	}

	at := pos - 1
	buf := make([]byte, blockBytes)
	for at < f.size {
		n, err := f.readAt(buf, at)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			break
		}
		if index := bytes.IndexByte(buf[:n], '\n'); index >= 0 {
			return at + int64(index) + 1, nil
		}
		at += int64(n)
	}

	return f.size, nil
}

func (f *sortedFile) previousLineStart(start int64) (int64, error) {
	if start <= f.bodyStart {
		return f.bodyStart, nil
	}

	end := start - 1
	for end > f.bodyStart {
		from := max(end-blockBytes, f.bodyStart)

		buf := make([]byte, end-from)
		n, err := f.readAt(buf, from)
		if err != nil {
			return 0, err
		}
		if index := bytes.LastIndexByte(buf[:n], '\n'); index >= 0 {
			return from + int64(index) + 1, nil
		}
		end = from
	}

	return f.bodyStart, nil
}

func (f *sortedFile) readLine(start int64) (string, int64, error) {
	if start >= f.size {
		return "", f.size, io.EOF
	}

	var line []byte
	at := start
	buf := make([]byte, blockBytes)
	for at < f.size {
		n, err := f.readAt(buf, at)
		if err != nil {
			return "", 0, err
		}
		if n == 0 {
			break
		}
		if index := bytes.IndexByte(buf[:n], '\n'); index >= 0 {
			line = append(line, buf[:index]...)
			return string(line), at + int64(index) + 1, nil
		}
		line = append(line, buf[:n]...)
		at += int64(n)
	}

	return string(line), f.size, nil
}

func keyOf(line string) string {
	if before, _, ok := strings.Cut(line, "\t"); ok {
		return before
	}
	return line
}

func (f *sortedFile) seekFirstAtLeast(key string) (int64, error) {
	lo, hi := f.bodyStart, f.size

	for lo < hi {
		mid := lo + (hi-lo)/2

		start, err := f.firstLineStartAtOrAfter(mid)
		if err != nil {
			return 0, err
		}
		if start >= hi {
			hi = mid
			continue
		}

		line, next, err := f.readLine(start)
		if err != nil {
			return 0, err
		}

		if keyOf(line) < key {
			lo = next
			continue
		}
		hi = start
	}

	return lo, nil
}

func (f *sortedFile) row(line string) ([]string, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != f.fields {
		return nil, fmt.Errorf("a row has %d fields, want %d", len(fields), f.fields)
	}
	return fields, nil
}

func (f *sortedFile) Lookup(key string) ([]string, error) {
	start, err := f.seekFirstAtLeast(key)
	if err != nil {
		return nil, err
	}
	if start >= f.size {
		return nil, ErrNotFound
	}

	line, _, err := f.readLine(start)
	if err != nil {
		return nil, err
	}
	if keyOf(line) != key {
		return nil, ErrNotFound
	}

	return f.row(line)
}

func (f *sortedFile) LookupRange(key string) ([]string, error) {
	start, err := f.seekFirstAtLeast(key)
	if err != nil {
		return nil, err
	}

	if start < f.size {
		line, _, readErr := f.readLine(start)
		if readErr != nil {
			return nil, readErr
		}
		if keyOf(line) == key {
			return f.row(line)
		}
	}

	if start <= f.bodyStart {
		return nil, ErrNotFound
	}

	previous, err := f.previousLineStart(start)
	if err != nil {
		return nil, err
	}

	line, _, err := f.readLine(previous)
	if err != nil {
		return nil, err
	}

	return f.row(line)
}

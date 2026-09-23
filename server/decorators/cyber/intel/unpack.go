package intel

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	ArchiveSuffix = Suffix + ".gz"

	maxUnpackedBytes = 2 << 30

	unpackingSuffix = ".unpacking"
)

var ErrTooLarge = errors.New("cyber: the archive unpacks to more than this build accepts")

func isCandidate(name string) bool {
	return strings.HasSuffix(name, Suffix) || strings.HasSuffix(name, ArchiveSuffix) || strings.HasSuffix(name, MMDBSuffix)
}

func unpackArchives(dirs []string) []*FileError {
	var problems []*FileError

	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ArchiveSuffix) {
				continue
			}

			archive := filepath.Join(dir, entry.Name())
			name := strings.TrimSuffix(entry.Name(), ArchiveSuffix)
			if _, known := specs[name]; !known || !NamePattern.MatchString(name) {
				problems = append(problems, &FileError{Path: archive, Class: ErrorName, Err: errors.New("the name is not one this build reads")})
				continue
			}

			err := unpackIfStale(archive, filepath.Join(dir, name+Suffix), name, maxUnpackedBytes)
			switch {
			case err == nil:
			case errors.Is(err, ErrSchema):
				problems = append(problems, &FileError{Path: archive, Class: ErrorSchema, Err: err})
			default:
				problems = append(problems, &FileError{Path: archive, Class: ErrorUnpack, Err: err})
			}
		}
	}

	return problems
}

func unpackIfStale(archive, target, name string, limit int64) error {
	archiveStamp, err := archiveStampLine(archive)
	if err != nil {
		return err
	}
	if _, _, err := parseStamp(archiveStamp, name); err != nil {
		return err
	}

	if current, err := fileStampLine(target); err == nil && current == archiveStamp {
		return nil
	}

	return unpack(archive, target, limit)
}

func archiveStampLine(archive string) (string, error) {
	source, err := os.Open(archive) // #nosec G304 -- a path built from a whitelisted name under an operator-configured directory
	if err != nil {
		return "", err
	}
	defer func() { _ = source.Close() }()

	decompressed, err := gzip.NewReader(source)
	if err != nil {
		return "", err
	}
	defer func() { _ = decompressed.Close() }()

	return firstLine(decompressed)
}

func fileStampLine(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 -- a path built from a whitelisted name under an operator-configured directory
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	return firstLine(file)
}

func firstLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(io.LimitReader(r, maxStamp)).ReadString('\n')
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf("%w: no stamp line in the first %d bytes", ErrSchema, maxStamp)
	}
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(line, "\n"), nil
}

func unpack(archive, target string, limit int64) error {
	source, err := os.Open(archive) // #nosec G304 -- a path built from a whitelisted name under an operator-configured directory
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	decompressed, err := gzip.NewReader(source)
	if err != nil {
		return err
	}
	defer func() { _ = decompressed.Close() }()

	staging, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*"+unpackingSuffix)
	if err != nil {
		return err
	}

	renamed := false
	defer func() {
		if !renamed {
			_ = staging.Close()
			_ = os.Remove(staging.Name())
		}
	}()

	written, err := io.Copy(staging, io.LimitReader(decompressed, limit+1))
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("%w: more than %d bytes", ErrTooLarge, limit)
	}

	if err := staging.Sync(); err != nil {
		return err
	}
	if err := staging.Close(); err != nil {
		return err
	}
	if err := os.Rename(staging.Name(), target); err != nil {
		return err
	}
	renamed = true

	return nil
}

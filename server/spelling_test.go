package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Banned repo-wide by CLAUDE.md, and check-style lints neither prose nor HTML,
// so nothing else would notice a British spelling reaching a reader.
//
// Every pair here was applied to the whole tree once. The guard is what stops it
// drifting back one comment at a time.
var britishSpellings = []string{
	"colour", "colours", "coloured", "colouring", "uncoloured",
	"centre", "centres", "centred", "centreline", "centrelines",
	"metre", "metres", "kilometre", "kilometres",
	"licence", "licences",
	"recognise", "recognised", "recognises", "recognising", "unrecognised",
	"normalise", "normalised", "normalises", "normalisation",
	"serialise", "serialised", "serialisable",
	"labelled", "labelling",
	"behaviour", "behaviours", "behavioural", "behaviourally",
	"honour", "honours", "honoured", "honouring",
	"defence", "neighbour", "neighbours", "neighbouring",
	"customise", "customised", "customises",
	"acknowledgement", "acknowledgements",
	"cancelled", "cancelling", "travelled", "travelling",
	"grey", "greys", "organise", "organised", "organisation", "organisations",
	"analyse", "analysed", "analysing", "catalogue", "whilst", "modelled",
	"fulfil", "initialise", "initialised", "realise", "realised",
}

// Word boundaries, which is the whole reason this is a regexp: `aria-labelledby`
// is an HTML attribute and `greyscale` is not a word this list is about.
var britishPattern = regexp.MustCompile(`(?i)\b(` + strings.Join(britishSpellings, "|") + `)\b`)

// Roots that are this project's own source. Everything outside them is either a
// build artifact or somebody else's.
var spellingRoots = []string{"../server", "../webapp/src", "../docs", "../public/help", "../build"}

var spellingExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".md": true,
	".html": true, ".css": true, ".json": true, ".sh": true,
}

// Upstream data carrying real place names and dictionary words, and the two
// generated manifests, whose text comes from plugin.json.
var spellingSkipFiles = map[string]bool{
	"airports.csv": true, "words4.txt": true,
	"manifest.ts": true, "manifest.go": true,

	// This file names every spelling it bans, so it cannot pass its own test.
	"spelling_test.go": true,
}

// Third-party identifiers that happen to be spelled the British way. A file
// name published by somebody else is data, not prose: build/notices has to
// recognize a dependency's LICENCE file, and cannot do that without naming it.
var spellingExemptions = map[string]map[string]bool{
	"build/notices/main_test.go": {"licence": true},
}

var spellingSkipDirs = map[string]bool{
	"node_modules": true, "dist": true, "coverage": true,
	"coverage-ct": true, "coverage-merged": true, "test-results": true,
}

func TestSourceUsesUSEnglish(t *testing.T) {
	for _, root := range spellingRoots {
		walkForSpelling(t, root)
	}

	checkSpellingIn(t, "../plugin.json")
	checkSpellingIn(t, "../Makefile")
	checkSpellingIn(t, "../CLAUDE.md")
	checkSpellingIn(t, "../build/maptiles/Dockerfile")
}

func walkForSpelling(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if spellingSkipDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if spellingSkipFiles[entry.Name()] || !spellingExtensions[filepath.Ext(path)] {
			return nil
		}

		checkSpellingIn(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("could not walk %s: %v", root, err)
	}
}

func checkSpellingIn(t *testing.T, path string) {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source paths
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	var exempt map[string]bool
	for suffix, words := range spellingExemptions {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
			exempt = words
		}
	}

	for number, line := range strings.Split(string(raw), "\n") {
		for _, found := range britishPattern.FindAllString(line, -1) {
			if exempt[strings.ToLower(found)] {
				continue
			}
			t.Errorf("%s:%d uses the British spelling %q; this project is US English",
				path, number+1, found)
		}
	}
}

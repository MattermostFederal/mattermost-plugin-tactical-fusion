package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type policy struct {
	NoticeFallbacks []fallback `json:"noticeFallbacks"`
}

type fallback struct {
	Component string `json:"component"`
	License   string `json:"license"`
	Text      string `json:"text"`
}

type dependency struct {
	Name      string
	Version   string
	Ecosystem string
	Notices   []notice
	Fallback  string
}

type notice struct {
	File string
	Text string
}

var licenseFile = regexp.MustCompile(`(?i)^(LICEN[CS]E|COPYING|NOTICE)([-._].*)?$`)

var sourceExtension = map[string]bool{
	".c": true, ".cc": true, ".cjs": true, ".cpp": true, ".cs": true, ".go": true,
	".h": true, ".java": true, ".js": true, ".json": true, ".jsx": true, ".mjs": true,
	".php": true, ".py": true, ".rb": true, ".rs": true, ".sh": true, ".toml": true,
	".ts": true, ".tsx": true, ".yaml": true, ".yml": true,
}

func main() {
	var (
		policyPath = flag.String("policy", ".licenses.json", "where the notice fallbacks are recorded")
		goList     = flag.String("go", "", "pipe-separated module path, version and directory")
		npmList    = flag.String("npm", "", "newline-separated package directories")
		outPath    = flag.String("out", "", "where to write the notices file")
		title      = flag.String("title", "this plugin", "what the notices file says it is for")
	)
	flag.Parse()

	if *outPath == "" {
		fail(fmt.Errorf("no -out given"))
	}

	pol, err := readPolicy(*policyPath)
	if err != nil {
		fail(err)
	}

	var deps []dependency

	if *goList != "" {
		found, err := readGoModules(*goList)
		if err != nil {
			fail(err)
		}
		deps = append(deps, found...)
	}

	if *npmList != "" {
		found, err := readNPMPackages(*npmList)
		if err != nil {
			fail(err)
		}
		deps = append(deps, found...)
	}

	if len(deps) == 0 {
		fail(fmt.Errorf("no dependencies to write notices for"))
	}

	bare, err := applyFallbacks(pol, deps)
	if err != nil {
		fail(err)
	}
	deps = bare

	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Ecosystem != deps[j].Ecosystem {
			return deps[i].Ecosystem < deps[j].Ecosystem
		}
		return deps[i].Name < deps[j].Name
	})

	if err := writeNotices(*outPath, *title, deps); err != nil {
		fail(err)
	}

	fmt.Printf("Third-party notices written for %d component(s): %s\n", len(deps), *outPath)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "notices: %v\n", err)
	os.Exit(1)
}

func readPolicy(path string) (policy, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return policy{}, fmt.Errorf("could not read %s: %w", path, err)
	}

	var pol policy
	if err := json.Unmarshal(raw, &pol); err != nil {
		return policy{}, fmt.Errorf("could not parse %s: %w", path, err)
	}
	return pol, nil
}

func readGoModules(path string) ([]dependency, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return nil, fmt.Errorf("could not read the module list %s: %w", path, err)
	}

	seen := map[string]bool{}
	var deps []dependency

	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true

		found, err := noticesIn(fields[2])
		if err != nil {
			return nil, err
		}

		deps = append(deps, dependency{
			Name: fields[0], Version: fields[1], Ecosystem: "Go module", Notices: found,
		})
	}

	if len(deps) == 0 {
		return nil, fmt.Errorf("%s named no modules", path)
	}
	return deps, nil
}

func readNPMPackages(path string) ([]dependency, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return nil, fmt.Errorf("could not read the package list %s: %w", path, err)
	}

	seen := map[string]bool{}
	var deps []dependency

	for line := range strings.SplitSeq(string(raw), "\n") {
		dir := strings.TrimSpace(line)
		if dir == "" || !strings.Contains(dir, "node_modules") {
			continue
		}

		manifest, err := os.ReadFile(filepath.Join(dir, "package.json")) // #nosec G304 -- inside node_modules
		if err != nil {
			return nil, fmt.Errorf("could not read the manifest in %s: %w", dir, err)
		}

		var pkg struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(manifest, &pkg); err != nil {
			return nil, fmt.Errorf("could not parse the manifest in %s: %w", dir, err)
		}

		key := pkg.Name + "@" + pkg.Version
		if pkg.Name == "" || seen[key] {
			continue
		}
		seen[key] = true

		found, err := noticesIn(dir)
		if err != nil {
			return nil, err
		}

		deps = append(deps, dependency{
			Name: pkg.Name, Version: pkg.Version, Ecosystem: "npm package", Notices: found,
		})
	}

	if len(deps) == 0 {
		return nil, fmt.Errorf("%s named no packages", path)
	}
	return deps, nil
}

func noticesIn(dir string) ([]notice, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not list %s: %w", dir, err)
	}

	var found []notice
	for _, e := range entries {
		if e.IsDir() || !licenseFile.MatchString(e.Name()) {
			continue
		}
		if sourceExtension[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, e.Name())) // #nosec G304 -- a dependency's own license file
		if err != nil {
			return nil, fmt.Errorf("could not read %s in %s: %w", e.Name(), dir, err)
		}

		if text := strings.TrimRight(string(raw), "\n"); strings.TrimSpace(text) != "" {
			found = append(found, notice{File: e.Name(), Text: text})
		}
	}

	sort.Slice(found, func(i, j int) bool { return found[i].File < found[j].File })
	return found, nil
}

func applyFallbacks(pol policy, deps []dependency) ([]dependency, error) {
	byName := map[string]string{}
	for _, f := range pol.NoticeFallbacks {
		byName[f.Component] = f.Text
	}

	used := map[string]bool{}
	var bare []string

	for i, d := range deps {
		if len(d.Notices) > 0 {
			continue
		}
		text, ok := byName[d.Name]
		if !ok {
			bare = append(bare, fmt.Sprintf("%s %s (%s)", d.Name, d.Version, d.Ecosystem))
			continue
		}
		deps[i].Fallback = text
		used[d.Name] = true
	}

	if len(bare) > 0 {
		return nil, fmt.Errorf("these ship without any license text, and the notices file "+
			"may not omit them silently.\n  %s\nRecord each one under noticeFallbacks in the "+
			"policy file, quoting what its package actually states",
			strings.Join(bare, "\n  "))
	}

	for _, f := range pol.NoticeFallbacks {
		if !used[f.Component] {
			return nil, fmt.Errorf("noticeFallbacks names %q, which either no longer ships or "+
				"now carries its own license file; remove the entry", f.Component)
		}
	}

	return deps, nil
}

const preamble = `This file lists the open source software distributed inside this plugin and
reproduces the license and copyright notices that those licenses require to
travel with it.

It is generated by build/notices from the license files the dependencies
themselves publish, over the set that is actually built into the shipped
artifacts: the Go modules linked into the server binary, and the npm packages
bundled into the webapp. Build-only tooling is not listed, because none of it
is distributed.

Two things are covered elsewhere and are not repeated here. The bundled map
data carries its own notices at public/map/LICENSE-OSM.txt (OpenStreetMap under
ODbL 1.0, and the OpenMapTiles schema under CC-BY 4.0), and the bundled font
glyph ranges carry theirs at public/map/fonts/LICENSE.txt (Noto Sans under SIL
OFL 1.1).

Nothing in this file applies to the plugin's own code, which is not open source.
`

func writeNotices(path, title string, deps []dependency) error {
	var b strings.Builder

	rule := strings.Repeat("=", 78)
	inner := strings.Repeat("-", 78)

	fmt.Fprintf(&b, "%s\nTHIRD-PARTY SOFTWARE NOTICES\n%s\n\nFor %s.\n\n%s\n", rule, rule, title, preamble)

	ecosystem := ""
	for _, d := range deps {
		if d.Ecosystem != ecosystem {
			ecosystem = d.Ecosystem
			fmt.Fprintf(&b, "\n%s\n%ss\n%s\n", rule, ecosystem, rule)
		}

		fmt.Fprintf(&b, "\n%s\n%s %s\n%s\n\n", inner, d.Name, d.Version, inner)

		if d.Fallback != "" {
			fmt.Fprintf(&b, "%s\n", d.Fallback)
			continue
		}

		for i, n := range d.Notices {
			if len(d.Notices) > 1 || !strings.EqualFold(n.File, "LICENSE") {
				fmt.Fprintf(&b, "--- %s ---\n\n", n.File)
			}
			fmt.Fprintf(&b, "%s\n", n.Text)
			if i < len(d.Notices)-1 {
				b.WriteString("\n")
			}
		}
	}

	fmt.Fprintf(&b, "\n%s\nEnd of third-party notices. %d component(s) listed.\n%s\n",
		rule, len(deps), rule)

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("could not create the output directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil { // #nosec G306 -- a notices file exists to be read
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}

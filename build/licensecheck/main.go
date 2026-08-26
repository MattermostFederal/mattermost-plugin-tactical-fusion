package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const undetected = "UNDETECTED"

type policy struct {
	Allowed    []string    `json:"allowed"`
	Denied     []string    `json:"denied"`
	Exceptions []exception `json:"exceptions"`
}

type exception struct {
	Component string `json:"component"`
	License   string `json:"license"`
	Reason    string `json:"reason"`
}

type bom struct {
	Components []component `json:"components"`
}

type component struct {
	Name     string    `json:"name"`
	Group    string    `json:"group"`
	Version  string    `json:"version"`
	PURL     string    `json:"purl"`
	Licenses []entry   `json:"licenses"`
	Evidence *evidence `json:"evidence"`
}

type evidence struct {
	Licenses []entry `json:"licenses"`
}

type entry struct {
	Expression string `json:"expression"`
	License    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"license"`
}

type verdict struct {
	Component string   `json:"component"`
	Version   string   `json:"version"`
	Ecosystem string   `json:"ecosystem"`
	Licenses  []string `json:"licenses"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason,omitempty"`
}

func main() {
	var (
		policyPath = flag.String("policy", ".licenses.json", "the license policy to enforce")
		goBOM      = flag.String("go-sbom", "", "CycloneDX SBOM for the Go module graph")
		goShipped  = flag.String("go-shipped", "", "newline-separated modules linked into the server binary")
		npmBOM     = flag.String("npm-sbom", "", "CycloneDX SBOM for the shipped npm tree")
		reportPath = flag.String("report", "", "where to write the resolved license report")
	)
	flag.Parse()

	pol, err := readPolicy(*policyPath)
	if err != nil {
		fail(err)
	}

	var verdicts []verdict

	if *goBOM != "" {
		shipped, err := readShipped(*goShipped)
		if err != nil {
			fail(err)
		}

		components, err := readBOM(*goBOM)
		if err != nil {
			fail(err)
		}

		for _, c := range components {
			if shipped != nil && !shipped[c.Name] {
				continue
			}
			verdicts = append(verdicts, judge(pol, c, "go"))
		}
	}

	if *npmBOM != "" {
		components, err := readBOM(*npmBOM)
		if err != nil {
			fail(err)
		}

		for _, c := range components {
			verdicts = append(verdicts, judge(pol, c, "npm"))
		}
	}

	if len(verdicts) == 0 {
		fail(fmt.Errorf("no components to judge; run `make sbom` first"))
	}

	sort.Slice(verdicts, func(i, j int) bool {
		if verdicts[i].Ecosystem != verdicts[j].Ecosystem {
			return verdicts[i].Ecosystem < verdicts[j].Ecosystem
		}
		return verdicts[i].Component < verdicts[j].Component
	})

	if *reportPath != "" {
		if err := writeReport(*reportPath, verdicts); err != nil {
			fail(err)
		}
	}

	problems := report(os.Stdout, pol, verdicts)
	if problems > 0 {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "license-check: %v\n", err)
	os.Exit(1)
}

func readPolicy(path string) (policy, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return policy{}, fmt.Errorf("could not read the policy %s: %w", path, err)
	}

	var pol policy
	if err := json.Unmarshal(raw, &pol); err != nil {
		return policy{}, fmt.Errorf("could not parse the policy %s: %w", path, err)
	}
	if len(pol.Allowed) == 0 || len(pol.Denied) == 0 {
		return policy{}, fmt.Errorf("%s declares no allowed or no denied licenses", path)
	}
	return pol, nil
}

func readBOM(path string) ([]component, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return nil, fmt.Errorf("could not read the SBOM %s: %w; run `make sbom` first", path, err)
	}

	var doc bom
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("could not parse the SBOM %s: %w", path, err)
	}
	return doc.Components, nil
}

func readShipped(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- a build-time path chosen by the Makefile
	if err != nil {
		return nil, fmt.Errorf("could not read the shipped module list %s: %w", path, err)
	}

	shipped := map[string]bool{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			shipped[trimmed] = true
		}
	}
	if len(shipped) == 0 {
		return nil, fmt.Errorf("%s names no modules", path)
	}
	return shipped, nil
}

func judge(pol policy, c component, ecosystem string) verdict {
	v := verdict{
		Component: qualify(c),
		Version:   c.Version,
		Ecosystem: ecosystem,
		Licenses:  licensesOf(c),
	}

	if len(v.Licenses) == 0 {
		v.Licenses = []string{undetected}
	}

	for _, l := range v.Licenses {
		switch {
		case l == undetected:
			v.Status = "unstated"
		case matches(pol.Denied, l):
			v.Status = "denied"
		case allowed(pol.Allowed, l):
			if v.Status == "" {
				v.Status = "allowed"
			}
			continue
		default:
			v.Status = "unrecognized"
		}

		if reason := excused(pol, v.Component, l); reason != "" {
			v.Status = "excepted"
			v.Reason = reason
			continue
		}
		return v
	}

	return v
}

func qualify(c component) string {
	if c.Group != "" {
		return c.Group + "/" + c.Name
	}
	return c.Name
}

func licensesOf(c component) []string {
	seen := map[string]bool{}
	var out []string

	collect := func(entries []entry) {
		for _, e := range entries {
			id := e.Expression
			if id == "" {
				id = e.License.ID
			}
			if id == "" {
				id = e.License.Name
			}
			if id = strings.TrimSpace(id); id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}

	collect(c.Licenses)
	if c.Evidence != nil {
		collect(c.Evidence.Licenses)
	}
	return out
}

func allowed(list []string, license string) bool {
	if hasID(list, license) {
		return true
	}

	expression := strings.Trim(license, "()")
	if !strings.Contains(strings.ToUpper(expression), " OR ") {
		return false
	}

	for arm := range strings.SplitSeq(expression, " OR ") {
		if hasID(list, strings.Trim(strings.TrimSpace(arm), "()")) {
			return true
		}
	}
	return false
}

func hasID(list []string, license string) bool {
	for _, candidate := range list {
		if strings.EqualFold(candidate, license) {
			return true
		}
	}
	return false
}

func matches(list []string, license string) bool {
	upper := strings.ToUpper(license)
	for _, candidate := range list {
		if strings.Contains(upper, strings.ToUpper(candidate)) {
			return true
		}
	}
	return false
}

func excused(pol policy, name, license string) string {
	for _, e := range pol.Exceptions {
		if e.Component == name && strings.EqualFold(e.License, license) {
			return e.Reason
		}
	}
	return ""
}

func writeReport(path string, verdicts []verdict) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("could not create the report directory: %w", err)
	}

	raw, err := json.MarshalIndent(verdicts, "", "  ")
	if err != nil {
		return fmt.Errorf("could not render the report: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("could not write the report: %w", err)
	}
	return nil
}

func report(out io.Writer, pol policy, verdicts []verdict) int {
	problems := 0

	for _, v := range verdicts {
		switch v.Status {
		case "denied":
			problems++
			fmt.Fprintf(out, "DENIED       %s@%s ships under %s.\n",
				v.Component, v.Version, strings.Join(v.Licenses, ", "))
			fmt.Fprintf(out, "             The bundle is distributed under a proprietary license, so this cannot ship.\n")
			fmt.Fprintf(out, "             Replace it, or add an exception with a reason to .licenses.json.\n")
		case "unstated":
			problems++
			fmt.Fprintf(out, "UNSTATED     %s@%s carries no license this build could read.\n", v.Component, v.Version)
			fmt.Fprintf(out, "             An unreadable license is treated as forbidden. Read its LICENSE file and\n")
			fmt.Fprintf(out, "             record what you found as an exception in .licenses.json.\n")
		case "unrecognized":
			problems++
			fmt.Fprintf(out, "UNRECOGNIZED %s@%s reports %s, which the policy neither allows nor denies.\n",
				v.Component, v.Version, strings.Join(v.Licenses, ", "))
			fmt.Fprintf(out, "             Decide what it is and add it to allowed or denied in .licenses.json.\n")
		}
	}

	problems += reportStaleExceptions(out, pol, verdicts)

	excepted := 0
	for _, v := range verdicts {
		if v.Status == "excepted" {
			excepted++
		}
	}

	if problems > 0 {
		fmt.Fprintf(out, "\nLicense gate FAILED: %d problem(s) across %d shipped component(s).\n",
			problems, len(verdicts))
		return problems
	}

	fmt.Fprintf(out, "License gate passed: %d shipped component(s), %d under a recorded exception.\n",
		len(verdicts), excepted)
	return 0
}

func reportStaleExceptions(out io.Writer, pol policy, verdicts []verdict) int {
	used := map[string]bool{}
	for _, v := range verdicts {
		if v.Status == "excepted" {
			used[v.Component] = true
		}
	}

	stale := 0
	for _, e := range pol.Exceptions {
		if !used[e.Component] {
			stale++
			fmt.Fprintf(out, "STALE        .licenses.json excuses %s under %s, and nothing shipped matches.\n",
				e.Component, e.License)
			fmt.Fprintf(out, "             It was dropped or relicensed. Remove the exception, or update it.\n")
		}
	}
	return stale
}

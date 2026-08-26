package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// The built-in documentation is static HTML under public/help, which Mattermost
// serves out of the bundle at /plugins/<id>/public/** with no route of our own.
//
// Nothing at build time checks that those pages still describe the code, so the
// checks live here. They exist because documentation that has quietly gone wrong
// is worse than none: a reader trusts it.
//
// The precedent for a test reading repo files is configuration_settings_test.go,
// which parses plugin.json, and icon_test.go, which reads the icon and its
// webapp counterpart.
const helpDir = "../public/help"

// helpPages is every page in the bundle. The sidebar on each one links to all
// of them, which TestHelpNavigationIsComplete enforces, so adding a page means
// adding it here and to every navigation block.
var helpPages = []string{
	"help.html",
	"dtg.html",
	"location.html",
	"airfields.html",
	"cot.html",
	"formats.html",
	"panel.html",
	"admin.html",
	"commands.html",
	"troubleshooting.html",
	"error-codes.html",
}

func readHelpFile(t *testing.T, name string) string {
	t.Helper()

	// #nosec G304 -- fixed, repo-relative path
	data, err := os.ReadFile(filepath.Join(helpDir, name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}

	return string(data)
}

// Every code has to be findable. A reader quoting TF-13004 in a report expects
// this page to say what it is, and an undocumented code makes the registry a
// promise the docs do not keep.
func TestEveryCodeIsDocumented(t *testing.T) {
	page := readHelpFile(t, "error-codes.html")

	// The leading boundary matters: without it this matches the "TF-8" inside
	// the <meta charset="UTF-8"> declaration at the top of every page.
	documented := map[int]bool{}
	for _, m := range regexp.MustCompile(`\bTF-(\d+)\b`).FindAllStringSubmatch(page, -1) {
		code, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("error-codes.html carries a malformed code %q", m[0])
		}
		documented[code] = true
	}

	declared := map[int]bool{}
	for _, code := range errcode.AllCodes {
		declared[code] = true
		if !documented[code] {
			t.Errorf("code TF-%d is declared but missing from error-codes.html", code)
		}
	}

	for code := range documented {
		if !declared[code] {
			t.Errorf("error-codes.html documents TF-%d, which no longer exists", code)
		}
	}
}

// A code cited on the troubleshooting page but never emitted sends a reader
// looking for a failure that cannot happen.
func TestTroubleshootingCitesRealCodes(t *testing.T) {
	page := readHelpFile(t, "troubleshooting.html")

	declared := map[int]bool{}
	for _, code := range errcode.AllCodes {
		declared[code] = true
	}

	for _, m := range regexp.MustCompile(`\bTF-(\d+)\b`).FindAllStringSubmatch(page, -1) {
		code, _ := strconv.Atoi(m[1])
		if !declared[code] {
			t.Errorf("troubleshooting.html cites TF-%d, which no longer exists", code)
		}
	}
}

// Every admin setting must be documented, and admin.html must not describe one
// that has been removed.
//
// The pairing is by an explicit data-setting attribute rather than by deriving
// an anchor id from the key. A convention like "EnableDTG becomes #enable-dtg"
// would have to be encoded here and would break the moment a key was named
// something the rule did not anticipate.
func TestEverySettingIsDocumented(t *testing.T) {
	page := readHelpFile(t, "admin.html")

	documented := map[string]int{}
	for _, m := range regexp.MustCompile(`data-setting="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		documented[m[1]]++
	}

	for key, count := range documented {
		if count > 1 {
			t.Errorf("admin.html documents %s %d times; a setting should have one section", key, count)
		}
	}

	declared := map[string]bool{}
	for _, setting := range loadSettings(t).allSettings() {
		declared[setting.Key] = true
		if documented[setting.Key] == 0 {
			t.Errorf("setting %s is declared in plugin.json but missing from admin.html", setting.Key)
		}
	}

	for key := range documented {
		if !declared[key] {
			t.Errorf("admin.html documents %s, which plugin.json no longer declares", key)
		}
	}
}

// Cross-page deep links are the whole navigation story here, and a renamed
// anchor breaks them silently: the browser lands at the top of the page and the
// reader never learns they missed the section.
func TestEveryCrossPageAnchorResolves(t *testing.T) {
	pages := map[string]string{}
	ids := map[string]map[string]bool{}

	for _, name := range helpPages {
		body := readHelpFile(t, name)
		pages[name] = body

		found := map[string]bool{}
		for _, m := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(body, -1) {
			found[m[1]] = true
		}
		ids[name] = found
	}

	// href="target.html#anchor", href="target.html", and href="#anchor".
	link := regexp.MustCompile(`href="([a-z0-9-]+\.html)?(?:#([^"]+))?"`)

	checked := 0
	for _, from := range helpPages {
		for _, m := range link.FindAllStringSubmatch(pages[from], -1) {
			target, anchor := m[1], m[2]
			if target == "" {
				target = from
			}

			if _, ok := pages[target]; !ok {
				t.Errorf("%s links to %s, which is not a page in the bundle", from, target)
				continue
			}
			if anchor == "" {
				continue
			}

			checked++
			if !ids[target][anchor] {
				t.Errorf("%s links to %s#%s, which has no matching id", from, target, anchor)
			}
		}
	}

	// Without this the test passes just as happily against a bundle whose links
	// the regexp stopped matching.
	if checked == 0 {
		t.Fatal("checked no anchors; the link pattern is not matching the pages")
	}
}

// The sidebar is the only navigation, so a page missing from one page's copy of
// it is a page that cannot be reached from there.
func TestHelpNavigationIsComplete(t *testing.T) {
	for _, name := range helpPages {
		t.Run(name, func(t *testing.T) {
			body := readHelpFile(t, name)

			for _, target := range helpPages {
				if !strings.Contains(body, `href="`+target+`"`) {
					t.Errorf("%s does not link to %s in its navigation", name, target)
				}
			}

			// Exactly one entry is marked current, and it is this page's own.
			active := regexp.MustCompile(`href="([a-z0-9-]+\.html)" class="active"`).FindAllStringSubmatch(body, -1)
			if len(active) != 1 {
				t.Fatalf("%s marks %d navigation entries active, want exactly 1", name, len(active))
			}
			if active[0][1] != name {
				t.Errorf("%s marks %s as the active page", name, active[0][1])
			}
		})
	}
}

// These pages have to render on an air-gapped host with no route out. Anything
// fetched from elsewhere would either fail to load or leak the fact that
// somebody opened the documentation.
func TestHelpPagesAreSelfContained(t *testing.T) {
	banned := []struct {
		pattern *regexp.Regexp
		why     string
	}{
		{regexp.MustCompile(`(?i)<script`), "no JavaScript: these pages are static by requirement"},
		{regexp.MustCompile(`(?i)\son\w+\s*=\s*"`), "no inline event handlers"},
		{regexp.MustCompile(`(?i)(src|href)="https?://`), "no remote assets or absolute links"},
		{regexp.MustCompile(`(?i)@import`), "no imported stylesheets"},
	}

	for _, name := range append(helpPages, "styles.css") {
		t.Run(name, func(t *testing.T) {
			body := readHelpFile(t, name)
			for _, b := range banned {
				if m := b.pattern.FindString(body); m != "" {
					t.Errorf("%s contains %q: %s", name, m, b.why)
				}
			}
		})
	}
}

// Banned repo-wide by CLAUDE.md. check-style does not lint HTML, so nothing
// else would notice.
func TestHelpPagesUseNoEmDash(t *testing.T) {
	for _, name := range append(helpPages, "styles.css") {
		body := readHelpFile(t, name)
		for _, form := range []string{"—", "&mdash;", "&#8212;"} {
			if strings.Contains(body, form) {
				t.Errorf("%s contains an em dash (%s); use a comma, a period or parentheses", name, form)
			}
		}
	}
}

// The System Console header is one of the three places the documentation is
// advertised, and the only one an admin is guaranteed to see. It is also the
// one place the plugin id is written out, since plugin.json defines it.
func TestSettingsHeaderLinksToTheDocs(t *testing.T) {
	want := fmt.Sprintf("/plugins/%s/public/help/help.html", manifest.Id)

	if header := loadSettings(t).SettingsSchema.Header; !strings.Contains(header, want) {
		t.Fatalf("plugin.json settings_schema.header does not link to %s: %q", want, header)
	}
}

// The bundle is what ships. A page on disk that is not listed here is a page no
// other test in this file covers, and one listed but missing would ship a
// sidebar full of dead links.
func TestHelpBundleMatchesTheListedPages(t *testing.T) {
	entries, err := os.ReadDir(helpDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", helpDir, err)
	}

	var onDisk []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".html") {
			onDisk = append(onDisk, entry.Name())
		}
	}

	listed := append([]string(nil), helpPages...)
	sort.Strings(listed)
	sort.Strings(onDisk)

	if strings.Join(listed, " ") != strings.Join(onDisk, " ") {
		t.Fatalf("public/help holds [%s] but the tests cover [%s]",
			strings.Join(onDisk, " "), strings.Join(listed, " "))
	}
}

// The slash commands carry a curated, matcher-verified example corpus, and it is
// the same corpus the decorator pages teach from. Nothing else keeps the two in
// step: a row added to example-details and not to a page leaves the docs quietly
// behind the command, which is the drift the pages exist to avoid.
//
// The pages may show more than the command does. They may not show less.
func TestEveryCommandExampleIsDocumented(t *testing.T) {
	bundle := map[string]string{}
	for _, name := range helpPages {
		bundle[name] = readHelpFile(t, name)
	}

	checked := 0
	for _, key := range detailSetOrder {
		for _, group := range detailSets[key].groups {
			for _, example := range group.examples {
				if undocumentedExamples[example.text] {
					continue
				}
				checked++
				if !documentedSomewhere(bundle, example.text) {
					t.Errorf("%s/%q: no help page shows %q",
						key, group.heading, example.text)
				}
			}
		}
	}

	// Without this the test passes just as happily against an empty catalog.
	if checked == 0 {
		t.Fatal("checked no examples; the catalog is not being read")
	}
}

// Same contract for the Cursor on Target documents, which are whole XML sources
// rather than single tokens.
func TestEveryCotExampleIsDocumented(t *testing.T) {
	page := readHelpFile(t, "cot.html")

	sources := cotExampleSources()
	if len(sources) == 0 {
		t.Fatal("no example sources; the list is not being read")
	}

	for name, source := range sources {
		if !strings.Contains(page, escapeForHelp(strings.TrimSpace(source))) {
			t.Errorf("cot.html does not carry the %q example", name)
		}
	}
}

// The registry is the list of <detail> elements this build claims to read, so a
// reader meets it as a table on the page. It grew from sixteen entries to
// twenty-six in one phase with nothing telling anybody to document the new ones,
// which is the same hole TestEveryCodeIsDocumented and TestEverySettingIsDocumented
// already close for codes and settings.
//
// One direction only, unlike the code test. The reverse is not checkable here:
// the page is full of <code> spans naming attributes, XML fragments and prose
// terms, so "a code span that is not a registry element" is the normal case
// rather than a defect.
func TestEveryRegisteredExtensionIsDocumented(t *testing.T) {
	page := readHelpFile(t, "cot.html")

	extensions := cot.Extensions()
	if len(extensions) == 0 {
		t.Fatal("the registry is empty; it is not being read")
	}

	for _, ext := range extensions {
		if !strings.Contains(page, "<code>"+ext.Element+"</code>") {
			t.Errorf("the registry reads <%s> but cot.html never names it", ext.Element)
		}
	}
}

// cotExampleSources is hand-maintained, and three other tests trust it to be the
// whole set. A constant added to the command and not to that map would weaken
// them all silently, this one included.
func TestEveryCotExampleSourceIsListed(t *testing.T) {
	listed := map[string]bool{}
	for _, source := range cotExampleSources() {
		listed[strings.TrimSpace(source)] = true
	}

	fenced := regexp.MustCompile("(?s)```(?:cot|xml)\\n(.*?)```")

	for _, chunk := range append(cotDetailChunks(true, true), cotExtensionChunks()...) {
		for _, atom := range chunk.lines {
			for _, m := range fenced.FindAllStringSubmatch(atom, -1) {
				if source := strings.TrimSpace(m[1]); !listed[source] {
					t.Errorf("%q posts an example that cotExampleSources does not list:\n%s",
						chunk.heading, source)
				}
			}
		}
	}
}

// A handful of rows exist to exercise the packing and budget logic rather than to
// teach a grammar, and putting them on a page would be noise. Every entry needs a
// reason; if this list grows, the docs are drifting rather than the test being
// wrong.
var undocumentedExamples = map[string]bool{}

func documentedSomewhere(bundle map[string]string, text string) bool {
	want := escapeForHelp(text)
	for _, body := range bundle {
		if strings.Contains(body, want) {
			return true
		}
	}
	return false
}

// The three replacements that matter inside element content, in the order that
// keeps the ampersand from eating the others. html.EscapeString is deliberately
// not used: it also rewrites quotes and apostrophes, which these pages write
// literally.
func escapeForHelp(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

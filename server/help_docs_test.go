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
	"geojson.html",
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
//
// The one script in the bundle is copy.js, which is loaded by src rather than
// written inline. Inline stays banned because a strict CSP on the host serving
// the bundle would need a nonce or a hash to run it, and there is nowhere to put
// one on a static file.
func TestHelpPagesAreSelfContained(t *testing.T) {
	banned := []struct {
		pattern *regexp.Regexp
		why     string
	}{
		{regexp.MustCompile(`(?i)\son\w+\s*=\s*"`), "no inline event handlers"},
		{regexp.MustCompile(`(?i)(src|href)="https?://`), "no remote assets or absolute links"},
		{regexp.MustCompile(`(?i)@import`), "no imported stylesheets"},
	}

	// RE2 has no negative lookahead, so the one allowed script is checked by
	// matching every script element and comparing it, rather than by a pattern
	// that tries to describe everything it is not.
	//
	// Counting opening tags as well is what closes the gap this had at first:
	// an UNTERMINATED `<script src="...">` matches no element, so element
	// comparison alone passed it while a browser still fetched and ran it.
	scriptElement := regexp.MustCompile(`(?is)<script.*?</script>`)
	scriptOpen := regexp.MustCompile(`(?i)<script`)

	for _, name := range append(helpPages, "styles.css", "copy.js") {
		t.Run(name, func(t *testing.T) {
			body := readHelpFile(t, name)
			for _, b := range banned {
				if m := b.pattern.FindString(body); m != "" {
					t.Errorf("%s contains %q: %s", name, m, b.why)
				}
			}

			elements := scriptElement.FindAllString(body, -1)
			for _, found := range elements {
				if found != allowedScript {
					t.Errorf("%s carries %q; the only script this bundle may load is %q",
						name, found, allowedScript)
				}
			}

			if opens := len(scriptOpen.FindAllString(body, -1)); opens != len(elements) {
				t.Errorf("%s has %d script openings but %d complete script elements; "+
					"an unterminated tag still runs in a browser", name, opens, len(elements))
			}

			// At most one, so a page cannot load copy.js twice. Combined with
			// TestEveryHelpPageLoadsTheCopyScript, which requires at least one,
			// that pins every page at exactly one.
			if len(elements) > 1 {
				t.Errorf("%s carries %d script elements; copy.js runs once per page",
					name, len(elements))
			}
		})
	}
}

// The one script tag the bundle may carry, verbatim.
//
// By src rather than inline, because a strict CSP on the host serving these
// files would need a nonce or a hash to run an inline body and there is nowhere
// to put one on a static file. Deferred, so it runs after the examples exist.
const allowedScript = `<script src="copy.js" defer></script>`

// The script no-ops on a page with no examples, so a page that forgets it costs
// nothing today and costs a silently button-less example the first time one is
// added. Cheaper to require it everywhere than to remember the rule.
func TestEveryHelpPageLoadsTheCopyScript(t *testing.T) {
	for _, name := range helpPages {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(readHelpFile(t, name), allowedScript) {
				t.Errorf("%s does not load copy.js", name)
			}
		})
	}
}

// The enhancement may never become the mechanism.
//
// copy.js returns before it touches the DOM when navigator.clipboard is absent,
// which is every plain-HTTP install, and those are the norm for this audience
// rather than an edge case. What is left has to be the affordance that shipped
// before it: user-select on the block and the drawn icon. A build that dropped
// either would leave exactly those readers with no way to take an example at
// all, and nothing else here would notice.
func TestTheCopyScriptOnlyEnhances(t *testing.T) {
	script := readHelpFile(t, "copy.js")

	if !strings.Contains(script, "navigator.clipboard") {
		t.Error("copy.js does not test for navigator.clipboard before using it")
	}
	if !strings.Contains(script, "typeof clipboard.writeText !== 'function'") {
		t.Error("copy.js does not check writeText before calling it")
	}

	styles := readHelpFile(t, "styles.css")
	for _, required := range []string{"user-select: all", ".copyable::before", ".copyable::after"} {
		if !strings.Contains(styles, required) {
			t.Errorf("styles.css no longer carries %q, which is the no-script fallback", required)
		}
	}
}

// Both shapes of copyable are wired, and only those two.
//
// An inline run and a block are the same promise to a reader: the same drawn
// icon on both. Wiring one and not the other was the state this replaced, and
// nothing about it looked broken on either page on its own.
//
// The block selector must stay `pre.copyable` and the inline one
// `code.copyable`. Every block wraps a bare <code>, so a selector that matched
// any descendant code would bind the block's own child a second time and copy
// it twice.
func TestTheCopyScriptWiresBothCopyables(t *testing.T) {
	script := readHelpFile(t, "copy.js")

	for _, selector := range []string{`'pre.copyable'`, `'code.copyable'`} {
		if !strings.Contains(script, selector) {
			t.Errorf("copy.js never selects %s", selector)
		}
	}

	// Neither handler may swallow the default, or the selection that is the
	// whole fallback stops happening on the pages that do have the script.
	for _, banned := range []string{"preventDefault", "stopPropagation"} {
		if strings.Contains(script, banned) {
			t.Errorf("copy.js calls %s, which would suppress the user-select fallback", banned)
		}
	}

	// The block markup the wiring assumes, swept over every page rather than
	// the one that happens to carry the examples today. A block that stopped
	// wrapping a <code> would copy the button's label along with the event.
	//
	// Attribute-tolerant: the blocks carry tabindex as well as class, so
	// matching a fixed opening tag pinned the attribute order too.
	opening := regexp.MustCompile(`(?i)<pre[^>]*class="copyable"[^>]*>`)
	wrapping := regexp.MustCompile(`(?i)<pre[^>]*class="copyable"[^>]*><code[^>]*>`)

	total := 0
	for _, name := range helpPages {
		body := readHelpFile(t, name)
		blocks := len(opening.FindAllString(body, -1))
		wrapped := len(wrapping.FindAllString(body, -1))
		if blocks != wrapped {
			t.Errorf("%s: %d of %d example blocks do not wrap a <code>", name, blocks-wrapped, blocks)
		}
		total += blocks
	}

	if total == 0 {
		t.Error("no example blocks found on any help page; point this test at wherever they moved")
	}

	// A scrolling box has to be reachable by keyboard, or the part of a long
	// example that is off screen cannot be read without a pointer.
	loose := regexp.MustCompile(`(?i)<pre(?:\s[^>]*)?>`)
	for _, name := range helpPages {
		body := readHelpFile(t, name)
		for _, tag := range loose.FindAllString(body, -1) {
			if !strings.Contains(tag, `tabindex="0"`) {
				t.Errorf("%s carries %q; pre is overflow-x: auto, so it needs tabindex=\"0\"", name, tag)
			}
		}
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

// The examples command carries a curated, matcher-verified corpus, and the
// decorator pages carry that and everything else. Nothing else keeps the two in
// step: a row added to a set and not to a page leaves the docs quietly behind
// the command, which is the drift the pages exist to avoid.
//
// The pages may show more than the command does. They may not show less.
func TestEveryCommandExampleIsDocumented(t *testing.T) {
	bundle := map[string]string{}
	for _, name := range helpPages {
		bundle[name] = readHelpFile(t, name)
	}

	checked := 0
	for _, key := range exampleSetOrder {
		for _, row := range exampleSets[key].rows {
			if undocumentedExamples[row.text] {
				continue
			}
			checked++
			if !documentedSomewhere(bundle, row.text) {
				t.Errorf("%s/%s: no help page shows %q", key, row.label, row.text)
			}
		}
	}

	// Without this the test passes just as happily against an empty catalog.
	if checked == 0 {
		t.Fatal("checked no examples; the catalog is not being read")
	}
}

// Same contract for the Cursor on Target documents, which are whole XML sources
// rather than single tokens. The page may show more than the command does, and
// every example the command posts has to be somewhere a reader can find it.
func TestEveryCotExampleIsDocumented(t *testing.T) {
	page := readHelpFile(t, "cot.html")

	if len(cotExampleOrder) == 0 {
		t.Fatal("no example sources; the catalog is not being read")
	}

	for i, example := range cotExampleOrder {
		for line := range strings.SplitSeq(example.source, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.Contains(page, escapeForHelp(line)) {
				t.Errorf("example %d posts a line cot.html does not show:\n%s", i+1, line)
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

// The counts in prose are the one thing about the settings that no other test
// reads, and they are stated in four places. Adding the GeoJSON section left
// the design note at "twenty-two across five" and the two help pages at
// "twenty-four", none of which had ever been true of the same manifest.
func TestTheStatedSettingCountsMatchTheManifest(t *testing.T) {
	switches, off := 0, []string{}
	for _, setting := range loadSettings(t).allSettings() {
		if setting.Type != "bool" {
			continue
		}
		switches++
		if on, ok := setting.Default.(bool); !ok || !on {
			off = append(off, setting.Key)
		}
	}
	sections := len(loadSettings(t).SettingsSchema.Sections)

	for _, claim := range []struct {
		file   string
		phrase string
	}{
		{"admin.html", spellNumber(t, sections) + " sections and " + spellNumber(t, switches) + " switches"},
		{"help.html", "The " + spellNumber(t, switches) + " switches in the System Console, in " + spellNumber(t, sections) + " sections"},
	} {
		if !strings.Contains(strings.ToLower(readHelpFile(t, claim.file)), strings.ToLower(claim.phrase)) {
			t.Errorf("%s does not state %q; the manifest has %d switches in %d sections",
				claim.file, claim.phrase, switches, sections)
		}
	}

	page := readHelpFile(t, "admin.html")
	for _, key := range off {
		if !strings.Contains(page, "<code>"+key+"</code> and") && !strings.Contains(page, "and <code>"+key+"</code>") {
			t.Errorf("%s ships off but admin.html's tagline does not name it among the ones that do", key)
		}
	}
}

func spellNumber(t *testing.T, n int) string {
	t.Helper()

	words := map[int]string{
		20: "twenty", 21: "twenty-one", 22: "twenty-two", 23: "twenty-three",
		24: "twenty-four", 25: "twenty-five", 26: "twenty-six", 27: "twenty-seven",
		28: "twenty-eight", 29: "twenty-nine", 30: "thirty",
		4: "four", 5: "five", 6: "six", 7: "seven", 8: "eight",
	}
	word, ok := words[n]
	if !ok {
		t.Fatalf("no spelling for %d; extend the table when the manifest grows", n)
	}

	return word
}

package decorators

import (
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Ranges that must never be scanned. Ported from the reference implementation
// in mattermost-plugin-aocanywhere (webapp/src/enhanced_text/tagger.ts), which
// is the non-obvious part of this algorithm.
// Inline constructs whose interior must never be rewritten.
//
// Block constructs (fenced and indented code) are found by scanning lines
// instead, because Go's regexp is RE2: it has no backreferences, so "a closing
// fence matching the opener" cannot be expressed here, and an unterminated
// fence cannot be bounded by a pattern that requires a closing one.
// nestDepth is how many levels of balanced delimiters the link expressions
// below match.
//
// Go's regexp is RE2, which has no recursion, so "balanced to any depth" cannot
// be written as a pattern. It is spelled out to a fixed depth instead. Four
// covers anything a person writes by hand; the cost of another level is a
// longer expression and nothing else, since RE2 matches in linear time
// regardless.
//
// The failure mode past the limit is a rewrite, not a missed one, so this is a
// bound on how much protection there is rather than a tuning knob. Raise it
// before narrowing it.
//
// The simple forms below soften the limit but do not remove it: they cover a
// token sitting *inside* the nesting, and not one sitting after it but still
// within the construct, as in "[a [b [c [d [e]]]] TOKEN](/uri)". Removing the
// limit outright needs a hand-written scanner counting delimiters, the way the
// fences below are scanned, which is the honest fix if this ever bites.
const nestDepth = 4

// balancedExpr builds an expression matching open/close delimited spans nested
// to the given depth. inner is the character class for ordinary content.
func balancedExpr(open, close, inner string, depth int) string {
	expr := open + `(?:` + inner + `)*` + close
	for i := 1; i < depth; i++ {
		expr = open + `(?:` + inner + `|` + expr + `)*` + close
	}
	return expr
}

// linkLabelExpr is the text between a link's brackets, and linkDestExpr is the
// parenthesised destination behind it.
//
// Both allow balanced delimiters, because CommonMark does: "[link [foo [bar]]]
// (/uri)" is a link verbatim from the spec, and "[x](/a(b)c)" and
// `[x](/a "(note)")` are links too. Refusing a raw "[" in the label, or ending
// the destination at the first ")", meant the construct was not recognised as a
// link at all and the text behind it was left open to rewriting.
//
// linkLabelExpr crosses newlines and bracketSpanExpr does not, which is the
// same split the simple forms had before nesting was added to them.
//
// A real label may wrap, so a link whose text runs over two lines still has to
// be recognised: failing to would leave its destination open to rewriting,
// which is corruption. A *standalone* bracket has no such claim on the reader's
// intent, and letting one span the message means a single stray "[" silently
// suppresses decoration for everything after it, which is why the catch-all
// below is bounded to its line.
var (
	linkLabelExpr   = balancedExpr(`\[`, `\]`, `[^\[\]\\]|\\.`, nestDepth)
	linkDestExpr    = balancedExpr(`\(`, `\)`, `[^()\n\\]|\\[^\n]`, nestDepth)
	bracketSpanExpr = balancedExpr(`\[`, `\]`, `[^\[\]\n\\]|\\[^\n]`, nestDepth)
)

// The forms the balanced expressions replaced, kept alongside them.
//
// Balanced matching recognises more, and recognises nothing at all when the
// delimiters do not balance. `[x](/u "a (b")` has an unpaired "(" inside a
// title, where CommonMark allows it, and so is not a link to the expression
// above; `[a\]` ends in an escape the simple form never looked for. In both
// cases the balanced expression matches nothing and leaves the interior open to
// exactly the rewriting it was added to prevent. These stop at the first
// closing delimiter, which is less than the whole construct but never nothing.
//
// Protection is the union of every expression here, so keeping both means it is
// never worse than either alone. Order carries no meaning.
const (
	simpleDestExpr  = `\([^)\n]*\)`
	simpleBracketRe = `\[[^\]\n]*\]`
)

var inlineProtectedRes = []*regexp.Regexp{
	// Inline and reference links, and images.
	regexp.MustCompile(`!?` + linkLabelExpr + linkDestExpr),
	regexp.MustCompile(`!?` + linkLabelExpr + simpleDestExpr),
	regexp.MustCompile(`!?` + simpleBracketRe + linkDestExpr),
	regexp.MustCompile(`!?` + simpleBracketRe + simpleDestExpr),
	regexp.MustCompile(`!?` + linkLabelExpr + simpleBracketRe),
	regexp.MustCompile(`!?` + simpleBracketRe + simpleBracketRe),

	// Link reference definitions, e.g. "[plan]: https://example.com".
	regexp.MustCompile(`(?m)^ {0,3}\[[^\]\n]*\]:[^\n]*`),

	// Any remaining bracketed span. A shortcut reference link is just "[label]",
	// and there is no way to know from the text alone whether a definition for
	// it exists elsewhere. Protecting the span costs only the decoration of a
	// token that was inside brackets.
	regexp.MustCompile(bracketSpanExpr),
	regexp.MustCompile(simpleBracketRe),

	// Angle autolinks and inline HTML, whose attributes can carry anything.
	regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9+.\-]*:[^<>\s]*>`),
	regexp.MustCompile(`</?[a-zA-Z][^<>\n]*>`),

	// Bare URLs, which Mattermost autolinks. Rewriting inside one destroys the
	// reader's link.
	regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s<>]+`),
	regexp.MustCompile(`\bwww\.[^\s<>]+`),
}

// Characters that would otherwise be re-parsed as markdown inside a link
// label. Escaping is the framework's job, not the decorator's.
var labelEscaper = strings.NewReplacer(
	`\`, `\\`,
	`[`, `\[`,
	`]`, `\]`,
	`*`, `\*`,
	`_`, `\_`,
	"`", "\\`",
	`~`, `\~`,
)

// Tagger rewrites tokens in a message into decorator links.
type Tagger struct {
	Registry *Registry

	// URLPrefix is everything before the decorator type, with no scheme or
	// host, e.g. "/plugins/com.mattermost.plugin-tactical-fusion/decorate".
	// On a subpath install it carries SiteURL's path, e.g.
	// "/mattermost/plugins/.../decorate".
	URLPrefix string
}

type byteRange struct{ start, end int }

func (r byteRange) overlaps(other byteRange) bool {
	return r.start < other.end && r.end > other.start
}

type candidate struct {
	byteRange
	decoratorIdx int
	patternIdx   int
	typ          string
	params       url.Values

	// label is what the link reads, which is the pattern's captured value
	// rather than the whole matched span. The two differ only for a pattern
	// that deliberately matches more than it links.
	label string
}

// Decorate returns the message with every recognised token replaced by a
// markdown link. It returns the input unchanged when nothing matches, so
// callers can compare identity to decide whether anything happened.
//
// Decorate is idempotent: an existing decorator link is itself a protected
// span, so running it over its own output is a no-op.
//
// # What it refuses to touch
//
// The result is written permanently into stored post text, so a mistake here
// destroys something a user wrote and cannot be undone. Protection is therefore
// per span rather than per message: a token sitting in ordinary prose is
// decorated even when the message also contains a link or a code block, but a
// token inside one of those is left exactly as written.
//
// findProtectedRanges is the whole safety story, so anything it fails to
// recognise is a corruption bug. Widen it with a test per construct.
func (t *Tagger) Decorate(message string, ref time.Time) string {
	if message == "" || t.Registry == nil {
		return message
	}

	protected := findProtectedRanges(message)
	candidates := t.findCandidates(message, ref, protected)
	if len(candidates) == 0 {
		return message
	}

	// Defensive, and expected to stay uncovered: resolveOverlaps always accepts
	// its first candidate, since nothing is claimed yet, so an empty result here
	// means an empty input, which the check above already returned on.
	accepted := resolveOverlaps(candidates)
	if len(accepted) == 0 {
		return message
	}

	return t.applyReplacements(message, accepted)
}

// findProtectedRanges collects every span whose interior must be left exactly
// as the author wrote it: code, links, and URLs.
//
// Overlapping spans are merged rather than discarded. Dropping a span because
// it overlapped an earlier one meant a construct could lose its protection
// entirely and have a link written into its interior, which is the opposite of
// what a protected range is for.
func findProtectedRanges(message string) []byteRange {
	ranges := blockRanges(message)
	ranges = append(ranges, codeSpanRanges(message)...)

	for _, re := range inlineProtectedRes {
		for _, m := range re.FindAllStringIndex(message, -1) {
			ranges = append(ranges, byteRange{m[0], m[1]})
		}
	}

	return mergeRanges(ranges)
}

// blockRanges finds fenced and indented code blocks.
//
// Scanned line by line because RE2 cannot express "a closing fence matching the
// opener", and because an unterminated fence has to run to the end of the
// message. Mattermost renders text after an unclosed ``` as code, so we must
// treat it as code too.
func blockRanges(message string) []byteRange {
	var ranges []byteRange

	fenceStart, fenceChar, fenceWidth := -1, byte(0), 0

	for offset := 0; offset < len(message); {
		end := strings.IndexByte(message[offset:], '\n')
		lineEnd := len(message)
		if end >= 0 {
			lineEnd = offset + end
		}
		line := message[offset:lineEnd]

		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		switch {
		case fenceStart >= 0:
			if indent <= 3 && closesFence(trimmed, fenceChar, fenceWidth) {
				ranges = append(ranges, byteRange{fenceStart, lineEnd})
				fenceStart, fenceChar, fenceWidth = -1, 0, 0
			}

		case indent <= 3 && fenceWidthOf(trimmed, '`') >= 3:
			fenceStart, fenceChar, fenceWidth = offset, '`', fenceWidthOf(trimmed, '`')

		case indent <= 3 && fenceWidthOf(trimmed, '~') >= 3:
			fenceStart, fenceChar, fenceWidth = offset, '~', fenceWidthOf(trimmed, '~')

		case isIndentedCode(line):
			ranges = append(ranges, byteRange{offset, lineEnd})
		}

		if end < 0 {
			break
		}
		offset = lineEnd + 1
	}

	// An opener with no closer protects the rest of the message.
	if fenceStart >= 0 {
		ranges = append(ranges, byteRange{fenceStart, len(message)})
	}

	return ranges
}

// fenceWidthOf returns the length of the run of marker characters a line opens
// with, or 0 if it does not start with one.
func fenceWidthOf(trimmed string, marker byte) int {
	width := 0
	for width < len(trimmed) && trimmed[width] == marker {
		width++
	}
	return width
}

// closesFence reports whether a line ends a fenced block opened with the given
// marker and width.
//
// Two rules, and getting either wrong means rewriting text the reader sees as
// code. The run must be at least as long as the opener, so "```" does not close
// "````". And nothing may follow it but spaces: an info string is allowed on an
// opening fence only, so "```js" continues the block rather than ending it.
func closesFence(trimmed string, marker byte, width int) bool {
	run := fenceWidthOf(trimmed, marker)
	if run < width {
		return false
	}

	// Whitespace, not just spaces. A carriage return matters most: a client
	// sending CRLF leaves one on every line, and reading it as content left the
	// block open over the whole rest of the message.
	return strings.TrimLeft(trimmed[run:], " \t\r") == ""
}

// isIndentedCode reports whether a line is indented far enough to render as a
// code block. Blank lines do not start one.
func isIndentedCode(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	if strings.HasPrefix(line, "\t") {
		return true
	}
	return strings.HasPrefix(line, "    ")
}

// codeSpanRanges finds inline code spans of any backtick width.
//
// Hand-scanned because matching a closing run of the same length as the opener
// needs a backreference, which RE2 does not have. An opener with no matching
// closer is literal text and is deliberately left unprotected.
func codeSpanRanges(message string) []byteRange {
	var ranges []byteRange

	for i := 0; i < len(message); {
		if message[i] != '`' {
			i++
			continue
		}

		openEnd := runEnd(message, i)
		width := openEnd - i

		if closeEnd, found := findClosingRun(message, openEnd, width); found {
			ranges = append(ranges, byteRange{i, closeEnd})
			i = closeEnd
			continue
		}

		i = openEnd
	}

	return ranges
}

// findClosingRun locates a backtick run of exactly the given width.
func findClosingRun(message string, from, width int) (int, bool) {
	for i := from; i < len(message); {
		if message[i] != '`' {
			i++
			continue
		}

		end := runEnd(message, i)
		if end-i == width {
			return end, true
		}
		i = end
	}

	return 0, false
}

// runEnd returns the index just past a run of backticks starting at i.
func runEnd(message string, i int) int {
	for i < len(message) && message[i] == '`' {
		i++
	}
	return i
}

// mergeRanges collapses overlapping and adjacent spans into a minimal set.
func mergeRanges(ranges []byteRange) []byteRange {
	if len(ranges) < 2 {
		return ranges
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })

	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.start <= last.end {
			last.end = max(last.end, r.end)
			continue
		}
		merged = append(merged, r)
	}

	return merged
}

func overlapsAny(r byteRange, ranges []byteRange) bool {
	return slices.ContainsFunc(ranges, r.overlaps)
}

// findCandidates runs every registered pattern and keeps the matches that are
// outside protected ranges and that their decorator accepts.
//
// A match rejected by Parse does not claim its range, so a shorter valid match
// at the same span can still win.
func (t *Tagger) findCandidates(message string, ref time.Time, protected []byteRange) []candidate {
	var candidates []candidate

	for di, d := range t.Registry.All() {
		for pi, p := range d.Patterns() {
			if p.Regexp == nil {
				continue
			}
			for _, loc := range p.Regexp.FindAllStringSubmatchIndex(message, -1) {
				r := byteRange{loc[0], loc[1]}
				if overlapsAny(r, protected) {
					continue
				}

				groups := submatches(message, loc)
				value := p.Value(groups)
				params, ok := d.Parse(value, ref)
				if !ok {
					continue
				}

				candidates = append(candidates, candidate{
					byteRange:    r,
					decoratorIdx: di,
					patternIdx:   pi,
					typ:          d.Type(),
					params:       params,
					label:        value,
				})
			}
		}
	}

	return candidates
}

func submatches(message string, loc []int) []string {
	groups := make([]string, len(loc)/2)
	for i := range groups {
		if loc[2*i] >= 0 {
			groups[i] = message[loc[2*i]:loc[2*i+1]]
		}
	}
	return groups
}

// resolveOverlaps picks a non-overlapping set: longest match wins, and
// registration order only breaks ties. Ordering by registration alone would be
// a hidden global coupling, where adding a decorator silently changes an
// existing one's behaviour.
func resolveOverlaps(candidates []candidate) []candidate {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if la, lb := a.end-a.start, b.end-b.start; la != lb {
			return la > lb
		}
		if a.decoratorIdx != b.decoratorIdx {
			return a.decoratorIdx < b.decoratorIdx
		}
		if a.patternIdx != b.patternIdx {
			return a.patternIdx < b.patternIdx
		}
		return a.start < b.start
	})

	var accepted []candidate
	var claimed []byteRange
	for _, c := range candidates {
		if overlapsAny(c.byteRange, claimed) {
			continue
		}
		accepted = append(accepted, c)
		claimed = append(claimed, c.byteRange)
	}

	return accepted
}

// applyReplacements rewrites right to left so earlier indices stay valid.
func (t *Tagger) applyReplacements(message string, accepted []candidate) string {
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].start > accepted[j].start })

	result := message
	for _, c := range accepted {
		link := "[" + labelEscaper.Replace(c.label) + "](" + t.buildURL(c.typ, c.params) + ")"
		result = result[:c.start] + link + result[c.end:]
	}
	return result
}

// URLFor returns the root-relative decorator URL for a set of params.
//
// Exported so callers that already hold parsed params, such as the examples
// command, can build a link without going through message text.
func (t *Tagger) URLFor(typ string, params url.Values) string {
	return t.buildURL(typ, params)
}

// buildURL returns the root-relative decorator URL. It never contains a scheme
// or host: the URL is stored permanently in post text, and an absolute one
// would break every historical post the day the server changes hostname.
func (t *Tagger) buildURL(typ string, params url.Values) string {
	query := params.Encode()

	// url.Values.Encode leaves parentheses unescaped, and an unbalanced ")"
	// terminates a markdown link destination.
	query = strings.NewReplacer("(", "%28", ")", "%29").Replace(query)

	return t.URLPrefix + "/" + typ + "?" + query
}

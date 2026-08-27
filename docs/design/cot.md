# Cursor on Target

## Cursor on Target

A CoT event posted as a fenced block or as the sole attachment is rendered as a
custom post type: a card carrying the event's identity, position, timing and
accuracy, with the position and its stated circular error drawn on a map.

This note is the whole rationale. None of it may go in a comment.

### Why CoT is not a decorator

The decorator framework is token to link to page. CoT produces no link, so it
has no query string, and `RenderPage` would have nothing to render. It also
could not work through the tagger even if it wanted to: a fenced code block is a
protected range, and `findProtectedRanges` exists precisely so nothing is ever
rewritten inside one.

So `server/cot/` is a sibling of `decorators/`, and recognition is a second step
in the hook rather than a new `Pattern`.

**What that costs later, recorded now while the choice is cheap.** The RHS
dispatches only through the decorator registry: `RhsView` is `get(selection.type)`,
`RhsTitle` does the same, and `selection.ts` is only ever set from
`parseDecoratorHref` plus `decorator.fromParams`. A Phase 2 CoT panel therefore
costs either a second RHS dispatch path, or a sham decorator registration that
also emits a stylesheet rule and a click route for a `/decorate/cot` page that
`http.go` would 404. Neither is free, and the panel is not worth either yet.

### Decoration and CoT stamping are mutually exclusive

`cotStamp` runs **first**, and a post it stamps is never decorated.

The reason is not tidiness. Text around the fence is allowed, and that text can
contain a coordinate. The card renders `lead` and `trail` as plain text nodes,
because a plugin post body has no access to Mattermost's markdown renderer, so a
decorator link written into that text could not render there either: the reader
would meet `[21.3353N 157.9483W](/plugins/...)` spelled out above the card.

Capturing `lead` and `trail` from the undecorated message and decorating anyway
was the obvious alternative and is declined. It leaves a decorator link in the
**stored** message that the webapp then suppresses, so mobile, exports and
Postgres search results would show a link the webapp does not, and the plugin
would have written a permanent rewrite into a message whose rendering it hides.

The cost is real and accepted: a coordinate or a date-time group written beside
a CoT event is not decorated. The position inside the event is linked by the
card regardless. `TestCotAndDecorationAreExclusive` holds it.

`TestSoleFencedBlockIsAlwaysProtected` closes a narrower question, that no
tagger candidate is ever found inside a fence. That is what keeps the two steps
from disagreeing about the same characters. It is not what keeps them apart.

### The hook restructure

Three early returns in the old `decoratePost` belonged to decoration alone, and
two of them would have killed the file case outright: `post.Message == ""` is
the file case by definition, and `p.decorators == nil` is a registry CoT does
not use. Only the `post == nil` check is shared.

`cotStamp` carries its own recover, with the API captured before the deferred
call for the reason `decoratePost` does it: logging through a nil or broken API
inside the recover would panic again from within the deferred function and
escape the hook. Because CoT now runs first, the recover's job is to leave the
post exactly as it found it so decoration still gets its turn.

**The commit is atomic.** The candidate props map is built and measured in full
before `Post.Type` and the props are written, with nothing between them that can
panic, and the work happens on a clone `cotStamp` owns. A half-stamp is worse
than no stamp: `Post.Type` survives every edit, there is no
`MessageWillBeUpdated` hook, and a post that got the type without the props
would lose search, embeds and translation forever while rendering as plain text.

### The props budget is a refusal path, not a formality

`Post.IsValid` rejects a post whose **whole props map** exceeds
`model.PostPropsMaxRunes`, and props are shared with every other integration's.
A cap derived from `maxCotBytes` rather than from that limit would let a CoT post
carrying a webhook's props be refused outright, which is the "author cannot post
at all" failure the comment block at the top of `hooks.go` exists to prevent,
reproduced in the props channel.

So the check counts runes over the marshalled JSON, the unit `Post.IsValid`
uses, against `model.PostPropsMaxUserRunes`, which is the limit less the 40000
runes the SDK reserves for its own pre-save modifications. Over budget, the
clone is discarded and the post is left alone.

It marshals with `json.Marshal` rather than `model.StringInterfaceToJSON`,
which discards the error and answers `""`. An unmeasurable props map would have
scored zero runes and sailed through the one gate that exists to stop the server
refusing the post: the gate has to fail closed, and through the SDK helper it
failed open on exactly the input it could not measure.

### Freshness: `edit_at`, and why there is no digest

The card is built from props written once, at post time. `Post.Type` survives an
edit; `Props` may not, since they are replaced by whatever the client sends and
are absent from `PreserveIdentityPropsFrom`'s rescue list. So a stamped post can
arrive describing something that is no longer there.

`post.edit_at !== 0` answers that exactly, on the post, with no code on either
side.

**A digest over the message was the first design and was wrong three times
over.** FNV-1a 64 hand-written in Go and TypeScript would have been a fourteenth
cross-language duplicate; it is underspecified across UTF-8 bytes, UTF-16
strings and JavaScript's missing 64-bit integer; and it would have been computed
inside a multi-plugin hook chain, where `RunMultiHook` hands the post to the next
plugin, so one co-installed formatting plugin would have turned every CoT post on
that install into raw XML forever with nothing logged.

Storing the whole message in props and comparing it directly was the other
candidate. It is exact, but it doubles the stored size of every CoT post and puts
the message in exports twice.

The file case carries no `edit_at` question of its own but does carry another:
`PostPatch` includes `file_ids`, so an attachment can be swapped out. The card
therefore also requires `post.file_ids` to still name the stamped file.

### Why the webapp never reads `post.message`

`lead`, `src` and `trail` are in props: the text before the event, the event
itself, and the text after it. The card renders them in that order.

This costs roughly one extra copy of the message. What it buys is that **no byte
offset, no fence rule and no digest has to mean the same thing in two
languages** - which is where every reviewer independently expected this design to
break. The Go fence scanner stays Go-only, and there is no cosmetic extractor in
TypeScript to drift from it.

`src` is capped for storage at `maxInlineSrcRunes`, separately from the 64 KiB
parse ceiling. A file-case `src` is bounded only by what `Parse` accepts and is
stored JSON-escaped on every such post. A CoT event is typically one to three
kilobytes, so the parse ceiling is a ceiling and not an expectation.

Over the cap the source is **truncated with a visible marker, not withheld**.
Withholding it read as reasonable until the fence case was traced: the webapp
never reads `post.message`, so a fenced event over the cap left no reader any way
to reach its XML at all, and the download link that was supposed to cover the
gap exists only for the file case.

### The fallback may never be blank

A file-case post has an empty message by construction, and
`docs/design/mapping.md` records, measured against Mattermost master, that
**attachments are dropped when a plugin owns the body**. A fallback that rendered
`post.message` alone would therefore leave a permanently blank post with no exit.

So the fallback renders the message, or, when there is none, a download link per
entry in `post.file_ids`. For the same reason the card **always** renders the
source file as a download link in the file case, unconditionally.

### The post type is forgeable, and is stripped

`Post.IsValid` accepts any `custom_`-prefixed type from an ordinary REST client,
and props under a plugin's key are not protected. So anyone who can post could
otherwise hand a reader a card whose rows AND whose source pane were both
authored to agree with each other, which is exactly what a reader opens that
pane to rule out. Pasting a lie is not the same thing: there the card is
derived from the XML on screen, so the two cannot disagree.

`cotStamp` therefore strips the type and the props from any post that arrives
already wearing them, and lets recognition decide again from the message. A real
event is simply re-stamped from its own text. The post is never refused, because
nothing on this path may refuse a post.

The strip is held in a variable declared **before** the deferred recover, and
the recover returns it. Returning nil from the recover would have put the
forgery back, which is the bug the first version of this had.

### What the parser refuses, and why token-aware

Refusals before decoding: over `maxCotBytes`, and not valid UTF-8.

Refusals during decoding, all on tokens rather than by scanning text:

- Any `xml.Directive`. `<!DOCTYPE ...>` arrives as a `Directive`, verified
  against the pinned SDK, so it cannot be smuggled past behind a comment or
  leading whitespace the way a `strings.Contains` check can.
- Any `xml.ProcInst` other than a leading `xml` declaration.
- More than one root element. `encoding/xml` decodes `<event/><event/>` without
  complaint, verified, so "exactly one" is an explicit check and a test.
- A root that is not `event`; depth over `maxCotDepth`; more than
  `maxCotElements` elements.

`Strict: true` with no entity map and no `CharsetReader` closes the rest: an
undefined entity is a syntax error, which kills entity expansion, and a non-UTF-8
`encoding=` declaration errors because no `CharsetReader` is set, which closes
the UTF-16 route. The byte cap is the real bound on CPU; the depth and element
budgets are defense in depth.

Every refusal the top of the document gets also holds inside `<remarks>`, which
is the one element that reads arbitrary author text: the depth and element
budgets, the directive refusal and the processing-instruction refusal are all
enforced there too. They were not, at first, and a payload buried in remarks
escaped both budgets entirely.

Prose is refused on **both** sides of the root, not just after it. "Not a real
event, ignore me: `<event .../>`" is as much a lie as the same words afterwards.

XML also rejects C0 control characters itself, so those never reach `sanitize`
through `Parse`. The bidirectional overrides are legal XML and do reach it, which
is why `sanitize` has its own tests rather than being exercised only through the
parser.

**`sanitize` strips the whole `Cf` category**, not just the five ranges it began
with. `unicode.IsControl` is `Cc` only, so U+200E, U+200F, U+200B and the rest
survived it; none of them belongs in a callsign, and two `uid` values that render
identically is impersonation of another track.

**The ranges themselves are gone**, and were dead from the moment the category
test went in above them. All ten codepoints they named, U+202A to U+202E,
U+2066 to U+2069 and U+FEFF, are in `Cf`, so the three arms below it could never
run. They read as a second, narrower defense and were none:
`TestTheCategoryTestCoversTheRangesItReplaced` sweeps every one of them rather
than sampling, so a Go release that moved any codepoint out of `Cf` fails there
instead of quietly letting an override back into a callsign.

**Every author-controlled string is sanitised, including the three that are not
fields.** `lead`, `trail` and `src` were the last unfiltered values, and `src` is
the one that matters most: the panel's source pane is what a reader opens to
check the card, so an override surviving there subverts the verification rather
than the claim. `sanitizeText` is `sanitize` without the trim, because trimming the
author's own message text would restyle what they wrote, which is a different
thing from removing what they cannot see.

An event must carry a non-empty `uid`, a non-empty `type` and a parseable `time`
to be stamped. Without those the card is a table of blanks, strictly worse than
the code block it hid.

### The source lives in the panel, not on the card

The card said what the event is; the "Show XML" disclosure under it said what the
event literally was. Both on one surface made the card the place a reader both
read a claim and checked it, which is more than a channel post needs to be: the
rows are the answer, and the raw source is the thing you go and look at when the
answer surprises you.

So the source is the panel's, reached through "Open details", and the card keeps
only the file download link, which is a different thing: for a file-case post it
is the sole route to the original attachment, and `Post.Type` has already cost
that post Mattermost's own attachment list.

The forgery and sanitising arguments elsewhere in this note are unchanged by the
move. They are about the pane being trustworthy wherever it is drawn, and it is
still drawn.

It is a `<details>`, collapsed, for the reason the processing path is: it is what
you go and look at, not what you read on the way past, and open it pushed the
extension groups a screen down the panel.

**A disclosure has to look like a control, not a heading.** The first version
reused `styles.summary`, which is the group heading style, so "As posted" sat in
the panel as uppercase micro-text with a small native triangle beside it, in a
column of sections headed exactly that way. It read as a heading over an empty
section rather than as something to open, and the processing path had the same
problem for longer. `Disclosure` is the shared answer: a bordered, tinted row, a
normal-case bold label, and the word **Show** or **Hide** in the link color
beside it. The word is the part that cannot be mistaken for a label, which is
why the state is held in React rather than left to the browser: `<details>` knows
whether it is open and CSS could rotate a caret, but nothing inline can make it
say so in words. It carries a `CopyButton`, the
location decorator's, inside its `<summary>`, in a span that calls
`preventDefault` so copying does not also toggle the disclosure. The button
hides itself on a plain-HTTP origin, which for an air-gapped install is the
norm, and the `<pre>` stays selectable there.

### Only the button opens the sidebar

A card-wide click handler was tried and removed. It made the whole 640px card
open the panel, and needed a guard for every part of it that means something
else by a click: an `a`, a `button`, the map frame, and a click that merely ends
a text selection. Four exceptions to one rule is the rule saying it was wrong.

A card is a block of readings people select text out of and links they follow.
"Open details" is the affordance, it is already a button, and it is the keyboard
and screen reader path. The wrapper takes no `onClick`, no `role` and no
`tabIndex`.

### Several events in one source

`Parse` returns every event the source carries, and the props hold an `events`
array. A batch of position reports or a set of markers pasted together is one
post, and one post is what it stays.

**One malformed event spoils the source.** A block whose third event is missing
a uid is a block somebody will read as three good ones, so nothing is stamped
rather than a card showing two of three. `maxCotEvents` refuses rather than
truncates for the same reason: a card showing the first thirty-two of two
hundred is quietly wrong about what was posted.

**The card lists them; the panel carries them.** Rendering every event in full
would put N maps and N tables in a channel, against a WebGL budget the inline
map already has to respect. The list names each track, colors its dot and links
its position, and the panel behind "Open details" has the full detail of every
one. A single event still gets the full card, unchanged.

**The map draws them all.** `LocationMap` takes `markers`, one per event, and
frames the whole set rather than opening on the first, which would leave the
rest off screen with nothing to say they were there. The image is named per
FEATURE rather than per layer, because one icon on the layer would paint a
hostile track in a friendly color, and an image is registered once per color
rather than once per marker. The accuracy circle is drawn only for a single
event: a ring per track reads as overlapping blobs rather than as positions.

**The panel could not be tested with two events until it was.** For most of this
work `CotPanelHarness` hard-coded `events: [{...}]`, a single event over an
all-empty baseline, and `events` was not a prop. The count header, the rule
between one event and the next, and `CotTitle`'s count-rather-than-callsign
title were therefore not merely untested but unreachable from a test, on the
one component that renders exactly what the "several events in one source" work
above is about. Line coverage hid it: the file read 91% of lines and 51% of
branches. The harness now takes an optional `events` array, mapped over the same
baseline, the way `CotPostBodyHarness` already did.

**The rule between events is asserted by its margin, not its border.**
`styles.later` is `1px solid rgba(var(--center-channel-color-rgb), 0.16)` plus a
`20px` top margin. The harness defines no theme, so the variable is undefined,
the color is invalid, and the browser drops the whole `border-top` shorthand:
`getComputedStyle` reports `0px` for a border React really did apply. The margin
in the same style computes normally and is the separation a reader sees, so it
is what the test counts. Counting the separated events rather than checking the
second one is deliberate: it fails both when the rule is dropped and when it is
given to every event.

**Every optional row shares one shape**, `x !== '' && <Row label='...'>{x}</Row>`,
repeated about a dozen times, and the harness baseline leaves them all empty.
Nothing about that shape stops a row being wired to its neighbor's field, and
no test would have caught it, so one test populates the whole set and reads the
`<dl>` back as label/value pairs rather than asserting the values are somewhere
on the page.

**The props version is 2**, and the webapp reads 1 as well, because posts
stamped before the array exist and still render. A bundle older than the bump
meets a version it does not know and falls back to the post's own text, which is
what the field is for.

### A bare event, with no fence

An event needs no fence around it. `decorators.SoleElementSpan` finds the span
from the first opening tag to the last closing one, so siblings come back
together, and the fence is still tried first so a labeled fence keeps its
stricter reading.

**It is tested against the CODE ranges, not the whole protected set**, and that
distinction is the entire subtlety. `findProtectedRanges` also protects inline
HTML, because a decorator link written into an attribute would corrupt it, and a
bare XML element IS inline HTML: testing against the full set refuses every
element this exists to find, which is exactly what the first version did.
Nothing here rewrites the message, so that half of the protection answers a
question this is not asking. What does carry over is the author's own statement
that a span is code, which is the fenced, indented and backtick ranges, and
`TestABareScanNeverReachesIntoCode` holds it.

The known limit: a message that closes one element and self-closes a later one
stops at the last closing tag and leaves the rest in `trail`. The alternative is
a second XML scanner in `decorators`, which is the thing that package refuses to
grow.

### Relations

`<link>` is read from `detail`. ATAK writes one on almost every event with a
`p-p` relation naming the device that produced it, usually carrying the sending
unit's callsign, which answers "who sent this". That is the one thing a relation
is good for without the other event in hand, so the card shows `parent` as
"Sent by" and the linked uids as "Relates to", and nothing tries to resolve a
link to another post.

**The `relation` value itself is parsed and then never read.** `addLinks` takes
`parent_callsign` and `uid` off every link whatever its relation says, so a
`p-c` link contributes exactly what a `p-p` one does. That is deliberate, since
the vocabulary is only meaningful with the other event in hand, but it has to be
SAID wherever two relations are shown side by side, which is why `cot.html`
carries a line stating that the card does not tell them apart: two examples
differing in one attribute imply that it does.

**`maxCotLinks` drops the extras and keeps the event**, which is the opposite of
`maxCotEvents` above. The asymmetry is the cost of being wrong in each case: an
event lost from a batch leaves a card quietly claiming a set it does not hold,
while a relation lost past the sixteenth costs one entry in a "Relates to" row
that already told the reader who sent it. `TestTheLinkCapDropsRatherThanRefuses`
pins the difference, because an examples row states it to readers.

### The marker is a crosshair, in the affiliation's color

**The shape** is a circle with a line across it and a line down it. A filled dot
reads as "somewhere around here"; a crosshair reads as "this point", which is
what an event is claiming.

Two things were tried and removed on the way, both recorded so they are not
tried again. Ticks radiating from the center instead of full lines made the
marker busy at 16px. A white disc filling the circle, meant to lift it off the
basemap, made it heavier rather than clearer.

`crosshairImage` builds it as a raw RGBA buffer rather than through a canvas or
an SVG, for two reasons: it is a pure function of its arguments, so it is tested
against its pixels rather than against a screenshot, and it needs no glyph, where
a Unicode crosshair would depend on font ranges this bundle trims for an
air-gapped install and could simply be absent.

**It is drawn from signed distances, not from a yes-or-no test.** Asking whether
a pixel is inside the shape gives every curve a stepped edge, which at this size
reads as grain rather than as a line. A distance gives each pixel the fraction of
it the shape covers, which is antialiasing, and `is antialiased rather than
stepped` is the test that keeps it. The bitmap is oversampled at four device
pixels per CSS pixel for the same reason; the cost is a few kilobytes.

The shape is stroked twice, the edge color first and wider. That outline is what
the palette's own note relies on ("the pin is distinguished by its outline against
land rather than by its fill"), and it is what lets the fill carry something else.
The wider pass has to grow the lines along their LENGTH as well as across them,
or each line ends in a bare fill pixel meeting the map, which is the one place
the outline has to be.

**The color** is the affiliation's, from the same `affiliationColor` the dot
beside the callsign reads, so the two cannot disagree about what a track is.

The caveat: on the map, color is the ONLY thing carrying affiliation, and the
hues are close in luminance, so hostile red and neutral green are one mark to a
reader with a common color vision deficiency. `markerLabel` is the other channel
and is why `markerColor` should never be passed without it. It is not a full
answer. MIL-STD-2525 solves this with SHAPE, a diamond for hostile and a
rectangle for friendly, and that is the honest fix if this proves to matter;
`toMarker` is the place it would go.

Only a surface that passes `markerColor` gets any of this. Every location surface
passes none and keeps the circle it has always drawn, which `the marker` in
`style.spec.ts` pins from both sides.

### The card carries its own hover

`registerLinkTooltipComponent` only ever offers a link Mattermost's own markdown
renderer drew. A plugin that owns a post body draws its own anchors, so nothing
offers them and a reader pointing at one gets no card at all. `mapping.md`
recorded that as expected but unverified when the inline map shipped; it is
verified now.

The inline map could shrug that off, because its hover is a map already on screen
underneath. This card cannot: a linked time's hover is the live countdown, and
the countdown is deliberately kept out of the channel, so without a hover there
is nowhere to see it short of opening the panel.

So `decorators/HoverLink.tsx` decides WHEN to show a card and nothing else. The
chrome and the routing stay in `Tooltip.tsx`, which now exports
`DecoratorHoverCard` for both callers, so a decorator still declares its hover
exactly once and neither path can drift from the other. It shows on focus as
well as on pointer, and the card takes no pointer events, which is what stops it
flickering itself in and out as the reader moves toward it.

### What the card will not claim

- **The card shows what the event says, and nothing derived from it.** The
  country was on the card once, worked out by running the position through the
  location package's polygon lookup. It came out because a Cursor on Target event
  states no country: putting one in a row beside `ce` and `hae` made a
  determination this plugin had made look like something the event had reported,
  which is the one thing every other rule here exists to prevent.

  `ce`, `le` and `hae` stay, and the difference is the whole point: they are
  attributes of the `<point>` element and are the event's own words.
  `TestNoCountryIsDerivedForAnEvent` holds both halves.

  The position link is the deliberate exception and is not a claim: it hands the
  reader's own coordinate tools a value the event stated, and everything derived
  from it is rendered over there, under that surface's own citation.

- **`9999999.0` is CoT for "not known"**, and real emitters write it as
  `9999999`, `9999999.0` and `9999999.00`. The test is on the parsed float,
  `>= 9999999`, or negative for `ce` and `le`. The row says "not stated"; it
  never shows the sentinel as a figure.
- **`0,0` is shown and not pinned.** It parses and is in range, and it is also
  overwhelmingly what an unset CoT point contains. Saying "position unknown"
  about a value we are holding would be asserting an ignorance we do not have;
  drawing the pin would put a marker in the Gulf of Guinea. Stating both facts is
  the only honest answer. The location package already refuses `0,0` for the same
  reason and in the same words, so this falls out of `location.Parse` rather than
  being re-implemented.
- **A position must look like a decimal.** `strconv.ParseFloat` accepts
  hexadecimal floats and exponent notation where JavaScript's `Number` does not,
  so `lat="0x1p+3"` validated in Go, was stored verbatim, was shown as a
  position with a working link, and was then read as `NaN` by the map, leaving
  the card and the picture disagreeing about whether there was a position at
  all. A strict decimal shape is checked before the value is carried.
- **Position digits are the event's own.** The `(f, v)` pair is built from the
  literal `lat` and `lon` attribute strings and validated against the `dd`
  grammar. `CLAUDE.md` requires rendering at the resolution the source carried,
  and a padded figure is a claim the event never made. Note that `dd` requires
  four fractional digits, so an event written more coarsely carries no link at
  all and the card shows the reading as text. That is the correct outcome and not
  a gap.
- **A type is decoded as deeply as it is known, and no deeper.** An atom's code
  path below the affiliation is looked up whole rather than letter by letter,
  because the letters do not compose into English on their own: `G-U-C` is
  Ground, Unit, Combat, and no per-letter gloss assembles into a name for it.
  The longest matching path wins.

  `atomPaths` is embedded from `server/cot/data/types.csv`, a plain data file
  this repository owns and maintains by hand; see the README beside it. It holds
  about a thousand paths and replaced a hand-written table of about thirty, which
  had three labels wrong and three entries that did not exist.

  **The affiliation is one character and the table is keyed below it**, so a
  single row answers for all eleven affiliations and the affiliation word is put
  in front at render time. **Case is part of the code**: `G-I-r` is a road and
  `G-I-R` is raw material production, and `TestTypeCodesAreCaseSensitive` pins
  that a lookup never folds them together.

  Labels are Title Case with acronyms and system names left in capitals (CSAR,
  ECM, SAM, RPV/UAV, SOF, VSTOL, HAWK, VULCAN), because the card renders
  `<affiliation> <label>` and both halves are read as one phrase. The
  hand-written `wholeTypes`, `howSources` and `affiliations` tables are cased the
  same way, since all four reach the same row; `TestLabelsAreCasedAlike` holds
  them together.

  Where the bare name of a code letter would be meaningless in that position, the
  label is composed instead: `a-f-G-U-C` reads "Friendly Ground Combat Unit"
  rather than "Friendly Combat", which is what the leaf letter alone means.

  **No two labels in the table are the same**, which is what
  `TestNoTwoTypesReadAlike` holds. A label is the whole of what the card says an
  event IS, so two paths sharing one is two things a reader cannot tell apart.
  Naming each leaf by its own code letter collided twice over: in the air branch
  the civil and military halves both read "Fixed Wing" and eleven fixed against
  rotary pairs matched each other, so a tanker was a tanker whether it was a
  KC-135 or a helicopter; and elsewhere thirty-six labels repeated, "Fire" five
  times across incident states and "Station" across a radio mast, a TV mast, a
  surface picket and a submarine. Every label now carries whatever distinguishes
  it, and `TestTheHandWrittenTypesDoNotRepeatTheTable` keeps `wholeTypes` from
  colliding with it from the other side.

  A path the table does not hold decodes to its longest known ancestor:
  `a-f-G-U-C-I-XYZ` keeps the infantry label and the trailing code is not guessed
  at. That is only honest because **the raw type is on the card in parentheses
  beside the label**, so a reader can always see what the label was derived from
  and what it did not cover. A whole type outside the atom tree still comes from
  the short hand-written table of well-known ones, and anything else renders the
  raw string and says it is unrecognized.

  The table has gaps: twenty-one paths have no row for their immediate parent.
  `S-C-A-L-A` (Assault Vessel) is one, with `S-C-A` present and `S-C-A-L` absent.
  That costs nothing, because `longestAtomPath` keeps the deepest match rather
  than stopping at the first miss, and
  `TestADeeperPathResolvesThroughAMissingParent` pins it.

### Time without a trusted clock

Clock drift of tens of minutes is routine on field kit and air-gapped VMs, and
two readers looking at one post must not disagree about whether to act on it.

- `time`, `start` and `stale` render as absolute Zulu DTG strings, through
  `dtg.FormatZulu`, so there is one DTG rendering in the repository.
- The validity window is `stale - time`, both from the event, so the figure is
  identical everywhere. Computed in Go.
- **Nothing ticks.** `dtg/Countdown.tsx` runs a one-second interval and a pulse
  and deliberately ignores `prefers-reduced-motion`, which was argued for a
  single RHS panel. Thirty position reports in a channel would be sixty timers
  and a wall of pulsing red, and, worse, a ticking clock on a one-time paste
  reads as a live feed. Nothing on the card ticks, which is the guarantee that
  matters; a line of prose saying so was tried and removed as clutter, since it
  restated what the absence of movement already tells a reader. The live
  countdown lives in the sidebar panel.

**The age reading is gone.** The card carried a Freshness row of
`stale_at - post.create_at`, computed in the webapp because `post.CreateAt` is 0
inside `MessageWillBePosted`. It was removed as a third way of saying what the
Stale row already says: that row carries the instant and the validity window
beside it, and a reader comparing two events compares those. `staleAfterPosting`
went with it.

**The caveat about the reader's clock is gone too.** The panel's countdown
carried a line saying it was counted against this device's clock, unlike every
other reading in the feature. It was true and it was noise: it asked the reader
to discount the one number the panel puts in the largest type on the page, every
time they opened it, for a drift most installs do not have. The fact it stated
is still true, and is now recorded here rather than in front of the reader.

**A stale event says `Stale`, and does not count.** `StaleCountdown` holds one
`setTimeout` for the stale instant rather than reading the clock each render, so
a panel left open crosses the boundary on its own. Past it, the countdown is
replaced by the standing word: a clock counting *up* from an expiry reads as a
live track, which is the opposite of what it means. One timer, not a ticker, so
the reason the card carries no clock is not quietly reintroduced.

**The heading goes with the countdown.** "Goes stale" is what a counting number
NEEDS, since a number alone says nothing about what it is counting to. Over the
standing word it was a future-tense label with its own value repeated
underneath, which is how it first shipped and read as a mistake. So the stale
state is the word by itself, in the urgent styling the countdown already uses,
and the instant it passed is still in the `Stale` row below.

**Staleness is drawn above the readings.** It decides whether the rest of the
panel is worth reading at all, so it is read first. It used to sit between the
readings and the remarks, which is where a reader finds it last.

### The CE circle

`LocationMap` gained one optional prop, `accuracyMeters`. Every existing caller
passes nothing and is unaffected.

A dot cannot be allowed to look the same at CE 3 m and CE 9 km: accuracy is the
channel through which this feature's worst failure mode arrives. MapLibre's own
circle layer takes a radius in **pixels**, so the drawn accuracy would change
with every zoom; a polygon is the only shape that keeps its meters.

**The vertices are geodesic.** A meter is a different number of longitude degrees
at every latitude, so the offsets are `dLat = m / DEGREE_METERS` and
`dLon = m / (DEGREE_METERS * cos lat)`, reusing the `DEGREE_METERS` already
pinned to Go. An equal-degree ring is right at the equator and increasingly wrong
toward the poles, which for a layer whose whole purpose is to stop the map
overstating a fix is the wrong way to be wrong.

A meter is drawn on the ground, but the accuracy is **read** from the string the
card already shows. `LocationMap` takes `accuracyLabel` rather than formatting
the number a second time: rounding meters again turned a stated `0.4` into
"within 0 meters", a claim of a perfect fix made only to a screen reader, while
the visible row beside it said `0.4 m`.

The ring is closed by copying its first position rather than by computing the
last one: `sin(2 pi)` is about 1e-16 rather than 0, so the computed form leaves a
ring GeoJSON considers unclosed. An accuracy that is not finite, not positive, or
sits where `cos(lat)` vanishes draws nothing rather than a ring of infinite
width.

An event whose `ce` is unknown draws the pin with no circle. A default radius
would be one this plugin invented.

### Switches

Two, and deliberately not three. `EnableCot` governs stamping only, matching the
format-switch invariant; a post already stamped keeps rendering. `EnableCotFile`
governs the only filestore read this plugin puts on the post path.

**There is no `EnableCotMap`.** `EnableLocationMapInline` already means "the map
under a post", and its parent ANDs with `EnableLocation` and `EnableLocationMap`
already live in Go beside `locationFormats`. A second switch would be a second
implementation of "is this on", which `features/types.ts` argues against by name.
Both surfaces read the same `features.mapInline`.
`TestCotHasNoMapSettingOfItsOwn` is where that decision gets revisited rather
than silently reversed.

The **reader's** switch is where the two surfaces part, and `CotMap`'s `surface`
prop is the whole of it. In the channel this is the map under a post and follows
the same `INLINE_ID` a coordinate-only post does, so a reader who hid one has hid
both. In the sidebar it is not a map under a post at all, and it follows the
Cursor on Target panel's own `map` section. It followed `INLINE_ID` there too for
a while, and the result read as a bug rather than as a setting: hiding the map
under coordinate posts silently blanked the sidebar map for an unrelated feature,
under a tickbox whose label said "Drawn in the channel". Both gates are still
read in the OUTER component, so the inner one never mounts either way.

### Customize your view

The panel carries the third preferences section, `cot`, holding the groups a
reader has **hidden**. Stored that way rather than as the ones they kept, for the
reasons `docs/design/preferences.md` argues once and this section inherits
whole: an empty list means "all of them", so a reader who never chose is stored
as nothing at all and "Restore defaults" stays a delete, and a section added in a
later version appears for everybody rather than being invisible to exactly the
readers who cared enough to choose.

**Groups rather than rows**, which is the one place this editor differs from the
location one. This is the longest panel in the plugin and its volume comes from
whole headings: one event can draw a map, a countdown, a fourteen-row grid,
remarks and six more headings before the source disclosure. A tickbox per reading
would have traded one long panel for a longer editor. Eleven ids, in the order
`EventSection` draws them: `map`, `stale`, `event`, `remarks`, `device`,
`precision`, `orientation`, `payload`, `shape`, `flow`, `source`.

`cot.Sections` in `server/cot/sections.go` is the catalog and
`webapp/src/cot/sections.ts` is the other half, held to the same ids in the same
order by `TestWebappCotSectionCatalogMatches`. The order is the panel's render
order and the editor's list order at once, so reordering for one and not the
other is not something the code can express. Ids reach the KV store: add and
retire freely, rename never. The server refuses an id it does not know
(`TF-14009`); both sides ignore one on the way in, so retiring a section cannot
lock a reader out of the settings around it.

**Four things are not sections and cannot be switched off**: the callsign
heading and type subhead, which name what the panel is about; the position note;
the `Source file` line; and the `Unrecognized` and `Dropped` notices. Those last
are admissions about what this build could not read, and a card that will not
name a country it did not receive does not offer a tickbox that hides its own
refusals either. The cost is that the notices' pointer to "As posted" dangles for
a reader who hid `source`, which is the right way round: the sentence still says
the event is unchanged, and the way to see it is one tickbox away.

**The editor resets on the payload object, never on a key derived from it.**
It was `count:firstUid`, and two position reports from one device are two posts
with one event each and the same uid, so clicking the second left the reader in
the editor: exactly what the reset exists to prevent. `setSelection` stores one
object per click, so the payload's identity changes once per selection and never
on a re-render, which is the property a derived key was trying and failing to
approximate. The test that covered this used distinct uids and passed throughout.

Hiding everything is allowed and leaves the callsign and the two footer links,
which is what makes it recoverable: the way back is the Customize link itself.
The footer is new with the editor; the panel had no Documentation link before.

**The card is untouched.** A reader's sections apply to the sidebar only. The
card is already a summary, `ClassSummary` is capped at 90 runes, and the two
surfaces do not share a section list, so several ids would have applied to one
and not the other.

### The props are frozen, and that is a decision

Every displayed string is rendered in Go at post time. A corrected type table or
an updated country polygon never reaches posts already stamped, so a card can
come to disagree with the location panel its own position link opens.

Two mitigations, both deliberate. The raw values (`cot_type`, `lat`, `lon`,
`ce_meters`) are stored beside the rendered ones, so a later version can
re-derive rather than being stuck with the prose. And `version` is the migration
lever: a card built from a version this bundle does not know falls back rather
than misreading.

### Telling an author their event was refused

An `xml` fence is ambiguous and silence is right for it. A `cot` fence is an
unambiguous statement of intent, and an author who gets nothing back cannot tell
malformed XML from a disabled switch from a covering note, and will retry by
pasting the event again.

So a refusal the author can act on sends an ephemeral naming the reason and its
`TF-NNNN`. Not for `EnableCot` off, which is not theirs to fix. **The text is
self-contained**, because an ephemeral sent from this hook carries no `PostId`
and cannot point at anything: it names the event they just tried to post rather
than saying "this post".

### The example is a hostile contact

Both commands demonstrate one event, and it is a target rather than a position
report. A unit's own position goes over the network to the people who need it;
a contact is the thing somebody pastes into a channel to show other people and
argue about, so it is the card a reader should meet first. It also puts the
crosshair on screen in the color that carries the most meaning.

It carries **no `__group`**. That element is the sender's team, and ATAK puts it
on a self position report rather than on a placed marker. On a hostile contact
the card would render "Team: Cyan" against the target, which reads as the target
being on that team. `TestTheExampleIsAHostileContact` pins both the affiliation
and the absence.

`how` is `h-e`, estimated, and the accuracy is tens of meters rather than the
sub-meter of a GPS fix, because a contact somebody plotted by eye is not known
to the precision of a device reporting itself. An example whose numbers claim
otherwise teaches the wrong thing about what `ce` means.

**The row in `examples` quotes the card's own label rather than restating it.**
That row exists to show that `a-h-G-U-C-A` becomes readable English, so prose
beside it naming a different reading is the one failure it cannot afford.
`cotExampleTypeLabel` parses the example and reads `type_label` out of the props
the card itself renders from, and `TestTheExampleRowQuotesTheCardsOwnLabel` holds
the two together. Changing the type, or its row in `types.csv`, moves the prose
with it.

### The examples the command posts, and why there are two

`examples` posts one message per decorator and then the Cursor on Target events,
which are not a decorator and never belonged in that machinery. An example set is
run through the tagger and asserted to decorate or not, which is a question about
tokens rather than about post types, and running an event through the tagger
would find the RFC 3339 timestamps inside it and hang a date-time link off it,
which says something false about how an event is read. So the events are
appended by `postCotExamples` rather than being a set.

**Two of them, and one each per post.** A card owns the whole post body, so an
event sharing a post with anything else renders that as plain text underneath it.
That settles the count as one post per event rather than one post with two.

They are deliberately different shapes. The first is a single hostile contact:
one event, a callsign, a position and remarks, which is the smallest thing that
still produces a card worth looking at, and it is what a reader meets the feature
through. The second carries three events and most of what a `<detail>` block can
say, so a reader sees the block layout, the affiliation colors on one map and the
extension groups in the sidebar all at once.

The rich one also carries a **drawn area**: a closed six-vertex polyline under a
`u-d-f` event, outlining a suspected hostile area with the contacts inside it.
That is what a shape is for in practice, and it is deliberately irregular rather
than a rectangle, because a rectangle is the one outline a reader can mistake for
a bounding box the plugin drew itself.

Drawing it needed a change. `CotMap` drew geometry only for a single-event card,
so the polygon would have appeared in the panel's Shape section and on no map,
which is the panel and the map disagreeing in front of the reader. **Exactly one
outline in a block is now drawn.** The argument against N shapes still holds, and
is why it is one rather than all: a pile of outlines says nothing about which
belongs to which track.

**Outlines only, and that is not laziness.** An outline carries absolute
vertices, so it lands where the event put it whatever else is on the map. An
ellipse is drawn around the map's PRIMARY position, which in a block is the first
event's, so a circle belonging to the third would be drawn around the first one's
marker. `soleOutline` refuses ellipses in a block for that reason, and
`label.spec.ts` pins it.

The accuracy ring is unchanged and stays single-event: a ring per track is a map
of overlapping blobs, and unlike a drawn area a ring qualifies one specific
position.

**The third example is an attachment.** It posts a MEDEVAC request as a
`medevac.xml` file rather than a fence, because the file path is a whole second
way in and nothing else demonstrates it: `plugin.API.UploadFile` credits the file
to `model.UploadNoUserID`, which is exactly the `cotFileCreator` branch that
exists for a companion plugin's uploads, so the example exercises it end to end.
It carries an XML declaration to show that one is accepted. With
`EnableCotFile` off the example is **skipped rather than posted as a fence**,
since the attachment is the whole point of it.

**The example is `.xml`, and `cotFileSuffixes` lists `.xml` first**, because that
is the extension a reader is likelier to have on hand: ATAK exports and every
other tool that emits this XML write `.xml` far more often than `.cot`, and an
example named for the rarer one reads as though the rarer one were required.
Both are accepted and neither is preferred by the code, which tries the suffixes
in order and stops at the first match, so the order is documentation rather than
behavior. The prose in `plugin.json`, the help pages and the example's own lead
all lead with `.xml` for the same reason and carry no more weight than that.

The rich one is held to that by `TestTheRichExampleFillsInMostOfTheDetailBlock`,
which names the props it has to write and, more importantly, asserts it does
**not** degrade: an event carrying that much detail is exactly the shape that
runs past the props budget, and a degraded card would silently demonstrate less
than the example claims.

Everything else that used to be posted, and there was a lot of it, is in
`cot.html`. See "The slash command" in [`decorators.md`](decorators.md) for why.

**The examples are real fenced blocks, and two invariants make that safe.** A
block labeled `cot` or `xml` is exactly what `cotSource` recognizes, so a post
made of them is a post describing the thing it is made of. Both ways that can go
wrong are closed by construction rather than by hoping the packing is kind.

*A fence-bearing chunk is one packing atom.* `packChunks` writes `chunk.lines[i]`
verbatim and only ever measures its rune count, so an atom holding a whole
chunk's worth of blocks (its notes, its fences and the XML between them) cannot
be split across two posts. The earlier shape wrote one line per row, which left a
fence able to land with its opener in one message and its closer in the next.

*Every fence-bearing atom carries at least two blocks.* `SoleFencedBlock` needs
there to be exactly **one** fence in the message, so two or more make it answer
no; and the bare-event fallback `SoleElementSpan` cannot see the events either,
because a fenced block is a code range. A message holding no fence at all is
equally safe. The one forbidden state is a message holding exactly one, which is
why the count is asserted per atom rather than per message: an atom is what a
message is assembled from, so holding the atoms to it holds every packing to it.

`TestNoCotChunkCarriesExactlyOneFence` is the second invariant, and
`TestEveryCotAtomFitsTheFloorBudget` is what keeps the first one affordable. An
atom is indivisible, so one larger than `safePostRunes - headingBudget` would
leave `postDetails` with nothing smaller to retry: the floor is the bottom rung
of that ladder, not a target. That is the real cost of fencing, and it is why the
examples are grouped two or three to an atom rather than all of one heading's.

**A pretty-printed event is only ever recognized behind a fence, and the rows say
so.** `isIndentedCode` treats any line indented four spaces or a tab as a code
range, and `SoleElementSpan` refuses to look inside one. Nested XML reaches four
spaces at its second level, and a hanging attribute indent reaches seven on the
first, so *no* readably indented event survives a bare paste. Only the flat,
one-line form does, which is also how an event arrives over the wire.

That is why "Posting one" leads with the flat event and then says to fence the
indented one, rather than showing the same event three ways and implying all
three are interchangeable. `TestTheFlatExampleIsRecognizedWithoutAFence` asserts
both halves, the flat one being found and the indented one not, because the rows
teach the difference between them and a row is worth nothing if only its
agreeable half is checked. The same correction is in `public/help/formats.html`,
whose "a fence is optional" bullet had claimed it of the indented example printed
directly above it.

**The examples are whole events, not `detail` fragments.** A fragment had to be
read alongside a note saying where to put it; a fenced event can be copied out of
the post and pasted straight back into the channel, which is the only way a
reader confirms that what they are looking at works. It also lets the section
cover the whole `<detail>` registry rather than the four elements that fitted on
one line each.

**The test that guards this asks `cotSource`, not the text.** It used to check
that no unstamped post contained the substring `<event`, which was true and
sufficient while the only event in the output was the live card's own post. It is
now false by construction: the details posts are full of events, in fences where
nothing reads them. `TestTheEventExampleIsAPostOfItsOwn` therefore runs every
created post through the recognition function itself, and the budget sweep in
`TestDetailsPostWhateverTheServerAccepts` asks the same question again at every
budget the retry ladder can choose, which is where a packing bug would appear
rather than at the default one.

The flat event is written out rather than sliced from the pretty one, because
collapsing the pretty source would have to know which of its whitespace sits
inside an attribute value. `TestTheFlatExampleIsTheCardsOwnEvent` holds the two
to the same uid, type, time and point, which is the same shape of guard
`TestTheFileExampleIsTheCardsOwnEvent` already applies to `cotDetailFile`. The
compact row in `examples` is sliced from the flat one, as it always was.

The block that demonstrates an unfenced event is itself fenced, labeled `text`.
It has to be a fence of some kind for the post to survive itself, and a label
`cotInfoString` refuses is what keeps the row from being read as the event it is
printing.

`longestDetailAtom` in the split test reads `detailMessageSets` rather than
`detailSetOrder`, so it covers this set as well. Listing the decorators there
measured a row of text while the packer was being handed a fenced block twenty
times its size, and the budget assertion silently stopped meaning anything.

`TestTheDetailEventsAreWhatTheirRowsClaim` holds the examples to their notes: a
row promising several events parses to several, and the link row's link parses
and names a parent. A row teaching the wrong shape cannot be caught by reading
it. `TestEveryRegisteredExtensionIsDemonstrated` reads `cot.Extensions()` rather
than a list somebody maintains, so an entry added to the registry is an entry the
examples section is required to show.

### A backtick in `plugin.json` breaks both builds

Both generated manifests embed `plugin.json` verbatim inside a literal a backtick
terminates: `server/manifest.go` uses a Go raw string and
`webapp/src/manifest.ts` a JavaScript template literal. A markdown fence in help
text is exactly how this happens, since a fenced block is what an admin reading
about CoT wants described, and the failure lands hundreds of lines away in
generated files nobody edits and git does not track.

`TestPluginManifestCarriesNoBacktick` catches it at the source. Describe a fence
in words.

## An attachment has to be the poster's own

`cotFileSource` checks `model.IsValidId` on the id, then `CreatorId`,
`PostId == ""` and `DeleteAt == 0`, before it asks the filestore for anything.

`post.FileIds` arrives from the client, and this hook runs **before** Mattermost
binds files to posts, so at that moment nothing else has checked that the sender
may read the id they named. Without these checks, quoting somebody else's file
id copies up to `maxInlineSrcRunes` of that file's text into `props["src"]`, and
those props are readable by everyone who can read the attacker's post. The card
would then render another user's file as though the poster had attached it.

The id shape is checked first so a malformed id never reaches the store at all,
which `TestCotRefusesAFileIdThatIsNotOne` asserts by counting lookups rather
than by checking the answer.

Whether Mattermost's own attach-time validation would independently refuse such
a post is **not** what this rests on. That validation happens after this hook has
already read the file and committed the props, and `docs/design/unverified.md`
records that the whole `GetFile`-inside-`MessageWillBePosted` path is unverified
against a running server.

## The size limit has one home

`cotFileSource` measures against `cot.MaxSourceBytes`. It used to declare its own
`cotMaxFileBytes = 64 * 1024`, while `MaxSourceBytes`'s doc comment said the hook
checked against it, and the help text printed `MaxSourceBytes`. Three copies, two
of which disagreed with the code. Raising the parser's cap left the hook refusing
at the old value; lowering it made the hook fetch a file the parser would then
throw away, defeating the pre-fetch guard's whole purpose.

## A stripped post still reaches the decorators

`cotStamp` returns `(post, stamped bool)` rather than a bare pointer, and
`decoratePost` acts on the flag.

The two answers had been conflated. A **stamped** post is finished, because the
card owns the body and decoration must not rewrite the text underneath it. A
**stripped** one is an ordinary message again, and returning it as though it were
finished cost it its decoration: spoofing a `custom_tf_cot` type onto a post was
a way to turn decoration off for it, silently.

The subtlety that made the first attempt at this wrong: `decorateMessage`
answers `nil` for "nothing changed". Handing that back when a strip had happened
would have meant "no change" to the hook and put the forged type straight back
on, so `decoratePost` returns the stripped clone itself when decoration finds
nothing. `TestAStrippedPostStillReachesTheDecorators` covers the first half and
`TestAForgedPostTypeIsStripped` the second.

## The map's label for a block names the affiliations

A block used to be announced as *"World map with the position marked. The marker
is 3 events."* Three things were wrong and one of them mattered.

**The one that mattered: no affiliation.** Color is the whole of what tells one
marker from another on a multi-event map, so a reader who gets no color got a
count and nothing else. `cot.md` already argues that color is never the only
channel, and points at `markerLabel` as the other one. That held for a single
event, whose label is its type and therefore begins with its affiliation word.
It failed for a block, which is the only case where markers must be told apart
at all, and where the single-event argument does not apply because one marker
needs no distinguishing.

`blockLabel` now tallies them: *"World map with 4 positions marked. The markers
are 2 friendly, 1 hostile and 1 unknown."*

**Counted in events, not markers.** An event with no usable position draws
nothing, and `CotCard`'s heading counts every event, so a map announcing its
marker count contradicted the heading two lines above it. The ratio is stated
only when the two differ (*"4 of 5 events: ..."*), since otherwise the opening
clause has already given the number and repeating it says nothing.

**The grammar follows the count.** `label` and `markerClause` take how many
markers there are and pick between "the position marked" / "N positions marked"
and "The marker is" / "The markers are". The old text was singular for any
number of them.

`AFFILIATION_WORDS` sits beside `AFFILIATION_COLORS` in `webapp/src/cot/types.ts`
and is keyed on the same ids, with `every affiliation with a color has a word`
holding the two together. An affiliation that gained a color and no word would
be a marker a screen reader cannot tell from any other, on the one surface where
telling them apart is the entire job. An affiliation this build does not know is
called `unstated` rather than dropped, which matches the map drawing it in
`UNCOLORED` rather than leaving it off.

## The map's filter is `isLinkable`, and it lives in `CotMap`

`CotCard` hands `CotMap` every event and `CotMap` decides which it can draw.
That used to be the other way round: the card filtered with `isLinkable` first,
and the map filtered again with a finiteness test on the numbers. Both halves
were wrong.

**The ratio clause could never fire.** `blockLabel` compares what it drew
against the total it was given, and the total it was given had already been
filtered, so the two were equal by construction and the "4 of 5 events" clause
was dead code. Meanwhile the card's own heading counted every event, so the
contradiction the ratio exists to resolve was still on screen: heading "5
events", label "3 positions marked", no explanation.

**The finiteness test admitted a positionless event.** An event the server gave
no position has `lat` of `''`, and `Number('') === 0`, which is finite. The
filter passed it and the map pinned it in the Gulf of Guinea, which is the
guessed position `LocationMap` says it never draws. It never happened only
because the caller's filter masked it, and that mask was the same bug as above.
Fixing either one alone would have shipped the other.

`isLinkable` is the honest predicate: the server writes `format` and `value`
only when it parsed a position it will stand behind, so that pair IS the
question. `isRenderable` is on top of it because the server accepts latitudes to
90 and Web Mercator stops near 85.05, and the FIRST marker decides where the
whole map opens: a block whose first event was past the limit replaced the
entire map with "too far north" and never drew the others.

`only` is derived from both counts (`events.length === 1 && drawn.length === 1`).
A post carrying three events of which one can be drawn is not a single-event
card, and treating it as one put that event's accuracy ring and its "Open
larger" link on a map the other two were missing from.

`_blockLabelForTesting` exists because the browser test hard-codes the string
`blockLabel` must produce. That pinned the consumer and left the producer free,
which is exactly how a clause that could never fire shipped with the suite green.

## An integration's attachment is read

`cotFileCreator` accepts `model.UploadNoUserID` as well as the poster's own id.

`plugin.API.UploadFile(data, channelId, filename)` takes **no user id**, so the
server credits a plugin's upload to `"nouser"`, which can never equal any post's
`UserId`. The ownership check therefore refused every attachment a companion
plugin posted, silently and forever, which for a Cursor on Target plugin is the
likeliest producer of `.cot` files there is.

It does not reopen what the check closed. A `"nouser"` file can only be created
by a plugin or by local mode on the host: a remote client cannot make one, and
cannot learn the id of one while it is unattached.

The refusal has its own code, `TF-11006`, rather than reusing
`HooksCotFileUnreadable`. The two say opposite things to an operator: that one
means the filestore did not answer, this one means it answered and the answer
was somebody else's file. Folding them together filed the only line that would
ever betray an attempted disclosure under a code whose published guidance is to
check that storage is reachable.

**`fakeAPI` now records the code from every warning.** It discarded the key and
value pairs, so no test could tell one refusal from another and the call site
kept an inherited code after the reason for it had changed. The ownership test
asserts the new code is present and the old one is not.

## The `<detail>` extension registry

`<detail>` has no schema. It is an open container into which ATAK,
FreeTAKServer, TAK Server and third-party plugins each write their own
conventions, so there is nothing to validate against and the only honest shape
is a registry of what this build knows, with everything else falling through to
the source pane.

`extensions.go` declares that registry. The parser reads it to decide which
children to collect, `eventProps` reads it to decide what to write, and
`cot_sync_test.go` reads it to build the fixture that holds the webapp to it.
Adding an extension is one row.

### The props key space is closed, and that is a security property

Every key an extension writes is `<prefix>_<key>` with both halves coming from
the registry. No author string is ever a props key.

That is not tidiness. `format` and `value` sit in the same map, and
`isLinkable` builds `/decorate/location?f=&v=` from them verbatim; `affiliation`
sits there too and keys the marker color, which `cot.md` already refuses to let
an author choose. An author-chosen key beside those three is the whole attack,
and `_flow-tags_`, whose attribute NAMES are the data, is the one element that
would have supplied it.

So `_flow-tags_` is not a registry entry at all. It is read by one function into
an ordered array under `flow`, where a name is a value and can collide with
nothing. `TestEventPropsKeysAreClosed` feeds it attribute names spelled
`format`, `value`, `affiliation`, `lat` and `uid`, and asserts the event's
top-level key set is still exactly what the registry can produce.

### No author string is ever a URL, an `src`, a `style` value or a `className`

This is the general rule the `color` and `usericon` entries rest on, and it is
worth stating once rather than re-deriving per element.

`color/@argb` is the first author-controlled value in this repository that
reaches a style property. React sets style values through `setProperty` without
sanitising, so `background: url(https://attacker/px)` in a swatch would be an
outbound request from every reader who opens the panel: a read receipt on a
tactical channel.

ATAK writes `argb` as a **signed 32-bit decimal** (`argb="-1"`), not as hex. Go
parses it, drops the alpha byte and writes a validated `#RRGGBB` or writes
nothing. The webapp re-validates against `/^#[0-9a-f]{6}$/i` before the value
reaches a style property, because a props blob is not a trusted input either:
the type is forgeable and the props under a plugin's key are not protected.

The alpha byte is dropped rather than applied. An `argb` of `#00FF0000` is a
fully transparent swatch, which is a row that says nothing, and the hex is
rendered as text beside the swatch so color is never the only channel. The
swatch is `aria-hidden` like the affiliation dot and carries a themed 1px
border, without which `#FFFFFF` on a light theme is an invisible square.

`usericon/@iconsetpath` is text for the same reason and is one refactor away
from being an `<img src>`, which is why the rule is written down rather than
left to each call site.

### The stated color draws the shape, and never the marker

The panel used to print a color the map then ignored, which is the panel and the
map disagreeing in front of the reader about the same event. `LocationMap` takes
an optional `geometryColor`, and `CotMap` passes the event's stated color when
there is exactly one event, which is already the condition the shape and the
accuracy ring are drawn under. Absent, the shape keeps the theme's own cell
color, which is what every location surface gets.

The **marker** still does not take it, and that is the line. A marker's color is
this plugin's claim about what a track IS: red is hostile, blue is friend, green
is neutral, amber is unknown. An author who states red for a drawn boundary has
said nothing about affiliation, and letting that reach the marker would put a
hostile-colored reticle on a friendly track. A shape has no affiliation to
contradict, so there is nothing for a stated color to overwrite.

The validation is unchanged and is now load-bearing twice: `statedColor` gates
the value before it reaches a style property, and `fillOf` in `LocationMap` gates
it again before it reaches MapLibre, falling back to the theme rather than
passing an unparsed string through. The fill is written as `rgba()` at the
theme's own alpha rather than as an eight-digit hex, which MapLibre's color
parser does not accept.

### Author URLs are not clickable, and what that does and does not buy

`__video/@url` and a `ConnectionEntry` address are author-controlled. They are
rendered as text, never as an anchor.

**What that buys:** no anchor is created. A plugin that owns a post body draws
its own markup and gets none of Mattermost's markdown renderer, so nothing
autolinks inside the card or the panel.

**What it does not buy** is that the URL is unreachable, and an earlier draft of
this note claimed it did by citing the page CSP from `decorators.md`. That CSP
governs `/decorate/*` and `/map`, which are this plugin's own routes. The card
and the panel render inside the Mattermost webapp, on Mattermost's origin, under
Mattermost's CSP, with this bundle already trusted. And in the fence case the
raw event is still in `post.Message`, which Mattermost's own markdown autolinks
on every fallback surface: mobile, search results, export, and this plugin's own
`Fallback` after an edit.

The limit is stated rather than papered over. `__video/@url` is kept off the
card entirely; the panel shows it as text under its own heading.

### Parent, namespace, and who wins a repeat

Three rules the parser did not have, each of which was a way for the card to
disagree with the pane a reader opens to check it.

**Parent.** `readChild` matched on `Name.Local` at any depth, so a `<contact>`
nested inside another extension became the event's callsign and a `<link>`
inside `<__video>` became a relation in the "Relates to" row. An extension is
now read only as a child of the `<detail>` element at depth 2, and a nested
child (`chatgrp`, `ConnectionEntry`) only under the block that owns it.

**The parent stack has to survive `readText`.** `readText` swallows the
`<remarks>` subtree and calls `counts.enter()` itself, so a stack maintained
only in `Parse`'s main loop desyncs the first time remarks contain markup, and
the symptom is a later sibling attributed to the wrong parent, which is the
exact bug the parent rule exists to fix. So the stack IS the depth: `enter`
takes the element name and pushes it, `leave` pops, and `depth` is the stack's
length.

**Namespace.** Matching on `Name.Local` alone read `<x:contact callsign="ADMIN"/>`
as the callsign. Elements and attributes are now required to be in no namespace.

**And the stack carries the namespace too**, which the first version of this
missed. Pushing `Name.Local` enforced the rule on the element being matched and
on none of its ancestors, so `<x:detail>` was indistinguishable from `<detail>`
to every parent test below it and everything inside one was read as the event's
own, while the pane a reader opens to check the card showed markup that was not
`<detail>` at all. `enter` pushes a qualified name, which can never equal a bare
registry element name.

**A nested child needs the element it is INSIDE to have been accepted**, and
that is per instance, not per name. Acceptance rides on the stack beside the
path, so a child asks about the element enclosing it rather than about a name.

The first version asked a flat set, which answered "some element with this name
was accepted somewhere earlier in this event". That let a legitimate
`<detail><__video/></detail>` vouch for a second `<__video>` parked outside
`<detail>` entirely, so a `ConnectionEntry` under the second one became a panel
row built from markup that was never in `<detail>`.

**A prefix bound to the empty URI is refused outright.** `xmlns:x=""` resolves
`<x:detail>` back to `Space == ""`, so the qualified push returns the bare name
and every namespace test in the file is undone at once. Go accepts that binding
although XML Namespaces forbids it, so `enter` refuses the source. The legal
default undeclaration `xmlns=""` arrives as `Space == ""`, `Local == "xmlns"`
and is untouched.

**A namespaced root is refused rather than stamped.** Every child of
`<event xmlns="urn:cot">` is skipped for carrying a namespace, while the
unprefixed `uid`, `type` and `time` attributes carry none and survive. The card
would therefore have told a reader the event stated no position when it stated
one, which is the claim this package exists not to make.

**First wins, for both a repeated element and a duplicate attribute.**
`encoding/xml` performs no duplicate-attribute check, verified against the
pinned SDK, and the two halves of the old code disagreed: `attrValue` was
first-wins while a repeated `<contact>` was last-wins because each arm assigned.
One rule now, and it is `attrValue`'s.

That rule is also what keeps the sync fixture honest. Because registry names are
a closed set and repeats add no keys, no cap can drop a key from a
registry-derived fixture, so there is no `maxCotBlocks` to pin and no way for the
cross-language guard to narrow as entries are added.

**"Repeats add no keys" was not true of nested children**, and that is the
second thing acceptance had to be about the instance rather than the name. A
repeated `<__chat>` is rejected by first-wins, but its `<chatgrp>` was still
read, so `chat_sender` from the first block rendered beside `chatgrp_uid0` taken
from the second. `TestARejectedRepeatDoesNotContributeItsChild` holds it.

**Attributes join the budget.** `budget.enter` counts `len(start.Attr)` as well
as the element. Attributes were free, and `_flow-tags_` is the first element
whose attribute count is author-chosen without bound.

### Classification decides layout, in two passes, and the type code is final

The card gains one summary line chosen by a class. Four of them, `chat`,
`medevac`, `sensor` and `video`, written into props only when they change
something, per the `putIfSet` precedent. Absent is the default and is today's
card.

**Pass 1 matches the type code and is final. Pass 2 may only promote an event
pass 1 left unclassified.**

A single ordered table of "type matches X **or** block present" was the obvious
design and is wrong. Under it, a hostile contact carrying an empty `<__chat/>`
classifies as chat, and its remarks are promoted into a message-shaped block:
ten bytes, chosen by the author, to re-shape somebody else's contact report. An
`a-*` atom is never re-classed by a `<detail>` child.

**Matching is case-sensitive**, because case is part of a CoT code everywhere
else in this package and a `classify` that folded it would disagree with the
label rendered beside it. Match kind (exact or prefix) is a field on the row
rather than a glob a reader has to interpret.

**Layout degrades to the default when the block the class names is absent.** A
`b-t-f` with no `__chat` element has no sender and no room, so the chat heading
would be empty chrome. Classification is type-driven; layout selection needs the
data to draw.

**Classes apply to single-event cards only.** The card already lists rather than
details a batch, and a post carrying a chat, a sensor and three position reports
has no one class. Each event keeps its own for the panel.

**A class is frozen in props like everything else here.** Two identical GeoChat
events posted either side of this change render differently in the same channel,
because the older one carries no class. That is the same trade the frozen-props
section above argues, and `cot_type` is stored, so a later version could
re-derive rather than being stuck with it.

### The chat card is a reading of an event, not a message

`__chat/@senderCallsign` is author-chosen text with no relationship to the
Mattermost identity that posted. Anything shaped like a quoted message from a
named sender borrows Mattermost's own attribution chrome, so an author who sets
`senderCallsign` to a colleague's name would hand a reader a message from that
person.

This is the same failure `cot.md` already argues for `uid` ("two `uid` values
that render identically is impersonation of another track") and for the example
carrying no `__group`, on a louder surface.

So the chat heading stays inside the card's own header treatment and is labeled
as what the event states. No blockquote, no avatar, no username styling. The
message text is rendered exactly once: the class suppresses the ordinary Remarks
row rather than drawing the same string twice.

### What the registry does and does not decode

`Unit` on an attribute is the whole formatting vocabulary. Empty means the
sanitised stated string; the rest parse the number and append the unit,
inheriting the `9999999.0` sentinel handling from `knownNumber`.

**A `Unit` that is appended IS its suffix, spacing included.** `unitMeters` was
`"m"` for most of this work while every renderer that used it hardcoded `" m"`
separately, so the constant and the string it stood for had quietly diverged.
Collapsing the renderers made them meet, and passing the constant where the
suffix was meant rendered `400m`. Two existing tests caught it, which is the
argument for the collapse rather than against it. `unitColor` and
`unitHashCount` select a decoder rather than name a suffix and are the exception.

**There are three numeric renderers, and the third is not redundant.**

| Renderer | Rule | Why |
|---|---|---|
| `signedText(raw, unit)` | any number the sentinel allows | pitch, roll, slope and sensor elevation are legitimately negative, and so is a radio signal in dBm |
| `positiveText(raw, unit)` | must be above zero | it describes an extent, and an ellipse axis of zero is no axis at all |
| `percentText(raw)` | non-negative, zero kept | a battery of `0` is a real reading |

`positiveText` and `percentText` differ only on zero, which is exactly the
distinction worth keeping visible: one field's zero is a measurement and the
other's is a malformed shape.

**`courseText` could not be reused for degrees.** It rejects anything below
zero, which is right for a course and wrong for `Attitude/@pitch` and `@roll`,
`track/@slope` and `sensor/@elevation`, all of which are legitimately negative.
Reusing it would have silently dropped every negative attitude.

These three replaced six near-identical functions that had accumulated one at a
time, the last of them named `numberText2`. They differed only in a string, and
`numberText2` was already the general form of two of the others. The collapse
changed no behavior, which is what the existing suite passing unchanged says.

**`Attitude/@yaw` renders as "Yaw", not "Heading".** Yaw is orientation about
the vertical axis; `track/@course` is the event's own word for direction of
travel. An event carrying both would otherwise show two rows both labeled
Heading and disagreeing.

**MEDEVAC counts are written even when they are zero**, and that fell out rather
than being built. The registry began with a `Zero` flag on an attribute, added
because a stated zero and an unstated field are different facts and a card
reading "1 priority" for an event that stated "0 urgent, 1 priority, 0 routine"
is wrong about casualties. Writing it showed the flag had nothing to do:
`putIfSet` and the block renderer both drop the empty string, not `"0"`, so a
stated zero already survived. The flag came out. `TestMedevacStatedZerosSurvive`
is what keeps that true, since it is now a property of a renderer rather than of
a field somebody can see.

**`_medevac_` attribute names are inconsistently cased in the wild.** Both
`Security` and `security` occur. Both spellings are separate registry rows
mapping to one key, which is data rather than a case-folding flag, and first-wins
resolves a collision with no new branch and no new rule.

**`_medevac_/@security` is shown as stated, not decoded.** The numeric 9-line
vocabulary (0 no enemy, 1 possible enemy, and so on) is plausible and this
plugin has no primary source for it that also states TAK writes it that way.
Shipping a decode that cannot be cited is the derived claim
`TestNoCountryIsDerivedForAnEvent` exists to prevent.

**An extension with no attributes is a presence row.** `archive` writes
`"stated"` under its own prefix. An empty block would contradict `putIfSet`, and
a row reading "Archive: true" is not a sentence a reader acts on.

### `_flow-tags_`

The one element whose attribute names are the data, and the reason it is handled
by hand rather than by a registry row.

- Stored as an **ordered array**, never a map. `json.Marshal` sorts map keys and
  the ordering IS the processing path.
- **`xmlns` declarations arrive as attributes.** `Name.Local` on
  `xmlns:x="urn:evil"` is `"x"`, verified against the pinned SDK, so an
  unfiltered reader would render a namespace URI as a hop's timestamp. Any
  attribute in or named `xmlns` is skipped.
- **`version` is excluded by name**, or the table gains a system called
  "version" whose timestamp is "1".
- **A name is dropped rather than truncated.** A truncated key is our word, not
  the event's, and two long names would truncate to two rows a reader cannot
  tell apart.
- **The cap drops from the front.** Flow tags are appended, so document order is
  oldest first, and dropping the tail would discard the most recent hops, which
  are the ones a reader is trying to see.
- **A timestamp that is not RFC 3339 keeps its hop** and is shown as stated.
  Omitting it would show a shorter route than the event described.

Rendered under "Processing path". The element name is not shown, and the times
go through `dtg.FormatZulu` like every other time in this repository. Rendering
them as a bare time of day was considered and refused: it is lossy across a day
boundary, and it would be a second DTG rendering.

### The source pane now covers everything the parser read

`maxInlineSrcRunes` was 8192 while `Parse` accepts `MaxSourceBytes`, 64 KiB, so
the pane a reader opens to check the card covered the first eighth of what the
card was derived from.

That was survivable while five elements were read from the top of a small event.
It is not survivable for a registry: an extension parsed from byte 20000 landed
in props with nothing in the pane to verify it against, which inverts the pane's
entire purpose. Worse, every claim in this note and in the plan that unknown
`<detail>` children "remain readable in As posted" was false past that boundary,
for exactly the extension-rich events the registry exists to read.

`maxInlineSrcRunes` is now `MaxSourceBytes`. The budget affords it:
`PostPropsMaxUserRunes` is 760,000 runes, against about 65,000 of `src`.

`detail_unknown` closes the other half. Once the panel enumerates blocks, an
event with none reads as "this event carried nothing" rather than "this build
did not recognize what it carried", so the parser counts the `<detail>` children
it skipped and the panel says so and points at the pane.

### Over budget, the card degrades before it is refused

`cotStamp` discarded the whole clone when the props map exceeded the budget.
With extension keys added, a post that stamps today could stop stamping, and the
reader would meet raw XML where they used to meet a card. That is a regression
this work would have introduced.

The ladder is now: measure the full map; over budget, rebuild without the
extension keys and measure again; only then refuse. The middle rung keeps
everything version 2 ever wrote, so the degraded card is exactly the card this
feature shipped with, and it is logged under its own code rather than sharing
the refusal's.

`TestABatchOfMaximalEventsStillStamps` measures the shape that actually decides
it: `maxCotEvents` events, each carrying every registry entry at its cap, with
`lead` and `trail` at `maxNoteRunes` **each** and `src` at its new cap. A test
built on one maximal event measures a case that was never in doubt. It also
asserts the extension keys are PRESENT, because without that the two rungs could
be swapped and every test would still pass while every post silently lost them.

**The degraded rung says so on both surfaces.** `detail_dropped` is the one key
it adds. Without it the panel draws no groups and no unrecognized count, which a
reader meets as "this event carried nothing" rather than "this did not fit":
the same false reading `detail_unknown` exists to prevent, arriving by a
different route. The card carries the notice too, because the degraded rung also
drops `class`, so a MEDEVAC card's summary line vanishes with nothing to explain
it and a reader who never opens the panel would see only the absence.

**A props map that will not marshal is its own failure**, under `TF-11008`.
Folding it into the size refusal told the author their event was too big for a
failure that has nothing to do with their event and that no rung of this ladder
can shed: the value came from something else already attached to the post.

**The card's one summary line refuses rather than clips, for the same reason.**
Dropping trailing readings until the line fits is right until one reading alone
is over the cap: popping to empty made the line vanish, and clipping put ninety
runes of an author's `urgent` value under "Patients stated" with the word itself
cut off. Neither is a reading. The line says the value is stated and points at
the panel.

**A figure too long to be a reading is refused rather than clipped.**
`FormatFloat` never uses exponent notation, so a subnormal like `1e-320` expands
to 324 runes against a stated field cap of 128. A clipped number still reads as
a number, which is worse than an absent row, so this follows the flow-name rule.
Negative zero is normalized for the same class of reason: `-0` reads as a
direction on a bearing and as a sign on a battery.

### Where each extension's shape came from

`<detail>` is convention rather than standard, so the registry records what each
row rests on. Anything not tied to a primary source renders as **stated** rather
than as a decode, which needs no second code path: `Unit == ""` already means
"the sanitised string the event wrote".

| Extension | Rests on |
|---|---|
| `contact/@endpoint` | ATAK-CIV, written on every self position report |
| `__group`, `track`, `remarks`, `link` | Already shipped; unchanged here |
| `uid/@Droid` | ATAK-CIV |
| `takv` | ATAK-CIV, the client's own version element |
| `precisionlocation/@geopointsrc`, `@altsrc` | ATAK-CIV |
| `precisionlocation/@pdop`, `@hdop`, `@vdop` | **Unverified.** Rendered as stated |
| `archive` | ATAK-CIV, presence only |
| `usericon/@iconsetpath` | ATAK-CIV |
| `color/@argb` | ATAK-CIV. The signed-decimal encoding is the part that matters and is why it is decoded rather than shown raw |
| `track/@slope` | ATAK-CIV |
| `status/@battery` | ATAK-CIV. `@readiness` is **unverified** and rendered as stated |
| `Attitude` | FreeTAKServer's CoT model |
| `sensor` | ATAK-CIV |
| `__video`, `ConnectionEntry` | ATAK-CIV |
| `__chat`, `chatgrp` | ATAK-CIV GeoChat |
| `_medevac_` | FreeTAKServer's CoT model. `@security` is **not decoded**, see above |
| `_flow-tags_` | The MITRE XSD in the ATAK repository, which is also where the open attribute set is stated |
| `shape` / `polyline` / `vertex` | ATAK-CIV drawing tools. **Unverified**: the nesting and the `closed` attribute are convention |
| `ellipse` | ATAK-CIV. **Unverified**: `major`/`minor` read as semi-axes in meters and `angle` as a bearing clockwise from north |
| `link/@point` as a route vertex | ATAK-CIV routes. **Unverified**: read as `lat,lon` or `lat,lon,hae` |
| `__chatReceipt` | ATAK-CIV GeoChat, beside `__chat` |
| `__serverdestination` | TAK Server routing. **Unverified** |
| `_radio` | FreeTAKServer's model, the same source as `Attitude`. **Unverified** |
| `__geofence` | ATAK-CIV. **Unverified**: the elevation and trigger vocabularies are convention |
| `attachment_list` | ATAK-CIV. Only the number of hashes is read, never the hashes |
| `TakControl` and its three children | The ATAK protocol negotiation documentation |

Two things this table is for. It says which rows a maintainer may tighten
without new evidence, and it says which ones a bug report should be believed
about first.

### The long tail, and what is not resolved

`attachment_list` is **counted, not printed, and never resolved.**

Phase 1 said resolution would come later and reuse `cotFileOwnedBy`. That
promise cannot be kept and should not be. ATAK writes content hashes; Mattermost
file ids bear no relationship to them, and no index exists between the two. The
only way to match would be to fetch and hash every file on the post inside
`MessageWillBePosted`, on the filestore path `unverified.md` records as never
tested against a running server, to produce a row nobody can click.

Printing the hashes is refused for a second, independent reason: a content hash
is longer than `maxFieldRunes`, so a two-hash list truncates mid-hash into a
value that looks like a hash and is not. That is the failure `_flow-tags_`
already answers by dropping rather than truncating. The count is the part a
reader can act on, and the list itself is still under "As posted".

**A radio signal carries its unit.** An unlabelled `-71` is the derived-claim
failure running the other way: the reader supplies the wrong unit instead of the
plugin supplying it.

**`TakControl` is read now, and still gets no class.** Phase 1 cut it because
the nested binding it needed did not exist and a control class would have bought
one version string. The binding exists now, from `chatgrp` and
`ConnectionEntry`, so the element costs four registry rows and nothing else. The
class argument is unchanged: a control event already renders correctly, because
`0,0` is never linkable and `t-x-takp-v` already has a label.

## Checklists

A `<checklist>` is read, its contents are counted by name, and nothing about
what a checklist *means* is decoded. The panel shows the element names the event
itself wrote and how many of each it carried. Before this the whole element was
one tally under "elements this build did not read".

### Why nothing is decoded

The schema could not be verified. No general CoT reference defines `checklist`;
the FreeTAK documentation describes the feature with no XML; and a code search
against ATAK-CIV returned nothing, though that search also returned nothing for
`takv` and `chatgrp`, so it is a broken search rather than a negative result.

The TAK feature is **ExCheck**, and its templates are uploaded XML *files*
managed through a REST API alongside file metadata: filename, hash, size,
submitter. That is a data package rather than a `<detail>` child, so an ExCheck
template pasted into a channel would be refused by `Parse` for carrying no
`<event>` long before any of this applied. Whether a real event ever carries a
`<checklist>` inline is itself unverified.

That is the same position `__network` was refused from, and the decision is the
same. Decoding `name` or `status` or a column type means inventing attribute
names, which produces rows that never populate while claiming knowledge this
build does not have. Geometry ships unverified because its failure is visible:
the shape does not draw and the element is counted. An invented attribute name
fails silently, as a blank row.

Counting needs no schema at all. Every string in the block is either a label
this build owns, "Checklist", or a name the event wrote itself.

### Why it is outside the registry

For both of the reasons geometry is, and either alone would be enough.

`addBlock` is first-wins per element name, so a checklist's repeated rows would
collapse into one set of keys and a reader would be told about the first task
and not the other eleven. And the registry model stops at depth four, so a row
inside a wrapper element is not merely undecoded but uncounted, because only
`readDetailChild` increments `Detail.Unknown`.

### Descendants are counted at any depth

Direct children would have been the smaller change and the wrong one. The
nesting is exactly what could not be verified: if rows sit inside a
`<checklistColumns>` wrapper, counting direct children reports one wrapper where
counting descendants reports the rows. Counting by the name the event used makes
the answer correct under either shape, and honest under a third nobody has seen.

`Seen` counts every descendant, including those past `maxChecklistKinds`, and is
not the sum of the kinds once the cap bites. That is the same separation
`Geometry.Seen` carries, and for the same reason: a counter that stops counting
when a list stops growing reports the cap as though it were the measurement,
which is a bug this repository has already shipped once.

## Geometry

A drawn shape, a circle and a route are the three event kinds whose content is
not a point. Before this they rendered as a card with one crosshair at whatever
`<point>` said, which for a drawn shape is its centroid and for a route is one
end, and the geometry that IS the event was counted as an unrecognized element.

### Why geometry is outside the registry

The registry maps attributes to flat props keys, and that is exactly what
geometry is not: an ordered list of coordinates whose order is the data. It is
handled by hand for the same reason `_flow-tags_` is, and it lands in props the
same way, as one array-valued key rather than as fifty-eight flat ones.

It also reaches deeper than the registry can. A vertex sits at depth five,
`event > detail > shape > polyline > vertex`, and the registry model stops at
four because that is as deep as an attribute-bearing extension ever goes.

### A `<link>` is a relation or a route vertex, and the shape says which

ATAK writes a route's points as `<link point="lat,lon,hae"/>`, and `<link>` was
already how this parser reads relations. So before this, a route's points were
swallowed into the relations list, contributed nothing (their `uid` is empty and
`putIfSet` drops it), and spent the relation budget: a forty point route
exhausted `maxCotLinks` and cost the reader the "Sent by" row that budget exists
for.

**The two are told apart by what the element carries, not by the event's type.**
A link with a `point` is a vertex; a link with a `uid` is a relation. They get
separate caps, so a long route cannot cost a relation and a relation cannot cost
a vertex. Keying off `b-m-r` instead was considered and refused: a route drawn on
an event this build types as something else would then have its points read as
relations, which is the bug this replaces rather than a different one.

### The vertex cap refuses to draw, and keeps the event

A third answer, and deliberately neither of the two already here.

`maxCotLinks` drops the extras and keeps going, because a relation lost past the
sixteenth costs one entry in a row that has already told the reader who sent it.
`maxCotEvents` refuses the whole source, because a card showing thirty-two of two
hundred events is quietly wrong about what was posted.

A truncated route is the second case wearing the first's clothes. Drawing the
first `maxCotVertices` points of a longer route puts a line on the map that
confidently ends somewhere the route does not, which is worse than drawing
nothing. But refusing the post over it is too harsh when the callsign, the
position, the times and the accuracy are all fine and all worth having.

So past the cap the event is kept, the geometry is **not drawn**, and the card
says why. The reader gets everything the event stated except the one thing this
build cannot render honestly, and is told which.

### A shape and a route are separate geometries

They shared one, and three things fell out of that. A `<link>` whose point did
not parse set the refusal flag and killed an otherwise perfect polygon. A route
point written after a shape was appended as another corner, so a closed area
grew a leg to wherever the link pointed. And links written before a shape left
the kind as `route`, which silently dropped the shape's own `closed`.

The two are read into their own values now, and `drawnGeometry` prefers the
shape: an event carrying both is describing the shape, and its links are still
relations wherever they carry a uid.

### Acceptance is what the element filled, not what the geometry holds

The depth-four branch marked a child accepted whenever the geometry was
non-empty afterwards, rather than when that child was the thing that filled it.
So an `<ellipse>` followed by a `<polyline>` marked the polyline accepted, and
the polyline's vertices were read into the ellipse. The card then said the shape
was not drawn while the map drew the ellipse anyway, which is the card and the
picture disagreeing about the same event.

`readShapeChild` reports whether it recognized the element, and only that marks
it accepted.

### A vertex is held to the whole gate a position gets

Vertex coordinates go through `decimalShape`, the gate `<point>` already uses,
**and the range check beside it**. Only the grammar half landed at first, which
is worse than neither: a latitude of 200 is a plain decimal, so it passed, and
`fitBounds` refuses a latitude outside 90 by throwing. The reader met a blank
map with no note, or lost the map entirely on the next Reset view.
`strconv.ParseFloat` accepts hexadecimal and exponent notation where JavaScript's
`Number` does not, and the failure mode is the one already recorded above: a
value that validates in Go, is stored verbatim, and is then read as `NaN` by the
map, leaving the card and the picture disagreeing. On a vertex list that would be
one silently missing corner rather than a missing pin.

A shape whose vertices do not all pass is not drawn at all. A polygon missing one
corner is a different polygon, not a partial one.

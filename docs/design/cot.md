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
budgets are defence in depth.

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
map already has to respect. The list names each track, colours its dot and links
its position, and the panel behind "Open details" has the full detail of every
one. A single event still gets the full card, unchanged.

**The map draws them all.** `LocationMap` takes `markers`, one per event, and
frames the whole set rather than opening on the first, which would leave the
rest off screen with nothing to say they were there. The image is named per
FEATURE rather than per layer, because one icon on the layer would paint a
hostile track in a friendly colour, and an image is registered once per colour
rather than once per marker. The accuracy circle is drawn only for a single
event: a ring per track reads as overlapping blobs rather than as positions.

**The props version is 2**, and the webapp reads 1 as well, because posts
stamped before the array exist and still render. A bundle older than the bump
meets a version it does not know and falls back to the post's own text, which is
what the field is for.

### A bare event, with no fence

An event needs no fence around it. `decorators.SoleElementSpan` finds the span
from the first opening tag to the last closing one, so siblings come back
together, and the fence is still tried first so a labelled fence keeps its
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
SAID wherever two relations are shown side by side: `example-details` shows a
`p-p` beside a `p-c` and carries a line stating that the card does not tell them
apart, because two rows differing in one attribute imply that it does.

**`maxCotLinks` drops the extras and keeps the event**, which is the opposite of
`maxCotEvents` above. The asymmetry is the cost of being wrong in each case: an
event lost from a batch leaves a card quietly claiming a set it does not hold,
while a relation lost past the sixteenth costs one entry in a "Relates to" row
that already told the reader who sent it. `TestTheLinkCapDropsRatherThanRefuses`
pins the difference, because an examples row states it to readers.

### The marker is a crosshair, in the affiliation's colour

**The shape** is a circle with a line across it and a line down it. A filled dot
reads as "somewhere around here"; a crosshair reads as "this point", which is
what an event is claiming.

Two things were tried and removed on the way, both recorded so they are not
tried again. Ticks radiating from the centre instead of full lines made the
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

The shape is stroked twice, the edge colour first and wider. That outline is what
the palette's own note relies on ("the pin is distinguished by its outline against
land rather than by its fill"), and it is what lets the fill carry something else.
The wider pass has to grow the lines along their LENGTH as well as across them,
or each line ends in a bare fill pixel meeting the map, which is the one place
the outline has to be.

**The colour** is the affiliation's, from the same `affiliationColor` the dot
beside the callsign reads, so the two cannot disagree about what a track is.

The caveat: on the map, colour is the ONLY thing carrying affiliation, and the
hues are close in luminance, so hostile red and neutral green are one mark to a
reader with a common colour vision deficiency. `markerLabel` is the other channel
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

  The table has gaps: `A-M` has no row while `A` and `A-M-L` both do. That costs
  nothing, because `longestAtomPath` keeps the deepest match rather than stopping
  at the first miss, and `TestADeeperPathResolvesThroughAMissingParent` pins it.

### Time without a trusted clock

Clock drift of tens of minutes is routine on field kit and air-gapped VMs, and
two readers looking at one post must not disagree about whether to act on it.

- `time`, `start` and `stale` render as absolute Zulu DTG strings, through
  `dtg.FormatZulu`, so there is one DTG rendering in the repository.
- The validity window is `stale - time`, both from the event, so the figure is
  identical everywhere. Computed in Go.
- The age reading is `stale_at - post.create_at`, both server-side values.
  Computed in the **webapp**, because `post.CreateAt` is 0 inside
  `MessageWillBePosted`: `referenceTime`'s own comment records that the server
  fills it in later.
- **Nothing ticks.** `dtg/Countdown.tsx` runs a one-second interval and a pulse
  and deliberately ignores `prefers-reduced-motion`, which was argued for a
  single RHS panel. Thirty position reports in a channel would be sixty timers
  and a wall of pulsing red, and, worse, a ticking clock on a one-time paste
  reads as a live feed. Nothing on the card ticks, which is the guarantee that
  matters; a line of prose saying so was tried and removed as clutter, since it
  restated what the absence of movement already tells a reader. The live
  countdown lives in the sidebar panel, which says what it is counted against.

### The CE circle

`LocationMap` gained one optional prop, `accuracyMeters`. Every existing caller
passes nothing and is unaffected.

A dot cannot be allowed to look the same at CE 3 m and CE 9 km: accuracy is the
channel through which this feature's worst failure mode arrives. MapLibre's own
circle layer takes a radius in **pixels**, so the drawn accuracy would change
with every zoom; a polygon is the only shape that keeps its metres.

**The vertices are geodesic.** A metre is a different number of longitude degrees
at every latitude, so the offsets are `dLat = m / DEGREE_METERS` and
`dLon = m / (DEGREE_METERS * cos lat)`, reusing the `DEGREE_METERS` already
pinned to Go. An equal-degree ring is right at the equator and increasingly wrong
toward the poles, which for a layer whose whole purpose is to stop the map
overstating a fix is the wrong way to be wrong.

A metre is drawn on the ground, but the accuracy is **read** from the string the
card already shows. `LocationMap` takes `accuracyLabel` rather than formatting
the number a second time: rounding metres again turned a stated `0.4` into
"within 0 metres", a claim of a perfect fix made only to a screen reader, while
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
The card reads the same `features.mapInline` and the same `INLINE_ID` reader
preference, so a reader who hid the map under coordinate posts has hid it here
too. `TestCotHasNoMapSettingOfItsOwn` is where that decision gets revisited
rather than silently reversed.

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
crosshair on screen in the colour that carries the most meaning.

It carries **no `__group`**. That element is the sender's team, and ATAK puts it
on a self position report rather than on a placed marker. On a hostile contact
the card would render "Team: Cyan" against the target, which reads as the target
being on that team. `TestTheExampleIsAHostileContact` pins both the affiliation
and the absence.

`how` is `h-e`, estimated, and the accuracy is tens of metres rather than the
sub-metre of a GPS fix, because a contact somebody plotted by eye is not known
to the precision of a device reporting itself. An example whose numbers claim
otherwise teaches the wrong thing about what `ce` means.

**The row in `examples` quotes the card's own label rather than restating it.**
That row exists to show that `a-h-G-U-C-A` becomes readable English, so prose
beside it naming a different reading is the one failure it cannot afford.
`cotExampleTypeLabel` parses the example and reads `type_label` out of the props
the card itself renders from, and `TestTheExampleRowQuotesTheCardsOwnLabel` holds
the two together. Changing the type, or its row in `types.csv`, moves the prose
with it.

### The examples section, and why it cannot use the decorator machinery

`example-details` posts one message per set, and Cursor on Target earns one
without being a decorator. `detailSet` is the wrong home for it twice over. Its
rows are run through the tagger and asserted to decorate or not, which is a
question about tokens rather than about post types; and running an event through
the tagger would find the RFC 3339 timestamps inside it and hang a date-time
link off the row, which says something false about how an event is read.

So `packDetails` walks `detailMessageSets`, a catalog of named chunk producers,
and Cursor on Target is the last entry. The catalog exists so a test asking
"which post does this heading belong to" reads it rather than listing the
decorators and quietly ignoring anything that is not one, which is exactly how
this section first went in with three tests still green.

**Every example is one line, and none of them is a fenced block.** Two separate
reasons, and both of them bite.

The packer's atomic unit is a line, so a fenced block spanning several of them
can be split across two posts: one message ends holding an unterminated fence
and the next opens with an orphan closing one. A single line cannot be split, so
this cannot happen however small a server's post limit turns out to be.

The other reason is that this post is describing the thing it is made of. A
fenced block labelled `cot` or `xml` is exactly what `cotSource` recognises, and
`SoleFencedBlock` needs only that there be **one** of them in the message. A
details post that happened to be chunked down to a single labelled fence would
be stamped: the card would own the body, and every other row in that post would
render as the plain text of its own markup. The events are therefore written
inside **inline code**, which is a code range, so `SoleElementSpan` does not see
an event in one and `SoleFencedBlock` has nothing to find at all.

Showing the fence syntax at all is done by printing the opening line, `` ```cot ``,
as a row of its own rather than by drawing a real block.

**The test that guards this asks `cotSource`, not the text.** It used to check
that no unstamped post contained the substring `<event`, which was true and
sufficient while the only event in the output was the live card's own post. It
is now false by construction: the details post is full of events, inside code,
where nothing reads them. `TestTheEventExampleIsAPostOfItsOwn` therefore runs
every created post through the recognition function itself, which is both
stronger and the actual invariant.

`TestTheDetailEventsAreWhatTheirRowsClaim` holds the examples to their notes: a
row promising several events parses to several, and the link row's link parses
and names a parent. A row teaching the wrong shape cannot be caught by reading
it.

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

**The one that mattered: no affiliation.** Colour is the whole of what tells one
marker from another on a multi-event map, so a reader who gets no colour got a
count and nothing else. `cot.md` already argues that colour is never the only
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
and is keyed on the same ids, with `every affiliation with a colour has a word`
holding the two together. An affiliation that gained a colour and no word would
be a marker a screen reader cannot tell from any other, on the one surface where
telling them apart is the entire job. An affiliation this build does not know is
called `unstated` rather than dropped, which matches the map drawing it in
`UNCOLOURED` rather than leaving it off.

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

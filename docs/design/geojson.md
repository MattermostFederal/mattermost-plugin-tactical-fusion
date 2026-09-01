# GeoJSON

A GeoJSON document posted as a fenced ```geojson block, or attached to a post as
its sole `.geojson` file, is rendered as a custom post type: a card listing how
many features the document holds and of what geometry, naming each one, showing
the properties it carries, and drawing them all on one map fitted to their
combined extent.

This note is the whole rationale. None of it may go in a comment.

## Why GeoJSON is a sibling of Cursor on Target, not of the decorators

The decorator framework is token to link to page. A GeoJSON document produces no
link, so it has no query string and `RenderPage` would have nothing to render,
and a fenced code block is a protected range that `findProtectedRanges` exists
to keep the tagger out of. Both arguments are the ones `docs/design/cot.md`
already makes, so `server/geojson/` sits beside `server/cot/` and recognition is
a second step in the hook rather than a new `Pattern`.

## Recognition is deliberately narrow

Only a ```geojson fence and a `.geojson` attachment. Not `json`, and not
`.json`.

Stamping a post sets `Post.Type`, and the invariant in `CLAUDE.md` spells out
what that costs: the post's Elasticsearch and OpenSearch matches, its
auto-translation and its embeds. `MessageWillBePosted`
runs once, there is no `MessageWillBeUpdated`, and the stamp is written into the
stored post. So a false positive is permanent, silent, and unfixable by the
author.

Cursor on Target accepts an ambiguous ```xml fence, and that is safe because an
XML document rooted at `<event>` carrying a uid, a type and a time is
essentially never anything else. JSON is not like that. A Mattermost server is
full of pasted JSON, and a structural pre-check on `type` plus the matching
member is not a strong enough discriminator to bet a post's search index on.

The asymmetry decides it: an author who wants the card can spell the fence
`geojson` at no cost, while an author who pastes a `json` fence and gets it
silently stamped has lost search on that post forever. `TestAJSONFenceIsNeverStamped`
and `TestAJSONAttachmentIsNeverRead` are the guards, so widening this has to
change a test rather than slip through.

## Format order stays format-major

`decoratePost` calls `cotStamp`, then `geoJSONStamp`, then decoration, and each
format asks its own fence, bare element and attachment questions in that order.

Source-major sequencing (every format's fence, then every format's bare element,
then the attachment) was drafted and cut. Two reasons:

- It cannot coexist with `cotStamp`'s shape. That function owns its own source
  precedence and has a production caller in `command_cot_example.go` plus nine
  test call sites. Splitting it would leave the tests exercising a wrapper while
  the production path changed underneath them.
- It silently changes what a refused source does. Today a ```cot fence that
  fails to parse ends the attempt (`hooks_cot.go`), and the attachment is never
  consulted. Under source-major it would fall through and stamp from a `.cot`
  attachment instead. That is a Cursor on Target behavior change, and an
  unrepairable one per post. `TestARefusedFenceDoesNotFallThroughToAnAttachment`
  pins today's behavior.

The one collision format-major leaves is a post carrying a `.cot` attachment
**and** a visible ```geojson fence, where CoT would stamp from the file the
reader cannot see, violating "the visible message always wins over an
attachment". That is closed where the gate already is: `cotFileSource` refuses
the attachment when the message's sole fenced block is a recognized `geojson`
fence. Three lines, guarded in both directions by
`TestTheVisibleFenceBeatsAnAttachmentAcrossFormats`.

## What the two stampers share, and what they must not

Shared, in `hooks_stamp.go`:

- `stripStampedTypes`, because the forged-type strip has to know every type this
  plugin stamps whichever recognizer runs first.
- `attachmentOwnedBy` and `attachmentCreator`, because that check is the
  security boundary between a post and a stranger's unattached file, and a
  second copy of it is one copy too many.
- `commitStamped`, the measure-and-commit ladder.

**Not shared: the recover.** Each stamper declares its own `defer` as its first
statement, and each spans its own strip, source-finding, filestore call, parse
and commit. Putting the recover inside `commitStamped` would leave
`GetFileInfo` outside every recover on the hook path, because `decoratePost`
calls the stampers from outside `decorateMessage`'s recover. `hooks_test.go`
records `panicOnFileInfo` as the only injection point that path has, and
`TestCotRecoversFromARealPanic` and
`TestAPanicAfterStrippingStillReturnsTheStrippedPost` both depend on it. A
shared recover also has no access to the `stripped` clone it is contractually
required to return.

**Not shared: a result type.** `hooks.go` already distinguishes all three states
from the existing pair. `(nil, false)` is nothing happened, `(clone, false)` is
stripped, `(clone, true)` is stamped. A struct naming what two return values
already name is an abstraction the code does not need.

**Not shared: the attachment gate.** Each format checks its own switch before
any API call, exactly as `cotFileSource` shipped. Gating on the union of the
switches would make an install with both file switches off ask the store about
every one-attachment post, and would read bytes for a format the admin turned
off. `TestGeoJSONRefusesBeforeAskingTheStoreWhenTheSwitchIsOff` guards it.

## The strip clears every key, not just the matching one

`stripStampedTypes` removes every props key in `stampedTypes` from every post,
and clears the type when it matches any row.

Removing only the row belonging to the post's own type leaves the sibling. The
commit copies the post's existing props forward, so an author posting
`props: {tactical_fusion_geojson: <forged>}` with an empty `Type` and a real CoT
fence would have that forged blob carried into stored props permanently,
counted against the rune budget and readable by everyone who can read the post.
Nothing renders it today, which makes it latent rather than live; it stops being
latent the moment anything dispatches on a props key.
`TestAForgedSiblingBlobIsAlsoStripped` is the guard.

`custom_tf_location` is deliberately **not** in the table. `stampStandalonePost`
writes that type from decoration rather than from recognition, and sweeping it
in would change forged-type handling on a shipped path that nothing asked to
change.

## The parse

### A token walk has no depth limit

`encoding/json` caps nesting at 10000, but that cap lives in the value scanner
and `Decoder.Token` bypasses it for `[` and `{`. Measured on Go 1.26.7: thirty
thousand nested `[` walks clean through `Token` and is refused only by
`Unmarshal`, which this package never calls. Both the decoder's token stack and
the walker's own recursion grow unbounded, so the walker counts its own depth
against `MaxJSONDepth`, and the fixtures for it nest inside `coordinates` and
inside `properties` as well as inside `GeometryCollection`.

### Repeated keys are first-wins

Matching `cot/parse.go`, which records the rule and why it is one rule rather
than two. Refusing them was drafted and cut: nothing in this plugin unmarshals
the document, so there is no last-wins reader inside the plugin to disagree
with, and a sentinel plus an ephemeral plus a help row plus a test class is a
large price for a document that is weird rather than harmful.

### Why `decimalShape` is not reused

`cot.go`'s `decimalShape` guards an XML **attribute**, which is an arbitrary
string. Its stated reasons are that `ParseFloat` accepts hexadecimal floats and
exponent notation that JavaScript's `Number` does not, and that a latitude of
sixty thousand leading zeros can be parsed, ranged and rendered as a coordinate.

A GeoJSON coordinate arrives through JSON's number grammar, which admits neither
hexadecimal floats nor a leading `+`. Half that rationale is unreachable here.
What the regex **would** do on this path is refuse legal GeoJSON: `1e-05` is
what `json.dumps` writes for `0.00001`, and more than fifteen decimals is
exactly where `ogr2ogr` sits at `COORDINATE_PRECISION=15`. Both are ordinary
output from the tools this feature exists to read, and both would render as a
card that lists features and refuses to draw them.

So the check is `maxCoordRunes`, then `ParseFloat` with its error handled, then
`math.IsInf`/`math.IsNaN`, then the range test. That closes the
sixty-thousand-zeros defect the regex was really for and accepts every number
RFC 7946 permits. `TestExponentAndLongDecimalCoordinatesAreDrawn` and
`TestUnreasonableCoordinatesAreNotDrawn` are the two halves.

The divergence between the two packages is deliberate. Do not reconcile it.

### Refusals and notes are different mechanisms

**Sentinel errors** refuse the whole document. A card showing the first 256
features of nine hundred is a card that is quietly wrong about what was posted,
which is the argument `maxCotEvents` already makes.

**Notes** are carried on the parsed value and rendered beside the feature: a
malformed or out-of-range position, an unclosed or too-short ring, a foreign
`crs`, a malformed `bbox`, an unlocated feature. Those are one feature's
problem, or one claim about the document, not a reason to refuse it.

Nothing appears in both lists. An earlier draft had `ErrForeignCRS` and
`ErrBadPosition` declared as sentinels and described as notes, which would have
produced two incompatible test suites.

Notes are **sentences authored in Go**, matching `GeometryTooLargeNote`,
`GeometryUnusableNote` and `position_note`. A closed cross-language `reason`
vocabulary was drafted on the belief that `cot.Geometry.Undrawable` was
machine-readable; it is a sentence. Nothing branches on the note in Phase 1, so
a Go string costs no cross-language surface and adds no sync row. The closed
vocabulary is kept for `kind`, which the card and the map both dispatch on.

### The caps

`MaxVertices` sits **above** what the byte cap allows, not below it. At realistic
precision 64 KiB holds roughly 2500 positions, so a cap of 2048 would bind first
and refuse a legitimate 60 KiB QGIS export that is inside every other limit.
4096 is above that ceiling, so the byte cap binds for realistic documents and
`MaxVertices` catches only the low-precision case, where `[1,2]` is six bytes
and 64 KiB holds far more positions than the map should draw.

Both halves are measured rather than asserted.
`TestTheLargestDocumentFitsThePropsBudget` builds the largest document the byte
cap admits and measures its encoded props (2519 positions, 257k runes, 34% of
the 760000 budget). `TestMaxVerticesFitsThePropsBudget` does the same at
`MaxVertices` with low-precision lexemes, and
`TestMaxVerticesBindsBeforeTheByteCapAtLowPrecision` proves the cap is
reachable at all, because a cap that can never bind is a constant that refuses
nothing.

There is no `MaxRings`: 33 rings of four vertices is 132 vertices, so it would
refuse nothing `MaxVertices` and the byte cap do not already refuse.

**Two differences from `cot.MaxVertices`, which is 512.** CoT's is **per shape**
where this is **document-wide**, and CoT's **truncates and records `Seen`**
where this **refuses**. Neither is a mistake and reconciling them would break
one of the two.

### `crs` and `bbox`

RFC 7946 removed `crs` and fixed the coordinate reference system. A document is
drawn only when `crs` is absent or names CRS84. Anything else, **including
`EPSG:4326`**, is read and listed but not drawn: many producers write that name
with latitude first, and this package cannot tell which from the document.
Drawing it anyway would put features in the wrong country silently.

`bbox` is validated for length and finiteness and then ignored for the extent,
which is computed from the coordinates: a stated box is a claim the coordinates
may not support. It is still read, because a malformed one is often the symptom
of a producer writing the axes the wrong way round.

## The props blob

### One uniform shape holds all nine types

Every geometry is a list of parts, every part a list of rings, every ring a list
of positions. A flat `positions` plus `rings` pair, as the first draft had,
cannot hold `MultiLineString`, `MultiPolygon` or `GeometryCollection`: it joins
two disjoint lines into one polyline and draws a polygon's hole as a solid
island.

The cardinality per type is fixed, and it has to be **stated** rather than
inferred, because the cross-language shape guard runs Go and TypeScript over one
shared fixture and therefore agrees by construction whichever encoding is
chosen. A wrong choice is invisible to that guard, which is why
`TestPartCardinalityMatchesTheTable` exists.

| GeoJSON type | parts | rings per part | positions per ring |
|---|---|---|---|
| `Point` | 1 | 1 | 1 |
| `MultiPoint` | N | 1 | 1 |
| `LineString` | 1 | 1 | N |
| `MultiLineString` | N | 1 | N |
| `Polygon` | 1 | R, ring 0 the exterior | N |
| `MultiPolygon` | 1, with `RingCounts` | every ring of every member, in order | N |
| `GeometryCollection` | one part per member, each carrying its own kind | as above | as above |
| null geometry (`none`) | 0 | | |

`MultiPoint` is N parts rather than one part of N positions so it is
structurally distinct from `LineString` at the parts level and not only by
`kind`. `MultiLineString` is N parts for the reason the parts level exists at
all.

`MultiPolygon` is the one case parts alone cannot express, because flattening it
to P parts of kind `Polygon` loses which rings belonged to which member polygon,
and that grouping has to survive a `MultiPolygon` nested inside a
`GeometryCollection`. So it stays **one part** carrying its own kind, and
`RingCounts` names how many rings each member contributed, in order. That is the
only place the shape needs a fourth number, and it is carried rather than
inferred. `TestNestedMultiPolygonKeepsItsGrouping` is the guard.

### The name is hoisted out of the properties bag

The lower props rung drops `properties`. An unhoisted name would therefore
vanish exactly when the document is largest, degrading the feature list to
"Feature 1, Feature 2, ..." in the one case a reader most needs it.

Precedence, fixed and tested: `properties.name`, `properties.title`,
`properties.label`, the Feature's own `id`, then `Feature N`. A blank or
whitespace-only value falls through to the next rung rather than winning.

### Property values are rendered to strings in Go

GeoJSON permits string, number, boolean, null, array and object. A string is
sanitized, a number is its lexeme, a boolean is `true`/`false`, a null is
dropped **along with its key** (rendering it as "null" would state a value the
document did not carry), and an array or object is marshaled and then truncated
to the rune cap with the same visible marker `src` uses.

Deciding this in Go rather than in the webapp is what keeps the rune measurement
the ladder depends on measuring what is actually stored, and gives the escaping
test one place to point at.

Properties are sorted by key. Go randomizes map iteration order and the blob is
written permanently, so an unsorted bag would list a feature's properties
differently on two runs and would make the measurement unrepeatable.

`renderValue`'s `json.Marshal` cannot fail on anything the walker produces: it
hands back only strings, `json.Number`, booleans, and its own object and slice
types, and `rebuild` converts the last two into `map[string]any` and `[]any`
first. The error branch drops the key rather than returning one, which is the
same outcome null already has, so an unreachable branch cannot invent a value.
Returning an error instead would make an unreadable property refuse the whole
document, which is the opposite of what the ladder is for.

### The ladder has two rungs, and `src` is never dropped

Everything, then without `properties`. Below that the document is refused.

A third rung dropping `src` was drafted and cut, for the reason `cot.go`
records: once `Post.Type` is set the webapp never reads `post.message`, so props
are the only copy of the document any reader can reach. Withholding it leaves a
reader with no way to check the card against anything. Truncating with a visible
marker is the honest version of the same limit.

`properties_dropped` is a presence key, mirroring CoT's `detail_dropped` and for
the same recorded reason: an absent `properties` array is otherwise
indistinguishable from a feature that genuinely carries none, which is the
common case in a real export.

## Measurement

A line gets a length, an area gets an area, a point gets neither, and a
`GeometryCollection` gets both when its members provide both: it is one feature,
so "how big is this" means the whole of it. A feature the parse noted is not
measured at all, because a figure printed beside "not drawn" would be standing
behind a shape this build has just said it will not stand behind.

**A sphere, not the ellipsoid.** `location` projects on WGS 84 because a
coordinate has to round-trip; nothing here does. The sphere is within about half
a percent at every latitude, which is far inside what the rendering claims, and
using `wgs84A` would be a precision this does not have.

**Three significant figures, and no more.** The sphere is approximate, the
coordinates are written to whatever resolution the author chose, and a route
measured off a map is not a surveyed distance. `12.4174 km` would be claiming a
precision none of those three support.

**Rendered in Go**, like property values and for the same reason: the card and
the panel both show it, and two renderers rounding the same figure separately is
how the two surfaces come to disagree.

`RingCounts` is what makes a `MultiPolygon` measurable. Its rings arrive as one
list, so without the member boundaries the second member's exterior would be
subtracted as though it were a hole in the first. A test asserts two identical
members measure twice one.

The figures are checked against numbers that do not come from this package: a
degree of latitude, London to Paris, and a one-degree square at the equator. A
test that only agreed with the code would share its mistakes.

## The unlabeled spellings are opt-in

`EnableGeoJSONUnlabeled` reads the three spellings that do not name the format:
a fence labeled `json`, a `.json` attachment, and a document with no fence at
all. It is one of two switches in the plugin that ships **off**.

The default is the argument. Stamping a post is permanent, it costs the post its
search matches and its embeds, the author cannot undo it,
and ordinary JSON is pasted into chat constantly. An install whose channels
carry overlays rather than API payloads can turn it on; nobody arrives at it.

It widens what is **read**, never what is **accepted**. A document that does not
parse as GeoJSON is still left alone, which a test pins with a package manifest,
a JSON Schema fragment and an API response. And a bare object the author fenced
as code is never read: `SoleObjectSpan` refuses anything overlapping a code
range, for the reason `SoleElementSpan` does.

`SoleObjectSpan` is a brace matcher and nothing more. It skips braces inside
strings and honors escapes, because a document carrying `}` in a property value
would otherwise end the span early and hand the caller a truncated object.
Deciding whether the span is really GeoJSON is the caller's parse, exactly as it
is for the element scanner, so this package does not grow a JSON reader.

Only a `geojson` fence earns an ephemeral. The ambiguous spellings fail in
silence, which is the argument `reportCotRefusal` makes about an `xml` fence: a
message on every one of them would fire constantly on ordinary posts.

## simplestyle, and the two gates it passes

RFC 7946 says nothing about styling, so a document's appearance lives in
ordinary properties under the names simplestyle-spec settled on.

| Property | Read for | On the wire |
|---|---|---|
| `marker-color` | point | `color` |
| `stroke` | line, polygon outline | `color` |
| `fill` | polygon | `color` |
| `stroke-width` | line, polygon outline | `width` |
| `stroke-opacity` | line, polygon outline | `line_opacity` |
| `fill-opacity` | polygon | `fill_opacity` |
| `marker-size` | point | `marker_size` |
| `marker-symbol` | nothing | not carried |

This build once read only the three that decide a **color**, on the argument
that widths, opacities and symbol names are a styling language and carrying them
would make this a renderer of somebody else's stylesheet. That was narrowed on
request, and the earlier line was in the wrong place: a document that says a
hazard area is red at a quarter opacity is saying something about the hazard,
and dropping it redrew the author's meaning in the theme's colors. The line now
falls between what this build can **draw** and what it cannot.

`marker-symbol` is the one still ignored, and it is a capability rather than a
decision. A symbol name indexes an icon sprite; this plugin ships an offline
basemap with generated glyph ranges and no sprite at all, and simplestyle's
vocabulary is Maki, some two hundred icons with their own license and bundle
cost. Shipping them is a separate decision with a `THIRD-PARTY-NOTICES` and a
bundle-size argument attached. It stays visible as a property row, which is what
every unread key gets, and a test asserts that rather than leaving it silent.

Each name is read **for its own geometry**, not taken wherever it appears. A
point carrying both `marker-color` and `fill` is drawn in the one meant for a
marker, and a point's `fill-opacity` is ignored rather than deciding how solid a
marker is.

**Widths are refused, never clamped.** `maxStrokeWidth` is 10 device pixels,
which is already very heavy; a `stroke-width` of 4000 is a solid screen with no
way for a reader to tell it from a rendering fault. A clamp would draw something
the document did not ask for while claiming to honor it, so an out-of-range
width falls back to the theme's, which is honestly not the author's width.

**A stated opacity of zero is a value**, not an absence. A document may
deliberately say a fill is invisible, leaving an outline with nothing inside it.

**Numbers travel as the document's own lexemes**, like `length` and `area`: the
text is what both surfaces show, and two renderers rounding one figure
separately is how they come to disagree. The webapp parses them at `styleOf` and
nowhere earlier, so a `NaN` never travels as a number.

**Two gates, not one.** Go validates a color to three or six hex digits and
normalizes to six; the webapp validates again through `fillOf` before the value
becomes paint, because a props blob is not a trusted input either. That is the
same two-sided posture `cot.go` records about its own `argb`.

The same applies to every number. `styleOf` re-checks the range rather than
trusting Go's check, and that second gate is load-bearing rather than
ceremonial: `Number('')` is 0 and `Number('1e999')` is `Infinity`, so an
unguarded parse turns an empty string into a real width and a nonsense one into
a line that covers the map.

### Why the paint is data-driven, and what that cost

A document colors its features differently and one layer has one scalar, so
`paintGeometry` reads `['coalesce', ['get', 'color'], <theme>]`.

The **fill is precomputed**, not derived by an expression. `fillOf` composites
at `GEOMETRY_FILL_ALPHA`, and an expression reading the line color straight into
`fill-color` would paint every shape opaque, which is a visible regression on
the Cursor on Target card. So `styleOf` writes both values onto the feature and
both have been through `fillOf`.

That is why `fill-opacity` is composited into the color while `stroke-width` and
`stroke-opacity` are their own paint properties: the fill's alpha has to travel
inside the color it belongs to, and a stated one **replaces** the theme's
compositing alpha rather than multiplying with it. A document asking for 0.25
means a quarter, not a quarter of 0.16.

### The pin layers are both built, and split by a filter

`buildStyle` chose between a symbol pin and a circle pin from `hasMarkers` at
construction. A panel reuses one map across selections, so a document with no
points built the circle layer and the NEXT document's markers drew as plain
theme dots with their stated color and size ignored. That is the defect
`buildStyle` already records fixing for the accuracy and geometry SOURCES; the
pin LAYER was left behind. Both are emitted now, filtered on `['has', 'icon']`
and its negation, so the data decides and nothing is decided at build time.

### marker-size, and the two write paths that had to become one

`applyView` had a positioned branch calling `drawableMarkers` and an extent-only
branch with its own inline `markedPoints` call that omitted the scale. GeoJSON is
always extent-only, so a stated `marker-size` was validated in Go, validated
again in the webapp, carried the whole way, and dropped at the last hop on the
one surface that draws it. `markerFeatures` is now the single builder.

The test that missed it mounted a POSITIONED harness, so it exercised the branch
that already worked. The regression test is on the extent-only path.

`markerScale` looks up through `Object.hasOwn`, not a bare index. A bare index
walks the prototype chain, so `marker_size: "constructor"` yielded a function
where the type promised a number; `cot/types.ts` guards its two tables the same
way and this was the lookup that skipped it. It is the one style value that does
not pass through `styleOf`, so this is its only gate.

### The admin switch differs from Cursor on Target's, deliberately

The card reads `mapInline` and the panel reads `mapPanel`, because in the
sidebar this is not a map under a post. Cursor on Target reads `mapInline` on
BOTH surfaces, argued in [`cot.md`](cot.md) "Switches" and revisited through
`TestCotHasNoMapSettingOfItsOwn`.

That difference is a decision on each side, not a drift, and it was nearly
"fixed" into a drift: the two doc comments were near-identical and both claimed
CoT's rule, so a reviewer reading either one would conclude the other was wrong.
The comments now say which switch each reads and why.

### A fill layer does not ignore a line

`geometry-fill` filters on `['==', ['geometry-type'], 'Polygon']`, and it has to.
A MapLibre fill layer does **not** skip a `LineString`: it closes the ring and
fills it, so an open shape was drawn as a translucent wash between its first and
last point, over the map, with the line running through it.

The source carried exactly the right geometry the whole time. `shapesFeature`
emits a `LineString` per ring for an open shape and one `Polygon` for a closed
one, and always did; the defect was that one layer drew both. It hid for as long
as every line took the theme's own faint fill, and surfaced the moment a document
could state a saturated color of its own, which is what found it.

The regression test reads `queryRenderedFeatures` on the layer rather than the
source, because a test that read the source would have passed against the bug.

`geometry-outline` sets `line-join: round` and `line-cap: round` in the same
place. The defaults are miter and butt, and a route bends: a mitered join at any
real width throws a spike out past the corner that reads as part of the drawing.

`marker-size` scales the one reticle through a data-driven `icon-size` rather
than adding an image per size. The reticles are already generated per color, so
three sizes would treble that for a difference a multiplier expresses exactly.

The cost, and it was anticipated before the change: the map's own color tests
used to read the layer's paint property back as a string. Against an expression
that reads `["coalesce",["get","color"],"#..."]` **whatever an author supplied**,
so the assertion that an unvalidated string never reaches the map would have
passed against any input at all. The harness now reads the FEATURE property,
which is where the value actually lands. Verified by deleting the `fillOf` gate
and watching those tests fail, including the one that mounts
`url(https://attacker.example/px)`.

## The webapp re-caps nothing

`readFeatures` and everything under it render exactly what Go produced. The
server already refuses every document past its own limits rather than truncating
one, so a cap repeated on the webapp side could only **disagree** with it: a ring
cut short there would close onto the wrong vertex and draw a polygon nobody
posted.

That is why there is no caps sync row. `TestWebappGeoJSONTruncatesNothing` holds
the reader to it by refusing `.slice(`, `.substring(` and `MAX_` in that module.

## Cross-language sync points

Four guards, in `server/geojson_sync_test.go`:

| Duplicate | Guard |
|---|---|
| Post type, props key, props version, source kinds | `TestWebappGeoJSONPostTypeMatches` |
| The `kind` vocabulary and its order | `TestWebappGeoJSONKindsMatch` |
| Every key the blob carries, walked rather than scraped | `TestWebappGeoJSONShapeMatches` |
| The webapp truncating nothing | `TestWebappGeoJSONTruncatesNothing` |

The shape guard **walks** the blob rather than scraping flat `text()` calls the
way the Cursor on Target guard does, because this blob is mostly nested: counts,
features, parts, rings, positions and properties are all below the top level, and
a scraper would have gone quiet on every coordinate.

There is no reasons guard, because notes are Go-authored sentences the webapp
renders verbatim.

`TestGeoJSONHasNoMapSettingOfItsOwn` holds the decision that the card reads the
location decorator's map switches, the way `TestCotHasNoMapSettingOfItsOwn`
holds CoT's.

## The sidebar panel

`showGeoJsonDocument` puts the payload in the selection store and opens the
right-hand sidebar, through the same `panels.ts` registry Cursor on Target uses.
That registry exists precisely because neither format is a decorator, so neither
can be dispatched through the decorator table.

### Its file is `panel.ts`, not `index.ts`

`tsconfig` sets `baseUrl` to `./src`, so a directory named `geojson` holding an
`index.ts` makes the bare specifier `geojson` resolve to it rather than to the
npm package of that name. That package is where `FeatureCollection` and
`Feature` come from, and both map modules import them, so the collision broke
the build in two files that have nothing to do with this one. Cursor on Target
can use `index.ts` because no package is called `cot`.

### "As posted" is the panel's, not the card's

The card shipped carrying its own copy of the source in a `Disclosure`, and the
panel carried a second one. Two things were wrong with that. The raw document is
the longest thing the payload holds, so the card put it in the channel under
everything else somebody had written, where nobody had asked for it; and
`Open details` is one click away and is where a reader who wants the source is
already going. The card now points at the panel, and `DETAIL_FAILED` and the
dropped-properties notice point there with it rather than at a row that is no
longer under them.

Cursor on Target never had a card copy, which is the shape this now matches.

### The two panels' rows are the same row

This one shipped wrapped in `styles.section`, the group-heading style, so
"As posted" rendered as uppercase micro-text beside a Cursor on Target panel
whose identical section rendered as a bordered control. That is exactly the
defect [`cot.md`](cot.md) records under "A disclosure has to look like a control,
not a heading", reintroduced one directory over, and it is why the shared
component was not enough on its own: a caller can still wrap it in a heading.

Matching means the wrapper goes, and the row gains the `CopyButton` and the
`role='region'`/`tabIndex` pane that the Cursor on Target one has. A test asserts
the summary's `text-transform` rather than a screenshot, because uppercase is the
specific thing that went wrong.

### Sections, and why the list is its own

`server/geojson/sections.go` is the catalog, and the ids reach the KV store, so
the rule is the one `cot/sections.go` records: add and retire freely, rename
never. `TestWebappGeoJSONSectionCatalogMatches` holds the TypeScript half to the
same ids in the same order.

The stored list is the HIDDEN sections, for the reason the location rows are:
empty means all of them, so a reader who never chose is stored as nothing, and a
section added later appears for everybody rather than being invisible to exactly
the readers who cared enough to choose.

The preferences blob gets a `geojson` key of its own rather than a field on
`cot`. The two panels have different sections, and one shared list would mean
hiding "Map" in one panel hid it in the other. A test asserts a Cursor on Target
section id is refused here.

### Point rows link, and only point rows

`addLocationLink` writes `format` and `value` for a feature that is a lone
`Point` whose position the location grammar will stand behind. Everything else
gets neither key and renders as text.

A polygon has no one position and a `MultiPoint` has several, so linking either
would be picking one and calling it the feature's. A coarsely written coordinate
is refused by `location.Parse` itself, which is the same refusal `cot.go`'s
`coarseNote` records: `-118.25` does not carry enough digits for the grammar to
stand behind.

Both keys are **optional**. A post stamped before this pair existed carries
neither, and `isLinkable` is what keeps those rows as text rather than as a link
that would land nowhere. The pair is the identity and nothing derived travels
with it, which is the contract every decorator link carries.

## Preferences

The key inside the preferences blob is `geojson`, and its wire name is checked
by name in `preferences_test.go` alongside every other: renaming one silently
discards everybody's saved settings on upgrade, and nothing else in either
language would notice.

## The shape guard reads raw records only

`TestWebappGeoJSONShapeMatches` credits a wire key as read only when it appears
against a parameter named `rawBlob`, `rawFeature` or `rawPart`.

That naming is not decoration. The guard first matched any `feature.<key>`, and
`isLinkable` contains `feature.format` on the **decoded** value, which satisfied
the guard while the reader had stopped reading `format` from the wire at all: a
mutation proved the guard passed with the read deleted. Naming the raw records
apart is what lets the guard tell a wire read from an ordinary property access,
and the tightened version immediately found a second key it had not been
crediting.

## The map

### Points are markers, shapes are geometries

`geometry-fill` and `geometry-outline` are a fill layer and a line layer, and
neither renders a point. The only point layer is `pin`. So a GeoJSON `Point`
goes through `markers` and everything else through `geometries`.

`LocationMap` says "Vertices are deliberately not markers", and this does not
contradict it: a GeoJSON `Point` is a position somebody reported, which is what
a marker means. A polygon's corners still are not.

A `GeometryCollection` contributes each part to whichever channel that part's
own kind selects, so one feature can appear in both.

Marker **images** stay registered inside `applyView` where they already are.
Moving them to style-build time would reintroduce the defect recorded there: a
color the first event did not carry had no image, and the symbol layer drew
nothing for it. What IS fixed at style-build time is the pin layer's TYPE,
chosen from `hasMarkers`, and `LocationMap` never calls `setStyle`.

### The plural prop carries rings

`geometries?: readonly MapShape[]`, where a `MapShape` is rings plus a closed
flag. Rings, not a flat point list, and that is the whole point: the `outline`
variant this replaced was flat, and its `outlineFeature` emitted `coordinates: [ring]`, a
single-ring polygon. Passing a holed polygon as two outlines paints the hole as
a solid island on the fill layer, which would flatten away exactly what the
parts-rings props shape exists to preserve.

`shapesFeature` in `maplibre.ts` replaced `outlineFeature` outright, and emits one `Polygon` with every ring for a closed shape, or a
`LineString` per ring for an open one.

The `ellipse` variant is not admitted into the plural prop: it is drawn around
the primary position, so a plural array would stack every one on a single
anchor.

`MapShape` carries **its own color**, which is what made `paintGeometry` a
`coalesce` over a feature property. What that cost, and what replaced the gate
the scalar used to be, is [above](#why-the-paint-is-data-driven-and-what-that-cost).

### Extent-only

Derived inside `useMapInstance`, not passed: `lat === null` **and** something to
frame. It was an explicit prop first, and the argument the prop was protecting
survives as the second half of that condition, because a null with NOTHING to
draw must still read as unavailable. See `mapping.md`, "One overlay path".
The original argument, which still holds: null already means "no
mappable position" to every existing caller and renders NO_POSITION over the
frame; overloading it would regress the location panel's pending and unknown
states. A test asserts null still means what it always meant.

The third state runs through `known` (which gates map creation), `applyView`'s
clearing bail, the opening camera, `positionNote`, `drawableMarkers`' pin
fallback, `label()` and the creation effect's dependencies.

Two things that looked like sites are not. The Reset button and the zoom readout
are gated on `note === null` alone, so they need no edit once `positionNote`
returns null for this case.

One that was easy to miss: the creation effect's deps are the **retry** for a
transient basemap failure. In extent-only mode `lat`, `lon` and `known` are
constant forever, so without `overlayKey` in that list a failed creation could
never be retried. `overlayKey` is declared ahead of both effects for that
reason.

`applyView` branches rather than rewriting four signatures. The positioned path
is untouched, including the narrowing that lets `overlayBounds`, `drawableMarkers`,
`drawableAccuracy` and `drawableGeometry` take plain numbers; the extent-only
path is its own short branch that draws the overlay, frames it, and draws no
pin, no cell and no accuracy ring, because all three are about a position this
surface does not have.

### The antimeridian, across all shapes

`unwrapLongitudes` used to run per shape, inside the bounds helper and inside
`outlineFeature`, with `unionOf` taking a raw min and max afterwards. That is
safe only while at most one shape is drawn, which is why Cursor on Target never
hit it: two features at 179..180 and -179..-178 each unwrap to themselves, union
to a 359 degree box, and slip under `spansTheWorld`'s 360 test, so the camera
frames the planet the wrong way round with both features at the edges.

So `overlayBounds` collects every marker and every ring of every shape, unwraps
them as ONE sequence, and takes the extremes of that. `shapesFeature` unwraps
across every ring of every shape for the same reason: unwrapping each
independently lets two shapes either side of the seam land in different world
copies.

Fixtures cover both the two-shape case and the single-shape case the old
per-shape code already handled, so the move does not regress it.

### Degenerate extents

A single point, a due-north line and a zero-area polygon each produce a box with
no width or no height. `fitBounds` takes those to `maxZoom`, which is a
street-level view of something that may be a country wide, so `degenerate`
detects them and the camera falls back to `zoomForSpan` around the box's center,
which is the answer the single-position camera has always given.

`frameBounds` returns null when there is nothing to frame beyond the position
itself, which is what keeps every ordinary one-coordinate surface on the camera
it has always used.

## Not implemented

- Documents larger than the source cap. A document over `MaxSourceBytes`, or
  past any of [the caps](#the-caps), is refused whole rather than truncated: a
  partly drawn overlay is a wrong overlay, and nothing distinguishes it from a
  complete one once it is stamped.
- A stamped post is never re-read. There is no `MessageWillBeUpdated` hook, so
  editing a document's text does not redraw it, exactly as for a decorator.

## The visible message beats an attachment, but only if it is really a document

Format order is Cursor on Target then GeoJSON, and each format asks its own
fence, bare element and attachment questions in that order. That leaves one
collision: a post carrying a `.cot` attachment AND a visible GeoJSON document
would stamp as Cursor on Target from the file, and the reader would get a card
describing a document their author never showed them. `cotFileSource` refuses
the attachment in that case, which keeps "the visible message always wins over
an attachment" true across formats without restructuring the hook.

`messageShowsGeoJSON` is that test, and it has been wrong three times, each in a
different direction.

It began as a fixed test for one spelling. That ignored the switch, so with
`EnableGeoJSON` off a `geojson` fence still suppressed a perfectly good `.cot`
attachment and the reader got no card at all; and it knew only the `geojson`
spelling, so with `EnableGeoJSONUnlabeled` on a visible `json` fence or a bare
document lost to the attachment it was supposed to beat. Deferring to
`geoJSONSource` fixed both, and keeps the two answers the same by construction.

It then answered "is there a GeoJSON-shaped source here", which is not the same
question as "will GeoJSON draw a card". With the unlabeled switch on,
`SoleObjectSpan` matches any brace pair, so `{5}`, `set it to {5}` and
`{"a":1}` each classified as a source, suppressed the attachment, and then
failed to parse. The post got nothing at all: no GeoJSON card, and not the
Cursor on Target card its attachment had earned. An ambiguous source therefore
suppresses nothing unless `geojson.Parse` succeeds, which is the same gate the
stamp itself applies.

A fence the author LABELED `geojson` still suppresses whether or not it parses,
which is why the test is `labeledGeoJSONFence` and not the parse alone. That
label is the author saying they meant a document, and it is already what earns
them the `TF-11010` notice and the warn. Lifting suppression there would hand
them a Cursor on Target card and say nothing about the fence they got wrong,
which is the one case where silence is not the kind answer.

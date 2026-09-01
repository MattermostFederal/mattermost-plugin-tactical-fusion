# GeoJSON rendering

## Overview

Render a GeoJSON document, posted as a fenced ```geojson block or as the sole
`.geojson` file attached to a post, as a custom post type: a card carrying what
the document holds (how many features, of what geometry, named where they name
themselves, with the properties each one carries) and every feature drawn
together on one map fitted to their combined extent.

The shape of the feature is the Cursor on Target card: recognize on the post
path, parse once in Go, write the parsed result into post props, and let the
webapp render props and never the message.

## Problem statement

`CLAUDE.md` names geospatial data as the first thing this plugin exists to
enrich, and GeoJSON is the interchange format for it. Every GIS export, every
mapping API, every ATAK and QGIS round trip, and every OpenStreetMap extract
emits GeoJSON. An operator who pastes one into a channel today sees a wall of
coordinate arrays: nobody reads a polygon out of
`[[[-118.25,34.05],[-118.24,34.06],...]]`, and nobody can tell from the text
whether the second ring is a hole or a second island.

The plugin already draws exactly this shape. `LocationMap` builds a
`FeatureCollection` and hands it to MapLibre; `build/mapdata/` reads Natural
Earth GeoJSON to generate the committed country polygons. What is missing is
recognition on the post path, a bounded parse, and a map that can draw more
than one shape at a time.

## Current state

`MessageWillBePosted` -> `decoratePost` tries `cotStamp` first and decoration
second. `cotStamp` declares its recover **first**, then strips a forged type,
gates on the switch, finds a source (fence, bare element, then sole
attachment), parses, and commits type and props together on a clone after
measuring the whole props map against `PostPropsMaxUserRunes`.

`webapp/src/panels.ts` is already a format-agnostic panel registry, built for
Cursor on Target precisely because CoT is not a decorator. `LocationMap`
already unions a marker spread with a shape's bounds and calls `fitBounds`.
`preferences.go` already stores a per-reader hidden-section list keyed per
feature.

### Current gaps

1. **`LocationMap` draws exactly one shape, and that shape has one ring.**
   `MapGeometry`'s `outline` variant is a flat point list
   (`LocationMap.tsx:797`), and `outlineFeature` emits
   `coordinates: [ring]` (`maplibre.ts:1175`), a single-ring polygon. Two
   outlines on the fill layer draw a hole as a solid island.
2. **The map is built around a primary position.** The bail at
   `LocationMap.tsx:340` is what type-narrows `lat`/`lon` to `number` for
   `shapeBounds` (`:838`), `drawableMarkers` (`:761`), `drawableAccuracy`
   (`:977`) and `drawableGeometry` (`:823`), so it is four signatures rather
   than one guard. `known` (`:286`) drives the creation effect (`:416`), whose
   deps (`:610`) are also the retry for a transient basemap failure. The
   opening camera reads `?? 0` at `:469`, `:474` and `:482-483`.
   `positionNote` (`:715`), `spreadOf`'s `length < 2` (`:931`) and `label()`
   (`:1015`) all assume a position exists.
3. **`geometry-fill` and `geometry-outline` are the only layers on the geometry
   source**, and they are `fill` and `line`. Neither renders a point. The only
   point layer is `pin`, and **its layer type** is decided once at style build
   from `hasMarkers` (`maplibre.ts:316`, `:598`); there is no `setStyle` in
   `LocationMap.tsx`. Marker **images**, by contrast, are registered inside
   `applyView` (`:328`) on purpose, per the defect recorded at `:324-327`.
4. **No JSON document parsing on the post path.** `cot/parse.go` is an XML
   token walk.
5. **`cotStamp` is called from production and from nine tests.**
   `command_cot_example.go:156` builds a CoT card by calling it directly;
   `hooks_cot_test.go` calls it eight times and `command_cot_example_test.go`
   once.
6. **`cotStamp`'s recover spans everything that can panic.** It is declared at
   `hooks_cot.go:28`, before `cotSource` (`:54`), the filestore call and
   `cot.Parse` (`:59`). `decoratePost` calls `cotStamp` at `hooks.go:78`,
   outside `decorateMessage`'s recover (`:103`), so there is no other recover
   on the hook path. `hooks_test.go:125-128` records `panicOnFileInfo` as "the
   injection point for the Cursor on Target recover. Nothing else on that path
   can be made to panic from a test", and two shipped tests depend on it.
7. **`cotFileSource` is the security boundary for reading attachments**, and it
   gates on `p.cotFilesEnabled()` **before** any API call (`hooks_cot.go:168`)
   and checks suffix and size before `GetFile` (`:187-195`).
   `hooks_test.go:130-132` exists so a test can assert a refusal happened
   before the store was asked.
8. **`cotSource` orders fence, then bare element, then attachment**, with the
   recorded reason "the visible message always wins over an attachment"
   (`hooks_cot.go:146`), and a source that is found but refuses to parse ends
   the attempt rather than falling through (`:68`).
9. **The plugin stamps a third post type.** `stampStandalonePost`
   (`hooks.go:174`) writes `custom_tf_location`, and the forged-type strip
   deliberately does not touch it.
10. **`paintGeometry` is the color validation choke point.** `HEX_COLOR` and
    `fillOf` (`LocationMap.tsx:801`) keep an author string out of a paint
    property, and `LocationMap.pw.tsx:1125` proves it by reading the paint
    property back as a string.

## Phase strategy

| Phase | Focus | Value |
|---|---|---|
| **Phase 1a** | Recognition, the bounded parse, the props blob, and the card: feature list, properties, no map | Correctness |
| **Phase 1b** | The multi-shape ringed map layer and the extent-only camera | **The picture** |
| **Phase 2** | Sidebar panel with hideable sections, richer property tables, point rows linking into the location tools | Depth |
| **Phase 3** | simplestyle-spec colors behind a Go validator and a re-sited webapp validator; ambiguous `json` fences and `.json` attachments behind their own switch; a bare unfenced document; measured length and area | Polish |
| **Phase 4** | Documents too large for props, served from a route that re-reads the file; GeoJSON inside a TAK data package; live feeds | Deferred |

Phases 1a and 1b are this plan and ship together, separated so the post hook
and every fallback path are green before any map work starts.

## Design principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Recognition | Unambiguous spellings only: a `geojson` fence and a `.geojson` file | A `json` fence or a `.json` attachment, where a false positive silently and permanently costs an ordinary post its search matches | `CLAUDE.md`, "Setting `Post.Type` costs the post its Elasticsearch/OpenSearch matches" |
| Format order | **Format-major**, CoT then GeoJSON, unchanged from today. The one collision is closed by a guard inside `cotFileSource` | Source-major sequencing, which changes what a refused source does and is a CoT behavior change | `hooks_cot.go:68`, `:146` |
| The recover | **Each format's stamper declares its own recover first**, spanning strip, source-finding, the filestore call, the parse and the commit | A recover inside a commit helper, which leaves `GetFileInfo` outside every recover on the hook path | `hooks_cot.go:28`, `hooks_test.go:125` |
| What is shared | Two helpers: the strip and the measure-and-commit loop. The ownership check moves house and keeps its shape | A format interface, a registry, a dispatcher, or a new result struct | `CLAUDE.md`, "Avoid abstractions the code that exists today does not need" |
| Parsing | Go only. The webapp reads what Go produced and never `post.message` | A second GeoJSON reader in TypeScript | `decorators/types.ts` |
| Coordinate order | `[longitude, latitude]`, per RFC 7946 section 3.1.1, asserted by a test named for it | Any place where the pair is passed positionally without the order in the name | `TestGEOREFIsLongitudeFirst` |
| Coordinate resolution | Keep the lexeme, bounded by length, then `ParseFloat` with its error handled, then finiteness, then range | Porting CoT's `decimalShape`, which guards an XML attribute and rejects legal JSON | Below, "Why `decimalShape` is not reused" |
| Repeated keys | First-wins, the rule the shipped parser already chose | A refusal, which turns a weird but harmless document into a post that does not render | `cot/parse.go:479-484` |
| Why a feature is not drawn | A note string authored in Go, as the shipped format does | A closed cross-language vocabulary nothing branches on yet | `GeometryUnusableNote`, `cot.go:435` |
| Geometry topology | One uniform parts-rings-positions shape, with a stated cardinality per type, preserved **all the way onto the map** | A ring-less map prop that flattens what the props shape exists to preserve | RFC 7946 sections 3.1.6, 3.1.8 |
| Over-budget documents | Refuse the whole document and leave it as text | Truncating to the first N features | `maxCotEvents` |
| `src` | Truncated with a visible marker, never withheld | A rung that drops it. Once `Post.Type` is set, props are the only reachable copy | `cot.go:96-103` |
| The whole overlay draws together | Every feature on one map. They are one drawing, not competing tracks | Borrowing CoT's `soleOutline` rule, which exists because separate events are separate claims | `CotMap.soleOutline` |
| No primary position | An explicit `extentOnly` prop threaded through every site that assumes one | Overloading `lat === null`, or passing a computed centroid | `LocationMap`, "no pin is ever drawn at a guessed position" |
| Author-supplied color | Phase 3, and `paintGeometry` stays scalar until then | Replacing the one function that validates color with an expression that does not | `HEX_COLOR`, `fillOf` |
| The post path | Cheap gates first, size caps, its own recover, an atomic commit | Anything that can stop somebody posting | `CLAUDE.md` invariants |

## Requirements

- [ ] A fenced ```geojson block that is the sole fenced block of a message is
      recognized, parsed and stamped.
- [ ] A post carrying exactly one attachment named `.geojson` is recognized,
      subject to the same ownership check Cursor on Target uses and to its own
      switch.
- [ ] `Point`, `MultiPoint`, `LineString`, `MultiLineString`, `Polygon`,
      `MultiPolygon`, `GeometryCollection`, `Feature` and `FeatureCollection`
      are all read, including a `Feature` whose `geometry` is `null`, each with
      a stated parts and rings cardinality, and none losing part or ring
      membership.
- [ ] Coordinates are read as `[longitude, latitude]` and each retains the
      digits the document wrote, including exponent notation.
- [ ] A polygon's first ring is its exterior and the rest are holes, and that
      survives into props **and onto the map**.
- [ ] The card states how many features the document holds, of which geometry
      kinds, lists each by the name it gives itself, and shows the properties
      each feature carries.
- [ ] Every drawable feature is drawn on one map, fitted to their combined
      extent, with no pin at any position the document did not state.
- [ ] The document as posted is always available under a disclosure, truncated
      with a visible marker rather than withheld.
- [ ] A document that is **recognized and then refused** leaves the post as
      ordinary text, and an author who wrote an explicit ```geojson fence is
      told why. A document that is never recognized is silent.
- [ ] Nothing on this path can stop somebody posting, including a panic inside
      `GetFileInfo`.
- [ ] Every Cursor on Target behavior is unchanged.
- [ ] Two admin switches, defaulting on, in their own settings section.
- [ ] Every failure and every log call carries a `TF-NNNN`.

## Out of scope

- The sidebar panel, hideable sections and reader preferences (Phase 2).
- A `json` fence, a `.json` attachment, and a bare unfenced document (Phase 3,
  behind their own switch). A false positive permanently and silently costs an
  ordinary post its search matches, its embeds and its attachment list, and the
  author cannot undo it. `geojson` costs an author nothing.
- simplestyle-spec colors and any style the document states (Phase 3).
- A standalone `/decorate/geojson` page. There is no link and no query string.
- Length and area measurement (Phase 3).
- Documents larger than the source cap (Phase 4).
- TopoJSON, GML, KML, shapefiles, and GeoJSON Text Sequences (RFC 8142).
- Editing. There is no `MessageWillBeUpdated` and a test asserts it stays
  absent.

## Technical approach

### What is shared, and what is not

The reason to share is **not** a saved filestore read. `cotFileSource` checks
its suffix and size before `GetFile` and the suffix sets are disjoint, so a
second stamper costs one extra `GetFileInfo` and no extra content read. The
real reasons are the ownership check and the strip.

**The recover is not shared.** `cotStamp` and `geoJSONStamp` each declare their
own `defer func(){ recover() }` as their first statement, exactly as
`hooks_cot.go:28` does today, so each spans its own strip, source-finding,
filestore call, parse and commit, and each returns its own `stripped` clone and
logs its own `TF-NNNN`. Putting the recover inside a commit helper would leave
`GetFileInfo` outside every recover on the hook path, would break
`TestCotRecoversFromARealPanic` and
`TestAPanicAfterStrippingStillReturnsTheStrippedPost`, and would give the
recover no access to the stripped post it is contractually required to return.

**There is no new result type.** `hooks.go:78-84` already distinguishes all
three states from the existing pair: `(nil, false)` is nothing happened,
`(clone, false)` is stripped, `(clone, true)` is stamped. Chaining a second
stamper works unchanged, and a struct naming what two return values already
name is an abstraction the code does not need.

Two helpers in `hooks_stamp.go`:

**1. `stripStampedTypes(post) (*model.Post, bool)`**, over an explicit table:

```go
var stampedTypes = []struct{ postType, propsKey string }{
    {cot.PostType, cot.PropsKey},
    {geojson.PostType, geojson.PropsKey},
}
```

`custom_tf_location` is deliberately **not** in it: `stampStandalonePost` writes
that type from decoration, and sweeping it in would change a shipped path with
no requirement behind it. The strip removes **every** key in the table on every
post and strips the type when it matches any row, because removing only the
matching row's key lets an author post a forged sibling blob that `maps.Copy`
then carries into stored props permanently.

**2. `commitStamped(post, rungs, codes) (*model.Post, bool)`**: the `maps.Copy`
plus `json.Marshal` plus `utf8.RuneCountInString` loop, the per-rung warning,
and the atomic `Type` plus `SetProps`. No recover. `codes` carries the calling
format's `TF-NNNN` values.

`attachmentOwnedBy` and `attachmentCreator` move here from `hooks_cot.go`
unchanged. Each format keeps its own file source function, which keeps the
shipped ordering: **its own switch first, before any API call**, then
`GetFileInfo`, then ownership, then suffix and size, then `GetFile`. Gating on
the union of the switches would make an install with both file switches off
call `GetFileInfo` on every one-attachment post.

### Format order, and the one collision

Format-major and unchanged: `cotStamp`, then `geoJSONStamp`, then decoration.
`cotStamp` keeps its signature, its `(stripped, false)` contract, its recover
and its internal fence-then-element-then-file order, so all nine call sites and
every existing test keep working, and a source that is found but refuses to
parse still ends the attempt rather than falling through to an attachment.

Source-major sequencing was drafted and cut. It cannot coexist with
`cotStamp`'s shape, and it silently changes what a refused source does: today
an unparseable ```cot fence stops there, and under source-major it would fall
through and stamp from a `.cot` attachment instead, which is both a CoT
behavior change and a violation of "the visible message always wins over an
attachment".

The only collision format-major leaves is a post carrying a `.cot` attachment
**and** a visible ```geojson fence, where CoT would stamp from the file the
reader cannot see. That is closed where the gate already is, in
`cotFileSource`: refuse the attachment when the message's sole fenced block is
a recognized `geojson` fence. Three lines and one test, against restructuring
the hook's control flow.

### Recognition

| Source | Accepted when | If it is not recognized | If it is recognized and then refused |
|---|---|---|---|
| ```geojson fence | It is the sole fenced block | Silent | Ephemeral to the author with a `TF-NNNN` |
| Sole attachment `.geojson` | `EnableGeoJSONFile` is on, the file is the poster's own or `nouser`, unattached, live, within the size cap | Silent | Silent, matching `reportCotRefusal`, which answers a labeled fence and never a file |

Nothing else in Phase 1.

### The parse and its budget

`server/geojson/`, a sibling of `cot/` and `decorators/`, stdlib only.
`json.Decoder` with `UseNumber()`, walking tokens.

**A token walk has no depth limit.** `encoding/json`'s `maxNestingDepth` of
10000 lives in the value scanner, which `Token()` bypasses for `[` and `{`.
Measured on Go 1.26.7: 30000 nested `[` walks clean through `Token()` and is
refused by `Unmarshal`. The `Decoder`'s token stack and a recursive walker's
own stack both grow unbounded, so the walker counts its own depth against
`MaxJSONDepth`, and the fixtures for it nest inside `coordinates` and inside
`properties`, not only inside `GeometryCollection`.

**Repeated keys are first-wins**, at every level including inside
`properties`, matching `cot/parse.go:479-484`: "First-wins matches attrValue...
One rule now." Refusing them was drafted and cut: nothing in this system calls
`Unmarshal` on the document, so there is no last-wins reader inside the plugin
to disagree with, and a sentinel plus an ephemeral plus a help row plus a test
class is a large price for a document that is weird rather than harmful. The
rule goes in the design note.

Unknown members are skipped: RFC 7946 section 6.1 permits foreign members.

| Cap | Value | Why |
|---|---|---|
| `MaxSourceBytes` | 64 KiB | The same number and reason as Cursor on Target: props ship to every client on every channel load |
| `MaxJSONDepth` | 32 | The stdlib does not supply one on this path. Matches `maxCotDepth` |
| `MaxFeatures` | 256 | An operational overlay. Past this it belongs in a file |
| `MaxVertices` | 4096, **derived by Task 3** | See below |
| `MaxCollectionDepth` | 1 `GeometryCollection` ancestor | RFC 7946 section 3.1.8 discourages nesting. One is legal and read; one inside another is refused |
| `MaxProperties` | 32 keys per feature, key 64 runes, rendered value 256 runes | The card renders them |
| `MaxNameRunes` | 128 | Matches `maxFieldRunes` |
| `MaxCoordRunes` | 32 | The lexeme is stored verbatim, so it is the one field `sanitize` does not bound |

No `MaxRings`: 33 rings of four vertices is 132 vertices, so it refuses nothing
`MaxVertices` and the byte cap do not already refuse.

**`MaxVertices` must sit above what the byte cap allows, not below it.** At
realistic precision 64 KiB holds roughly 2700 positions, so a cap of 2048 would
bind first and refuse a legitimate 2500-vertex, 60 KiB QGIS export that is
inside every other limit. 4096 is above that ceiling, so the byte cap binds for
realistic documents and `MaxVertices` catches only the low-precision case
(`[1,2]` is six bytes, so 64 KiB can hold far more). Task 3 solves for the
largest vertex count whose worst-case encoded props fit the budget **with every
other cap at its maximum**, and keeps the measurement as a test. Stating it as
"a document sitting on every cap at once" was circular; this is the
non-circular form.

Two differences from CoT's `MaxVertices` of 512 go in the design note so nobody
"fixes" the divergence: CoT's is **per shape** where this is document-wide, and
CoT's **truncates and records `Seen`** (`cot/geometry.go:65`) where this
**refuses**.

**Sentinel errors**, which refuse the whole document: `ErrTooLarge`,
`ErrNotUTF8`, `ErrNotGeoJSON`, `ErrUnknownType`, `ErrTooManyFeatures`,
`ErrTooManyVertices`, `ErrTooDeep`, `ErrNestedCollection`, `ErrTrailing`.

**Notes**, authored in Go and carried on the parsed value: a malformed or
out-of-range position, an unclosed or too-short ring, a foreign `crs`, a
malformed `bbox`, an unlocated feature. Nothing appears in both lists.

Notes are **sentences, not codes**, matching `GeometryTooLargeNote` and
`GeometryUnusableNote` (`cot/geometry.go:21`) and `position_note`
(`cot.go:435`). A closed cross-language `reason` vocabulary was drafted on the
belief that `cot.Geometry.Undrawable` was machine-readable; it is a sentence.
Nothing branches on the note in Phase 1, so a Go-authored string costs no
cross-language surface and adds no sync row. The closed vocabulary is kept for
`kind`, which the card and the map both dispatch on.

**Why `decimalShape` is not reused.** `cot.go:414` guards an XML attribute,
which is an arbitrary string, and its stated reasons are that `ParseFloat`
accepts hex floats and exponent notation, and that a lat of sixty thousand
leading zeros can be parsed, ranged and rendered. A GeoJSON coordinate arrives
through JSON's number grammar, which admits neither hex floats nor a leading
`+`, so half that rationale is unreachable here. What the regex would do on
this path is **reject legal GeoJSON**: `1e-05` is what `json.dumps` writes for
`0.00001`, and more than fifteen decimals is exactly where `ogr2ogr` sits at
`COORDINATE_PRECISION=15`. Both would land on this plan's own manual test as a
card that lists features and refuses to draw them.

So the check is `MaxCoordRunes`, then `ParseFloat` with its error handled, then
`math.IsInf`/`math.IsNaN`, then the range test. That keeps the sixty-thousand-
zeros defect closed and accepts every number RFC 7946 permits. The design note
records why the divergence is deliberate.

**Positions.** `[lon, lat]` or `[lon, lat, alt]`, longitude first, validated to
`|lon| <= 180` and `|lat| <= 90`. A position failing any check gives its
feature a note and excludes it from the extent; it does not refuse the
document. Unwrapped longitudes are a rendering artifact and are never stored.

**Unlocated features.** RFC 7946 section 3.2 makes `"geometry": null` legal.
Such a feature carries `kind: "none"`, an explicit member of the closed
vocabulary, and a note.

**Polygons.** Ring 0 is the exterior, the rest are holes (section 3.1.6). A
ring must be closed and hold at least four positions to be drawable.
Winding order is advisory, neither checked nor normalized.

**`crs`.** RFC 7946 removed it. A document is drawn only when `crs` is absent
or names `urn:ogc:def:crs:OGC:1.3:CRS84`. Anything else, **including
`EPSG:4326`**, is read and listed but not drawn, because many producers write
`EPSG:4326` latitude-first.

**`bbox`.** Validated for length (4 or 6) and finiteness, then ignored for the
extent. A malformed one gets a note, because it is often the symptom of a
producer writing the axes the wrong way round.

**Empty documents.** `"features": []` is legal. The card says the document
names no features and draws no map.

### The props blob

```
custom_tf_geojson  /  tactical_fusion_geojson  /  version 1
{
  version, source, file_id, file_name, lead, trail, src,
  properties_dropped: "1",           // present only on rung 2
  note,                              // "" or why the document is not drawn
  counts: {features, points, lines, polygons, collections,
           unlocated, undrawable},
  features: [ {
      name,                          // hoisted, so rung 2 keeps it
      kind,                          // the closed vocabulary, below
      note,                          // "" or why this feature is not drawn
      parts: [ { kind, rings: [ [ {lon, lat, alt}, ... ], ... ] } ],
      properties: [{key, value}],    // dropped by rung 2
  } ]
}
```

**Cardinality per type, stated because the sync guard cannot infer it.**
`TestWebappGeoJSONShapeMatches` runs Go and TypeScript over one shared
`Fixture()`, so both sides agree by construction whichever encoding is chosen.
A wrong choice is therefore invisible to the guard and has to be fixed here:

| GeoJSON type | parts | rings per part | positions per ring |
|---|---|---|---|
| `Point` | 1 | 1 | 1 |
| `MultiPoint` | **N** | 1 | 1 |
| `LineString` | 1 | 1 | N |
| `MultiLineString` | **N** | 1 | N |
| `Polygon` | 1 | R (ring 0 exterior) | N |
| `MultiPolygon` | P | R | N |
| `GeometryCollection` | one part per member geometry, each carrying its own `kind` | as above | as above |
| `null` geometry (`kind: "none"`) | 0 | - | - |

`MultiPoint` is N parts rather than one part of N positions so that it is
structurally distinct from `LineString` at the parts level and not only by
`kind`. `MultiLineString` is N parts for the reason the shape exists: one part
of N positions would join two disjoint lines into one polyline.

A `MultiPolygon` **nested inside a `GeometryCollection`** is the one case parts
alone cannot express, since flattening it to P parts of `kind: Polygon` loses
the multipolygon grouping. It is therefore represented as **one part carrying
`kind: MultiPolygon`**, whose `rings` are every ring of every member polygon in
order, with a `ring_counts` array on that part naming how many rings each
member contributes. That is the only place the shape needs a fourth number, and
it is carried rather than inferred.

`kind` is a **closed vocabulary**: the seven GeoJSON geometry types plus
`"none"`. `TestWebappGeoJSONKindsMatch` holds both sides to it.

`name` is **hoisted out of the properties bag**, because rung 2 drops
`properties`. Precedence, fixed and tested: `properties.name`,
`properties.title`, `properties.label`, the Feature's `id`, then `Feature N`.

**Property values are rendered to strings in Go.** GeoJSON permits string,
number, boolean, null, array and object. A string is sanitized; a number is its
lexeme; a boolean is `true`/`false`; a null is omitted along with its key; an
array or object is `json.Marshal`ed and then truncated to the 256-rune cap with
the same visible marker `src` uses. Deciding this in Go rather than the webapp
is what keeps the rune measurement honest and the XSS test meaningful.

`properties_dropped` is a presence key, mirroring CoT's `detail_dropped`
(`cot.go:141-146`): an absent `properties` array is otherwise indistinguishable
from a feature that genuinely has none. A Go test asserts rung 1 lacks it.

`src` is always present, truncated with a visible marker rather than withheld,
per `cot.go:96-103`.

### The props ladder

Two rungs: everything, then without `properties`. Below that the document is
refused. A third rung dropping `src` was cut for the reason `cot.go:96-103`
records.

### The card

Heading, summary, map, feature list, disclosure.

- Heading: "GeoJSON: 12 features".
- Summary: the geometry mix, the unlocated and undrawable counts when non-zero,
  the document-level note when there is one, and "properties omitted to fit"
  when `properties_dropped` is set.
- Map (Phase 1b).
- Feature list: name, geometry kind, and for a single `Point` the coordinate
  rendered through the location decorator's `FormatDD` as `cot.go:452` does, so
  it can link into the location tools in Phase 2. Lines and polygons state
  their vertex and ring counts. **Each feature shows the properties it
  carries**, as a plain key/value list, which is what makes the `properties`
  payload load-bearing in Phase 1 rather than data nothing displays. Phase 2's
  panel gives them a richer table; this is their first appearance, not their
  only one. The list scrolls inside a `max-height` container.
- Disclosure: the document as posted.

Every author-controlled string is a React text node. No
`dangerouslySetInnerHTML`, and a Playwright case mounts a property key and
value containing markup and asserts it renders as text.

`fromProps` refuses a version outside `READABLE_VERSIONS`, and an unreadable
version joins the `Fallback` list beside props missing, `edit_at !== 0`, and a
`file_id` no longer in `file_ids`. GeoJSON declares its own
`READABLE_VERSIONS` rather than assuming CoT's module-private symbol
(`webapp/src/cot/types.ts:22`) is an exported pattern. The fallback may never
render nothing.

### The map

**Points are markers; lines and polygons are geometries.** `geometry-fill` and
`geometry-outline` render no point. Points go through `markers`, which draws
through the `pin` source. Because the **pin layer's type** is chosen once at
style build from `hasMarkers` (`maplibre.ts:316`, `:598`) and `LocationMap`
never calls `setStyle`, a GeoJSON map must pass a non-empty `markers` array on
its first render whenever the document has any point at all. Marker **images**
stay registered inside `applyView` (`LocationMap.tsx:328`) where they are
today: moving them to style-build time would reintroduce the defect recorded at
`:324-327`, where a color the first event did not carry had no image and the
symbol layer drew nothing for it.

`LocationMap.tsx:173` says "Vertices are deliberately not markers", and this
does not contradict it: a GeoJSON `Point` is a reported position, which is what
a marker means. A polygon's corners still are not.

**N shapes, each with rings.** `geometry?: MapGeometry` gains a sibling
`geometries?: ReadonlyArray<MapShape>`, where

```ts
type MapShape = {rings: ReadonlyArray<ReadonlyArray<{lat: number; lon: number}>>; closed: boolean};
```

Rings, not a flat point list. The `outline` variant and `outlineFeature` emit
`coordinates: [ring]` (`maplibre.ts:1175`), a single-ring polygon, so passing a
holed polygon as two outlines paints the hole as a solid island on the fill
layer. That would flatten away exactly what the parts-rings props shape exists
to preserve, and it would contradict a requirement, an acceptance criterion and
two table rows. A plural builder beside `outlineFeature` emits one `Polygon`
per shape with `coordinates: rings`, or a `LineString` when `closed` is false.
`shapeBounds` iterates rings.

The `ellipse` variant is not admitted: it is drawn around the primary
`lat`/`lon`, so a plural array would stack every one on a single anchor. The
singular `geometry` keeps it and every existing call site unchanged.

`MapShape` carries **no color field**, and `paintGeometry` is **unchanged** in
Phase 1. An expression would paint CoT's stated color opaque where `fillOf`
composites it at alpha 0.16, and would delete the `HEX_COLOR` gate while making
`LocationMap.pw.tsx:1125` pass vacuously, since that test reads the paint
property back as a string.

**A `GeometryCollection` feature** contributes each of its parts to whichever
channel that part's own `kind` selects: `Point`/`MultiPoint` parts become
markers, the rest become shapes. One feature can therefore appear in both.

**Extent-only.** An explicit `extentOnly?: boolean`. Null `lat`/`lon` already
means "no mappable position" and renders `NO_POSITION`; overloading it would
regress the location panel's pending and unknown states.

The sites, corrected. Two things previously listed are **consequences, not
sites**: the Reset button (`:664`) and the zoom readout (`:677`) are gated on
`note === null` alone, so they need no edit once `positionNote` returns null.
Four were **missing**:

| Site | What it needs |
|---|---|
| `known` (`:286`) | The third state, and it drives the creation guard at `:416` |
| `applyView`'s clearing bail (`:340`) | Made conditional. It is what **type-narrows** `lat`/`lon` to `number`, so `shapeBounds` (`:838`), `drawableMarkers` (`:761`), `drawableAccuracy` (`:977`) and `drawableGeometry` (`:823`) all change signature |
| The opening camera | `?? 0` at `:469`, `:482` and `:483`, plus `coveredBy(details, opening)` at `:474`, which decides whether the detail tier loads at all |
| `positionNote` (`:715`) | Return null for the extent-only case |
| `spreadOf` (`:931`) | Its `length < 2` rule |
| `drawableMarkers` (`:761`) | Its pin fallback |
| `label()` (`:1015`) | Every branch says "position marked". Extent-only says what the overlay **is**, in the shape `CotMap.blockLabel` established: "World map with 12 features drawn." A map whose only accessible channel is words must not claim a position it never drew |
| The creation effect's deps (`:610`) | `[known, lat, lon, applyView]` is the **retry** for a transient basemap failure (`:405-410`). In extent-only mode all three are constant forever, so the retry becomes a permanent latch unless a fourth dep changes with the overlay |
| `overlayDigest` (`:782`) | Must key on `geometries` or the camera never re-fits |

**Antimeridian, across all shapes.** `unwrapLongitudes` runs **inside**
`outlineFeature` (`maplibre.ts:1162`) and inside `shapeBounds`, per shape, and
`unionOf` then takes raw min and max. Two features at 179..180 and -179..-178
union to a 359 degree box, and `spansTheWorld` tests `>= 360`, so it is missed
and the camera frames the planet backwards. The existing code is safe only
because CoT draws at most one outline. Unwrapping moves to run across every
shape and every marker together before the union, and a fixture covers **both**
the multi-feature case and the single-feature case that today's per-shape code
already handles, so the move does not regress it.

**Degenerate extents.** A single point, a due-north line and a zero-area
polygon each produce a zero-width or zero-height box. `spreadOf`'s null path
plus `zoomForSpan`, and `fitBounds`' existing `maxZoom: MAX_ZOOM`, are the
right answer; a fixture covers each.

**Switches.** The map follows the location decorator's switches on both
surfaces, and `TestGeoJSONHasNoMapSettingOfItsOwn` holds that decision the way
`TestCotHasNoMapSettingOfItsOwn` holds CoT's.

### Switches and preferences

`EnableGeoJSON` and `EnableGeoJSONFile`, both defaulting on, in a new `geojson`
section of `plugin.json`, with the CoT help text's warnings about lost search
matches, embeds, attachment lists and translation. `EnableGeoJSON` governs
stamping only; the webapp registers the post type unconditionally.

Phase 2's panel will store hidden sections per reader. The `geojson` key inside
the preferences blob is chosen **now**, in the design note, because those keys
reach the KV store and are renamed never.

### Error codes

| Code | Meaning |
|---|---|
| 11009 | `HooksGeoJSONPanic` |
| 11010 | `HooksGeoJSONUnreadable` |
| 11011 | `HooksGeoJSONPropsTooLarge` |
| 11012 | `HooksGeoJSONPropsUnmeasurable` |
| 11013 | `HooksGeoJSONPropertiesDropped` |

`HooksCotFileUnreadable` (11003) and `HooksCotFileNotOwned` (11006) keep their
**numbers**, because a number in `public/help/error-codes.html` is a contract,
and their **Go identifiers** become `HooksAttachmentUnreadable` and
`HooksAttachmentNotOwned`, because a constant saying `Cot` logged for a GeoJSON
attachment is a call site telling a lie.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Accept a ```json fence or a `.json` attachment? | No, in Phase 1 | A false positive permanently and silently costs an ordinary post its search matches |
| Format-major or source-major? | **Format-major, unchanged** | Source-major cannot coexist with `cotStamp`'s shape and silently changes what a refused source does |
| The `geojson` fence plus `.cot` attachment collision | A guard in `cotFileSource` | Three lines where the gate already is |
| Where does the recover live? | In each format's stamper, declared first | A recover inside a commit helper leaves `GetFileInfo` uncovered and cannot return the stripped post |
| A new `stampResult` struct? | No | `hooks.go:78-84` already encodes all three states in the existing pair |
| The strip table | CoT and GeoJSON only; every key cleared on every post | Sweeping in `custom_tf_location` changes a shipped path; clearing one key leaves a forged sibling blob |
| Attachment gating | Each format's own switch **before** `GetFileInfo` | `hooks_test.go:130` exists to assert a refusal before the store is asked |
| JSON depth | An explicit `MaxJSONDepth` | Measured: `Token()` bypasses the stdlib limit |
| Repeated keys | First-wins, no sentinel | `cot/parse.go:479` already chose this rule |
| Coordinate lexemes | Length cap, `ParseFloat` with its error handled, finiteness, range | `decimalShape` guards an XML attribute and would reject `1e-05` and `ogr2ogr`'s 15 decimals |
| Why a feature is not drawn | A Go-authored sentence | The precedent cited for machine-readable codes is itself a sentence |
| `kind` for a null geometry | `"none"`, in the closed vocabulary | It has no geometry class and the vocabulary is sync-tested |
| Property values | Rendered to strings in Go, nulls omitted, arrays and objects marshaled then truncated | Keeps the rune measurement honest and the XSS test meaningful |
| `properties` in Phase 1 | Carried **and rendered** on the card | Otherwise it is Phase 2 scaffolding, and props are permanent so a later phase could not backfill it |
| Props rungs | Two. `src` is never dropped | `cot.go:96-103` |
| `MaxVertices` | 4096, above the byte cap's ~2700 ceiling, derived by solving with every other cap maxed | 2048 would bind first and refuse a legitimate 60 KiB export |
| The plural map prop | Carries **rings**, no color | A flat point list cannot express a hole, which is what the props shape exists to preserve |
| Marker images | Stay in `applyView` | Moving them to style build reintroduces a recorded defect. The style-build constraint is the layer type |
| Extent-only | An explicit prop through nine sites, two of the earlier ten dropped as consequences and four added | The mitigation is the list being exhaustive |
| Antimeridian | Unwrap across all shapes and markers before the union | Per-shape unwrapping plus a raw union gives a 359 degree box `spansTheWorld` misses |
| Sidebar panel | Phase 2 | Mirrors how CoT shipped |

## Files to modify

| File | Change |
|---|---|
| `server/hooks_stamp.go` | New. `stripStampedTypes`, `commitStamped`, `attachmentOwnedBy`, `attachmentCreator` |
| `server/hooks_cot.go` | Rewired onto the two helpers; keeps its recover, signature and contract. Gains the `geojson` fence guard in `cotFileSource` |
| `server/hooks_geojson.go` | New. `geoJSONStamp`, with its own recover declared first |
| `server/hooks.go` | `decoratePost` chains `geoJSONStamp` after `cotStamp` |
| `server/geojson/parse.go` | New. The bounded decode, `MaxJSONDepth`, first-wins, the sentinels |
| `server/geojson/types.go` | New. `Document`, `Feature`, `Part`, `Ring`, `Position`, the `kind` vocabulary |
| `server/geojson/geojson.go` | New. `Props`, both rungs, the coordinate check, property-value rendering, name precedence, an exhaustive `Fixture()` |
| `server/geojson/*_test.go` | New, including the `MaxVertices` derivation and a cardinality test per type |
| `server/geojson_sync_test.go` | New, beside `server/cot_sync_test.go` |
| `server/configuration.go`, `server/plugin.go` | The two switches and their gates |
| `server/errcode/codes.go` | Five new codes; two renamed identifiers keeping their numbers |
| `plugin.json` | The `geojson` settings section |
| `webapp/src/geojson/types.ts` | New. `fromProps`, its own `READABLE_VERSIONS`, the `kind` vocabulary |
| `webapp/src/geojson/GeoJsonCard.tsx`, `GeoJsonMap.tsx`, `GeoJsonPostBody.tsx` | New, plus harnesses and `.pw.tsx` |
| `webapp/src/index.tsx` | Register the post type unconditionally |
| `webapp/src/decorators/location/map/LocationMap.tsx` | `geometries` as `MapShape`, `extentOnly` through nine sites, cross-shape unwrapping, `overlayDigest` |
| `webapp/src/decorators/location/map/maplibre.ts` | A plural ringed builder beside `outlineFeature` |
| `webapp/src/decorators/location/map/LocationMap.pw.tsx` | Cover both |
| `public/help/geojson.html`, `error-codes.html`, `admin.html`, `help.html` | New page, rows, links |
| `docs/design/geojson.md` | New. All of the rationale above |
| `docs/design/cot.md`, `mapping.md`, `preferences.md` | Point at it |
| `CLAUDE.md` | The new area, its design note, its invariants, its sync rows |

## Tasks

**Phase 1a**

1. [ ] `docs/design/geojson.md`, written first, including why `decimalShape` is
       not reused, why format-major was kept, and the CoT `MaxVertices`
       divergence.
2. [ ] `server/geojson/` types and the bounded parse: `MaxJSONDepth`,
       first-wins, the coordinate check, the sentinel/note split, ring rules,
       `crs`, `bbox`, `"geometry": null`, foreign members, empty collections.
3. [ ] The props blob: the cardinality table implemented and tested per type,
       the nested-`MultiPolygon` `ring_counts` case, the hoisted name,
       property-value rendering, both rungs, `properties_dropped`, an
       exhaustive `Fixture()`. **Derive** `MaxVertices` by solving with every
       other cap maxed; keep the measurement.
4. [ ] `hooks_stamp.go`: the two helpers and the widened strip. Move
       `attachmentOwnedBy`/`attachmentCreator`. Rewire `cotStamp` with its
       recover, signature and contract intact, and its shipped gate ordering
       (switch, then `GetFileInfo`, then ownership, then suffix and size, then
       `GetFile`) unchanged. Existing CoT suite green, including
       `TestCotRecoversFromARealPanic` and
       `TestAPanicAfterStrippingStillReturnsTheStrippedPost`.
5. [ ] `geoJSONStamp` with its own recover, the fence and file rules, the
       ephemeral, and the `cotFileSource` collision guard with a test in each
       direction.
6. [ ] Switches, error codes, `AllCodes`, the two renames.
7. [ ] `webapp/src/geojson/types.ts`, `fromProps`, `READABLE_VERSIONS`, and the
       sync guards.
8. [ ] The card, its properties list, its fallback, its scrolling list, its
       Playwright tests.
9. [ ] Register the post type in `index.tsx`.

**Phase 1b**

10. [ ] `maplibre.ts`: the plural ringed builder emitting one `Polygon` per
        shape with `coordinates: rings`.
11. [ ] `LocationMap`: `geometries` as `MapShape`; `shapeBounds` over rings;
        `overlayDigest` keyed on them; unwrapping moved to run across every
        shape and marker before the union. `paintGeometry` untouched.
12. [ ] `LocationMap`: `extentOnly` through the nine sites tabled above,
        including the four function signatures the `:340` bail type-narrows and
        the creation-effect retry dep.
13. [ ] `GeoJsonMap`: points through `markers` with a non-empty array on first
        render, shapes through `geometries`, `GeometryCollection` parts split
        by their own `kind`, the `markerLabel`, its switches, its tests.

**Both**

14. [ ] `public/help/geojson.html`, error-code rows, admin rows, index links.
15. [ ] `/tactical-fusion examples` gains a GeoJSON example, measured against
        `safePostRunes`.
16. [ ] `CLAUDE.md`: the area, the design note, the invariants, the sync rows.
17. [ ] `make check-style && make test && make sbom-audit`.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| A panic escapes `MessageWillBePosted` and stops somebody posting | Each stamper declares its own recover first, spanning its filestore call and parse. `panicOnFileInfo` still reaches it, and both shipped recover tests stay green |
| The refactor breaks Cursor on Target | Task 4 lands alone with the signature, contract, recover and gate ordering intact and the suite green |
| An ordinary post stamped by mistake | Only `geojson` and `.geojson` are recognized, and a test asserts `json` and `.json` are ignored |
| An invisible attachment beats a visible fence | The `cotFileSource` guard, tested in both directions |
| A refused source falling through to an attachment | Format-major keeps today's behavior: a found-then-refused source ends the attempt |
| A hole drawn as a solid island, or two lines joined into one | Rings preserved from the parse through props to `coordinates: rings`, with holed-polygon and `MultiLineString` fixtures at both ends |
| Go and TypeScript disagreeing about cardinality | The table is stated, and a Go test asserts it per type. The shape guard cannot catch it, which is why the table exists |
| A legitimate QGIS export refused | No `decimalShape`; `MaxVertices` above the byte cap's ceiling; the manual test names both exports |
| A crafted document exhausting memory | `MaxJSONDepth` in the walker; the byte cap ahead of everything |
| A forged sibling props blob surviving | The strip clears every key on every post, with a test |
| A format an admin turned off reading files | Each format's switch before `GetFileInfo` |
| The color validation choke point deleted | `paintGeometry` untouched in Phase 1; `LocationMap.pw.tsx:1125` still reads a scalar |
| A two-feature antimeridian document framing backwards | Cross-shape unwrapping, with fixtures for the multi- and single-feature cases |
| The extent-only map silently never retrying a basemap failure | The creation effect's dep list is one of the nine tabled sites |
| Phase 1b delivering an empty "position unavailable" box | All nine sites tabled, with `label()`'s wording decided |
| A 256-feature card taller than the viewport | The list scrolls inside a `max-height` container |

## UX summary

| Scenario | Behavior |
|---|---|
| A ```geojson fence with 12 features | A card: heading, geometry mix, a map fitted to all 12, a scrolling feature list with each feature's properties, the document under a disclosure |
| A ```json fence, GeoJSON or not | Nothing, in Phase 1 |
| A ```geojson fence that will not parse | An ordinary code block, plus an ephemeral naming the reason and a `TF-NNNN` |
| A `.geojson` attachment that will not parse | An ordinary post, silently, matching `reportCotRefusal` |
| An unparseable ```cot fence beside a `.cot` attachment | Nothing, exactly as today |
| A ```geojson fence beside a `.cot` attachment | The GeoJSON card. The attachment is refused by the guard |
| A document of 900 features | An ordinary code block, plus an ephemeral |
| A `FeatureCollection` with no features | A card saying the document names no features, and no map |
| A polygon-only document | A map fitted to the polygon, no pin anywhere, announced as "World map with 1 feature drawn" |
| A polygon with a hole | The hole is drawn as a hole |
| A `MultiLineString` of two disjoint lines | Two lines, not one |
| A `Feature` with `"geometry": null` | Listed with a note, `kind: "none"`, not in the extent |
| A coordinate written `1e-05` | Drawn |
| A `crs` naming EPSG:4326 | The card, the list, no map, and a line saying the axis order cannot be confirmed |
| A longitude of 200, or of twenty thousand zeros | Every other feature drawn; that one noted |
| A single-point document | Centered at the default span, not zoomed to maximum |
| Two features across the antimeridian | Framed across the seam |
| Rung 2 was used | Features still listed by name, and the card says the properties were omitted to fit |
| Someone edits the post | The card stands down and the document renders as text |
| A bundle too old to read the props version | The same fallback |
| `EnableGeoJSON` off | New posts are ordinary text; existing cards keep rendering |

## Testing plan

**Go unit.** Every sentinel and every note, kept apart. `MaxJSONDepth` nesting
inside `coordinates` and inside `properties`. First-wins on repeated keys at
both levels. The coordinate check against `1e-05`, sixteen decimals, twenty
thousand zeros, `1e99999`, `-0`, and a hex float that JSON cannot deliver.
Longitude order. **A cardinality test per type**, including the nested
`MultiPolygon` and its `ring_counts`. Ring closure and minimum length. Nested
collections refused, a single one read. `crs` absent, CRS84 and EPSG:4326.
Malformed `bbox`. Foreign members. An empty `FeatureCollection`. Property
values of each JSON type. Name precedence per fallback level. Both rungs
measured, the `MaxVertices` derivation, and rung 1 lacking
`properties_dropped`.

Recognition: a `geojson` fence; a `.geojson` attachment; one not the poster's;
one already bound to a post; one over the cap. A `json` fence and a `.json`
attachment both ignored, asserted.

The stamp path: `TestCotRecoversFromARealPanic` and
`TestAPanicAfterStrippingStillReturnsTheStrippedPost` still green; the same two
for GeoJSON. A refusal before `GetFileInfo` with both file switches off. The
collision guard in both directions. A found-then-refused CoT fence not falling
through to an attachment. A forged `custom_tf_cot`, a forged
`custom_tf_geojson`, and one format's type with the other's props key, each
clean and each still decorated. `custom_tf_location` untouched.

**Cross-language**, in `server/geojson_sync_test.go`. Four tests, not seven:
`TestWebappGeoJSONPostTypeMatches` (folding in the source kinds, as
`TestWebappCotPostTypeMatches` already does), `TestWebappGeoJSONShapeMatches`
over the generated `Fixture()` with a nested walk and an optional-key
allowlist, `TestWebappGeoJSONKindsMatch`, and
`TestGeoJSONHasNoMapSettingOfItsOwn`. A reasons test is unnecessary now that
notes are Go-authored sentences, and a "truncates nothing" test was cut: it
would assert the absence of code in another language by scraping for `slice(`,
which fails on unrelated edits and gets deleted by the first person it
inconveniences.

**Playwright component.** The card on each rung, with the omission stated, and
with properties of each JSON type rendered. Each fallback, including an
unreadable version. A property key and value containing markup, asserted as
text. The map with one polygon, a holed polygon, a `MultiLineString`, a
`GeometryCollection` splitting across both channels, mixed geometry, points
only, nothing drawable, each degenerate extent, a two-feature antimeridian
document, and a single-feature antimeridian document. `LocationMap` with
several ringed shapes; in `extentOnly` mode asserting no pin, no "position
unavailable" note, and a label naming the overlay; and the existing
`geometryColor` security case still asserting on a scalar paint property.

**Manual, recorded in `docs/design/unverified.md` if not run.** A real QGIS
export and a real ATAK export, pasted and attached.

## Acceptance criteria

- [ ] `make check-style && make test` green, including `map-data-check`.
- [ ] Every Cursor on Target behavior unchanged: signature, contract, recover
      span, and gate ordering.
- [ ] A panic in `GetFileInfo` is recovered for both formats.
- [ ] A document over each cap leaves the post postable.
- [ ] No pin at any position no feature states, and no extent-only map renders
      the "position unavailable" note or claims a marked position.
- [ ] A polygon's hole renders as a hole; a `MultiLineString` renders as two
      lines.
- [ ] A `1e-05` coordinate is drawn.
- [ ] A `json` fence and a `.json` attachment are provably ignored.
- [ ] `LocationMap.pw.tsx:1125` still asserts on a scalar paint property.
- [ ] `make sbom-audit` green. No new dependency in either language.
- [ ] `docs/design/geojson.md` holds the rationale and no new code comment does.
- [ ] `public/help/geojson.html` renders air-gapped, light-only, with `copy.js`
      as its only script.

## Checklist

- [ ] **Design note first**, and no prose comments in the code.
- [ ] **Sync rows.** Four, each with a Go test that fails when either side moves
      alone.
- [ ] **Error codes.** Four edits each, plus the two renames keeping numbers.
- [ ] **Preferences namespace.** The `geojson` key chosen in the design note now.
- [ ] **Slash command.** `/tactical-fusion examples` gains a GeoJSON example.
- [ ] **No em dashes** anywhere.
- [ ] **Conventional commits.** `feat:` for the feature, `refactor:` for the
      helper extraction.

# Air-Gapped Mapping

## Overview

Show a decorated coordinate on a map, on both surfaces, with no network. A
bundled Natural Earth vector basemap renders through server-generated inline SVG
on the standalone page and through MapLibre GL JS in the right-hand sidebar. A
Region row names the country the position falls in. A raster tile pack is a
build-time opt-in the panel offers as a second basemap when it is present.

## Problem Statement

The Location decorator already answers "what is this coordinate" in seven
notations. It does not answer "where is that", which is the question a reader
opens the sidebar with. Today the panel shows `17S PU 12345 67890` beside
`34.0561° N, 118.2500° W` and leaves the reader to know that the first is in
California.

The constraint that shapes every decision below is the target environment:
disconnected, DDIL and fully air-gapped installs behind an SBOM and a CVE gate.
No tile service, no font CDN, no sprite host, no geocoder. Whatever the map
needs has to be in the bundle already.

## Current State

Everything upstream of the map is built and tested.

| Capability | Where | Status |
|---|---|---|
| DD, DMS, DDM, USMTF, MGRS, UTM recognition | `server/decorators/location/grammar.go`, `parse.go` | Shipped |
| Normalization to one WGS 84 lat/lon | `coord.go:199-208` (`Location.Point()`) | Shipped |
| Transverse Mercator forward and inverse | `geodesy.go` | Shipped |
| Seven derived readings at the token's resolution | `format.go`, `convert.go` | Shipped |
| Copy buttons on both surfaces | `CopyButton.tsx`, `location/page.go:64` | Shipped |
| Per-reader row hiding | `rows.go`, `server/preferences.go:227` | Shipped |
| Standalone page for clients with no webapp bundle | `location/page.go`, `decorators/page.go` | Shipped |

Sections 8 through 12 of the request describe code that exists. This plan covers
only the delta.

### Current Gaps

- **No numeric position on the wire.** `Conversion` is six formatted strings
  (`convert.go:19-27`). For a grid link the webapp's `coord` is `null`
  (`webapp/src/decorators/location/index.ts:25`), because the inverse projection
  is Go-only. A map has nothing to plot.
- **No basemap of any kind.** The entire repo's non-code payload is
  `assets/icon.svg`, 1,232 bytes.
- **The standalone page cannot fetch anything.** `decorators/page.go:236-238`
  serves `default-src 'none'` with script pinned by sha256 digest.
- **The plugin serves no static files.** `server/http.go:33-49` routes only
  `/api/v1/*` and `/decorate/<type>`; everything else 404s. The **only** static
  route is Mattermost's own, over the bundle's `public/` directory.
- **webpack has one entry, one output, no asset rule, no code-splitting**
  (`webpack.config.js:32-34,84-93`).
- **No dependency has ever been added to the webapp for a feature.** Four
  runtime deps, three of them webpack externals.

## Phase Strategy

The phases are ordered so that the riskiest decision is made **last and with
evidence**, not first and on faith.

| Phase | Focus | New dependencies | Value |
|---|---|---|---|
| **1a** | Go SVG renderer, the page, the Region row on both surfaces | **None** | A map for every client including mobile, and a place name everywhere |
| **1b** | MapLibre panel, basemap fetch, hiding the map | `maplibre-gl` | Pan and zoom in the sidebar |
| **2** | Raster tile pack, build-time opt-in, basemap toggle | None | Relief for installs that want it |
| **3** | Overlays, multiple points, CoT, external tile server | None | Deferred |

**The test for what belongs in 1a is whether 1a has a consumer for it.** An
earlier version of this plan failed that test in four places and said so in its
own prose: `world.geo.json` was labelled "for MapLibre (1b)" while sitting in
1a's file table with an 1a budget test; `Lat`/`Lon` went into 1a beside the
sentence "The page needs none of this"; and the whole hidden-row plumbing went
into 1a beside "The page cannot honour this preference". If 1b had then been
cancelled, all of it was dead code plus a rewritten sync test.

So the line is drawn at **what the page and the table can use today**:

- **1a is the server plus one row in the panel.** `paths.go` and `admin.go`, the
  SVG, the page, and the Region row, which needs no map and renders on both
  surfaces. It touches four webapp files, all for that one row.
- **1b is everything only a map consumes.** `world.geo.json` and its budget,
  `Lat`/`Lon` and the sync-test rewrite, MapLibre, and the hidden-map plumbing.

One deliberate exception, and it is the rolling-upgrade requirement in §8: the
**server-side** `SectionMap` constant and the `KnownRow` widening ship at the end
of **1a**, where they are inert, so that no node can refuse a `hidden_rows`
value that a newer webapp has already started sending. The webapp half, and the
Customize entry a reader can actually click, are 1b.

**1a ships standalone.** No npm dependency, no webpack change, no WebGL
question, no CSP question. If 1b were cancelled tomorrow, nothing in 1a is
orphaned.

That ordering is deliberate. Four independent reviews across two drafts have
argued `maplibre-gl` is being taken on ahead of need; see Decisions and §11.
Splitting the phase does not settle that argument, but it stops Phase 1 being
blocked by it and means 1b is decided against a working 1a.

## Design Principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Basemap payload | Natural Earth 110m vector, one dataset both renderers read | 341 raster PNGs as the default | §1 |
| Page rendering | Inline SVG in `BodyHTML`, no external references | Widening `default-src 'none'` | `decorators/page.go:221-235` |
| Page theming | CSS custom properties in `pageStyles` | A `theme` argument in Go | `page.go:151-155` |
| Panel rendering | MapLibre, lazy, degrades to hidden | Blocking the panel on WebGL | `LocationPanel.tsx:178-196` |
| Precision | Draw the token's **cell**, never a bare pin | A 111 km token and a 1 m token looking identical | `format.go:155-183` |
| Missing position | No pin, and say so | A pin at `0, 0`, or clamped to the Mercator limit | `convert.ts:27-38` |
| Null island | `Number.isFinite`, never `if (lat && lon)` | The truthiness bug this repo declined to inherit | `CLAUDE.md`, prior-art bug 3 |
| Place names | Derived in Go, served as a string, cited to its source | A sovereignty claim | §6 |
| Reader control | One more id in the existing hidden list | A new preferences section | `preferences.go:86-93` |
| Static assets | Everything served lands under `public/` | A new Go route | `Makefile:194-196` |

## Reference Patterns

- `webapp_sync_test.go:115-148` shows how a catalog is held identical across Go and
  TypeScript. The map's shared constants get the same treatment.
- `format_test.go` `renderFixtures` + `format.spec.ts` are the same inputs and
  expected strings in two languages. The cell fixtures follow this shape.
- `preferences/store.ts:56` is a module-state cache. The basemap cache is the same
  idea with no TTL.
- `geodesy_test.go` anchors against an authority outside this repository before
  any round trip. Web Mercator gets the same.
- `build/manifest/` → `server/manifest.go` is a build-time Go generator writing
  into a served package. `build/mapdata/` follows it, **except** on the
  committed-versus-ignored question; see §3.
- `LocationPanel.tsx:223-241` is the local versus remote value split and the
  `converting…` / `unavailable` placeholders.

## Requirements

- [ ] A decorated coordinate shows a map on the standalone page (1a) and in the panel (1b)
- [ ] Both work with the network cable pulled, first load, cold cache
- [ ] The map never draws a pin it cannot place, including beyond the Mercator limit
- [ ] The drawn cell reflects the token's resolution, at the right size and shape
- [ ] A reader can name the country a coordinate falls in, with the map hidden and with no WebGL
- [ ] A reader can hide the map, and hiding it survives a reload
- [ ] The standalone page's CSP is byte-identical, and gains no `img-src` or `font-src`
- [ ] Both surfaces are usable by a screen reader and by keyboard
- [ ] The page's response stays under its stated per-request byte budget
- [ ] A raster tile pack can be built in, and the panel offers it when present

## Out of Scope

- Coordinate recognition, normalization, conversion, copy buttons (shipped)
- Multiple points, lines, tracks, routes, polygons (Phase 3). Nothing in v1
  produces more than one point: a link carries one `(f, v)`, and aggregating a
  channel's coordinates raises a "which posts can this reader see" question that
  is a feature of its own
- CoT parsing and TAK interoperability (Phase 3)
- Altitude on the internal model (see Decisions)
- An external tile-server URL setting (Phase 3, where the request also puts it)
- MBTiles, vector tiles, regional packages, terrain, imagery (Phase 3)
- Offline geocoding, place-name search, reverse lookup beyond the Region row
- A hover card for Location. It has none today, deliberately
  (`location/index.ts:142-151`), and a WebGL context behind every cursor pass is
  the worst possible version of that

## Technical Approach

### 1. Why vector is the default, and raster is not

Three things decide it, and **size is the weakest of them**:

- **Themeing.** The plugin paints itself from the reader's theme on both
  surfaces. A raster tile has one lightness baked in; matching the sidebar means
  two tile sets.
- **The page.** An SVG renderer cannot draw from PNG tiles, and
  `default-src 'none'` blocks the page from fetching them regardless, including
  `data:` URIs, so "inline a base64 PNG" is not available either. Rendering on
  both surfaces decides this on its own.
- **Overzoom.** Past its maximum, raster goes blurry, which is the map
  equivalent of rendering `35°00'00"N` for a token that said `35°N`. Vector goes
  geometrically coarse instead, which is honest about what it is.

**On size, stated correctly.** Natural Earth's published figures (coastline
83 KB, land 68 KB, lakes 23 KB) are **zipped shapefiles**, and the shipped
artifact is GeoJSON. Realistic unsimplified GeoJSON is ~250 KB for land and
0.7-1 MB+ for admin-0 with its full attribute table. So the honest comparison is
**~1-2 MB of source, simplified to a 400 KB budget**, against 3.4 MB best case
and 10-25 MB realistic for raster. Still decisive, but one order of magnitude,
not two. The previous draft of this plan compared zipped shapefiles against
unzipped PNGs and overstated it.

Raster is not discarded. It becomes what §16 of the request called a map
package: a build-time opt-in, in Phase 2.

### 2. The basemap dataset

`build/mapdata/`, a Go program run by `make map-data`, reading Natural Earth
110m from `build/mapdata/source/` and writing:

```
public/map/world.geo.json                     lon/lat, for MapLibre (1b)
server/decorators/location/mapdata/paths.go   pre-projected, for the SVG (1a)
server/decorators/location/mapdata/admin.go   admin-0 polygons, for the Region row
```

Layers: **land**, **lakes**, **admin-0 boundary lines**. Not ocean, which is the
background. Not rivers, which are noise at this scale.

Two byte reductions that make the budget reachable, both from review:

- Use `ne_110m_admin_0_boundary_lines_land` (~60 KB shapefile) rather than
  `admin_0_countries` polygons, whose outlines duplicate the land layer's
  coastlines wholesale. The polygons are still needed for the Region row (§6),
  but only server-side and only in `admin.go`, never in the fetched GeoJSON.
- Round to **2 decimals**, not 4. At z6 on a 300 px viewport one pixel is about
  600 m; 2 decimals is 1.1 km and already sub-pixel at the zoom cap. Roughly
  halves the byte count.

Strip every attribute except the one the Region row reads.

**Budgets, each with a test:**

| Artifact | Phase | Budget |
|---|---|---|
| The page's emitted SVG, per response | 1a | A **regression pin**, not a target. §5 admits the number depends on whether Mattermost gzips plugin responses, which changes it by 3-4x, so it is set from the first measured run and thereafter only moves deliberately |
| `public/map/world.geo.json` | 1b | 400 KB raw, 120 KB gzipped. A real target: it is fetched by every reader who opens a coordinate |
| `server/decorators/location/mapdata/admin.go` | 1a | Uncapped, and **at source precision**. See §6: rounding it corrupts borders |
| `build/mapdata/source/` | 1a | Recorded, not capped, but it takes the repo's non-code payload from 1,232 bytes to megabytes permanently. Say so in the commit |

**The generator must be standard-library only.** There is one `go.mod` at the
repo root, so `build/mapdata/` is in the shipping module and
`cyclonedx-gomod mod` (`Makefile:505`) enumerates it. A shapefile reader or a
Douglas-Peucker package pulled in here goes through `grype --fail-on high` and
CodeQL forever, despite never reaching a released binary. `build/manifest/` set
the precedent; hand-writing the geodesy series set it harder.

### 3. Generated artifacts are committed, and the check is not idempotence

All three generated files are **committed**. This differs from
`build/manifest/`, whose outputs are gitignored and regenerated by `make apply`
as a `dist` prerequisite, and the difference is deliberate: `paths.go` must be
present or `go build ./server/...` fails on a clean checkout, `world.geo.json`
must be present or its budget test has nothing to measure, and an air-gapped
`go test` must work without running a generator. Record this in `CLAUDE.md`,
because it reads as an inconsistency against `build/manifest/` otherwise.

**The CI check is `make map-data && git diff --exit-code`**, not idempotence.
Determinism is not the property that matters: a generator edited without
regenerating is perfectly idempotent and fully drifted. The check must be that
the committed artifacts are what the committed source produces through the
current generator.

Drift between the two representations is guarded three ways, and the middle one
must be **bidirectional**:

1. One generator, one run.
2. `TestProjectedPathsMatchTheGeoJSON`, sample the GeoJSON, project, assert the
   point lands on the corresponding path; **and** compare per-layer feature and
   vertex counts in both directions, so geometry present in `paths.go` and
   absent from the GeoJSON, or a layer dropped entirely, cannot pass unsampled.
3. The `git diff` check above.

`paths.go` and `admin.go` carry `// Code generated ... DO NOT EDIT.` headers, or
`make check-style` lints a large generated file. A new package with no test
reads 0% under `make coverage`'s `-coverpkg=./server/...`, which is the
under-reporting `CLAUDE.md` already calls out.

### 4. Getting a position onto the wire

`Conversion` gains three fields, appended:

```go
type Conversion struct {
	MGRS    string  `json:"mgrs"`
	UTM     string  `json:"utm"`
	Decimal string  `json:"decimal"`
	DMS     string  `json:"dms"`
	DDM     string  `json:"ddm"`
	USMTF   string  `json:"usmtf"`
	Region  string  `json:"region"`  // 1a
	Lat     float64 `json:"lat"`     // 1b
	Lon     float64 `json:"lon"`     // 1b
}
```

**`Region` lands in 1a and `Lat`/`Lon` in 1b**, because each arrives with its
consumer: the Region row renders in the panel with no map, while nothing in 1a
reads a numeric position (the page has `loc.Point()`). That split is also what
makes `TestConversionCarriesNoUnreadFields` satisfiable at the end of each phase
rather than failing by construction at the end of 1a.

The `// 1a` / `// 1b` markers above are annotations for this document. They do
not appear in the code, per the no-prose-comments rule.

**The sync test does not survive `Lat`/`Lon`, and an earlier draft was wrong to
say it would.** `webapp_sync_test.go:231` is:

```go
regexp.MustCompile(`(?m)^\s+(\w+):\s*string;`)
```

It matches `string` fields only, while the Go side reflects **all** fields
(`:238`). `lat: number` matches nothing, so the counts diverge and the test
fails.

The obvious repair, widening to `(?:string|number)`, **weakens** it. While
every field is a string, name agreement implies type agreement; with two numbers
present, `lat: string` in TypeScript against `float64` in Go type-checks,
ships, and puts the pin at `NaN`. The test must capture the TypeScript type and
compare it against the Go field kind, as `(name, type)` pairs.

`convert.go:5-18` carries that constraint today as a doc comment ("Exactly those
rows and no others... Sending those too would put fields on the wire that nothing
reads"). Under the no-comments rule it is removed rather than amended, so the
constraint has to survive elsewhere: it moves to the `CLAUDE.md` Mapping section,
and `TestConversionCarriesNoUnreadFields` makes it enforceable rather than
advisory.

**On numbers at all.** `CLAUDE.md` says this endpoint returns strings "because
the rendering rules are the interesting part and they live in Go". That
reasoning is about rendering a reading as text and still holds: no new text is
derived from these floats. A pin's position is geometry, and the resolution rule
reaches it through the cell (§5), not through digit counts.

Rejected alternative: parse the existing `decimal` string back. It is rounded to
the token's resolution, so `35N079W` would move the pin up to 55 km, and it means
a second parser for text this repo deliberately formats lossily.

`Convert` already requires `loc.Point()` to succeed before returning `ok`
(`convert.go:73`), and `api.go:118-122` returns 400 on `!ok`, so **there is no
path to a 200 without a real position.**

**Two consumer-side rules, both load-bearing:**

- **`0, 0` is a position.** Never `if (lat && lon)`. This is bug 3 in the prior
  art this repo declined to inherit.
- **`convert.ts:78` is an unchecked cast** (`response.json() as Promise<Conversion>`).
  A captive portal or transparent proxy, the ordinary DDIL failure, returns
  200 with unrelated JSON, and `lat` is `undefined`, giving `NaN` rather than
  zero. The required predicate is `Number.isFinite(lat) && Number.isFinite(lon)`
  plus a range check, which passes `0` and rejects both `undefined` and a
  hostile `1e9`. `TestNullIslandDrawsAPin` alone will not force this; state it.

The page needs none of this: it holds a `Location` and calls `loc.Point()`.

### 5. The page renderer

```go
func renderMapSVG(lat, lon, cellDegLat, cellDegLon float64) string
```

**Floats only. No `theme string`, and that is not tidiness.** Every other
argument is a `float64`, and Go's type system rather than escaping is what makes
the SVG safe; a string parameter is the one thing that could carry markup, and
it would have to be threaded from `RenderPage` through `renderBody`, which is
exactly where a future change substitutes `params.Get("_theme")` for the
validated `decorators.ThemeFromParams`.

It is also **wrong**. `Page.Theme` is `""` when the param is absent, which is the
mobile case this page exists for, and the shell resolves that in CSS with
`@media (prefers-color-scheme: dark)` (`page.go:151-155`). Go cannot know the
reader's OS preference, so a Go-baked palette paints a light map onto a dark
page for exactly the reader the page is for.

Instead: emit classed elements (`<path class="map-land">`), add `--map-land`,
`--map-water`, `--map-line`, `--map-pin` to `pageStyles` (`location/page.go:18`)
with the same `[data-theme="dark"]` plus media-query pair the shell uses. Note
that `fill="var(--x)"` as a presentation attribute does not work; it needs a CSS
rule.

**The SVG is clipped to the view at generation time.** `viewBox` crops what is
drawn but not what is sent, so an unclipped world ships the whole basemap in
every page response, 400 KB of HTML per link, on `Cache-Control: private,
max-age=300` (`location.go:353`), on the mobile DDIL clients this page serves,
with no shared-cache option available because the CSP forbids an external file.
Emit only paths intersecting the rendered extent. Budget the result at **60 KB**
with a test, in the same spirit as the basemap budget. Measure whether
Mattermost gzips plugin `ServeHTTP` responses before fixing the number; it
changes it by 3-4x.

`renderMapSVG` returns a string and cannot fail. If the position is
unrenderable (§7), `renderBody` **omits the SVG and emits the explanatory line
in its place**, beside `loc-note`, then renders the table. Omitting silently
would leave a reader at 89.9° N wondering whether the map failed to load. It
never reaches `WriteError`: the coordinate is valid, only the picture is missing.
**No error code is allocated**, because there is no reachable branch to name;
the previous draft promised codes for paths that do not exist.

### 6. The Region row

A new `region` entry in `Rows`, derived server-side by point-in-polygon against
`mapdata/admin.go`, rendered as prose and **not copyable**.

It is what makes the map answer the question in the Problem Statement. An
unlabelled 110m coastline at 300 px identifies Italy and does not identify Chad
from Niger. It also works with the map hidden, with no WebGL, and with the
basemap fetch failed, which none of the map itself does.

**One implementation, in Go**, served as a string over the wire, which is
exactly the rule `CLAUDE.md` states for this endpoint and the reason the Region
row is a string while `lat`/`lon` are numbers.

**It cites its source and is worded as a lookup, not a determination.** The row
renders `United States (Natural Earth 110m)`. Naming a country from a coordinate
is an assertion about a border, and at 110m with a public-domain dataset it is a
coarse one; Crimea, Kashmir, Taiwan, the Golan and Western Sahara are all places
where a plugin stating a coordinate is "in" a country is a problem of a
different kind from a missed decoration. Citing the dataset in the row itself is
what keeps it a basemap lookup. `public/help/panel.html` says the same at
length, and `CLAUDE.md` records the reasoning.

**Name the file and the field, because that is where the sovereignty decision
actually lives.** A citation string does not make an unnamed attribute neutral:
whether this row says "Taiwan" or "China", "Western Sahara" or "Morocco", is
decided entirely by which column is read.

- File: **`ne_110m_admin_0_countries`**, not `_countries_lakes`. Lakes are a
  separate layer already and a lake is not a region.
- Field: **`ADMIN`**, the de facto administering entity, not `SOVEREIGNT` (the
  claimed sovereign) and not `NAME` (a display abbreviation that varies).

`ADMIN` is the defensible default for an operational readout: it answers "who
administers the ground here", which is the question a reader with a grid
reference is asking. `SOVEREIGNT` answers a legal question this plugin has no
business answering. **This is a policy choice, not a technical one**, and it is
recorded here so it can be overridden deliberately rather than discovered in
production.

**Bounding-box prefilter.** The generator emits a bbox per feature so a ray cast
runs only against candidates. Roughly 250 features tested per render, on
`page.go:124`, on an unauthenticated route that §5 already budgets for.

**Precision: `admin.go` keeps the source's own coordinates.** The 2-decimal
rounding in §2 is a *rendering* argument ("one pixel is about 600 m at z6") and
must not reach the polygons. Quantizing to 1.1 km stops adjacent countries
sharing edges, so borders develop overlaps and gaps, and the gaps would then be
misread as the intended "no answer" case below. `admin.go` is server-side and
outside the fetched-artifact budget, so there is nothing to buy by rounding it.

**Four edges, defined rather than discovered:**

- **No polygon** (ocean, Antarctica's gaps): the row is **omitted**. It never
  guesses at a nearest country.
- **Two polygons** (a point on a shared border is inside both): **lowest feature
  index wins**, and the generator emits features in a stable sorted order so the
  answer cannot change when the source is regenerated. Pinned by a test.
- **Antimeridian**: Natural Earth splits geometries at 180, so a naive ray cast
  is usually correct, but "usually" is not a specification. Pinned with a
  Chukotka and a Fiji fixture.
- **Generalization error**: 110m displaces borders by roughly a kilometre. That
  number goes in `panel.html` beside the citation, because it is the figure that
  tells a reader how far to trust the row.

**The panel cannot express "omitted" today, and that is a real defect in the
naive design.** `LocationPanel.tsx:223-228`'s `remote()` maps any falsy value to
`converting…` or `unavailable`, and the omission filter at `:296`
(`values[row.id] !== ''`) therefore never fires for a server-supplied value. An
ocean coordinate would render **`Region: unavailable`** in the sidebar while
`page.go:123-127` omits the row entirely, and "unavailable" reads as an outage
rather than as an answer. `Conversion.Region: ""` is also indistinguishable from
a response that never carried the field.

So the Region row **does not go through `remote()`**. It renders only when
`conversion.status === 'ready'`, and an empty value at that point means omitted,
not pending. While loading it is absent rather than showing a placeholder, since
a place name that appears a moment later is not a degradation.

**A cheaper implementation was considered and rejected**: a 0.5° raster grid of
country indices, run-length encoded, two integer divisions to look up. It avoids
ray casting, multipart polygons and the antimeridian entirely, and its ±55 km
error is arguably inside the row's own coarseness disclaimer. Rejected because it
moves the complexity into the generator, which now has to rasterize polygons
correctly, and because ±55 km is wrong often enough near borders that the
disclaimer would be doing real work rather than being a caveat. The bbox
prefilter closes most of the performance gap it was solving.

### 7. Resolution, and the places the map cannot go

**The cell, not a circle.** The previous draft drew a circle of radius equal to
the resolution. That is wrong twice:

- **It doubles the uncertainty.** `resolutionAt` returns a **cell edge**
  (`format.ts:327`, and MGRS's `100000 / 10^(digits/2)` at `:422`). A radius of
  111 km draws a 222 km footprint for a token whose square is 111 km. Overclaiming
  uncertainty is as much a claim as underclaiming it.
- **A circle is the wrong shape, and to this audience the wrong symbol.** A ring
  around a point reads as a range ring, weapons, MEZ, comms, threat, or as GPS
  accuracy. Neither is "how precisely the author typed it". It also contradicts
  the panel's own note four inches below it: "A grid reference names a square,
  and the position shown for one is its center" (`LocationPanel.tsx:309-311`).

So: draw the **cell**. For the angular grammars a lat/lon rectangle from each
axis's own `axisResolutionDegrees`, which also fixes the mixed-precision case
`34.0561N,118.2W`, where a single figure sized from the coarser half is wrong in
one direction. For MGRS, the grid square. A 1° cell at 35°N is 111 km tall and
91 km wide, which a rectangle expresses and a radius cannot.

`gridResolutionText` returns a **string** (`format.ts:411-424`); a numeric export
is needed on both sides. Where it returns `''` on a non-matching token, the cell
is not drawn.

Below about 6 device pixels the cell is dropped and only the dot is drawn. That
threshold is **not** a synced constant: the page measures SVG user units and
MapLibre measures screen pixels, so the same number means different sizes, and a
disagreement about whether a 1 m cell stops drawing at 5 px or 7 px is not
observable. Each renderer owns its own.

**For MGRS the cell is a lat/lon rectangle bounding the grid square, and the plan
states that as an approximation rather than an identity.** An MGRS square is
axis-aligned in UTM grid space, so it is rotated by the grid convergence (up to
about 3° inside a zone, more at high latitude) and its edges are not parallels.
At z6, where a 100 km square is tens of pixels, the difference is invisible. The
fixtures pin the bounding rectangle, and say so.

**Paired cell fixtures**, token → cell size in map units, in the `renderFixtures`
style but as a **separate table**. `webapp_sync_test.go:165-169` parses
`format.spec.ts`'s existing fixture table with a fixed seven-column regex and
asserts `len(found) == len(renderFixtures)`; adding a column breaks it. Metres to
map units needs the same 1/cos(lat) Mercator factor on both sides and nothing
else pins it.

**Beyond the Mercator limit, there is no pin.** `parse.go:353` sets
`maxLatDeg = 90`, and `dd`, `ddh`, `dms`, `ddm`, `latd`, `latm` all validate
against it, so `89.9000, 12.0000` is a decoratable token today. Web Mercator caps
at ±85.0511°; clamping to it would draw a pin **550 km from where the token
says**, silently, in a plan whose stated rule is "no pin, and say so". At exactly
90 the projection is infinite.

`convert.go:48-51` already documents the sibling case, past 84 N / 80 S the
grid rows come back empty while "the position itself is perfectly good", and
this follows it: beyond ±85.05 the basemap cannot show the position, so the map
is omitted and a line says so. Every reading still renders. Only the textual
grammars reach this, since MGRS and UTM top out at 84°N, which is precisely why
fixture-driven testing will miss it.

**The antimeridian.** A pin at 179.95°E, or a cell straddling 180°, wraps.
MapLibre wraps sources; the fixed-extent SVG will draw a shape running off one
edge with nothing on the other. The page clamps the extent to the antimeridian
and draws the truncated cell; the Pacific is a real theatre and silence here is
not acceptable.

**Zoom.** Capped at z6, where 110m data stops being honest. Default z3. But a
constant zoom is not a constant answer, because Mercator scale is 1/cos(lat): the
same 300 px is ~2,940 km at the equator, ~2,420 km at 34°N, ~1,000 km at 70°N.
The synced constant is therefore a **target ground span**, not a zoom level, with
`z = f(span, lat)` clamped to [0, 6] computed on each side. That puts the
latitude correction in one place and makes the constant the honest one.

**±85.0511287 is a synced constant too.** It is a second number with two
implementations, and if the panel's limit differs from Go's, one surface draws a
map and the other does not for the same link, silently, in the way the band class
once did. It lives in `span.ts` with the span and the max zoom, and the Go sync
test pins all three.

The basemap names itself and its limit in a caption, which is also where the cell
is explained, below the 6 px threshold the cell is not drawn, so the caption is
the only thing distinguishing a 1 m token from a 500 m one.

### 8. Hiding the map, and the read path that silently undoes it

The stored value is what a reader **hid**, so a reader who has customised their
rows still gets the map when this ships. That promise is
`server/preferences.go:86-93` and needs no change.

The map is **not** a `Row`: `Rows` entries carry `Value func(...) string` and
the panel filters on `values[row.id] !== ''` (`LocationPanel.tsx:296`).

```go
// SectionMap is hideable but is not a row of the table.
const SectionMap RowID = "map"
```

`RowID` is a type alias (`rows.go:17`), so this compiles. `KnownRow` widens to
`rowByID[id] || id == SectionMap`. **It is not renamed.** The previous draft
proposed `KnownHideable` across "three call sites"; there is exactly **one**
(`server/preferences.go:236`), plus the definition and two comments. Cross-package
exported-name churn to improve one identifier is not worth a diff in a second
package and a reworded user-facing error string.

**The TypeScript read path drops the id, and this is the plan's most dangerous
detail.** `rows.ts:57` derives `KNOWN` from `ROWS`, `isRowID` tests `KNOWN`, and
`preferences/types.ts:133-146`'s `asRowIDs` discards anything `isRowID` rejects.
So: reader hides the map → the server stores `["map"]` → the next `fromWire`
throws it away → **the map comes back and the setting is unreachable through the
UI**, with nothing logged on either side. This is the silent-split family
`webapp_sync_test.go:208-220` exists for.

Required, and **`webapp/src/preferences/types.ts` was missing from the previous
draft's file list**:

- `MAP_ID` in `rows.ts`, plus an explicit `HIDEABLE = new Set([...ROWS.map(r => r.id), MAP_ID])`
  and `isHideableID`
- **A new union, not a widened `RowID`:**
  `export type HideableID = RowID | typeof MAP_ID`, and
  `LocationPreferences.hiddenRows` (`types.ts:41`) becomes `HideableID[]`.
  Widening `RowID` itself does not compile: `LocationPanel.tsx:270` declares
  `const values: Record<RowID, string>` as an exhaustive object literal, so
  admitting `'map'` makes that literal missing a required key and
  `npm run check-types` fails. The separate union satisfies
  `savePreferencesSection` typing, keeps `values` and `isRowVisible` keyed on
  real rows, and mirrors the Go side, where `SectionMap` is deliberately not in
  `Rows`.
- `asRowIDs` tests `isHideableID`
- **`MAP_ID` must not be written in the row-catalog shape.**
  `webapp_sync_test.go:118-127` matches
  `{id: '…', label: '…', copyable: …` anywhere in `rows.ts` and requires the
  count to equal `len(Rows)`. A map entry written that way fails that test with a
  misleading message. Give it a distinct shape and pin it with its own regex.
- **A round-trip CT test**: hide the map, re-read preferences from the wire,
  assert it is still hidden. A write-only test passes while this bug is live

**`Customize`'s warning breaks too.** `Customize.tsx:155` is
`const shown = ROWS.length - hidden.length`. With `'map'` in `hidden` but not in
`ROWS`, hiding ten rows plus the map gives `shown === 0` and fires the
everything-is-hidden warning while a row is still on screen; hiding all eleven
gives `-1`, so the warning is **silent in exactly the case it exists for**, the
case §8 calls recoverable.

**Rolling upgrade.** An old node's `validHiddenRows` refuses `"map"` on PUT while
a new node accepts it, so the same Save succeeds or fails depending on which node
answers. The repo already treats the cluster as real
(`preferences_cache.go`). Ship the server widening before the webapp offers the
control.

**The page cannot honour this preference**, because it has no session
(`location/page.go:117-120`), so the map is unhideable there. Inherent, consistent
with the DTG precedent, and it belongs in the UX table rather than as a surprise.

### 9. Accessibility

Absent from the previous draft entirely, in a repo that has
`aria-activedescendant` in `ZonePicker`, `role="status" aria-live="polite"` at
`location/page.go:133`, and `aria-label` on every copy button. A focusable WebGL
canvas inserted above the existing copy buttons is an active regression.

| Surface | Required |
|---|---|
| Page SVG | `role="img"`, a `<title>` as first child referenced by `aria-labelledby`, every decorative path `aria-hidden="true"` |
| Panel canvas | `role="img"` and an accessible name on the container if non-interactive; `tabindex="-1"` on the canvas so it leaves the tab order. If interactive, a real name and discoverable keyboard instructions |
| Both | Pin and cell at 3:1 non-text contrast (WCAG 1.4.11) against land **and** water, in both themes. The palette derives from `--center-channel-bg`/`--center-channel-color`, so derived tints will be low-contrast by construction; the marker colour must not be derived from them |
| Both | Shape, not colour alone, distinguishes pin from cell |
| Both | 200% text zoom (WCAG 1.4.4) must not let a fixed-px map consume the viewport |
| Panel | `prefers-reduced-motion`: `jumpTo`, never `flyTo`. A world-crossing animation per click is a vestibular risk and pointless in a 300 px box |
| Panel | `forced-colors: active`: a canvas ignores forced colours and becomes an unreadable blob. Windows High Contrast is common on hardened government VDI, the stated target. Another degrade |

**The accessible name collides with an existing test.**
`location_test.go:452` asserts `strings.Count(body, "18S UJ 23478 06483") == 2`.
A `<title>` naming the coordinate makes it 3. Update the test deliberately, its
intent (no lead line repeating the coordinate above the table) still matters, and
an accessible name is not a lead line, but the previous draft's stated reason
("an SVG is not one") was not the reason it would break.

### 10. Loading, degrading and refusing

| State | Table today | Map |
|---|---|---|
| Non-grid link | Values local, immediate | Pin immediate, from `coord` |
| Grid link, conversion loading | `converting…` | Basemap, no pin, `converting…` |
| Conversion `failed` | `unavailable` | Basemap, no pin, `unavailable` |
| Conversion `rejected` (400) | Whole panel replaced | Not reached |
| Position beyond ±85.05° | Rows render normally | No map. `This position is too far north for the map.` / `…too far south…` |
| Preferences still loading | None | Map not mounted; see below |
| First open, chunk + basemap in flight | None | Reserved box, `Loading map…` |
| Chunk or basemap fetch exceeds 10 s | None | Falls through to the degrade line |
| WebGL2 unavailable | None | `This browser cannot draw the map.` |
| Basemap fetch fails, or its digest mismatches | None | `The map could not be loaded.` |
| Forced colors active | None | `The map is hidden in high contrast mode.` |

Those five strings are written out here rather than described, because
`troubleshooting.html` is specified to quote exact user-facing text, and because
the polar pair is implemented twice, in Go for the page and in TypeScript for the
panel. A described string is a string that drifts.

**The map reuses the table's vocabulary.** The previous draft invented
`locating…` and `position unavailable` beside the table's `converting…` and
`unavailable`, driven by the same request, in the same panel. One request, one
vocabulary. Every string above is written down here because
`troubleshooting.html` is specified to quote exact user-facing strings.

**Nothing may fail the panel.** Every map failure hides the map and leaves every
reading on screen. **No pin is ever drawn at a guessed position.**

**`usePreferences()` returns defaults while loading** (`types.ts:64`,
`hiddenRows: []`), so a naive mount boots MapLibre, a WebGL context and a 400 KB
fetch for a reader who hid the map, then tears it all down. Gate the mount on
preferences having resolved.

**Never retry.** "Never throw" is not "eventually give up" and it is not "try
again": a broken deploy would become a request loop from every open panel. One
attempt, a 10 s timeout, then degrade.

**Teardown.** Browsers cap live WebGL contexts at roughly 8-16. `LocationPanel`
stays mounted across selection changes, `LocationPanel.tsx:167-176` exists
because of that, so a map created per payload without `map.remove()` leaks a
context per click until the browser starts killing the oldest and previously-fine
maps go black. Add the teardown, a `ResizeObserver` calling `map.resize()` for
the resizable RHS, and a CT test that mounts and unmounts twenty payloads.

### 11. The panel, and the dependency

**`maplibre-gl` is Phase 1b and nothing in Phase 1a needs it.**

The recorded cost, corrected from the previous draft. `security.yml:117-124` runs
Grype over the npm SBOM with `fail-build: true` at severity `high`, and
`maplibre-gl` brings on the order of 20-30 new SBOM entries (`@mapbox/*`,
`@maplibre/*`, `earcut`, `geojson-vt`, `gl-matrix`, `kdbush`, `pbf`, `potpack`,
`quickselect`, `supercluster`, `tinyqueue`, `vt-pbf`, ...) into a webapp whose
production dependency list is four, three of them externals.

**The previous draft's mitigation does not exist.** It said suppression in
`.grype.yaml` "is the process the repo already has". `.grype.yaml`'s own header
names the two legitimate reasons: a dev-only transitive dependency not present in
the shipped bundle, or a runtime dependency Mattermost externalizes so the
vulnerable version never ships. **MapLibre is neither**, it ships and it runs in
the reader's browser. The honest process for a shipped runtime dependency is
**upgrade or pin, never suppress**. Record that instead.

Two further corrections: laziness defers bytes over the wire and changes neither
the SBOM surface nor reachability, so it is not a security argument. And the
"bundle grows by less than 500 KB" criterion the previous draft carried was
unfalsifiable, see Acceptance Criteria, which now states units and scopes.

**Serving the chunk. Mattermost already does this, and neither previous draft
knew it.**

Draft one set `__webpack_public_path__ = pluginBaseUrl()`, yielding
`/plugins/<id>/<chunk>.js`, which `http.go:44-49` rejects. That much was a real
bug. Draft two responded by relocating the whole bundle into `public/webapp/`
and moving `bundle_path` with it. That works, but it is unnecessary, and it is
**self-contradictory**: §13 proposes extending the self-contained test to walk
all of `public/` banning `.js`, which would ban the plugin's own bundle.

The actual mechanic, from `plugin/environment.go:547-580` in
`mattermost/server/public@v0.4.3`:

```go
bundlePath = filepath.Join(env.pluginDir, id, bundlePath)
destinationPath := filepath.Join(env.webappPluginDir, id)
...
if err = utils.CopyDir(filepath.Dir(bundlePath), destinationPath); err != nil {
...
os.Rename(sourceBundleFilepath,
    filepath.Join(destinationPath, fmt.Sprintf("%s_%x_bundle.js", id, manifest.Webapp.BundleHash)))
```

Mattermost copies the **entire directory containing `bundle_path`** into the
static plugin directory, then renames only `filepath.Base(bundlePath)`. So
sibling chunks emitted next to `main.js` in `webapp/dist/` are already copied and
already served, at `/static/plugins/<id>/<chunk>.js`.

The fix is therefore one line of config and nothing else:

- **`plugin.json` is untouched.** `bundle_path` stays `webapp/dist/main.js`.
- **`output.path` is untouched.** Chunks land in `webapp/dist/` beside `main.js`.
- **`.gitignore` is untouched.** `.gitignore:2` is a bare `dist/`, which already
  matches at any depth.
- **`Makefile` is untouched.** `Makefile:201-204` keeps copying `webapp/dist`,
  which is exactly what has to ship.
- `chunkFilename` gets `[contenthash]`, or an upgraded plugin can load a stale
  chunk against a fresh bundle.
- `__webpack_public_path__` → `` `${basename}/static/plugins/${manifest.id}/` ``,
  asserted to start with a single `/` and contain no scheme and no `//` before
  assignment. This promotes `window.basename` from a value that builds `fetch`
  URLs into one that decides **where the browser loads executable JavaScript
  from**, and the server side of this repo already applies exactly that rule to
  `SiteURL`.

Four findings against draft two disappear with the relocation: `make dist`
breaking because `Makefile:201-204` copies a `webapp/dist` that no longer exists
while `HAS_WEBAPP` is still set from a non-empty `bundle_path`; §13's own test
banning the bundle; needing `output.clean` to stop stale contenthash chunks
shipping out of `public/`; and having to spike whether Mattermost tolerates a
`bundle_path` inside `public/` at all.

**The assignment goes in `index.tsx`, not `plugin_url.ts`.** It has to run before
the first dynamic import either way, and `plugin_url.ts` is imported by
`convert.ts`, which is imported by `LocationPanel.tsx`, which the `.pw.tsx`
suites mount. Playwright CT serves those through Vite as real ESM, which is
strict mode, so a module-scope `__webpack_public_path__ = …` in that import graph
is a `ReferenceError` in every component test. `playwright-ct.config.ts:10-16`
needs no mirrored output setting, but it does need the assignment kept out of the
modules it loads.

**Style built in code from CSS variables**, so there is no `style.json` file.
**No `symbol` layers, therefore no `glyphs` and no `sprite` URL**, which is the classic
air-gapped trap. Assert it on the style object, since a Phase 3 overlay will want
a label. **`dragRotate: false`** and rotation disabled: a rotatable map with no
compass means a reader misreads every bearing off it. **A scale bar**, which also
does half the cell's explaining. **`cooperativeGestures: true`**, or a wheel-zoom
target at the top of a scrolling sidebar hijacks every scroll toward the copy
buttons. **A recenter control**, since two drags in a 300 px box puts the
coordinate off-screen with no indication which way.

**`maplibre-gl.css` is a real import** routed through `style-loader`
(`webpack.config.js:56-72`), which injects an inline `<style>` at runtime and so
depends on Mattermost's own CSP permitting inline styles. Add it to the spike.

**Basemap fetch hygiene:** `credentials: 'omit'`, a byte cap, no retry, and
`?v=<manifest.version>` on the URL since Mattermost sets caching headers the
plugin does not control. The generator emits a SHA-256 into the generated
TypeScript and `basemap.ts` verifies it with `crypto.subtle`, as much an
availability control as a security one, since a half-deployed asset in an
air-gapped enclave otherwise renders a silently wrong-shaped world. It must
degrade the way `clipboardAvailable()` does, because `crypto.subtle` is undefined
on a plain-HTTP origin, which `CLAUDE.md` records as the norm here.

**Height budget: 180 px total**, map plus caption plus margin, so roughly a
150-160 px canvas. Measured from `LocationPanel.tsx:36-74`: today's panel is
about 470 px, and on a 1366x768 VDI display the RHS has about 557 px of content
height. Anything larger pushes the tail of the table below the fold on the
resolution this audience actually runs. Reserve the height before the map loads,
or the table jumps when the chunk lands and a reader mid-click hits the wrong
row. Budget it with a test, the way the basemap bytes are budgeted.

**An "Open larger" link** under the map, pointing at the standalone page with
`_page=1`. A 150 px map answers "roughly where" and nothing else; the page
already exists, already gets a map, and is already reachable. One line, and it
converts the sidebar map from too-small into an index into a bigger one.

### 12. No admin switch

Every switch in `plugin.json` governs **decoration**, and `RenderPage` never
consults configuration, so a link written into a message keeps working after its
format is turned off. A map switch would govern **rendering**, and would make
historical links change over time. The reader preference covers the case. Purely
additive if operators ask.

### 13. Phase 2: the raster tile pack

`make map-tiles` generates `public/map/tiles/{z}/{x}/{y}.png` plus a `tiles.json`
descriptor, via GDAL, which is **not** a CI dependency. The directory is
gitignored and the default `make dist` ships vector only.

`public/` is copied verbatim by `Makefile:194-196` and served by Mattermost with
**no Go route**, so this needs no server code. The request's proposed
`/plugins/tactical-fusion/maps/tiles/{z}/{x}/{y}.png` route is unnecessary, and
`http.go:45` would reject it anyway.

Four security and correctness constraints, all from review:

- **`public/` is unauthenticated.** Anything under it is retrievable without a
  login, and the *set of tiles present* is an enumeration oracle for the area an
  install cares about. Document this in the imperative in `docs/` and in
  `panel.html`, and restrict the documented workflow to public-domain basemaps.
  An enclave that wants imagery of its actual AO needs an authenticated route,
  which is Phase 3.
- **`public/` is also a same-origin script-execution surface**, outside this
  plugin's CSP and outside `X-Content-Type-Options` (`http.go:31` covers only
  `ServeHTTP`). `TestHelpPagesAreSelfContained` scopes to `public/help` and to a
  hardcoded file list (`help_docs_test.go:26,269`), so `public/map/` inherits
  nothing. **Extend that test to walk `public/map/`**, banning `.svg`, `.js` and
  `.html` there. Scoped to `public/map/` rather than to all of `public/`, because
  the MapLibre `setWorkerUrl()` fallback may legitimately put a `.js` worker
  elsewhere under `public/`; if it does, name it as a reviewed exception. An
  earlier draft wrote this rule as "all of `public/`" while also relocating the
  plugin's own bundle into `public/webapp/`, so the test would have banned the
  bundle. §11 no longer relocates anything, and the rule is scoped anyway.
- **`tiles.json` is never handed to MapLibre as a source spec.** Its `tiles`
  field is an array of URL templates, so a pack author would choose where the
  browser sends requests with the reader's coordinate in `{z}/{x}/{y}`, which
  fails closed air-gapped but not on DDIL, where it is a covert channel. And
  `attribution` is HTML by convention and MapLibre renders it into the DOM.
  `tiles.ts` reads only validated scalars (`minzoom`, `maxzoom`, `format`,
  `bounds`), **constructs the URL template itself** from `pluginBaseUrl()`, and
  rejects any absolute URL, scheme, `//` or `..`. Render `attribution` as text or
  drop it. Same shape as `validateParams`: never trust a field, re-derive it.
- **Cap Terrain at its own `maxzoom`.** The pack is z0-z4 and `MAP_MAX_ZOOM`
  targets z6, so switching to Terrain at the cap delivers exactly the blur §1
  rejects raster for. `maxzoom` is per-source in MapLibre; make it so and clamp
  the camera on toggle.

**Repacking invalidates the release signature.** `Makefile:128` runs
`release-sign` and `release-checksum`; a bundle with a pack dropped in matches
neither the published `.sig` nor the `.sha256`. Document only
`make map-tiles && make dist`, build from source, re-sign locally, and state
plainly in `docs/SECURITY.md` that a modified bundle is no longer the signed
artifact. An operator-built pack is also never scanned by the release pipeline's
`clamscan` (`Makefile:650`); PNG decoders have a real CVE history, so the pack is
the operator's trust boundary and theirs to scan.

**`make dist` must refuse to ship an unintended pack.** `Makefile:194-196` copies
the working tree with no exclusion, so once a developer has run `map-tiles` every
later `make dist` and `make release` silently includes tiles.
`TestBundleShipsNoRasterTilesByDefault` asserts against the **committed tree**,
not the filesystem, or `make test` breaks for the very operator who used the
feature.

The toggle is labelled **Basic / Relief**, not Outline / Terrain: Natural Earth
raster is shaded relief, and "Terrain" implies elevation data it does not carry.
The choice is **persisted** in the existing preferences section, or a reader who
prefers relief re-picks it on every coordinate forever. Detection is resolved
before first paint or its space is reserved, so the control does not pop in and
shift layout. `tiles.json` present but tiles missing, the interrupted copy,
renders blank squares silently in MapLibre; treat it as a degrade row.

**The page stays vector regardless.** It cannot fetch tiles under
`default-src 'none'`, and the toggle is an interaction it does not have.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Raster or vector by default? | Vector | Themes for free, required by the SVG renderer, honest overzoom. Smaller by ~10x, not ~100x |
| One renderer or two? | Two | MapLibre cannot run under `default-src 'none'` |
| Panel renders SVG fetched from the API? | No, but recorded | It needs `dangerouslySetInnerHTML`, which `LocationPanel.tsx:158` says the panel does not use, and it would make a non-grid link wait on a request for a map it can draw locally. The previous draft never weighed this option and answered "one or two renderers" against a strawman |
| Accept `maplibre-gl`? | Yes, in **1b**, decided against a working 1a | Phase 2 and 3 need a tile engine. Three reviews argued it is ahead of need; the phase split is the response. Recorded cost: 20-30 SBOM entries permanently on every PR's Grype path, and **suppression is not an available mitigation** for a shipped runtime dependency |
| Numbers on the wire? | Yes, `lat` and `lon` | Position is geometry, not a reading. The sync test must compare types, not just names |
| Region as a string? | Yes | The rendering rule lives in Go, which is exactly what `CLAUDE.md` says this endpoint is for |
| Assert a country? | Yes, cited to Natural Earth 110m, omitted when there is no answer | Without it the map does not answer its own Problem Statement. Citing the dataset is what keeps it a lookup rather than a determination |
| Which admin-0 field? | **`ADMIN`** from `ne_110m_admin_0_countries` | The de facto administering entity, which is the question a reader with a grid reference is asking. `SOVEREIGNT` answers a legal question this plugin should not answer. **A policy choice, recorded so it can be overridden deliberately** |
| Ties on a shared border? | Lowest feature index, with the generator emitting a stable sorted order | Otherwise the answer changes when the source is regenerated |
| Raster country-index grid instead of polygons? | No | Cheaper, but moves the complexity into a generator that must rasterize correctly, and its ±55 km error is wrong often enough near borders that the row's disclaimer would be doing real work |
| Relocate the bundle to serve chunks? | **No** | `plugin/environment.go:558` copies the whole `bundle_path` directory to the static dir, so chunks beside `main.js` are already served at `/static/plugins/<id>/`. Draft two's relocation was unnecessary and would have been banned by §13's own test |
| Widen `RowID` to admit `'map'`? | No, a separate `HideableID` union | `LocationPanel.tsx:270` is `Record<RowID, string>` as an exhaustive literal; widening breaks `check-types` |
| Keep Phase 2? | Yes, as chosen, with the dissent recorded | Two reviews argued for cutting it: its output is superseded at Phase 3, and `Makefile:194-196` copies `public/` with no exclusion, so it puts a hazard on the signed-release path. The dist guard and the committed-tree test are the answer to that, and they are the cost of keeping it |
| Widen the page CSP? | No | The reasoning at `page.go:221-235` is about a public route echoing author text. `img-src` and `font-src` being **absent** is what blocks `<image>`, `<feImage>` and CSS `url()`, so their absence is tested, not just the policy's sameness |
| Admin switch? | No | Every existing switch governs decoration; a render switch would make historical links change |
| Map on by default? | Yes | An off-by-default headline feature is indistinguishable from a broken one |
| Map as a `Row`? | No, a section id | `Rows` entries carry a string `Value` and the panel filters on it |
| Rename `KnownRow`? | No | One call site, and `SectionMap`'s own name already says it is not a row |
| Cell or circle? | **Cell** | A radius doubles the uncertainty, cannot express 111 km tall by 91 km wide, and reads as a range ring to this audience |
| Synced zoom constant? | A target **ground span**, not a zoom level | Mercator scale is 1/cos(lat), so a constant zoom is not a constant answer |
| Generated artifacts committed? | Yes, unlike `manifest.go` | A clean checkout must build and an air-gapped `go test` must run without a generator |
| Idempotence check? | No, `git diff --exit-code` | A generator edited without regenerating is idempotent and drifted |
| Error codes for the map? | None | `renderMapSVG` runs after `validateParams` and is pure arithmetic. There is no branch to name, and a code with no call site passes both existing guards and rots |
| Altitude now? | No | No grammar parses it and nothing consumes it. `Location` holds text not floats so `canonical()` round-trips; adding fields there is where that guarantee lives. Phase 3, with CoT |
| A GeoJSON normalization layer? | No | `Location.Point()` is already the normalization |
| YAML config from the request? | Not implementable | The settings schema is flat, and §12 explains why v1 needs no key |

## Files to Modify

### Phase 1a, no new dependencies

| File | Change |
|---|---|
| `build/mapdata/main.go` | **New.** Stdlib-only generator, emits `paths.go` and `admin.go` |
| `build/mapdata/source/` | **New.** Natural Earth 110m land, lakes, boundary lines, `ne_110m_admin_0_countries` |
| `Makefile` | `map-data`; CI `git diff --exit-code` check |
| `server/decorators/location/mapdata/paths.go` | **New, generated, committed.** Pre-projected paths |
| `server/decorators/location/mapdata/admin.go` | **New, generated, committed.** Admin-0 polygons at source precision, `ADMIN` field, per-feature bbox |
| `server/decorators/location/mapsvg.go` | **New.** `renderMapSVG`, floats only, clipped, classed |
| `server/decorators/location/mapsvg_test.go` | **New.** Anchors, cell sizing, allowlist XML, byte budget |
| `server/decorators/location/region.go` | **New.** Bbox prefilter, point-in-polygon, `RegionText()` |
| `server/decorators/location/region_test.go` | **New.** Ocean, shared border, antimeridian, feature ordering |
| `server/decorators/location/format.go`, `format_test.go` | Numeric resolution accessor; the separate cell fixture table |
| `server/decorators/location/page.go` | SVG above the table, the polar line, map CSS custom properties in `pageStyles` |
| `server/decorators/location/rows.go` | `region` row; `SectionMap`; `KnownRow` widened (both inert until 1b) |
| `server/decorators/location/convert.go` | `Region` only; drop the doc comment, move its constraint to `CLAUDE.md` and a test |
| `server/decorators/location/webapp_sync_test.go` | Row catalog with `region`; `(name, type)` pairs |
| `server/decorators/location/location_test.go` | Update `TestRenderPageHasNoLeadLine...` for the accessible name |
| `server/command_check.go` and its test | The Region row in `/tactical-fusion check` |
| `webapp/src/decorators/location/rows.ts` | `region` row |
| `webapp/src/decorators/location/convert.ts` | `region`; `Number.isFinite` validation, not a bare cast |
| `webapp/src/decorators/location/format.ts`, `format.spec.ts` | Export the numeric resolution; the separate cell fixture table |
| `webapp/src/decorators/location/LocationPanel.tsx` | Region row value, rendered outside `remote()` |
| `public/help/panel.html` | The map, the Region row, its caveat and its ~1 km generalization error |
| `public/help/help.html` | Nav |
| `CLAUDE.md` | Mapping section |

### Phase 1b, the panel

| File | Change |
|---|---|
| `webapp/package.json` | `maplibre-gl`, version pinned |
| `webapp/webpack.config.js` | `chunkFilename` with `[contenthash]`. **Nothing else** |
| `webapp/src/index.tsx` | `__webpack_public_path__`, validated, before any dynamic import |
| `public/map/world.geo.json` | **New, generated, committed.** Basemap for MapLibre |
| `build/mapdata/main.go` | Also emit `world.geo.json` and its digest |
| `Makefile` | The `world.geo.json` budget check |
| `server/decorators/location/convert.go` | `Lat`, `Lon` |
| `server/decorators/location/webapp_sync_test.go` | The `(name, type)` rewrite; drift test; `MAP_ID` with its own regex |
| `webapp/src/decorators/location/rows.ts` | `MAP_ID` (distinct shape); `HIDEABLE`; `isHideableID`; `HideableID` |
| `webapp/src/preferences/types.ts` | `asRowIDs` uses `isHideableID`; `hiddenRows: HideableID[]` |
| `webapp/src/decorators/location/convert.ts` | `lat`, `lon` |
| `webapp/src/decorators/location/Customize.tsx` | Map entry; fix `shown` for the non-row id |
| `webapp/src/decorators/location/map/LocationMap.tsx` | **New.** States, cell, zoom, caption, teardown, resize |
| `webapp/src/decorators/location/map/maplibre.ts` | **New.** Lazy import, WebGL2 probe, style builder, no glyphs |
| `webapp/src/decorators/location/map/basemap.ts` | **New.** Fetch, digest check, module cache, no retry |
| `webapp/src/decorators/location/map/span.ts` | **New.** Ground span, max zoom, Mercator limit |
| `webapp/src/decorators/location/map/*.pw.tsx`, `*.spec.ts` | **New.** Component and unit tests |
| `webapp/src/decorators/location/LocationPanel.tsx` | Mount the map, gated on preferences |
| `docs/SECURITY.md` | The dependency's upgrade-not-suppress posture |

### Phase 2

| File | Change |
|---|---|
| `Makefile` | `map-tiles`, GDAL-gated, excluded from `dist` |
| `.gitignore` | `public/map/tiles/` |
| `server/help_docs_test.go` | Walk `public/map/` as well as `public/help` |
| `webapp/src/decorators/location/map/tiles.ts` | **New.** Validated scalars only; URL built locally |
| `webapp/src/decorators/location/map/LocationMap.tsx` | Basic / Relief toggle, per-source `maxzoom` |
| `webapp/src/preferences/types.ts` | Persist the basemap choice |
| `docs/SECURITY.md`, `docs/` | Unauthenticated `public/`; signature invalidation; scan-your-own-pack |
| `public/help/panel.html` | The toggle, worded conditionally |

## Tasks

### Phase 1a

1. [ ] `build/mapdata/`, stdlib-only; check in Natural Earth source; emit `paths.go` and `admin.go`
2. [ ] `make map-data` and the CI `git diff --exit-code` check
3. [ ] `renderMapSVG`: Web Mercator, clipped to the extent, classed elements, floats only
4. [ ] Anchor tests against outside authority before any round trip
5. [ ] Numeric resolution accessors in Go and TypeScript; the separate cell fixture table
6. [ ] The cell: per-axis rectangle, MGRS bounding rectangle, paired fixtures
7. [ ] Polar and antimeridian behaviour, with tests, and the two polar strings
8. [ ] `region.go`: bbox prefilter, ray cast, `ADMIN` field, lowest-index tiebreak
9. [ ] `region_test.go`: ocean, shared border, antimeridian, stable feature ordering
10. [ ] Wire the SVG and the Region row into `renderBody`; map CSS in `pageStyles`
11. [ ] Accessibility on the page SVG; update `location_test.go:452`
12. [ ] `Region` on `Conversion`; row catalog both sides; panel renders it outside `remote()`
13. [ ] Region row in `/tactical-fusion check`
14. [ ] `SectionMap` and the `KnownRow` widening, server side only, inert until 1b
15. [ ] Help docs including the ~1 km generalization figure; `CLAUDE.md` Mapping section
16. [ ] `make check-style && make test && make sbom-audit`

### Phase 1b, spikes first

17. [ ] Pin the `maplibre-gl` version and confirm its ESM/WebGL2 posture against that exact version
18. [ ] Confirm a `[contenthash]` chunk in `webapp/dist/` resolves at `/static/plugins/<id>/` on a **root** install and on a **subpath** install
19. [ ] Confirm MapLibre runs under Mattermost's CSP, including the blob: worker and `style-loader`'s inline `<style>`; fallback is `setWorkerUrl()` at a same-origin file
20. [ ] Confirm Playwright's headless chromium provides WebGL2; decide stub versus drive
21. [ ] Measure time-to-first-map, cold cache, first coordinate opened
22. [ ] `chunkFilename`; validated `__webpack_public_path__` in `index.tsx` only
23. [ ] Generator also emits `world.geo.json` and its digest; budget check; drift test
24. [ ] `Lat`/`Lon` on `Conversion`; rewrite the sync test to compare `(name, type)` pairs
25. [ ] `HideableID`, `MAP_ID` in a distinct shape, `HIDEABLE`, `isHideableID`, `asRowIDs`
26. [ ] Fix `Customize.tsx:155`; add the map entry; round-trip CT test
27. [ ] `span.ts` with span, max zoom and the Mercator limit; paired anchors; `basemap.ts`
28. [ ] `maplibre.ts`: lazy import, WebGL2 probe, style from CSS variables, no glyphs, no rotate
29. [ ] `LocationMap.tsx`: the state table, cell, span-derived zoom, caption, scale bar, recenter, cooperative gestures, teardown, resize
30. [ ] Mount gated on preferences; height budget with a test; Open larger link
31. [ ] Accessibility on the canvas; reduced motion; forced colors
32. [ ] `make check-style && make test && make sbom-audit`

### Phase 2

33. [ ] `make map-tiles`, GDAL-gated, documented, excluded from `dist`
34. [ ] `tiles.json` schema; `tiles.ts` reading validated scalars and building its own URL
35. [ ] Basic / Relief toggle, per-source `maxzoom`, persisted choice, reserved space
36. [ ] `TestBundleShipsNoRasterTilesByDefault` against the committed tree
37. [ ] Extend the self-contained test to `public/map/`
38. [ ] Security and operator documentation

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| The lazy chunk 404s | Emit beside `main.js`; Mattermost already copies that directory to `/static/plugins/<id>/`. Spike both root and subpath installs |
| `__webpack_public_path__` breaks Playwright CT | Assign it in `index.tsx` only. Vite serves the component-test import graph as strict-mode ESM, where a module-scope assignment is a `ReferenceError` |
| The Region row reads `unavailable` over open ocean | It renders outside `remote()`, only when the conversion is ready. Empty then means omitted, not pending |
| Quantized borders develop gaps and overlaps | `admin.go` keeps source precision; the 2-decimal rounding is display-only and does not reach it |
| Hiding the map silently reverts | `isHideableID` on the read path plus a round-trip CT test, not a write-only one |
| A pin drawn 550 km off at high latitude | No map beyond ±85.05°, with a test. Only textual grammars reach it, so fixtures will not find it |
| The cell overstates uncertainty | Sized from the cell edge per axis, with paired fixtures |
| The page ships 400 KB per request on DDIL | Clipped at generation; a 60 KB per-response budget with a test |
| Mobile dark mode gets a light map | No `theme` argument; CSS custom properties resolve it |
| A GHSA against `maplibre-gl` blocks unrelated PRs | Upgrade or pin. Suppression is **not** available for a shipped runtime dependency |
| WebGL2 absent on hardened VDI, the actual target | 1a works without it. In 1b, an ordinary degrade; the Region row still answers the question |
| WebGL contexts leak until maps go black | `map.remove()` on teardown, plus a mount/unmount CT test |
| The two artifacts drift | One generator, bidirectional counts, `git diff --exit-code` |
| The generator's dependencies enter the Grype gate | Stdlib only, following `build/manifest/` |
| Readings pushed below the fold | 180 px budget with a test, measured against 1366x768 |
| A tile pack leaks an install's AO | `public/` is unauthenticated; documented in the imperative, public-domain basemaps only, authenticated route deferred to Phase 3 |
| A repacked bundle fails its own signature | Document `make map-tiles && make dist` only; state it in `docs/SECURITY.md` |
| A stray local pack ships silently | Test against the committed tree; `make dist` guard |
| `TestRenderPageHasNoLeadLine...` breaks | Expected, from the accessible name, not from the SVG itself. Update its intent deliberately |

## UX Summary

| Scenario | Behaviour |
|---|---|
| Reader clicks a `dd`/`ddh`/`dms`/`ddm`/USMTF coordinate | Map with pin immediately; Region row after the request |
| Reader clicks MGRS or UTM | Basemap immediately, `converting…`, then the pin |
| Conversion fails | Basemap, no pin, `unavailable`. Every local reading still shown |
| Link is not a coordinate this plugin issued | Whole panel is `Not a coordinate`, as today |
| Coordinate is `0, 0` | A pin at null island, like any other position |
| Coordinate is at 89.9° N | No map, one line. Every reading renders |
| Coordinate is at 179.95° E | Map clamped at the antimeridian, cell truncated |
| Coordinate is in open ocean | Map and pin; Region row omitted |
| Token is a 111 km USMTF LATD | A visible cell, 111 km by 91 km at 35°N |
| Token is a ten-figure MGRS | A dot; the caption carries the resolution |
| Browser has no WebGL2 | One line where the map would be. Region row and table intact |
| Reader hides the map | Panel is exactly what it is today, and it stays hidden after a reload |
| Reader had customised rows before upgrade | Map appears, because the stored list names what was hidden |
| Reader hides everything | The warning fires correctly, and Customize is the way back |
| Reader on mobile opens the link | The standalone page, with the SVG map above the table |
| Reader on the page wants the map hidden | Not possible; the page has no session. Documented |
| Reader wants a bigger map | Open larger, to the standalone page |
| Reader scrolls the sidebar over the map | The page scrolls; zoom needs ctrl or the buttons |
| Reader pans away | Recenter control |
| Relief pack installed | A Basic / Relief toggle, remembered, capped at the pack's own zoom |
| Relief pack absent | No toggle, nothing logged |

## Testing Plan

**Go unit**
- Web Mercator anchors, outside authority, before any round trip: `0, 0` → unit `0.5, 0.5`; lat 85.0511287 → `0`; lon 180 → `1`
- `TestPolarPositionDrawsNoMap`, paired with `TestNullIslandDrawsAPin`
- `TestAntimeridianCellIsClamped`
- `TestCellMatchesTheResolutionRow`, per axis, over `renderFixtures`
- `TestRegionIsOmittedOverOcean`; `TestRegionCitesItsSource`
- `TestProjectedPathsMatchTheGeoJSON`, sampling **and** bidirectional counts
- `TestBasemapFitsItsBudget`; `TestPageResponseFitsItsBudget`
- `TestPageMapIsAnAllowlist`, parse the emitted SVG with `encoding/xml` and
  permit only `svg`, `g`, `path`, `rect`, `circle`, `title`, `desc` and a fixed
  attribute set. **An allowlist, never a denylist**, matching
  `allowedRawRunes`. Assert it parses to EOF as well-formed XML, since the SVG
  precedes the table and an unbalanced one swallows the rest of the document.
  Ban `<![CDATA[` and `<style>` inside the SVG explicitly: both are live in
  foreign content and both defeat `html.EscapeString`
- `TestPageCSPHasNoImgSrcAndNoFontSrc`, their **absence** is what blocks
  `<image>`, `<feImage>` and CSS `url()`. Assert directive shape, not the digest,
  which legitimately changes when `copyScript` does

**Go sync**
- `Conversion` as `(name, type)` pairs, in order
- Row catalog including `region`; `MAP_ID` against `SectionMap`
- Target ground span and max zoom against `span.ts`

**TypeScript unit**
- Projection and span anchors, the same table as Go
- `convert.ts` rejects `undefined`, `NaN` and out-of-range, and accepts `0`
- `basemap.ts` caches, never throws, never retries, honours the digest
- `tiles.ts` rejects an absolute URL, a scheme, `//` and `..`

**Component (Playwright CT)**
- Hide the map, re-read from the wire, assert still hidden
- `Customize` warning fires at zero rows and not before
- Grid link shows `converting…` then a pin
- `failed` shows `unavailable` and no pin
- Map not mounted while preferences load
- WebGL2 absent leaves every row and the Region row
- Twenty mount/unmount cycles leak no context
- Panel height stays inside budget at 1366x768
- Toggle absent when `tiles.json` 404s; rejected when it names an absolute URL

**Manual, before merge**
- `make deploy`; post one token per grammar; open each in light and dark
- Pull the network cable and reload
- Open a link on the mobile app and confirm the SVG page
- Screen reader pass on both surfaces; Windows High Contrast; 200% zoom

## Acceptance Criteria

- [ ] **1a**: every recognized grammar shows a map and a Region row on the standalone page
- [ ] **1a**: no npm dependency added, no webpack change, no webapp build change
- [ ] **1b**: the panel shows the same map; the chunk loads at `/static/plugins/<id>/` on root **and** subpath installs; `plugin.json` and `Makefile` are unchanged
- [ ] Both render with no network, first load, cold cache
- [ ] `server/decorators/page.go`'s CSP string is byte-identical, and a test pins the absence of `img-src` and `font-src`
- [ ] **1a**: the page's response stays under its budget (number set from the first measured run, then pinned as a regression bound)
- [ ] **1b**: `world.geo.json` under 400 KB raw and 120 KB gzipped
- [ ] **1b** entry chunk (`main.js`) grows by less than 20 KB gzipped; the lazy chunk is unbudgeted but measured and recorded
- [ ] A 111 km token and a 1 m token are visibly different, and the 111 km cell is 111 km, not 222 km
- [ ] No pin is drawn when the position is unknown, unrenderable, or beyond ±85.05°
- [ ] `0, 0` draws a pin
- [ ] Every map failure leaves every reading and the Region row on screen
- [ ] Hiding the map survives a reload; Restore defaults brings it back
- [ ] Both surfaces pass a screen-reader and keyboard pass; 3:1 non-text contrast in both themes
- [ ] The Region row names an administering entity, cites Natural Earth 110m, and is omitted rather than shown as `unavailable` where there is no answer
- [ ] The panel fits its 180 px map budget at 1366x768 with the Resolution row above the fold
- [ ] `make check-style && make test && make sbom-audit` pass
- [ ] `make map-data && git diff --exit-code` is clean
- [ ] **Phase 2 only**: a built pack produces a toggle; no pack produces silence; a stray local pack cannot ship

## Checklist

- [ ] **Diagnostics**: no diagnostics channel in this plugin; map failures degrade in place
- [ ] **Slash command**: `/tactical-fusion check` prints readings as text and gains nothing from a map. It **does** gain the Region row, which is text, add it there
- [ ] **Error codes**: none allocated, deliberately. See Decisions
- [ ] **Help docs**: `panel.html` covers the map, the cell, the Region row's caveat, and hiding it
- [ ] **`CLAUDE.md`**: a Mapping section recording why vector, why two renderers, why the CSP did not move, why the Region row cites its source, and that the SVG's safety depends on the script digest pin so nobody reverses that decision without seeing what it now protects

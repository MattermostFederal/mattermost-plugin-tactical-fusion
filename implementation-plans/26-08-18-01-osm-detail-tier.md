# An OSM detail tier above z9

> **Status: partly superseded. This is a historical record of what was planned,
> not a description of what shipped.** It is kept for the reasoning behind the
> design. Read `docs/design/mapping.md` for the implemented contract; where the
> two disagree, that file is right and this one is not to be used to "restore"
> anything.
>
> Known divergences, all deliberate:
>
> - **One archive became one archive per region.** The plan assembles every
>   region into a single `public/map/detail.pmtiles`. What shipped is one
>   `<command>-<area>.pmtiles` per region, because a merged archive declares one
>   rectangular bounds covering everything between its regions, so coverage was
>   unknowable and stale, partial, uncovered and truncated all rendered alike.
> - **Build-time regions became a runtime package system.** The plan has no
>   package route and no package state, on the argument that it keeps the
>   feature small. An area an operator cannot add without a plugin release is an
>   area they cannot add, so `server/packages.go`, `/api/v1/packages`, the
>   `/packages` route and `LocationMapPackagesDir` all exist.
> - **A buffered `osmium extract` was abandoned.** A multi-country region is
>   merged rather than cut, because cutting drops a way whose vertices all fall
>   outside the box and would erase a national border invisibly.
>
> Nothing below this banner has been rewritten to match, deliberately: the value
> of a plan is what it says was intended at the time.

## Overview

Natural Earth stops being the right source somewhere around z9. This adds a
second, OpenStreetMap-derived tier in the OpenMapTiles schema, drawn only from
z10 up, built per region rather than per planet, and assembled into one extra
archive by a build profile. The global Natural Earth tier is untouched.

Phase 1 ships one real region, **Hawaii**, so that every new invariant runs in
CI rather than skipping, and so that the licence and rendering work is exercised
by bytes that actually ship. Which regions and which depth ship after that is a
measurement, and Task 1 is to take it.

**Why Hawaii is the right pilot, and where it is worse than a trivial one.** It
is on the requested list, it carries INDOPACOM's own headquarters, and unlike a
single small island it exercises nearly everything this tier has to get right:
a metro area for the label collision index and `place` ranking, a real road
hierarchy down to `tertiary`, named water, and two international aerodromes.

It also exercises **the design's one live fork for free**. The main islands span
about 600 km with open ocean between them, so a single bbox is mostly empty
tiles, which is exactly the disjoint-coverage case Task 1 has to trace. A
compact pilot would have proved the rendering and left that question untouched.

The cost is that it is **not trivially small**. Hawaii is roughly fifty times
Guam's land area and contains Honolulu, so where Guam was 1-4 MB this is an
estimate with real spread and a real chance of eating a third of the bundle
headroom on its own. That is a Task 1 measurement and it has a stated fallback;
see the size table and the risk register.

## Problem statement

`MAX_ZOOM` is 17 and `DATA_MAX_ZOOM` is 9. Between them MapLibre overzooms:
lines stay crisp and only their generalisation is wrong, so a coastline that may
be five kilometres from where it is gets drawn at street scale with nothing on
screen saying so. The repo already records that as the accepted cost of letting
a reader inspect a fine cell at its own size, and records the notice that used
to say so being removed.

For an audience acting on grid references, the honest fix is not a notice. It is
data that is actually good at those zooms, which for roads, coastlines, towns
and airfields means OpenStreetMap.

**State the value narrowly, because it is narrower than it looks.**
`TARGET_SPAN_METERS` is 400 km, which opens the panel at about **z6**, and
`zoomForSpan` clamps to `DATA_MAX_ZOOM` on top of that. So no surface ever
*opens* onto this tier: it is four or more zoom levels below the first thing a
reader sees, and it is reached only by a deliberate gesture. That gesture is
exactly what `MAX_ZOOM = 17` was raised to 17 to permit, so the tier is not
speculative, it is the data that gesture currently lands on being wrong. But the
cost is paid by every install and the value is opt-in per view, and **whether
`TARGET_SPAN_METERS` should change once detail exists is the largest open
product question here.** It is deliberately not answered in this plan; it is
named so that Phase 2 answers it before spending bundle headroom.

## Current state

| Thing | Today |
|---|---|
| Basemap | `public/map/world.pmtiles`, 43,074,410 bytes, committed |
| Schema | bespoke: ten layer names invented in `build/maptiles/build.sh` |
| Source | Natural Earth 110m/50m/10m, public domain, no credit printed |
| Depth | z0-9 (`MAXZ`, `DATA_MAX_ZOOM`, `TestArchiveDepthMatchesTheData`) |
| Camera | z0-17 (`MAX_ZOOM`), overzoomed past 9 |
| Opening view | ~z6 (`TARGET_SPAN_METERS`, clamped to `DATA_MAX_ZOOM`) |
| Style | one vector source, 17 layers, `buildStyle()` in `maplibre.ts` |
| Layer order | `cell-fill`, `cell-outline`, `pin` are last, and `style.spec.ts` asserts every symbol layer precedes them |
| Client | `basemap.ts` probes a 127-byte header, one archive, no TTL |
| Controls | nav top-right, scale bottom-right, zoom readout bottom-left, `attributionControl: false`, and **no controls at all in preview** |
| Toolchain | tippecanoe 2.78.0 + fontnik in a pinned container, `make map-tiles` |
| Distribution | everything bundled; nothing is fetched from outside the plugin |

### Current gaps

- No source that is correct at z10-14.
- The style has no seam: every layer draws at every zoom above its `minzoom`,
  so there is nowhere for a second tier to take over.
- `basemap.ts` and `TestArchiveCarriesEveryLayerTheStyleDraws` both assume
  exactly one archive; `styleSourceLayers` in `archive_test.go` scrapes
  `'source-layer': '...'` out of the whole file with no notion of which source a
  layer belongs to.
- No attribution surface anywhere. Deliberately: Natural Earth is public domain
  and the credit was dropped from all three surfaces. That precedent does not
  survive contact with OSM.

## The size arithmetic, which decides the shape of everything below

### The ceilings, from the repo's own prior measurement

`implementation-plans/26-08-16-01-basemap-detail-budget.md` established these
and they still hold:

| Ceiling | Value | Note |
|---|---:|---|
| `FileSettings.MaxFileSize` | 104,857,600 | gates plugin upload |
| bundle today | ~68 MB | 51 MB when the archive was 25 MB; it is now 43 MB |
| **in-bundle headroom** | **~30 MB** | |
| documented nginx `client_max_body_size` | 50 MB | **already exceeded**, still unverified |
| git history | unbounded, and every rebuild adds a full copy | no LFS, `*.pmtiles binary` |

### What OSM at z10-14 actually weighs

Anchors with an authority outside this repo: Planetiler reports **81 GB** for
the planet at OpenMapTiles z0-14 (v0.10.1), and Protomaps reports ~120 GB for a
planet at z0-15, with z0-14 roughly half of that. Land area is ~149M km².

The thirteen named areas of responsibility are roughly 4-5M km² of land, about
3% of the total, weighted **above** average density (Japan, Korea, Taiwan,
Ukraine, the Baltics) and **below** it (Horn of Africa, and three areas that are
mostly water). Filtering to the layers this plugin draws plausibly removes half
to two thirds of a full OpenMapTiles build, since buildings, POIs, landcover,
landuse and minor ways are most of it.

**Estimate, to be replaced by measurement in Task 1. Plan against the upper
bound**: one reviewer put dense single countries at 150-300 MB each at z14 and
the whole set past 4 GB, which is above this range and is not obviously wrong.

| Profile | Estimate | In-bundle? |
|---|---:|---|
| DoD set, z10-14 | 0.8 - 4 GB | no, by 30-130x |
| DoD set, z10-13 | 200 MB - 1 GB | no, by 10-30x |
| DoD set, z10-12 | 50 - 250 MB | no, by 2-8x |
| DoD set, z10-11 | 12 - 60 MB | **borderline** |
| Taiwan alone, z10-14 | 30 - 150 MB | no |
| **Hawaii, main islands, z10-14** | **8 - 30 MB** | **probably, and it is the pilot** |
| Hawaii, Oahu only, z10-14 | 3 - 10 MB | fallback if the above overruns |
| Guam alone, z10-14 | 1 - 4 MB | second fallback |

Tiles quadruple per level, so **z10-12 is about fifteen times cheaper than
z10-14** and z10-11 about seventy times cheaper. That is the whole trade and it
is not linear in anything a reader would guess.

### The three consequences

1. **z14 for the DoD set cannot ship in the plugin bundle.** Not by tuning: by
   one to two orders of magnitude.
2. **It cannot be committed to git either**, for the reason the previous plan
   found: pmtiles do not delta-compress, so every rebuild is a permanent full
   copy in every clone forever.
3. **Something much smaller can ship now.** Hawaii's main islands at full depth
   are an order of magnitude under the DoD set, are on the requested list, and
   are what makes Phase 1 a shipped feature with live guards rather than a
   spike. If the measurement puts it over the headroom, the pilot narrows to
   Oahu rather than the phase changing shape.

## The measurement, taken (Task 1)

Hawaii main islands, z10-14, from `hawaii-260818.osm.pbf`. **The estimate was
8-30 MB. The measured figure is 6.81 MB**, which is under the bottom of the
range rather than inside it.

| Stage | Bytes |
|---|---:|
| planetiler output | 11,329,212 |
| after the `tile-join` class filter | **6,808,864** |

Per zoom, from planetiler's `--output_layerstats` (uncompressed layer bytes,
before the class filter):

| Zoom | Bytes | Share | Tiles |
|---|---:|---:|---:|
| z10 | 309,281 | 1.7% | 234 |
| z11 | 616,843 | 3.3% | 840 |
| z12 | 1,608,614 | 8.7% | 3,234 |
| z13 | 4,561,215 | 24.6% | 12,810 |
| z14 | 11,413,882 | 61.7% | 50,447 |

**The zoom curve is the number that decides Phase 2**, and it corrects this plan
in the safe direction: z10-12 is **13.7%** of a z10-14 build and z10-11 is
**5.0%**, where the estimate above said about 6.7% and 1.5%. Tile counts
quadruple per level but per-tile bytes shrink, so the two effects partly cancel
and depth is roughly seven times cheaper than assumed. Everything else in the
size table stands.

Extrapolating Hawaii's 240 bytes per km² of land over the roughly 4.5M km² of
the thirteen areas of responsibility puts the full set **on the order of 1 GB at
z10-14**, which is inside the estimated range and near its bottom. Against ~30 MB
of headroom that is still out by a factor of thirty, with z10-12 out by three to
eight and z10-11 at the boundary. **Hawaii is more completely mapped than the
Horn of Africa and less than Japan, so Taiwan must be measured before Phase 2
spends anything.**

The build is byte-reproducible from the pinned sources: two runs produced the
identical archive, `960e3a27b26ceb56228283359c65dee963a83d4ad781c5686d535eb08b7dde64`.

The gap behaviour was confirmed: Hawaii's bbox is mostly ocean, the archive
carries no tiles there, and the pmtiles client resolves the absence in its own
directory rather than issuing a request per missing tile.

The full breakdown, per layer and with the decisions behind it, is in
`build/maposm/README.md`.

## Amendment, 2026-08-19: packages are separate files, dropped in or uploaded

At the author's direction, the "one archive, build-time regions" decision is
**reversed**. Each region becomes its own file, named
`<command>-<area>.pmtiles` (`indopacom-hawaii.pmtiles`), and an operator adds
one by putting it in a directory or uploading it in the System Console.

### What this costs, which is what the original decision was buying

A runtime package system: discovery, a list on the wire, per-package sources and
layers in the style, a route that serves files the bundle does not contain, and
an upload path with its own authorisation and validation. All of that was out of
scope and is now in it.

### What it buys, including one thing the plan had written off

- **Coverage becomes knowable.** This is the real gain. A PMTiles header carries
  one rectangular bounds, so the merged archive declared nearly the whole world
  and filtered nothing, and the plan recorded that "stale, partial, uncovered and
  truncated all render identically" as an accepted limitation with its fix
  deferred to Phase 3. Separate files each carry a tight bbox, so MapLibre skips
  out-of-area requests **and** the plugin can tell a reader which areas it has.
- **An operator can add an area without this repository.** The generator, the
  toolchain and the release cycle stop being on the path.
- **The bundle stops growing with coverage.** The 30 MB headroom is no longer
  what caps how much detail an install can have, which was the constraint every
  size table in this plan is organised around.

### The storage constraint, which decides the shape

**There is no streaming file API.** `plugin.API.ReadFile` and `GetFile` both
return the whole file as `[]byte`, and PMTiles is read by byte range, so any
storage reached through the plugin API loads an entire archive into memory on
every tile request. At Hawaii's 6.8 MB that is merely wasteful; at a real
region's 100-500 MB it is fatal.

`os.Open` with `http.ServeContent` is therefore the only viable reader: it
answers Range without reading the whole file. That fixes two things at once:

- the drop-in location is a **real directory on the server's filesystem**, named
  by an admin setting;
- an **upload** has to land in that same directory, because there is nowhere
  else the server can range-read from.

**In a cluster every node needs that directory**, which in practice means shared
storage or a copy per node. An upload reaches only the node that served it.
That is an operational requirement rather than a defect, and it has to be said
plainly in the admin help rather than discovered.

### The shape

| Concern | Decision |
|---|---|
| Naming | `<command>-<area>.pmtiles`, lower case, validated against a strict pattern because the name reaches a URL |
| Bundled packages | `public/map/packages/`, served by Mattermost's own static handler, which already answers Range for `world.pmtiles` |
| Dropped-in packages | an admin-configured directory, served by a new plugin route through `http.ServeContent` |
| Discovery | the server lists both, validates each header, and serves `{name, url}`; the client does not care which is which |
| The pages | get the list in the shell, as `data-packages`, because `/api/v1` needs a session the pages do not have |
| The style | one vector source and one layer set **per package**, generated from the same definitions the single `detail` source used |
| Upload | a `custom` admin console setting, which is the only setting type that can carry a file, backed by an upload route restricted to system administrators |

The seam, the caps, the credit, the probe rules and every measurement above are
unchanged: this changes where archives come from and how many there are, not
what is in them or how they are drawn.

## Phase strategy

| Phase | Focus | Value |
|---|---|---|
| **Phase 1** | Generator, schema, the z10 seam, attribution, **Hawaii shipped**, everything measured | the mechanism, proven end to end and running in CI |
| **Phase 2** | Answer the opening-zoom question, then choose and ship the largest profile that fits | most of the reader-visible value |
| **Phase 3** | Out-of-bundle distribution, and a coverage list so a reader can tell "no package here" from "broken" | the original z14 ask |
| **Phase 4** | Non-Latin glyphs, per-package switches, a data currency policy | deferred |

### Phase 1 scope (this plan)

Everything below is Phase 1 unless a task says otherwise.

## Design principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Schema | OpenMapTiles, unmodified layer and field names | a bespoke schema | user request; `planetiler-openmaptiles` |
| Generator | Planetiler in a pinned container, writes `.pmtiles` directly | the OpenMapTiles PostGIS pipeline | `build/maptiles/Dockerfile` |
| Class filtering | a `tile-join -j` pass, since Planetiler filters whole layers only | forking the OMT profile | `build.sh` already runs `tile-join` |
| Regions | a **build-time** bbox list joined into ONE archive | a runtime package manager | `tile-join` already merges parts |
| The seam | a per-source layer partition at exactly z10, **unconditional** | a cap that depends on what shipped | argued below |
| Layer order | detail layers go **before** `cell-fill`, `cell-outline` and `pin` | appending | `style.spec.ts` already asserts it |
| Probing | one fetch-and-latch machine, two accept rules | two copies of the latching logic | `basemap.ts` records getting it wrong twice |
| Attribution | a text line beside the map | a MapLibre control | control corners are already spoken for |
| Distribution | same-origin, under `public/`, in Phases 1 and 2 | an external tile host, a widened CSP | `PageMapping` |

## Reference patterns

- `build/maptiles/build.sh` + `Dockerfile` + `sources.lock` - the whole shape of
  a pinned, containerised, committed-output generator. The new one is a sibling.
- `build/maptiles/README.md` - where the decisions inside a generator live.
- `webapp/src/decorators/location/map/basemap.ts` - the definitive/transient
  split, the in-flight identity check, the memo with no TTL.
- `server/decorators/location/archive_test.go` - the only place that can see
  both the binary and the TypeScript. Every new invariant lands here.
- `LocationMap.tsx`'s caption row - an existing place for text beside the map.

## The technical approach

### 1. A second generator, `build/maposm/`

A sibling of `build/maptiles/`, not an extension of it: different source,
different toolchain, different output, different licence obligations. There is
no line of `build.sh` a Planetiler run would reuse, and sharing a directory
would put an ODbL pipeline and a public-domain one behind one README and one
`sources.lock`, when the licence text has to travel with exactly one of them.

```
build/maposm/
  Dockerfile        planetiler pinned by version, JRE base, tippecanoe for tile-join
  regions.txt       name, buffered bbox, and which profiles it belongs to
  build.sh          one planetiler run per region, a filter pass, then tile-join
  fetch-sources.sh  Geofabrik extracts
  sources.lock      each extract by digest AND date
  README.md         the decisions inside it
```

`make map-osm PROFILE=<name>` writes `public/map/detail.pmtiles`.
`make osm-sources` fetches the extracts, as `map-sources` does today. Neither is
a prerequisite of anything, and the output is committed only for a chosen
profile.

**A region is a bbox and nothing else.** No polygons, no per-region metadata on
the wire, no runtime notion of a package. The DoD bundle is a column in
`regions.txt`. **Phase 1 writes only the pilot rows**; the other twelve bboxes
are data with no consumer until the measurement says which of them survive it.

**Filtering happens in two places, because the tools split it that way.**
`planetiler-openmaptiles` offers `--only-layers` / `--exclude-layers`, which is
whole layers and no finer. Restricting `transportation` to the classes this
plugin wants needs either a forked profile or a filter pass, and the filter pass
is much cheaper: tippecanoe's `tile-join` takes `-j`, a per-layer filter in the
Mapbox GL Style filter syntax, and `-x` to drop attributes, and it already reads
and writes PMTiles in the version this repo pins, which is how
`build/maptiles/build.sh` assembles its own archive today. **The merge is
reading every tile anyway**, so the class filter is free there and needs no
fork.

Layers kept: `transportation`, `transportation_name`, `place`, `water`,
`water_name`, `waterway`, `aeroway`, `aerodrome_label`, `boundary`. Dropped:
`building`, `housenumber`, `poi`, `landcover`, `landuse`, `park`,
`mountain_peak`.

Road classes kept: `motorway`, `trunk`, `primary`, `secondary`, `rail`, plus
**`tertiary` from z12**. Tertiary was going to be dropped and should not be: in
rural terrain it is the connecting network, and without it a z13 map of the Horn
of Africa is disconnected fragments, which is the opposite of operational
context. Dropped: `minor`, `service`, `track`, `path`, `ferry`.

`water_name` is kept for the same reason: an unnamed lake at z12 is worse
context than the Natural Earth tier it replaced.

**One archive, not many.** Each region is a separate Planetiler run bounded by
its bbox; `tile-join` merges them, exactly as `build.sh` already merges zoom
bands. This is what removes the entire runtime package system from the design
and is the single largest simplification available here.

**Bboxes are buffered.** A bbox-clipped extract truncates roads, waterways,
coastlines and boundaries at its edge and drops label anchors just outside it,
so a pilot that looks right in the middle of a region fails at its rim. Every
region is cut with a buffer (start at 0.25 degrees) and tiles are kept for the
unbuffered box, so adjacent regions meet without a seam and `tile-join` is never
asked to reconcile two versions of one tile.

**An island pilot cannot test this, and that is the one thing Hawaii is worse
at.** Its bbox edges fall in open ocean, so nothing crosses them and a
truncation bug would look identical to a correct build. The same is true of
Guam and of Taiwan, so no pilot on the INDOPACOM list exercises it. The buffer
is therefore built and documented in Phase 1 and **verified in Phase 2 against
the first continental region**, which is Ukraine, the Baltics, the Levant or the
Horn of Africa. Recorded here rather than left to be discovered when two
adjacent land regions first meet.

### 2. The seam at z10

MapLibre's layer `minzoom` is inclusive and its `maxzoom` is exclusive: a layer
with `maxzoom: 10` is hidden at zoom ≥ 10, one with `minzoom: 10` is shown at
zoom ≥ 10. The pair partitions at exactly 10 with no gap and no overlap, and
that is the whole mechanism.

| Concern | z0-10, from `basemap` (NE) | z10+, from `detail` (OMT) |
|---|---|---|
| land fill | `land`, **uncapped, still overzooms** | (OMT has no land layer) |
| water | `lakes`, capped | `water`, drawn **above** `land` |
| rivers | `rivers`, capped | `waterway` |
| water names | none | `water_name` |
| national borders | `boundary_lines`, capped | `boundary`, `admin_level<=4` |
| provincial borders | `admin_1_lines`, capped | `boundary`, `admin_level<=4` |
| roads | `roads`, capped | `transportation` |
| road names | none | `transportation_name` |
| railways | `railroads`, capped | `transportation`, class `rail` |
| urban extent | `urban_areas`, capped | (dropped; `place` carries it) |
| places | `populated_places`, capped | `place` |
| provinces | `admin_1_labels`, capped | `place`, class `state` |
| airfields | `airports`, capped | `aeroway` + `aerodrome_label` |
| country labels | `country_labels`, already `maxzoom: 6` | unchanged |

**`land` is the one NE layer that must keep overzooming**, because OpenMapTiles
has no land polygon: it draws water over a land-coloured ground. So the seam
puts accurate OMT `water` over a generalised NE `land` fill, and where the two
disagree at a coastline the accurate one wins visually, which is the right way
round. Every other NE layer in that column is capped, or the two tiers draw the
same road, the same lake and the same border twice with kilometres between them.

**Detail layers are inserted before `cell-fill`, `cell-outline` and `pin`, never
appended.** Those three are last in the array today and `style.spec.ts` asserts
every symbol layer precedes them. Appending would draw OSM roads and place
labels over the pin and over the resolution cell, at exactly the zooms this tier
exists for, and would break that test on the commit that adds the layers.

**The cap is unconditional, and that is the plan's one genuinely contested
decision.**

- *For*: above z10, Natural Earth's ~5 km of positional error is 33 px at z9 and
  65 px at z10, and `DATA_MAX_ZOOM` was 8 for precisely that reason before it
  was raised as a deliberate trade. A road drawn 130 px from where it is, at
  z12, for an audience acting on coordinates, is not context. Capping is
  removing a claim the data cannot support, which is the same argument the
  repo already makes about padding `35°00'00"N` onto a token that said `35°N`.
- *Against*: outside a covered region, z10 and up becomes land, water and
  coastline and nothing else. A reader who zooms in over Nebraska gets an empty
  frame where today they get roads, and cannot tell that from a broken archive.
- *Decision*: cap. It is one constant to reverse, `SEAM_ZOOM`, and it must be
  written down in `CLAUDE.md` and in `public/help/panel.html` in words, because
  a reader will otherwise report the empty frame as a bug.

**Coverage is not something the archive can advertise.** A PMTiles header
carries a single rectangular bounds, so a merged archive spanning Hawaii to the
Baltics declares nearly the whole world and filters nothing. The pilot reaches
this in miniature: Hawaii's own bbox is mostly ocean with no tiles in it, which
is why the trace below can be taken in Phase 1 rather than waiting for a second
region. The expected
behaviour over a gap is that the pmtiles client resolves the tile in the
archive's own directory, finds no entry, and issues no tile request, so the cost
is directory lookups rather than one wasted round trip per tile. **Task 1 must
confirm that with a network trace**, because the whole "no manifest"
simplification rests on it. If it costs a request per missing tile, the answer
is one archive per region with tight bounds, and that is a real fork in this
design rather than a tweak.

**Stale, partial, uncovered and truncated all render identically** as bare land
and water. That is an accepted Phase 1 limitation with a named Phase 3 fix: a
coverage list carried in `/api/v1/features` and in the page shell, which is the
only thing that can tell a reader "there is no package here" apart from "the
archive is broken".

### 3. The client

`basemap.ts` keeps **one** fetch-and-latch function and gains an accept rule
parameter. The latching, the timeout, the in-flight identity check and the
definitive/transient split are ninety lines whose comments record having been
got wrong twice; a second copy is two places to get them wrong again. What
actually differs between the two archives is the URL, the memo, and the zoom
rule:

- global: `minZoom == 0 && maxZoom >= DATA_MAX_ZOOM`
- detail: `minZoom == DETAIL_MIN_ZOOM && maxZoom >= DETAIL_MIN_ZOOM`

**The detail rule is a floor, not an equality.** An operator who builds z10-12,
which the estimate table treats as the likely Phase 2 answer, must not get
silence. The archive's own `maxZoom` is already returned and is what the style
uses; there is no client-side `DETAIL_MAX_ZOOM` at all, and the build depth is
pinned in Go against `build.sh` where it belongs.

**A 404 is silent; a timeout is not.** A missing detail archive is a
configuration, not a fault, and must produce no note, no warning and no banner,
because a global-only build is a supported profile. A *transient* failure is the
opposite and is the defect this design would otherwise ship: the style is built
once inside the creation effect and the map is created once and moved, so a
timed-out detail probe yields a panel with no detail source for its whole life,
indistinguishable from a correct global-only install. It is logged, it does not
latch, and the next map creation retries it. That is the same distinction
`basemap.ts` already draws, applied to the one case where the two failures look
identical on screen.

`buildStyle(archive, detail, colors)` takes an optional second archive and
inserts its layers in the right place when it is there.

**Preview mode omits the detail source entirely.** A hover card is constructed
`interactive: false` and its zoom comes from `zoomForSpan`, which cannot exceed
`DATA_MAX_ZOOM` and at 320 px returns about z6, so it can never reach z10 and
can never draw an OSM tile. Carrying a source it cannot use would mean printing
an OSM credit on a card with no OSM on it. A test pins that preview cannot reach
`DETAIL_MIN_ZOOM`, because that argument is the only thing keeping the omission
lawful.

### 4. Attribution, which is not optional

This reverses a documented decision, and the reversal is forced rather than
chosen. Natural Earth is public domain, so its credit was a courtesy and was
dropped from all three surfaces. OSM is ODbL and the OpenMapTiles schema is
CC-BY; both require credit, and the OSMF guideline names the corner of the map,
**or a position adjacent to it**, as the expected place.

**It is a text line beside the map, not a MapLibre control.** Three reasons, and
the third is the one that decides it:

- all four corners are spoken for: navigation top-right, scale bottom-right,
  zoom readout bottom-left, and MapLibre control positions are fixed anchors, so
  attribution in a corner means moving something that is already there;
- a *compact* `AttributionControl` is a button whose text appears on click,
  which on a 300 px panel is the only form that fits and is a poor reading of
  "reasonably calculated to make people aware";
- `LocationMap` already has a caption row beside the map, which is where the
  "Open larger" link lives, and the map page already has a bar beneath the
  picture carrying the author's text. Both are the natural home for one line.

So: `© OpenMapTiles © OpenStreetMap contributors`, linked to `openmaptiles.org`
and `openstreetmap.org/copyright`, rendered **whenever the detail source is in
the style** and never otherwise. On the panel and the inline post it joins the
caption row, independently of whether an "Open larger" link is there. On the map
page it joins the bar beneath the picture. On the hover card there is no detail
source, so there is nothing to credit. `attributionControl: false` stays, and
the scale bar does not move.

**Share-alike needs its own answer, and a licence file is not the whole of it.**
A filtered regional extract is a Derivative Database, so anything published
outside the operator's own organisation has to offer it under ODbL and say how
to reach the source of the derivation. That is cheap here and should be written
down rather than assumed: `public/map/LICENSE-OSM.txt` carries the notice and
the licence URL, and names `build/maposm/` plus the dated `sources.lock`, which
is a public repository. `make bundle` enforces its presence exactly as it
enforces the fonts' `LICENSE.txt`, and only when a detail archive is present.

`public/help/panel.html` carries the same credit in text alongside the OSM
extract **date**, because a stale basemap in an operational tool is a hazard a
reader has no other way to detect.

### 5. What does not change

Nothing in Go changes at all in Phase 1: no route, no setting, no error code, no
API field. `public/` is served by Mattermost before `ServeHTTP` sees a request,
the archive is same-origin, and `PageMapping`'s `connect-src 'self'` already
covers it. `TestPageCapabilityDecidesTheWholePolicy` must still pass byte for
byte, and if it does not, something here has gone wrong.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Custom schema or OpenMapTiles? | OpenMapTiles | as asked; and it is what Planetiler's profile emits, so schema and tooling arrive together |
| Which generator? | Planetiler, containerised | writes `.pmtiles`, takes `--bounds`, one jar, Apache 2.0, no PostGIS |
| How to filter road classes? | `tile-join -j` at the merge | Planetiler filters whole layers only; a fork is a fork forever, and the merge already reads every tile |
| Convert the global tier to OMT too? | **no** | it works, it is measured, its tests are the repo's tightest, and the payoff is cosmetic symmetry rather than a shared style |
| Regions at runtime or build time? | build time | a bbox list plus `tile-join` gets the whole feature with no manifest, no discovery, no per-package state, no new route |
| One archive or one per region? | one, unless Task 1 finds a per-tile request cost over the gaps | the design's one live fork |
| What ships in Phase 1? | **Hawaii, main islands, z10-14** | on the requested list, INDOPACOM's own headquarters, and the smallest pilot that exercises a metro area, a road hierarchy, named water, aerodromes **and** the empty-tile gap case at once. It is what makes every new guard run in CI instead of skipping. |
| Which Hawaii bbox? | the main islands (about 154-161°W, 18-23°N) | the Northwestern chain out to Midway adds two thousand kilometres of empty bbox and no feature worth a tile |
| If Hawaii overruns the headroom? | narrow to Oahu, then to Guam | the pilot's job is to prove the mechanism and light the guards; its extent is negotiable and the phase is not |
| Write all thirteen bboxes now? | no, the pilot rows only | twelve of them have no consumer until the measurement says which survive |
| Seam zoom | 10, as `SEAM_ZOOM` | where `DATA_MAX_ZOOM` stops being honest, and the level the request names |
| Cap NE detail conditionally? | **no, unconditional** | it is right on its own merits; see the argument above, and it is one constant to reverse |
| Client-side `DETAIL_MAX_ZOOM`? | none; a floor, not an equality | an equality would silently reject every profile but one |
| One probe or two? | one machine, two accept rules | the latching logic is documented as having been got wrong twice |
| Attribution mechanism | a text line in the caption row | every corner is taken, and a compact control hides the credit behind a click |
| Attribution on the hover card? | no source, so no credit | preview cannot reach z10, and a test pins that |
| Boundaries from OSM? | yes, `admin_level<=4`, `disputed=1` dashed | the third of the three coherent options CLAUDE.md already names, and OSM carries the flag NE's stripped fields did not |
| Military classification? | not shipped | identical to the existing airfield decision: a viewpoint, and an unreliable one |
| Tertiary roads? | from z12 | below that, clutter; above it, their absence is a disconnected network |
| Buildings, POIs, landuse? | not shipped | as asked; and they are most of the bytes |
| Label language | `name:latin`, `name` fallback | the bundled ranges are Latin-only; a CJK face alone is larger than the whole current archive |
| An admin switch for detail? | **no**, Phase 1 | tiles are fetched only past z10, the archive is same-origin, and a switch that saves nothing measurable is a setting an admin has to reason about for no reason. Revisit with Phase 3. |
| Change `TARGET_SPAN_METERS`? | **not here** | it is the largest open question and it belongs to Phase 2, decided with the measurement |

## Files to modify

| File | Change |
|---|---|
| `build/maposm/` | new: `Dockerfile`, `build.sh`, `regions.txt`, `fetch-sources.sh`, `sources.lock`, `README.md` |
| `Makefile` | `map-osm`, `osm-sources`; `bundle` checks the detail archive and `LICENSE-OSM.txt` when the archive is present |
| `public/map/detail.pmtiles` | new, Hawaii only |
| `public/map/LICENSE-OSM.txt` | new |
| `.../map/span.ts` | `SEAM_ZOOM` / `DETAIL_MIN_ZOOM = 10` |
| `.../map/basemap.ts` | an accept-rule parameter, `loadDetail()` with its own memo, a silent 404 and a logged timeout |
| `.../map/maplibre.ts` | `buildStyle(archive, detail, colors)`; the caps; the OMT layer set inserted before the cell and pin; the `attribution` field |
| `.../map/LocationMap.tsx` | the credit line in the caption row, the detail archive threaded through, omitted in preview |
| `.../map/style.spec.ts` | the partition, the layer order, the attribution |
| `.../map/basemap.spec.ts` | the second accept rule, the 404/timeout split |
| `.../map/LocationMap.pw.tsx` | the credit renders and does not render; a missing archive is silent |
| `.../map/asset_fixtures.ts` | serve the detail archive in component tests |
| `server/decorators/location/archive_test.go` | detail archive shape, depth against `build.sh`, budget, **per-source** layer agreement |
| `public/help/panel.html` | both tiers, the seam, what an uncovered area looks like, the credit, the extract date |
| `CLAUDE.md` | the Mapping section: the seam and its contested cap, the size arithmetic, the attribution reversal, the disputed-boundary decision this settles, the opening-zoom question |

## Tasks

Ordering matters in two places and is called out where it does.

0. [ ] **Verify the ingress ceiling.** The bundle is already past the 50 MB
   `client_max_body_size` in Mattermost's own documented nginx configuration and
   nobody has checked what the target deployment allows. A 413 at upload is not
   a Mattermost error and does not say what it is. Blocking for Phase 2, not for
   Phase 1.
1. [x] **Measure.** Stand up `build/maposm/` far enough to run Planetiler on one
   dense region (Taiwan) and the pilot (Hawaii) at z10-14 with the filtered
   layer set. Record bytes **per region and per zoom** - not a header field, so
   enumerate tile entries from the directory, group them, and say how shared
   tile bodies were attributed. Record the worst-case joined tile. Confirm the
   gap behaviour above with a network trace. Write the table into
   `build/maposm/README.md` and back into this plan.
2. [ ] Extrapolate the thirteen regions and record the four candidate profiles
   against the in-bundle headroom and the git-history cost. With Task 0, this is
   the whole input to Phase 2.
3. [x] `regions.txt` with the pilot rows, buffered, and a profile column.
4. [x] `build.sh`: per-region Planetiler runs, the `tile-join -j` class filter
   and merge, and a single `MAXZ`-style source of truth for depth matching
   `build/maptiles`.
5. [x] `sources.lock` and `fetch-sources.sh`, pinned by digest **and** date.
6. [x] `span.ts`: `SEAM_ZOOM` / `DETAIL_MIN_ZOOM`.
7. [x] `basemap.ts`: the accept-rule parameter, `loadDetail()`, the 404/timeout
   split.
8. [x] **Together, in one commit**: `maplibre.ts`'s caps and OMT layers, the Go
   per-source extraction and agreement test, and `style.spec.ts`. Splitting them
   leaves `make test` failing in between, because `styleSourceLayers` scrapes
   source-layer names with no notion of which source they belong to and would
   report every OMT layer as missing from the Natural Earth archive.
9. [x] **Then** commit `public/map/detail.pmtiles` for Hawaii, which is what
   turns the new Go assertions from skipped to run. Not before task 8, or they
   fail. This is the commit that adds a binary to history permanently, so take
   Task 1's measurement first and narrow the bbox if it overruns.
10. [x] Attribution: the source field, the caption line, `LICENSE-OSM.txt`, the
    `make bundle` check, the preview omission.
11. [x] Remaining tests, per the plan below.
12. [x] `build/maposm/README.md`, the Mapping section of `CLAUDE.md`, and
    `public/help/panel.html`.

## Testing plan

**Go, in `archive_test.go`.** These run for real from task 9 onward, which is
the point of shipping Hawaii; they still guard cleanly against an archive built to
a different depth:

- the detail archive's magic, spec version, tile type, min zoom equal to
  `SEAM_ZOOM` read out of `span.ts`, and max zoom equal to the depth
  `build/maposm/build.sh` declares, which is the same two-sided pin
  `TestArchiveDepthMatchesTheData` already applies to the global archive;
- **per-source** layer agreement in both directions. This needs the extraction
  replaced, not extended: today it scrapes `'source-layer': '...'` from the
  whole file, and with two sources each archive would report the other's layers
  as undrawn;
- a loose budget for the detail archive, and a **combined** figure checked
  against the bundle headroom, which is the number that actually gates an
  install and which nothing checks today.

**TypeScript unit (`style.spec.ts`, `basemap.spec.ts`):**

- the partition: every capped NE layer carries `maxzoom: SEAM_ZOOM`, every
  detail layer carries `minzoom: SEAM_ZOOM`, and no *concern* is drawn by both
  tiers at any zoom;
- `land` is the one NE layer **not** capped, asserted by name, since capping it
  is what would blank the map outside a covered region;
- every detail layer's index is below `cell-fill`'s, which is the existing
  ordering assertion extended rather than a new one;
- `attribution` is on the detail source and names both OSM and OpenMapTiles with
  their links;
- preview builds a style with no detail source, and `zoomForSpan` cannot return
  `SEAM_ZOOM` at any latitude or width the surfaces use;
- `loadDetail` on a 404 returns null, latches, and logs nothing; on a timeout it
  returns null, does **not** latch, and logs once;
- an archive whose min zoom is not `SEAM_ZOOM` is rejected, and one built
  shallower than z14 but at or past `SEAM_ZOOM` is accepted.

**Component (`LocationMap.pw.tsx`):**

- the credit line renders on the panel, the inline post and both pages whenever
  the detail source is present, and is absent from the hover card;
- a 404 on the detail archive draws the map with no note and no console warning;
- the caption row still lays out at the panel's 300 px width with both the
  credit and an "Open larger" link in it.

**Manual, once, in the pilot region:**

- an Oahu coordinate at z12-14 renders a legible road hierarchy, place labels
  that do not collide, named water and both international aerodromes;
- panning across a channel shows the empty-tile behaviour, with a network trace
  confirming no request goes out per missing tile.

**Deferred to Phase 2, because an island pilot cannot reach it:**

- a coordinate on a **region edge** renders continuous roads and coastline
  across the boundary, which is what the bbox buffer exists for. Every candidate
  region in Phase 1 is surrounded by ocean, so this waits for the first
  continental one.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| The unconditional cap is the wrong call and readers miss NE detail above z10 | it is one constant, `SEAM_ZOOM`, and both readings are recorded here so the reversal is a decision rather than a rediscovery |
| Nobody ever sees the tier, because the opening view is ~z6 | named as the largest open question, answered in Phase 2 before headroom is spent, not silently ignored |
| The measurement lands far above the estimate | Phase 1 ships only the pilot, so nothing large is committed on the strength of an estimate |
| **The pilot itself overruns the headroom.** Hawaii is ~50x Guam's land area and holds a metro area, so its 8-30 MB estimate has real spread against ~30 MB of room | measure in Task 1 **before** the commit in Task 9; narrow to Oahu, then to Guam. The pilot's extent is negotiable and its job, lighting the guards, is not |
| Committing the pilot is permanent in git history | it is measured first and it is committed once; the fallbacks exist precisely so the number never has to be discovered after the commit |
| A committed detail archive bloats git forever | commit only a chosen profile, once; intermediates in `.gitignore`; the prior plan's rule is *rebuild often, commit rarely* |
| The gap behaviour costs a request per missing tile | measured in Task 1; the fallback is one archive per region with tight bounds, named as a fork rather than discovered late |
| Stale, partial and uncovered are indistinguishable | accepted for Phase 1, documented in `panel.html`, and the Phase 3 fix (a coverage list on the wire) is named |
| Region-edge artifacts pass a centre-of-region pilot | buffered bboxes, and an edge coordinate in the acceptance criteria |
| Geofabrik dailies are not archived, so a pinned digest becomes unfetchable | pin the digest **and** the date; document an internal mirror as the supported reproducibility path; an unfetchable pin is a rebuild, not a corruption |
| OSM data ages and a stale basemap misleads | print the extract date in `panel.html` and carry it in the archive metadata; a currency policy is Phase 4 |
| OSM boundaries carry political positions this plugin has refused to take | `admin_level<=4` only, `disputed=1` dashed, decided explicitly rather than inherited |
| Non-Latin place names render as nothing | `name:latin` with fallback; **verify** Planetiler's transliteration populates it for Hangul, CJK, Arabic and Ge'ez, and record what it does not cover |
| Two sources double the ways a map fails silently | the partition test, the per-source agreement test and the logged transient failure are the three things that make a silent tier failure loud |
| The bundle passes `MaxFileSize` and fails at the reverse proxy | Task 0, blocking for Phase 2 |
| Planetiler adds a container and a JRE to the build | build-only, like tippecanoe, so it reaches no SBOM and no Grype gate. Confirm it holds. |

## Out of scope

- Converting the global Natural Earth tier to the OpenMapTiles schema.
- Any runtime package manager: per-package install, enable, discovery, or state.
- Hosting packages outside the plugin bundle, and every CSP question that comes
  with it. Phase 3.
- A coverage list on the wire. Phase 3.
- Changing `TARGET_SPAN_METERS` or the `zoomForSpan` clamp. Phase 2, and named
  above as the question that decides how much this tier is worth.
- Non-Latin glyph ranges. Phase 4.
- Buildings, footpaths, POIs, shops, amenities, address-level data. Permanently.
- Raster tiles, an online tile service, or any runtime network dependency.
- Changing `MAX_ZOOM` or anything about the cell and pin.

## Acceptance criteria

- [x] `make map-osm PROFILE=pilot` produces a `detail.pmtiles` in a clean
      container from pinned sources, and prints bytes per region and per zoom.
- [ ] At an Oahu coordinate at z12 the map shows OSM roads, places, named water
      and an aerodrome, and no Natural Earth road, lake, border or place label.
- [ ] Over the channel between two islands at z12 the map shows water and
      nothing else, and the network trace shows no tile request going out for
      the empty tiles.
- [x] The pilot archive's measured size is recorded here **before** it is
      committed, and is under the in-bundle headroom. 6,808,864 bytes.
- [ ] Outside a covered region at z12 the map shows land, water and coastline,
      and `panel.html` says that is what it means.
- [ ] With the detail archive removed, every surface renders, no warning is
      logged, and the map above z10 is land, water and coastline.
- [ ] With the detail archive slow rather than missing, a warning is logged and
      the next panel opened retries.
- [ ] The credit is on screen on the panel, the inline post and both pages
      whenever OSM data is in the style, and absent from the hover card.
- [ ] No detail layer is drawn above the cell or the pin.
- [x] `make check-style && make test` pass, with the new Go assertions **running
      rather than skipping**.
- [x] `make dist` produces a bundle under 100 MiB, and this plan records the
      remaining headroom. **73,952,550 bytes, leaving about 29.5 MB.**
- [x] `CLAUDE.md`, `build/maposm/README.md` and `public/help/panel.html` all
      state the seam, its contested cap, the uncovered-area behaviour, the
      attribution and the extract date.

## Checklist

- [ ] **Diagnostics**: nothing here is user-initiated on the server; no
      diagnostics posts.
- [ ] **Slash command**: no. The detail tier has no state a reader can query.
- [ ] **Error codes**: none in Phase 1; Phase 3 will need them.

## Review record

Reviewed by Codex, Gemini, `design-flaw-finder` and `simplicity-reviewer`. What
changed as a result, since several of these were load-bearing:

- **Nothing ever opens into the tier** (both agents, independently). The opening
  view is ~z6 and `zoomForSpan` clamps to 9. Now stated in the Problem
  statement and named as Phase 2's blocking question.
- **Appending detail layers would draw OSM over the pin and the cell** and break
  an existing ordering assertion. Now an insertion, with a test.
- **A transient detail failure would be permanent and invisible**, the same
  unreachable-retry defect `LocationMap.tsx` documents twice. Now a 404/timeout
  split.
- **The new Go guards would skip in CI for all of Phase 1.** Now the pilot
  ships, so they run.
- **A strict `maxZoom` equality would reject every profile but one.** Now a
  floor, and there is no client-side max at all.
- **Two probe functions would duplicate ninety lines** whose comments record two
  past defects. Now one machine, two accept rules.
- **A compact `AttributionControl` hides the credit behind a click** and every
  corner is already taken. Now a text line in the caption row; the scale bar
  does not move.
- **`tile-join` does read and write PMTiles and does take `-j`** - one reviewer
  said otherwise, and this repo's own `build/maptiles/build.sh` is the
  counter-example. That is what makes the class filter cheap.
- **Planetiler filters whole layers only**, so the class filter had to move.
- **NE `lakes` and `boundary_lines` would have double-drawn** against OMT `water`
  and `boundary`. The partition table is now complete and `land` is the single
  documented exception.
**Amended 2026-08-19**: the pilot moved from Guam to **Hawaii** at the author's
direction. It is a better pilot on every axis except size: it exercises a metro
area, a full road hierarchy, named water, two aerodromes and the empty-tile gap
case, where a single small island exercised almost none of them. Two things
follow and are now written into the plan rather than assumed. Its 8-30 MB
estimate has real spread against ~30 MB of headroom, so Task 1's measurement
becomes a gate on Task 9's commit, with Oahu and then Guam as fallbacks. And its
bbox edges are open ocean, so the **buffered-bbox behaviour is not testable by
this pilot at all** and moves to Phase 2's first continental region.

- **Region edges need buffered bboxes**; per-zoom bytes are not a header field;
  the ODbL story needs a source offer, not only a licence file; `tertiary` and
  `water_name` come back; the ingress ceiling becomes Task 0.

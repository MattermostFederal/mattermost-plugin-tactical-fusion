# PMTiles Basemap

## Context

The Location decorator already draws a map on all three surfaces (RHS panel,
`/decorate/location`, `/map`) through one `LocationMap` component and one
MapLibre style. The basemap behind it is a single 168 KB inlined GeoJSON
`FeatureCollection` (`public/map/world.geo.json`), generated from Natural Earth
110m by `build/mapdata/main.go`, carrying three layers discriminated by a
`properties.layer` string: land, lakes, borders.

That basemap is deliberately coarse and deliberately mute. `MAX_ZOOM` is 6, and
the style carries no `glyphs`, no `sprite` and no `symbol` layers, so nothing on
it is named. A reader who zooms in gets a polygonal coastline and no place
names, and the country a coordinate falls in reaches them only through the map's
accessible label.

This plan replaces that basemap with **Natural Earth vector tiles packaged as a
single PMTiles archive read over HTTP range requests**, and adds **text labels**.

## Scope

Confirmed with the requester:

- **In:** the PMTiles swap. Natural Earth, z0-z6, with labels.
- **In:** range requests via the `pmtiles` npm package.
- **In:** the `.pmtiles` artifact is a **release artifact, not committed to git**.
- **In:** bundled glyph PBFs and symbol layers, deliberately reversing the
  current no-labels rule.
- **Assumed** (raised, not answered; change these and the plan follows):
  glyphs are a bundled Latin subset rather than the browser-font fallback, and
  the archive is served from `public/` rather than through a new Go route.
- **Out:** OSM regional packages, the multi-package framework, operator-supplied
  external tile sources, CoT, layer-control UI, GEOREF. Each is its own plan.

## Verified findings

Checked against the running stack, MapLibre's source and Mattermost's, not
assumed. Several contradict the original request or my own first reading.

### HTTP Range works on the plugin static route

PMTiles is entirely dependent on byte-range reads, and whether Mattermost
honours `Range` on `/plugins/<id>/public/**` was unverified. It does.

`ServePluginPublicRequest` ends in `http.ServeFile`, and probing the local stack
with the plugin deployed:

| Probe | Result |
|---|---|
| Plain `GET` | `200`, `Accept-Ranges: bytes`, `Content-Length: 168161` |
| `Range: bytes=0-99` | `206`, `Content-Range: bytes 0-99/168161`, 100 bytes |
| `Range` + `Accept-Encoding: gzip` | `206`, no `Content-Encoding`, byte-exact |
| Multi-range | `206 multipart/byteranges` |
| Missing file | clean `404`, `X-Content-Type-Options: nosniff` |

Standard `http.ServeContent` behaviour, and compression negotiation does not
corrupt offsets. **This was the single largest risk in the proposal and it is
retired.** The same probe confirms the route is unauthenticated.

### `MAX_ZOOM` is not cross-checked against Go

`webapp_sync_test.go` pins `MERCATOR_LIMIT` and `DEGREE_METERS` only. `MAX_ZOOM`
exists solely in `span.ts:17`, `span.spec.ts`, `LocationMap.tsx:256` and prose.
Changing the zoom ceiling is a webapp-local decision.

The drift that *does* need pinning is new: `MAX_ZOOM` against the archive's own
header `maxzoom`.

### Labels need no CSP change

MapLibre resolves exactly two style fields as URLs, `glyphs` and `sprite`.
Glyph ranges load through `loadGlyphRange` → `getArrayBuffer` → `makeFetchRequest`,
on the **main thread**, i.e. over fetch, which CSP governs under `connect-src`.
`PageMapping` already grants `connect-src 'self'`.

So bundled same-origin glyphs are permitted by the **existing, unmodified**
policy, `font-src` stays absent, and the tests pinning both policy strings
verbatim (`TestPageCapabilityDecidesTheWholePolicy`,
`TestMapPagesCarryExactlyTheMappingPolicy`) pass untouched. Only a **sprite**
would need widening, and this plan adds none.

The CSP objection was the strongest argument against labels recorded in
`CLAUDE.md`, and it does not survive measurement.

### Glyph PBFs are optional

With no `glyphs` URL, or when a range request fails, MapLibre 6 draws the glyph
locally with TinySDF from the browser's own fonts and emits a `warnOnce`. It
does not fail the tile and does not fire a map `error`.

**So "labels require glyphs" is false.** Bundling buys two specific things:
identical rendering across OS font sets, and survival on privacy-hardened
browsers that block canvas readback, where TinySDF yields empty bitmaps and
labels vanish. That is the trade being made for ~250 KB, and it should be stated
that way rather than as a requirement.

### A custom protocol needs no worker code

A URL with a non-`http(s)` scheme is looked up in `REGISTERED_PROTOCOLS`. The
worker's copy is empty, so it falls through to an actor message answered on the
main thread, where the protocol *is* registered; tile buffers return as
transferables.

**Therefore no new `asset/resource` rule, no `?copy` analogue, and
`webpack.config.js:44-75` is untouched.** This matters because MapLibre worker
behaviour has already bitten this repo twice.

### The plugin directory does not survive a restart

`SyncPlugins` removes and reinstalls every plugin from the filestore on startup
and on cluster events, and `GetBundlePath()/public` is the very directory
Mattermost serves. **Anything an operator drops there is gone on the next
restart.**

This is why operator-supplied archives are genuinely deferred rather than
merely descoped: satisfying them needs a path *outside* the bundle, which
Mattermost cannot serve, which means a Go route. Recorded here so the next plan
does not rediscover it.

### What already exists and must not be re-planned

The request describes a great deal of shipped code as new:

- **The normalized geo model.** `Location`/`Axis`/`Grid` with `Point()`
  (`coord.go:188-208`), fed by nine grammars. §13 is done.
- **GeoJSON as the rendering interface.** `pin` and `cell` are already `geojson`
  sources fed by `setData` (`maplibre.ts:215-271`, `LocationMap.tsx:169-200`).
  §14 is done.
- **The map page and its controls.** `/map`, `fill`, zoom, pan, reset view, copy
  buttons, "Open larger". §15-16 are done.
- **Per-reader show/hide** with server validation and a checkbox UI, already
  carrying one non-row entry (`map`). The nearest thing to §17.

Genuinely new: the tile format, the pipeline, labels, and distribution.

## Design principles

| Concern | Approach | Avoid |
|---|---|---|
| Archive identity | One digest, pinned at build time and enforced when the bundle is assembled | A per-reader hash that cannot see a whole file |
| Missing tiles | Empty buffer, drawn as nothing | Throwing, which reports an ocean as a broken deploy |
| Failure reporting | Keep the definitive-vs-transient latch verbatim | One stalled fetch latching the map off for the session |
| Labels vs the answer | Symbol layers strictly below `cell` and `pin` | Any `text-*` property, which cannot see non-symbol layers |
| Zoom ceiling | Bounded by the archive header and the cell overlay | A second hardcoded constant that can drift |
| One basemap | Delete the GeoJSON path | Two representations that render different worlds |

## Technical approach

### 1. Distribution

Range support being confirmed means the archive sits under `public/map/` and is
served by Mattermost with no Go route, exactly as `world.geo.json` is today.
Zero new server code.

- **Generated out of band** by `make map-tiles`, a prerequisite of nothing.
- **Published as a GitHub Release asset** on a `mapdata-YYYY.MM` tag, versioned
  separately from the plugin. Publication is a documented manual step;
  `release.yml` is not extended here.
- **Gitignored** at `public/map/*.pmtiles`, so it never enters the pack and
  `make release-check`'s dirty-tree gate stays green.
- **Fetched** by `make map-fetch`, which verifies SHA-256 against the committed
  `build/maptiles/basemap.sha256`.

**Commit the digest, not the artifact.** That one move gives the bundle guard
something to check, keeps `release-check` clean, and keeps CI free of tooling.

Building a bundle with a map now needs the network once. The plugin still runs
fully air-gapped; the build machine does not have to be the air-gapped one.

### 2. The `make dist` guard

`bundle` copies `public/` with `cp -r` and no exclusion (`Makefile:215-217`), and
neither `make clean` nor `make nuke` touches `public/`, so a stale archive
persists and keeps shipping. Guards, mirroring the two that already exist at
`Makefile:227-242`:

- Archive **absent** → `make dist` warns and continues; `make release` refuses.
  Shipping a release with no map should be a decision, not an accident.
- Archive **present but wrong** (stale, truncated, or an HTML captive-portal
  page saved by `curl`, which `curl -f` does not catch because the portal
  returns 200) → hard `exit 1`. Checks: SHA-256 against the committed digest,
  and the first seven bytes are `PMTiles`.

The tar is `-z` (`Makefile:244-248`); PMTiles is internally compressed, so the
bundle grows ~1:1 and the gzip pass over it is wasted CPU.

### 3. Integrity

`basemap.ts` hashes the whole response body (`:135-150`). Under range requests
there is never a whole body, so that check moves rather than disappearing:

- **Build time** — the bundle guard above hashes the archive once, where the
  whole file is in hand, and refuses to assemble around the wrong one.
- **Runtime** — the client validates what a range reader honestly can: magic
  `PMTiles` and spec version 3 at offset 0 (which a 404 HTML page or a captive
  portal fails immediately), `tile_type == MVT`, and `maxzoom >= MAX_ZOOM`.

This is a **net improvement, not a retreat**, and the reason should not be lost:
`crypto.subtle` is undefined on plain-HTTP origins, which `CLAUDE.md` itself
calls the norm for on-prem and air-gapped installs, so today's check silently
passes on exactly the installs it exists for. A build-time digest has no such
hole.

**The definitive-vs-transient latch (`basemap.ts:43-76`) is preserved verbatim.**
Definitive: 404, bad magic, wrong tile type, insufficient maxzoom. Transient:
timeout, network throw.

### 4. The style

`buildStyle` loses its `basemap` argument — no data is embedded any more — and
derives URLs from a module constant and `pluginBaseUrl()`.

```
sources:
  basemap: {type:'vector', tiles:['tfmap://world/{z}/{x}/{y}'],
            minzoom:0, maxzoom:MAX_ZOOM}
  cell:    {type:'geojson', data: emptyCollection()}   // unchanged
  pin:     {type:'geojson', data: emptyCollection()}   // unchanged
glyphs: `${pluginBaseUrl()}/public/map/fonts/{fontstack}/{range}.pbf?v=...`
// sprite: still absent
```

The scheme is `tfmap`, not `pmtiles`, so it cannot collide with the package's
own registration and a stray `pmtiles://` in a config is obviously not ours.

| Layer | Was | Becomes |
|---|---|---|
| `water` | background | unchanged |
| `land` | `filter: ['==',['get','layer'],'land']` | `source-layer: 'land'`, filter deleted |
| `lakes` | same pattern | `source-layer: 'lakes'` |
| `borders` | same pattern | `source-layer: 'boundary_lines'` |
| `country-label` | — | **new** symbol layer |
| `cell-fill`, `cell-outline`, `pin` | unchanged | unchanged, **and stay last** |

Every `['get','layer']` filter goes: that discriminator existed only because
three layers shared one GeoJSON source. A half-migrated filter draws nothing and
looks like a data problem, so a test asserts no filter mentions `'layer'` again.

The source's `maxzoom` is `MAX_ZOOM`, so a future raise of the camera ceiling
cannot make MapLibre request a zoom the archive lacks.

### 5. Protocol wiring

Register **inside `loadAndConfigure()`** (`maplibre.ts:84-110`), after the
`setWorkerUrl` block. That location is load-bearing: it sits behind the `loading`
promise memo so registration is exactly-once for free, it necessarily precedes
any `new maplibre.Map`, and module scope would require `maplibregl` at module
scope, un-lazying the 973 KB chunk. `LocationMap.tsx` needs no change.

**`map.on('error')` becomes a trap and must be handled.** Today one fetch sits
behind the basemap; with tiles, every absent ocean tile reaches
`LocationMap.tsx:277-281` and, while `!ready.current`, prints "The map could not
be loaded." over a perfectly good map. So the handler returns
`{data: new ArrayBuffer(0)}` for a tile the archive omits, and throws only when
the archive itself is unusable.

Add `removeProtocol` to `_resetForTesting()` so the harness reset stays honest.

### 6. Labels

**Country labels only, in phase one.** Points at Natural Earth's
`LABEL_X`/`LABEL_Y` rather than centroids (Norway's centroid is in Sweden), text
from **`ADMIN`** — the same field `CLAUDE.md` already records as the sovereignty
decision for the Region lookup, so the map and the Region string cannot
disagree. Ranked by `LABELRANK`.

`populated_places` is held back deliberately: it multiplies the archive, and
*which* cities get named at z≤6 is an editorial-political surface this plugin
has not had to defend yet. If it ships later, filter by `SCALERANK <= 4` so the
choice is a citable rule rather than a judgment.

The payoff worth naming: retiring the Region row left the country reaching a
reader only through the accessible label — "no country for anyone reading it
with their eyes". A country label puts it back on every surface.

**Font.** Noto Sans Regular, one face, directory named `NotoSans-Regular` with
no space (`{fontstack}` is substituted verbatim into a path). Four ranges —
`0-255`, `256-511`, `512-767`, `8192-8447` — covering Basic Latin, Latin-1
Supplement, Latin Extended-A/B and General Punctuation, which spans every
`ADMIN` value including "Côte d'Ivoire", "Türkiye" and "Åland". **≈250 KB.** A
full BMP set is ~10 MB per face.

**The licence obligation, stated precisely.** Noto Sans is SIL OFL 1.1, and
converting a TTF into SDF glyph ranges creates a *Modified Version*, which is
still Font Software under that licence. Three consequences:

1. **The notice ships with the files.** OFL §2 allows redistribution "provided
   that each copy contains the above copyright notice and this license", so
   `public/map/fonts/LICENSE.txt` carries the Noto copyright line and the full
   OFL 1.1 text. `make bundle`'s `cp -r public` ships it once it exists.
2. **The PBFs stay OFL and cannot be relicensed.** This repo is
   `All Rights Reserved` (`LICENSE:1`), so the glyph files are the one part of
   the bundle that is not proprietary. OFL is not copyleft over the software
   that bundles it, so nothing else is affected.
3. **Verify the Reserved Font Name** in the `OFL.txt` of the exact version
   packaged. Google's Noto files generally declare none, which is what makes
   the `NotoSans-Regular` fontstack name safe; an RFN would forbid a modified
   version carrying it.

**No visible attribution is required** — the notice must travel with the files,
not appear in the UI. That is what distinguishes this from the Natural Earth
credit the project deliberately dropped: that was a courtesy over public-domain
data, this is a condition of redistribution.

For contrast, since the glyph decision was raised and left open: the TinySDF
path carries **no** obligation at all, because nothing is redistributed, and
Apache-2.0 alternatives (Roboto, Open Sans) trade the OFL notice for a `NOTICE`
obligation rather than avoiding one.

A generator test walks every `name` the archive carries, collects codepoints and
asserts the matching range file exists, so a country whose name reaches an
unshipped range fails the build rather than the field.

**Contrast.** `MapColors` gains `label` and `labelHalo`, written as plain 6-hex
literals because `shell_test.go` parses them with `ParseUint(..., 16, 32)` and
asserts each literal appears in `maplibre.ts`. Light `#101418`/`#eef2f7`, dark
`#e8edf5`/`#12161d`; four new pairs added to the contrast table, all clearing
4.5:1 against both land and water. The halo is a legibility aid, not the
contrast mechanism.

**Collision.** MapLibre's collision index sees only symbols, so no `text-*`
property can protect the pin or the cell. **Draw order is the only mechanism**:
every symbol layer sits below `cell-fill`, and a test asserts that by index
rather than leaving it a convention.

### 7. Zoom

**`MAX_ZOOM` stays 6**, but the justification is rebuilt, because the existing
comment ("past this the 110m basemap stops being honest") was generous even for
110m and would be wrong for tiles. At 512 px tiles z6 is ~1.22 km/px, and
Natural Earth's positional accuracy is ~50 km (110m), ~25 km (50m), ~5 km (10m).
**No Natural Earth product is honest at z6 by that standard**; 110m was already
several zoom levels past it.

The honest restatement: **the ceiling is set by the cell overlay, not the
basemap.** The 6 px floor on drawn cells was removed on the grounds that a small
cell should stay invisible until the reader zooms in; a 10 km grid reference is
about 11 px across at z6. Take z6 away and that reasoning collapses.

A new `span.spec.ts` test asserts `MAX_ZOOM` equals the committed archive's
header `maxzoom` — pinning the drift in the one place that can see both sides.
No Go change.

### 8. The `pmtiles` dependency

Checked against the registry: `pmtiles@4.5.0`, **BSD-3-Clause**, 380 KB
unpacked, one transitive dependency (`fflate`). Two new SBOM entries.

The cost is recorded rather than discovered: it is a **runtime** dependency, so
it faces `grype --fail-on high` on every PR, every push to `main` and a weekly
cron, and neither documented suppression rationale in `.grype.yaml` (dev-only,
Mattermost-externalized) covers it. An advisory means upgrade or pin.

**The escape hatch is vendoring, not suppression**: constrain the generator to
emit a deliberately boring archive (v3, clustered, single root directory, zero
leaf directories, tile type MVT) and a ~150-300 line reader becomes a header
parse, one varint directory decode, a Hilbert tile-id lookup and a
`DecompressionStream('gzip')`. Write that decision down now so it is not made
under release pressure.

Note the geodesy precedent points the other way and does not transfer: hand
writing was justified there by maths frozen since 1987, and PMTiles has already
gone v2 to v3.

### 9. The pipeline

`build/maptiles/`, **not Go and not in the root module**, which is what keeps
`cyclonedx-gomod` from enumerating tile tooling that never reaches a released
binary — the same reason `build/mapdata/` is stdlib-only.

```
build/maptiles/
├── README.md          prerequisites and how to reproduce
├── Dockerfile         pinned tippecanoe + pmtiles CLI
├── build.sh           tippecanoe over the NE sources, then pmtiles convert
├── sources.txt        pinned Natural Earth URLs + SHA-256 each
├── basemap.tag        the pinned mapdata release tag        (tracked)
└── basemap.sha256     the expected artifact digest          (tracked)
```

`tippecanoe`, the `pmtiles` CLI and GDAL are absent from this machine and none
becomes a CI dependency. Containerised so the toolchain is pinned rather than
whatever a developer has.

**Reproducibility is by pinning, not by committing.** Claim *pinned*
reproducibility, not byte-identical-across-versions: tippecanoe is deterministic
given identical input, flags and version, and `pmtiles convert` embeds a
generator string. The verification is the recorded digest.

**Sources are downloaded, not committed.** Only the plugin must run air-gapped;
the pipeline that builds its basemap need not, and the 10m sources are one to
two orders of magnitude larger than the committed 110m set. This is the
distinction the request's §21 conflates.

`build/mapdata/` and the country lookup are untouched: `admin.go` and
`region.go` do not read `world.geo.json`, and stay on 110m so the row's
`(Natural Earth 110m)` citation stays honest. The `.pmtiles` is deliberately
**not** added to `map-data-check`, which regenerates before diffing and is a
prerequisite of `make test` inside a 20-minute CI cap.

### 10. The GeoJSON basemap is deleted

`world.geo.json`, `basemap_digest.ts`, the digest-writing code in
`build/mapdata/main.go`, and their two entries in `map-data-check` all go.

The argument is this repo's own: "two representations of one basemap is two
things that can disagree." A GeoJSON fallback is worse than the `paths.go` case
already deleted for that reason, because the two would render *different
worlds* — one labelled, one not — and the fallback is the path nobody looks at.
It would also need a second `buildStyle`, a second layer-id set, and a second
test suite.

**The cost, stated:** a fresh clone that has not run `make map-fetch` gets no
map, where today it gets one. That is one documented command in the setup, and
`make test` is unaffected because the committed **fixture archive** (a few KB,
one tiny region) is what the component tests serve. The fixture is the only
`.pmtiles` that may enter git.

## Test migration

| Test | Becomes |
|---|---|
| `style.spec.ts` "fetches nothing from the network" | **Replaced, stronger:** every URL in the style is either `tfmap://` or a plugin-relative path under `/public/map/`. Keeps `not.toMatch(/https?:/)`, adds a protocol-relative `//` check, keeps `sprite` absent. Names where bytes may come from, rather than which two fields are forbidden |
| `style.spec.ts` "no symbol layer" | **Inverted:** every symbol layer's `text-font` has a directory under `public/map/fonts/`, read off disk |
| `style.spec.ts` "only geojson sources" | **Split:** `cell`/`pin` still inlined `geojson`; `basemap` is `vector` with a one-element `tiles` array |
| *(new)* `style.spec.ts` | No filter mentions `'layer'`; every basemap layer names a `source-layer`; every symbol layer's index < `cell-fill`'s |
| `basemap.spec.ts` | Shape and comments survive. URL and byte cap move to the archive; the three "not GeoJSON" definitive cases become header rejections; latching, concurrency, `credentials:'omit'` and `crypto.subtle` tests keep their exact form |
| *(new)* `pmtiles.spec.ts` | Header round-trip, a known tile id yields plausible MVT, an omitted tile yields empty rather than throwing, a truncated buffer throws rather than reading past the end |
| `LocationMap.pw.tsx` | `serveMapAssets()` fulfils the fixture archive with `Accept-Ranges`/`206` honoured and serves glyphs from disk. Its comment about the style fetching nothing else must be rewritten. **New:** glyph routes left unrouted still produce a drawn map (the TinySDF contract) |
| `basemap_test.go` | Raw budget only; **drop the gzip budget**, since tiles are already compressed inside and a gzip figure would be meaningless. Layer assertions move to the generator's own test |
| `TestMapPaletteCarriesItsContrast` | Four new label/halo pairs |
| CSP tests, `webapp_sync_test.go` | **Unchanged** |
| *(new)* `public/map/` file-type test | Allow only `.pmtiles` and `.pbf`. `public/` is unauthenticated and outside this plugin's CSP and `X-Content-Type-Options`, and `help_docs_test.go` scopes to `public/help` so `public/map/` inherits nothing |

## Phases

| Phase | Work | Exit condition |
|---|---|---|
| **1. Measure** | `build/maptiles/` pipeline; build one archive at candidate layer sets | A real archive exists; size, zoom range and legibility are numbers, not estimates |
| **2. Render** | `pmtiles` dep, protocol registration, vector source, header validation, error-handler fix | A coordinate draws from the archive on all three surfaces |
| **3. Label** | Glyph subset, `country-label`, palette additions, contrast pairs, OFL notice | Labels render in both themes and never cover pin or cell |
| **4. Ship** | `map-fetch`, bundle guards, gitignore, release publication, delete the GeoJSON path, docs | `make dist` refuses a wrong archive; `make test` green on the fixture |

Phase 1 is a spike and may change Phase 3's layer list. Nothing after it is
committed to until its numbers are known.

## Files

| File | Change |
|---|---|
| `build/maptiles/**` | **New.** Pipeline, pinned sources, digest, README |
| `Makefile` | `map-tiles`, `map-fetch`; bundle guards; drop two entries from `map-data-check` |
| `.gitignore` / `.gitattributes` | `public/map/*.pmtiles` (fixture excepted); `*.pmtiles`/`*.pbf` binary |
| `webapp/package.json` | `pmtiles` runtime dependency |
| `.../map/maplibre.ts` | Vector source, `source-layer`, `country-label`, `glyphs`, protocol registration, label colours as literals |
| `.../map/basemap.ts` | Archive loader: header validation replaces the body digest; latch preserved verbatim |
| `.../map/LocationMap.tsx` | Error handler must not treat an absent tile as a broken deploy |
| `.../map/span.ts` | `MAX_ZOOM` justification; header agreement test |
| `public/map/fonts/**` | **New.** Four glyph ranges plus `LICENSE.txt` |
| `public/map/world.geo.json`, `basemap_digest.ts` | **Deleted** |
| `build/mapdata/main.go` | Drop the basemap and digest writing; keep `admin.go` |
| `server/.../basemap_test.go`, `shell_test.go` | Per the migration table |
| `public/help/panel.html` | `:204` states "The basemap is Natural Earth at 1:110 million. That is coarse on purpose" — this falsifies it. Also labels, the new setup step, and the Latin-subset limitation |
| `CLAUDE.md` | The Mapping section's "one 168 KB cacheable response" argument is now contradicted and must be rewritten, not quietly outlived |
| `docs/SECURITY.md` | The archive is digest-checked at build time; a hand-dropped one is not covered by the release `clamscan` |

## Risks

| Risk | Mitigation |
|---|---|
| **DDIL request amplification** — one 168 KB response becomes 20-40 range requests, directly contradicting a documented design property | Accepted deliberately as the price of the format. `immutable` caching, `MAX_ZOOM` unchanged, individually small tiles. Rewrite the CLAUDE.md paragraph rather than leaving it stale |
| An absent ocean tile reported as a broken deploy | The protocol handler returns an empty buffer; throwing is reserved for a corrupt archive |
| A `pmtiles`/`fflate` advisory blocks a release with no suppression available | Pre-decided: upgrade, pin, or vendor the reader. Constrain the archive shape now so vendoring stays a ~300-line job |
| A stale archive ships over a signed release | Bundle-time digest and magic-byte guards; `make clean` does not cover `public/`, so this is the realistic failure |
| Labels cover the pin or clutter low zoom | Symbol layers below `cell-fill`, asserted by index; `LABELRANK` ranking; populated places held back |
| Non-Latin names blank under a Latin subset | Generator test fails the build if a shipped name needs an unshipped range |
| A fresh clone has no map | One documented `make map-fetch`; tests use the committed fixture |
| OFL notice missed, or the font declares a Reserved Font Name | `LICENSE.txt` shipped beside the fonts; RFN checked against the packaged version's own `OFL.txt` before the fontstack is named |

## Verification

1. `make map-tiles` produces an archive; record size and zoom range.
2. `make dist` — absent archive warns and continues; corrupted archive fails on
   the digest; an HTML page saved as `.pmtiles` fails on magic bytes;
   `make release` refuses an absent archive.
3. `make check-style && make test` green, including `map-data-check`.
4. `make deploy`, then in a real browser:
   - a coordinate in the panel draws the tiled basemap with country labels;
   - `/map` and `/decorate/location` draw the same thing;
   - the pin and cell are never covered by a label;
   - light and dark both legible.
5. DevTools shows `206` responses against the `.pmtiles` and no whole-file
   transfer — the range path is actually being used.
6. Block the glyph requests: labels still render via TinySDF, no error note.
7. Delete the archive and reload: "The map could not be loaded.", latched, and
   every reading still on screen.
8. Pull the network cable and repeat step 4 on a cold cache.
9. `make sbom-audit` to see the new dependencies through the Grype gate.

## Checklist

- [ ] **Diagnostics**: a missing or invalid archive is an operator-visible
      condition; surface it through `/tactical-fusion check`, this repo's
      established place for operator health.
- [ ] **Slash command**: no new subcommand. The archive is not a per-message
      concern; `check` is the right existing surface.

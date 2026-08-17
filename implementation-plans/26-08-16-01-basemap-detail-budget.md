# Spending the basemap's detail budget

## The question

The archive grew to 25 MB where roughly 50 MB was expected. What would more
detail look like, and what does it cost?

## The premise is wrong in both directions, and the correction is the answer

**48 MiB was never an allowance.** `TestArchiveFitsItsBudget` says so itself:
*"Still loose on purpose: this catches an order-of-magnitude mistake ... not a
layer added on purpose."* It is a smoke alarm. Reading the gap under it as
budget-to-spend converts the check into a target, after which it catches nothing.

**But there is real headroom at the ceiling that actually gates installs.**
Verified, not assumed:

| Thing | Bytes | Source |
|---|---:|---|
| `public/map/world.pmtiles` | 25,000,953 | on disk |
| plugin tar.gz | 51,304,681 | `dist/` |
| `FileSettings.MaxFileSize` default | 104,857,600 | `server/public@v0.4.3/model/config.go:1884` |
| **bundle headroom** | **~53.5 MB** | |

Mattermost gates plugin upload through the file API, so `MaxFileSize` is the
ceiling. `plugin.json` declares `min_server_version: 11.8.0`, far past the
versions that defaulted to 50 MiB, so that legacy default reaches no server this
plugin runs on and the question is narrower than it first looks. There is no
separate plugin-size limit.

So the archive could roughly double and still install. **The reason to be careful
is not the ceiling.**

### Four budgets, four payers

This is the framing the repo is missing, and it decides everything below.

| Budget | Payer | Bounded today by |
|---|---|---|
| **Git history** | every clone, every CI checkout, forever | **nothing** |
| Per-tile weight | the reader, on their link, only for tiles they open | **nothing** (see below) |
| Bundle size | the upload path, and any reverse proxy in front of it | nothing |
| Archive size | whoever carries media to an air-gapped site | `TestArchiveFitsItsBudget`, loosely |

**Git history is the binding constraint, and it is not the one anybody was
looking at.** `.gitattributes` marks `*.pmtiles binary`, there is no LFS, and the
archive is committed directly. Gzipped tile bodies do not delta-compress, so
**every rebuild adds a full copy to history permanently.** Today that is ~25 MB
per rebuild. A "measure, then choose" method that commits after each experiment
costs a clone more than the feature does. That reframes the work: **rebuild
often, commit rarely.** Measure in the container, keep the intermediate archives
out of git, and commit once per shipped decision.

**Per-tile weight is bounded by nothing, contrary to appearances.**
`MAX_TILE_BYTES=250000` is on the `detail` run alone; every other run passes
`--no-tile-size-limit`, and `tile-join` passes it too. The tile a reader actually
fetches is the **joined** tile, the sum across all parts, and nothing caps that.
So raising the `detail` cap does not set the reader's worst case, and no number
in this plan about per-tile weight is anchored until somebody measures the
current worst-case joined z8 tile. That measurement is the first task.

Tiles are fetched by HTTP range, so a bigger archive costs the reader **nothing
except in the tiles they open**. That is what makes detail in dense places cheap
to ship and expensive to draw, and it is why these budgets must not be traded
against each other by accident.

**One ceiling is still unverified.** Mattermost's own documented nginx
configuration sets `client_max_body_size 50M`, which a 51 MB bundle is already at
the edge of and a 75 MB one would exceed, failing as a 413 at upload rather than
as a Mattermost error. Nothing in this repo records it. Confirm it against the
deployment this plugin targets and write the answer into `CLAUDE.md`; a plan that
answers only the config key gets a false all-clear.

## What "more detail" can mean, and what each costs

### 1. Deeper: z9 taken, z10 still declined

This section first declined z9. It was then taken deliberately and built, and
what follows is the measured result rather than the estimate.

**The estimate was +18-25 MB. The measured cost is +16.5 MB**, archive
25,000,953 to 41,530,128 bytes, +66%. It lands at 39.6 MiB, still under the
existing 48 MiB smoke alarm, and takes the bundle to about 67.8 MB against the
100 MiB ceiling.

| Part | z0-8 | z0-9 | delta |
|---|---:|---:|---:|
| `detail` (urban, roads, rail) | 12,190,879 | 20,639,458 | +8,448,579 |
| `outline_10m_deep` (land, lakes, borders) | ~5,000,000 | 8,876,270 | +~3.9 MB |
| `context` (rivers, admin-1) | 5,621,738 | 9,244,642 | +3,622,904 |
| `places` | 1,545,138 | 2,240,014 | +694,876 |
| `labels` | 83,161 | 100,166 | +17,005 |
| `outline_10m` / `_50m` / `_110m` | 1,961,085 | 1,961,085 | 0 |

Addressed tiles went 40,810 to 151,501 and unique tile bodies 23,049 to 69,849,
which is where the bytes went: **z9 carries no geometry z8 did not.** The sources
are the same 10m files.

**What was bought, stated narrowly so it is not re-derived as generous.** Vector
tiles magnify without blurring, so this is not the raster-sharpness argument. z9
buys halved coordinate quantization inside a tile (a 4096-unit extent spans about
38 m at the equator at z8, about 19 m at z9) and more room for the collision
index to place labels. It buys no accuracy.

**What it costs.** 5 km of source error is about 16 px at z8 and 33 px at z9, so
a coastline drawn 5 km from where it is now reads twice as emphatically. That is
a real step toward the failure z10 was refused for, taken with the arithmetic in
front of us rather than around it. It also adds ~41 MB to git history on the
commit that lands it.

**z10 is still declined**, at 65 px of possible error, which is where a
generalised coastline stops reading as generalisation at all.

**A different question still hides behind this one.** Even at z9 a 1 m MGRS cell
is sub-pixel, so a reader still cannot inspect the cell whose size is supposed to
carry the token's resolution. Zoom levels do not fix that; an honest "basemap
detail ends here" cue with overzoom does, and it costs zero bytes in every budget
above. Still its own piece of work.

### 2. Denser: available, cheapest, highest value

Detail already in the sources and possibly being discarded.

Where the bytes are after the z9 build, from the `echo` in `build.sh` (a
`make map-tiles` run leaves these in `build/maptiles/work/`; reproducing the
table needs a docker run):

| Part | Layers | Zooms | Bytes |
|---|---|---|---:|
| `detail` | urban_areas, roads, railroads | 5-9 | 20,639,458 |
| `context` | rivers, admin_1_lines | 4-9 | 9,244,642 |
| `outline_10m_deep` | land, lakes, boundary_lines | 7-9 | 8,876,270 |
| `places` | populated_places | 3-9 | 2,240,014 |
| `outline_10m` | land, lakes, boundary_lines | 5-6 | 1,649,412 |
| `outline_50m` / `outline_110m` / `labels` | | | 411,839 |

An earlier draft of this table read these with `du`, which rounds up to the disk
block and inflated every row.

Archive header, for scale: z0-9, 151,501 addressed tiles, 69,849 unique tile
bodies, 41.4 MB of tile data.

Two levers:

- **`detail` is the only run carrying `MAX_TILE_BYTES=250000
  --drop-densest-as-needed`.** Whether that cap ever binds is **unknown**, because
  `build.sh` runs tippecanoe `--quiet`. The hypothesis worth testing is that
  roads, rail and urban polygons are being dropped over exactly the places a
  coordinate most often lands. It is a hypothesis, not a finding.
- **`SIMPLIFICATION=4` applies everywhere except the z7-8 outline tier.** The
  reasoning already written above that exception (Douglas-Peucker tolerance
  scales as 1/2^z, so full detail costs 81% more vertices at z5 and 11% at z8)
  was applied to the outlines and to nothing else.

### 3. Wider: available, but most candidates are worth less than they look

New Natural Earth 10m layers. The real per-layer cost is larger than it reads;
see the checklist under "Adding a layer" below. `TestArchiveCarriesEveryLayerTheStyleDraws`
fails a half-done one in both directions, which is the right guard and also the
reason not to add six at once.

## Investigated and found not to be a bug: coastal pins

An earlier draft of this plan reported that `ne_10m_land` omitted Diego Garcia
and Kwajalein, and proposed adding `ne_10m_minor_islands` to fix it. **That was
wrong, and the record of why is worth keeping.**

`ne_10m_land` carries 11 features but **6,837 polygons**, and both atolls are in
it. The apparent misses were imprecise test coordinates landing just outside a
coastline drawn at 1:10m. Measured distance from each point to the nearest land
edge:

| Position | In a polygon | Nearest land edge |
|---|---|---:|
| Diego Garcia airfield (72.4111, -7.3131) | no | 33 m |
| Kwajalein islet (167.7333, 8.7167) | no | 892 m |
| Naval Support Activity Bahrain (50.61, 26.21) | no | 160 m |
| Wake Island | yes | |
| Guam, Andersen | yes | |

`ne_10m_minor_islands` does not contain Diego Garcia at all, precisely because
`land` already does, so the proposed fix would have added 1.3 MB of source and
changed nothing.

**The real property, which is not fixable and not new:** a coordinate on a narrow
coastal feature (an atoll rim, reclaimed land, a spit) can render tens to
hundreds of metres outside the drawn coastline. Natural Earth models Diego Garcia
as its actual rim rather than a filled atoll, which is correct, and the rim is
under 2 km wide. At the panel's opening z5-6 that offset is sub-pixel, and at z8
892 m is roughly 3 px.

This is the same honesty property `MAX_ZOOM = 8` already encodes, stated from the
pin's side rather than the coastline's, and it is independent evidence for the z9
refusal: the limit is the source's generalisation, not the tiling. Worth one line
in `CLAUDE.md` so it is not rediscovered as a bug a third time.

## Stopping rule

Without one, "ships if its cost is proportionate" has no meaning and Phase 2 has
no end. Proposed, to be agreed before any of it runs:

- No single change may grow the archive by more than **20%**.
- The archive may not pass **40 MB**, leaving the bundle under 70 MB and clear of
  a 100 MiB ceiling with room for the binaries to grow.
- No change may grow the **worst-case joined z8 tile** past a threshold set from
  the measurement in task 1, proposed as 1.5x today's.
- At most **one commit of `world.pmtiles` per shipped decision**, never one per
  experiment.

## Recommended order

### 1. Measure, and commit nothing

- **Worst-case joined z8 tile, and where it is.** Every per-tile number here is
  unanchored until this exists. It needs a PMTiles v3 directory walk **including
  leaf directories**, which the archive now has. Decide once where that walker
  lives: in Python in the container, where the toolchain already is but CI never
  runs it, or in Go, where `make test` runs it but it must be written from
  scratch. The same walker serves the per-zoom byte breakdown and any later
  per-tile test, so write it once.
- **Whether the `detail` cap binds, and what it drops.** `--quiet` is in
  `tippecanoe_common` (`build.sh:19-25`), shared by every run, so this means
  restructuring that array into per-run flags rather than deleting one word.
  Note that `--drop-densest-as-needed` derives a minimum feature spacing for the
  **whole zoom level**, not per tile, so one dense tile over the Ruhr thins that
  zoom everywhere, which is the strongest argument for raising the cap, if it
  binds. Do not expect `--json-progress` to hand back a clean per-layer drop
  count; verify what the pinned tippecanoe emits, and otherwise compare input and
  output feature counts with `tippecanoe-decode`. `--coalesce` is on the same run
  and is a second knob affecting the same numbers.
- **The proxy ceiling**, and one line in `CLAUDE.md` recording it.

### 2. Buy density, if the measurements support it

1. **Stop overriding `--maximum-tile-bytes`.** Raising the cap to 500 KB is
   returning to tippecanoe's own default, not choosing a larger bound, which
   makes it much cheaper to justify. **Do not remove the cap**: an uncapped z8
   tile over a city is unbounded, and that is the reader's link.
2. **`SIMPLIFICATION=1` at z7-8** for `context`, `detail` and `places`, splitting
   those runs by zoom band exactly as `outline_10m` / `outline_10m_deep` already
   are, so z0-6 stays byte-identical and the diff is reviewable. Accept, again
   and deliberately, that this reintroduces a seam: `--detect-shared-borders` and
   simplification run per invocation, so geometry can change visibly at the z6/z7
   boundary. That is already true of the outline tier.

Both are config changes with an existing precedent in the same file: no new
sources, no new layers, no palette work. **If the measurements are comfortable,
this is probably the whole job.**

### 3. At most one or two layers, chosen with those numbers in hand

| Layer | Source | Why | Rough raw source |
|---|---|---|---:|
| Airfields | `ne_10m_airports` | Mission-relevant, ~890 points | ~1.2 MB |
| Admin-1 labels | `ne_10m_admin_1_label_points` | Fills the naming gap between `country-label` (maxzoom 6) and `place-label` in sparse regions | ~4.2 MB |

Use `ne_10m_admin_1_label_points`, **not** `ne_10m_admin_1_states_provinces`:
the latter is a ~39 MB polygon layer and nothing here would style the polygons.

**Airfields carry a note, not a capability.** The Location decorator permanently
excludes `LOC:`, `DEPLOC:`, `ARRLOC:` and `ICAO:` because those introduce an
airfield code whose position must be looked up rather than computed. Drawing
airports does not change that and must not be documented as if it does.

## Adding a layer: what it actually takes

The plan first drafted this as "six coordinated edits". Checking the code found
more, and two of the extras fail in ways worth knowing in advance.

1. `sources.txt` line.
2. **`UPDATE_LOCK=1 ./build/maptiles/fetch-sources.sh`.** `sources.lock` is
   generated wholesale and cannot be hand-edited, and no Makefile target passes
   that variable. **There is a latent bug here worth fixing while nearby:** the
   verification loop iterates over *lock* lines rather than *sources.txt* lines,
   so a source added without regenerating the lock is fetched, never verified,
   ships unpinned, and prints `sources verified against $LOCK` while doing it.
3. `strip.py` keep-list. It **lowercases every kept field**, so style expressions
   must `['get', ...]` the lowercase name. That is why the style reads
   `scalerank` and `name_en`. It also **fails the run** if a named field is
   absent anywhere in the file, which is the first thing that will bite on
   `ne_10m_airports`.
4. `build.sh` run assignment. `strip_fine` hardcodes the `10m` prefix, so this is
   a constraint on the source list rather than a coincidence: a 50m source needs
   different plumbing.
5. Style layer in `maplibre.ts`, plus a `MapColors` entry **only if the paint is
   new**. Each new colour is a measured contrast pair in the one place this repo
   records having got the palette wrong by eye once already. Note that
   `TestMapPaletteCarriesItsContrast` is an opt-in named list, so a new
   below-floor context colour will not fail it and has to be judged by hand.
6. **`style.spec.ts` asserts the exact sorted list of source-layers.** Miss it
   and the Playwright suite fails. It is the loudest of these, and the only one that
   fails immediately.
7. **Glyph ranges, for a symbol layer.** `make bundle` checks exactly four
   ranges. If a label layer needs a fifth, `glyphs.js`, that Makefile list and
   `build/maptiles/README.md` all move together.
8. Note for whoever names a layer: `styleSourceLayers` in `archive_test.go`
   matches `'source-layer': '([a-z0-9_]+)'`, so a name with a capital or a hyphen
   is silently **skipped** rather than flagged, and would ship undrawn and
   unchecked in one direction. Every Natural Earth name fits; keep it that way.

## Considered and not taken

| Option | Why not |
|---|---|
| z9 / z10 | The source cannot support it, it costs about as much as everything else combined, and it doubles the git-history cost of every rebuild. The camera ceiling is a separate question with a cheaper fix |
| Lowering `roads`/`railroads`/`urban_areas` to z4 | `zoomForSpan` puts the sidebar at z5-6 and the map page at z7-8, so these layers already start at the opening zoom. z4 is only reached by zooming out, which is the case `build/maptiles/README.md` holds them back from |
| Ports, ice shelves, glaciers, physical-feature labels | Each defensible, none urgent. Six new layers is a 60% increase in the style's source-layers and six passes through the checklist above. Revisit individually |
| `ne_10m_roads_north_america`, regional river and lake supplements | Sharper over CONUS, unchanged everywhere else. For readers who are deployed that is the wrong bias, and it is invisible to whoever ships it |
| Bathymetry | Large, and sea-floor contours under a land coordinate are noise |
| Non-Latin glyph ranges + `NAME` over `NAME_EN` | Genuinely better in theatre and cheap for Cyrillic, Greek and Arabic, but it is a **labelling policy** change deciding whose spelling of a place the map uses, not a detail change. Its own decision. CJK is out of reach at any budget |
| Shipping the boundary classification | The headroom removes the size objection and leaves the policy one, which is the only one that ever mattered. Still unresolved, still not a size decision |

## Tests

**Keep `TestArchiveFitsItsBudget` a constant.** Do not re-derive it from
`ceiling − compressed(binaries)`. That test runs against the working tree and can
see neither the binaries nor the tarball, so the derived figure goes stale
silently the moment the binaries grow, and a map-data test then fails for a
reason that has nothing to do with map data, pointing the reader at
`make map-tiles`, which will not fix it. That is the exact failure `map-data-check`
was already narrowed to avoid. Lower the constant to a hand-derived number with
the derivation in the comment, so it stops reading as an allowance.

**Bound the bundle where the bundle exists.** The install risk is the tar.gz and
nothing checks it. This repo already solved this shape once: the whole-file
SHA-256 moved into `make bundle` "because that is where the whole file is in
hand", which is where the pmtiles-presence, magic-byte and glyph-range checks
live too. Add the size check beside them, against the recorded ceiling.

**A per-tile test needs the walker from task 1, and a measured threshold.**
Asserting the build flag instead is tempting and is not enough, because the flag
bounds one part and the reader fetches the join. Write it only after the current
worst case is known, and only once: the same walker serves the measurement and
the test.

Unchanged and still load-bearing: `TestArchiveDepthMatchesTheCameraCeiling`
(still z8), `TestArchiveCarriesEveryLayerTheStyleDraws` (both directions, per new
layer), `style.spec.ts`'s source-layer list.

## Files

| File | Change |
|---|---|
| `build/maptiles/fetch-sources.sh` | Verify against `sources.txt`, so an unpinned source is loud |
| `build/maptiles/sources.txt`, `sources.lock` | Any chosen layer, via `UPDATE_LOCK=1` |
| `build/maptiles/build.sh` | Per-run `--quiet`; `--maximum-tile-bytes`; z7-8 zoom-band splits; new `--named-layer`s |
| `webapp/src/decorators/location/map/maplibre.ts` | Style layer per new layer; palette entry only where the paint is new |
| `webapp/src/decorators/location/map/style.spec.ts` | Source-layer list |
| `server/decorators/location/archive_test.go` | Budget constant lowered with its derivation; per-tile test, after task 1 |
| `Makefile` | Bundle size check beside the existing `make bundle` guards |
| `build/maptiles/README.md` | Measured drop counts and worst-case tile; what each new layer is for |
| `CLAUDE.md` | The bundle and proxy ceilings and their sources; the git-history cost of a rebuild; why a coastal pin can sit just off the drawn coastline; why z9 was declined again |
| `public/help/panel.html` | What the basemap now shows |
| `public/map/world.pmtiles` | Rebuilt and committed **once per shipped decision** |

## Verification

- `make map-sources && make map-tiles && make test`
- `make dist`, then compare the tar.gz against the recorded ceiling
- Open a dense city, a remote desert and a polar position, and compare z8 against
  the current build for what the raised cap recovered
- Check the z6/z7 boundary on a coastline for a visible seam from the new split

## Open questions for the author

1. Does the git-history cost change the appetite? A 40 MB committed archive
   rebuilt a few times is a different conversation from a 40 MB bundle.
2. Where does the PMTiles walker live: container Python, or Go under `make test`?
3. Is the stopping rule above the right one?

## Checklist

- [ ] **Diagnostics**: no user-initiated action or error path here; the basemap
      failure latch in `basemap.ts` already reports a broken deploy
- [ ] **Slash command**: none. The basemap has no reader-facing operation

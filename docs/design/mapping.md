# Mapping

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Two ways a map used to fail silently

Both were found by driving a real browser against a running server rather than
by reading the code, and both are recorded here because neither is visible from
the source alone.

### A map that is built and never becomes ready

MapLibre tiles in a **worker**. If that worker never arrives, no source ever
finishes, `load` never fires, and because that is not an `error` the error
handler never runs either. The note stays "Loading map…" forever and a reload is
the only way out. `asset_fixtures.ts` records the same failure from the other
side, where it is used deliberately to hold a map between construction and load.

A worker URL that 404s is how this happens in the field. The URL comes from
webpack's public path, so a wrong `window.basename`, a blocked `.mjs`, or a
bundler whose asset shape `assetUrl` does not recognize all produce it.

So readiness is **bounded**, exactly as `loadMapLibre` and `basemap.ts` bound
their own fetches: `READY_DEADLINE_MS` after the map is constructed, a map that
has not fired `load` reports `NO_BASEMAP` and is torn down, which hands back its
WebGL context and lets a later attempt retry. The deadline is generous rather
than tight, because `load` waits for the first tiles and these readers are on
constrained links; the point is to end an infinite wait, not to police a slow
one. `_setReadyDeadlineForTesting` is how the suite exercises it without sitting
through twenty seconds.

### The MapLibre worker and its shared chunk ship in a hashed directory

MapLibre tiles in a worker, and that worker is a module which imports
`./maplibre-gl-shared.mjs` by that literal relative name. So the shared chunk
cannot be content-hashed the way every other emitted asset is: the name the
worker asks for is fixed.

Mattermost serves `/static/plugins/**` with `Cache-Control: max-age=31556926`,
a year. A fixed name under that policy is a file a browser will not re-fetch
until long after it has stopped matching the worker beside it. Upgrade MapLibre
and a fresh, content-hashed worker is paired with a year-old shared chunk: the
worker fails to start, no source finishes tiling, `load` never fires, and the
map sits on "Loading map…" with no error. It is the same silent failure the
worker's own comment describes, arriving by a different route.

A hashed **directory** fixes what a hashed filename cannot. `maplibreAssetDir`
in `webpack.config.js` keys a directory on the contents of both files and emits
the pair into it. The worker's relative import resolves inside whatever
directory the worker was loaded from, so the name it asks for is preserved while
the URL moves whenever either file does.

`make bundle` checks the layout, because webpack is not the only way the pair
can end up wrong and the failure is invisible in exactly the builds nobody looks
at. The guard it replaced was `ls <worker> && ! -f <shared>`, which passed
silently on a bundle with **no worker at all**, and would have passed silently
again the moment the worker moved out of the top level. The current one fails on
a missing worker, a missing shared chunk, and a pair that is not in a
content-keyed directory; all three were verified by sabotaging a tree rather
than by reading the recipe.

### The basemap's cache buster is a digest, not the version

Mattermost re-extracts the bundle on every install, so `world.pmtiles` gets a
fresh modification time even when its bytes do not change. It is served out of
`public/`, where this plugin sets no headers at all: no `ETag`, no
`Cache-Control`, only `Last-Modified`. A browser holding cached byte ranges
therefore revalidates with `If-Range: <old time>`, the validator no longer
matches, and the server answers a 16 KB range request with **HTTP 200 and the
entire 43 MB archive**. `probeArchive` reads 127 bytes and refuses that, so a
browser that takes this path reports that the map could not be loaded until its
cache is cleared.

The SERVER half is measured and reproducible. Whether a given browser hits it
depends on the state of its own range cache: a warm profile driven through a
redeploy in Chrome revalidated cleanly and kept working, so this is a latent
trap rather than a guaranteed one. What is not in doubt is that the response
below is what the server sends when a browser does ask this way.

```
Range: bytes=0-16383  +  If-Range: <stale>   ->  200, Content-Length: 43074410
Range: bytes=0-16383  +  If-Range: <current> ->  206, Content-Length: 16384
```

Keying the URL on the archive's **content** fixes it. A redeploy that changes
nothing keeps the URL and its cached ranges; a new archive lands on a URL no
browser holds an entry for, so nothing ever revalidates a range against a
validator that moved underneath it. The digest is computed at build time by
`webpack.config.js` and falls back to the plugin version for a webapp-only build
where the archive is not on disk to hash.

**Serving the archive from the plugin's own route was considered and rejected.**
It would let us set a stable `ETag` and a long `Cache-Control`, which is the
tidier fix, but it would also put every tile range request through the plugin's
RPC bridge instead of Mattermost's static file server. `servePackage` accepts
that cost for an optional, small archive; the global basemap is the one every
map reads, and the digest fixes the same failure without moving it.

## Mapping

The location panel and both server-rendered location pages draw a world map,
and the map names the country a position falls in. Everything is bundled:
no tile service, no map API, nothing fetched from outside the plugin.

**All three surfaces are the same code.** Not just one map library: one
implementation of the readings table, the resolution rules, the copy buttons and
the map. `LocationReadings` is the table and the map together, with no opinion
about where its data came from; `LocationPanel` wraps it in the sidebar's
environment and the page bundle wraps it in the page's. They did not: the two Go
pages rendered their own table and their own hand-written SVG map, which meant
two implementations of the projection, two of the palette, two of the resolution
rule, two sets of copy buttons and two answers to "how far out is far enough".

The cost of consolidating is stated in **The page content policy** and **What
the pages gave up** below, and neither is small.

### The basemap is vector tiles, and generated

Two generators, with different jobs and different toolchains.

`build/mapdata/` is stdlib-only Go, run by `make map-data`. It reads Natural
Earth 110m from `build/mapdata/source/` and writes exactly one thing:
`server/decorators/location/mapdata/admin.go`, the admin-0 polygons at source
precision, which is what the country lookup is computed from. It draws no map.

`build/maptiles/` builds what every map actually draws:
`public/map/world.pmtiles`, a PMTiles archive of vector tiles covering z0-z9,
plus the glyph ranges under `public/map/fonts/`. It carries twelve layers:
coastlines and lakes, country and province boundaries, roads, railways, rivers,
urban areas, airfields, and the names of countries, towns and provinces. It runs
`tippecanoe` and `fontnik` in a
container, by `make map-tiles`, and is **a prerequisite of nothing**: it is
never reached by `make test` and never runs in CI, which is why the archive is
committed rather than built on demand. See `build/maptiles/README.md` for the
decisions inside it that are decisions rather than mechanics.

**Airfields are a landmark, and that is all they are.** They carry the one hue on
the map that is neither basemap gray nor pin red, and they are held to the same
measured 3:1 roads are, because an aerodrome beside a coordinate is something a
reader is looking for rather than something they are meant to notice only when
they look. Natural Earth also classifies some of them as military, in five
inconsistently spelled variants of a `type` field; that classification is
deliberately not shipped and not drawn. It would be a viewpoint, and an
unreliable one. **None of this changes what the decorator does with an airfield
code**: `LOC:`, `DEPLOC:`, `ARRLOC:` and `ICAO:` stay permanently excluded,
because a facility's position must be looked up rather than computed, and
drawing airports does not look anything up.

**Province names and airfield names yield to town names**, which is decided by
layer order alone and cannot be expressed in a text property, because the
collision index sees symbols and nothing else. **MapLibre runs placement from the
top of the layer list down, so the LAST symbol layer wins**, and the label layers
are therefore ordered least wanted first: countries, provinces, airfields, towns.
A town is the better landmark when somebody is placing a coordinate, and
provinces fill the sparse regions, which is the gap they were added for, since
`country-label` stops at z6.

This is worth stating because the file said the opposite for a long time
("symbols are placed in layer order and first placed wins"), and appending three
label layers after `place-label` on the strength of that comment silently
suppressed the town label on Paris. The failure is quiet by construction: every
layer still draws, the map still loads, and the only symptom is a label that is
no longer there. `label layers are ordered least wanted first` in `style.spec.ts`
pins the order now.

It replaced `public/map/world.geo.json`, a single 168 KB inlined
`FeatureCollection`, which went when labels arrived: a GeoJSON source cannot
carry a `source-layer`, and keeping it beside the archive would have meant two
representations of one basemap rendering two different worlds, one labeled and
one not. That is the same rule `paths.go` was deleted under, when the Go SVG
renderer went: **two representations of one basemap is two things that can
disagree**, and the browser one is the one anybody would notice was wrong.

**The cost is one cacheable response becoming many.** The GeoJSON basemap was a
single 168 KB fetch, and the tile pyramid is read by HTTP range request, so a
reader opening a coordinate makes tens of them. That was accepted deliberately
for the detail and the labels, and it is a real cost on a constrained link.
What softens it: the archive is immutable and cached per install, and
`MAX_ZOOM` bounds how many tiles a reader can ask for.

Raster tiles were the original proposal and lost on grounds that mostly still
hold: a raster tile has one lightness baked in, so matching the reader's theme
needs two tile sets, and the palette here carries a measured contrast ratio in
both. Past its maximum a raster basemap goes blurry, which is the map equivalent
of rendering `35°00'00"N` for a token that said `35°N`. Vector tiles keep both
properties and give up the single-fetch one.

Both generated artifacts are **committed**, unlike `build/manifest/`'s output,
which is gitignored and regenerated by `make apply`. A clean checkout must build
and an air-gapped `go test` must run without anyone having run a generator
first, and for the archive and the fonts there is one more reason: they ship,
and their toolchain is a container nobody should need in order to run the
tests.

`admin.go` is encoded as **delta fixed-point base-36 strings parsed at package
init**, not as Go literal slices. That is 116 KB compiling in about a tenth of a
second; the same data as literals is a known Go compile-time problem.
`mapdata.go` holds the decoder and is hand-written.

`make map-data` regenerates the country polygons. The check is
`make map-data-check`, which names its one generated artifact individually
rather than watching the directory:
`mapdata.go` is hand-written and sits beside `admin.go`, so a directory-wide
check reported an ordinary edit to the decoder as stale map data and told the
reader to run a generator that would not fix it. It is `git diff --exit-code`,
**not** an idempotence check: a generator edited without
regenerating is perfectly idempotent and fully drifted. It runs as a prerequisite
of `make test`, because a guard nothing invokes is not a guard, and this
particular drift is invisible everywhere a developer looks (see Build and test).

The generator is **standard-library only**. There is one `go.mod` at the repo
root, so `build/mapdata/` is in the shipping module and `cyclonedx-gomod`
enumerates it; a shapefile reader pulled in here would sit on the Grype gate
forever despite never reaching a released binary.

### The OpenStreetMap detail tier, above the seam

Natural Earth stops being the right source somewhere around z9, and past it the
camera overzooms: lines stay crisp and only their generalisation is wrong, so a
coastline that may be five kilometers from where it is gets drawn at street
scale with nothing saying so. A second tier answers that with OpenStreetMap in
the **OpenMapTiles schema**, generated by `build/maposm/` and drawn from z10 up.
It is not one archive: each region is its own `<command>-<area>.pmtiles` under
`public/map/packages/` or in the operator's package directory, for the reasons
under "Areas are separate files" below. The decisions inside the generator are in
`build/maposm/README.md`; what follows is everything that reaches the client.

**`SEAM_ZOOM` is 10 and is the whole mechanism.** MapLibre's layer `minzoom` is
inclusive and its `maxzoom` is exclusive, so a Natural Earth layer capped at 10
and an OpenStreetMap layer starting at 10 partition exactly: no zoom at which
neither draws, none at which both do. The failure in one direction is a blank
band and in the other it is the same road drawn twice, kilometers apart, which
reads as a rendering artefact rather than as two sources disagreeing. `no
concern is drawn from both sources at any zoom` in `style.spec.ts` pins the pairs.

**`land` is the one Natural Earth layer that is NOT capped**, and it is asserted
by name. OpenMapTiles has no land polygon: it draws water over a land-colored
ground. So the accurate OMT `water` is drawn **above** the generalised NE `land`
fill, and where the two disagree at a coastline the accurate one wins visually.
Capping `land` would leave a white frame everywhere the detail tier does not
reach, which is a different and much louder failure than the one below.

**The cap was unconditional and now depends on coverage**, which reverses the
decision this file recorded as contested. It was capped everywhere, so on any
install, in any area no package covered, z10 and up was land, water and
coastline and nothing else. The argument for that: five kilometers of source
error is 33 px at z9 and 65 px at z10, `DATA_MAX_ZOOM` was 8 for exactly that
reason before it was raised as a deliberate trade, and a road drawn 130 px from
where it is at z12, for an audience acting on coordinates, is not context. The
argument against, recorded at the same time: a reader who zooms in over Nebraska
gets an empty frame and cannot tell that from a broken archive.

The empty frame is what readers actually hit, so the cap now lifts where nothing
covers. `buildStyle` takes `overzoomGlobal`, and `syncGlobalReach` re-decides on
every `moveend`, because the style is built once and the map is moved thereafter:
without it, panning out of a covered area keeps the cap and panning into one
draws both tiers over the same road. `coveredBy` answers from the package's own
PMTiles bounds, which the client already reads in the 127 header bytes it probes.

It tests the **viewport against the package bounds, not the center against
them**, and the asymmetry is the point. A center test lifts the cap as soon as
the middle of the frame leaves a package, so a reader who pans until Oahu sits
at the edge gets the generalised tier overzoomed across the whole view while the
accurate one still draws Oahu: the same road twice, kilometers apart, which is
the one failure the seam exists to prevent. Capping whenever any covered ground
is on screen cannot do that, and costs a blank margin at the edge of coverage
instead.

`SEAM_CAPPED_LAYERS` is a list rather than "every basemap layer carrying a
maxzoom", which is what it was first and was wrong: `country-label` carries
`maxzoom: 6` to hand over to the town labels, so deriving swept it up and the
sync overwrote a handover threshold with a seam cap on every install, global
only ones included. A test holds the list to the style in both directions.

**Both halves of the original argument still hold**, and the resolution keeps
both: inside coverage nothing is overzoomed, so no generalized road is ever drawn
where an accurate one exists, and outside it the reader gets context rather than
a blank frame. What is given up is that a generalized coastline now magnifies at
street scale where no package reaches, which `public/help/panel.html` names in
words rather than leaving to be inferred. The `roads` and `rivers` widths gained
a `MAX_ZOOM` stop for the same reason the detail layers have one: an
interpolation that ends at z9 holds that pixel width while the map doubles
around it and reads as thinning out exactly where a reader zoomed in.

**Remove is offered only where Remove can work.** A bundled area is inside the
plugin and a release replaces it, but the System Console listed it beside a
dropped-in one with the same button, so pressing it surfaced `os.Remove`'s
ENOENT to an admin as "the package could not be removed": a feature that reads
as broken rather than as one that was never possible. `mapPackage` now records
which directory it came from, the list carries a `removable` array beside the
names, and the row says "ships with the plugin" instead of offering a button.
The shadow case falls out of the same field: removing a drop-in that covers a
bundled package of the same name succeeds and the name stays listed, because the
bundled one underneath resurfaces, and the row losing its button is what stops
that reading as a failed delete. Both write routes answer with `removable`
beside `packages` for that reason, and the uploader has to read both: reading
only `packages` back left `removable` holding whatever the opening GET returned,
so an area an admin had just uploaded rendered as "ships with the plugin" and a
shadowing drop-in kept a button after the removal that made it bundled again.

**An area outlives a plugin upgrade, and a stamp says when it does not.**
Areas are large enough that an operator will upgrade the plugin without moving
them, and nothing stops that: no version gates a package, the `?v=` in its URL
is cache busting only, and depth is a floor rather than a match. Two changes
would break that portability, though, and only one of them was visible: the seam
moving off z10 is caught by the `minzoom == 10` check but reported as a corrupt
file, and a change to `DETAIL_SOURCE_LAYERS` was not caught at all and simply
drew nothing.

`build.sh` therefore stamps `tactical-fusion-map/<n>` into the archive's PMTiles
metadata name, which is the only field `tile-join` can set that the format
already has, and the server reads it back during discovery. An unstamped archive
is schema 1, because every area published before this existed is unstamped and
requiring the stamp would have rejected all of them. A mismatch is `TF-18008`
and names which side is behind, distinct from `TF-18002`, which means the file
is broken. Bump it only when an older archive becomes wrong rather than merely
shallower. Only the first few kilobytes of the metadata blob are read to find
it: the name is the first field OpenMapTiles writes, and a whole tilestats block
on a dense area runs to hundreds of kilobytes that discovery would otherwise
decompress on every pass.

**A package is replaceable, so the route sends an ETag.** The uploader and the
drop-in directory both overwrite an archive in place, under a URL carrying only
the plugin version, so nothing about the URL moves when the bytes do. PMTiles is
read by offsets the client took from a directory it read earlier, and those
offsets against a different file are garbage rather than an error, so this is
the one staleness here that is not merely cosmetic. pmtiles.js already watches
the ETag across its own requests and re-reads the directory when it moves; with
none sent it had nothing to compare and that recovery never fired. The tag is
size and modification time rather than a digest, because this runs per tile
request and hashing a 500 MB archive to answer one is not a trade worth making.
`Cache-Control` came down from five minutes to the one the client already caches
the package list for, which bounds how long a replacement can be stale; the ETag
makes the revalidation at the end of it a 304 rather than a re-download.

**Discovery is memoised for five seconds, because it was on the tile path.**
MapLibre asks for one byte range per tile, and each one reached `servePackage`,
which read both package directories and opened, read and closed every archive to
re-validate its header: thirteen file opens per tile on the shipping roster,
continuously while a reader pans. The TTL is what keeps "drop a file in and it
appears with no restart", and it is far shorter than the client's own minute so
an operator never waits on this layer; `installPackage` and `removePackage` drop
the memo outright so the System Console reflects a change at once. The key
includes the directories, since `LocationMapPackagesDir` changes under a running
plugin. A rejected package is logged once per path rather than once per
discovery, which before the memo meant once per tile. Keying that memo by path
has a cost worth naming: replacing a bad archive with another bad one under the
same name is silent until the plugin restarts. The alternative is stat-ing every
rejected file on every pass to notice, which is the work this exists to avoid.

**Three branches on the package path are deliberately untested.** Everything
else in `packages.go`, `api.go` and `servePackage` is exercised, including every
refusal an operator can provoke, so the gaps are worth naming rather than
rediscovering. `installPackage`'s 512 MB ceiling needs 512 MB streamed through
`io.Copy` onto disk to reach; the wall clock is not worth it, and lowering the
constant would be changing what ships to suit a test. The `temp.Close()` failure
beside it, and `file.Stat()` failing in `servePackage` on an `*os.File` that
opened a moment earlier, cannot be provoked from outside the process at all. The
other two uncovered branches in `server/` are the starter-template guard in
`setConfiguration` and the registry error in `OnActivate`, both of which say so
in a comment where they sit.

**Coverage is a rectangle, so it is coarser than the data.** A package header
carries one bbox, and `indopacom-japan` spans the Ryukyus to Hokkaido, which
means it contains Korea. On an install holding Japan and not Korea, a Seoul
coordinate reads as covered and stays capped. One rectangle cannot exclude Korea
while keeping Yonaguni, so this is a limit of the shape rather than a defect to
fix in the client.

**Nothing ever OPENS into this tier.** `TARGET_SPAN_METERS` is 400 km, which puts
the opening view near z6, and `zoomForSpan` clamps to `DATA_MAX_ZOOM` on top of
that. The tier is four or more zoom levels below the first thing anybody sees and
is reached only by a deliberate gesture, which is the gesture `MAX_ZOOM = 17`
exists to permit. Whether `TARGET_SPAN_METERS` should move now that there is
something worth zooming into is open, and is the question to answer before
spending bundle headroom on more regions.

**Coverage cannot be advertised, so four states render alike.** A PMTiles header
carries a single rectangular bounds, so a merged archive spanning several regions
declares nearly the whole world and filters nothing. Uncovered, stale, partial
and truncated therefore all look like bare land and water. A coverage list
carried in `/api/v1/features` and in the page shell is the fix and is not built.

**The client keeps one probe machine with two accept rules.** The
definitive/transient split, the timeout and the in-flight identity check in
`basemap.ts` are the parts whose comments record having been got wrong, so there
is one copy of them and `createProbe` varies the URL, the memo and the zoom rule.
The global rule is `minZoom == 0 && maxZoom >= DATA_MAX_ZOOM`; the detail rule is
`minZoom == SEAM_ZOOM && maxZoom >= SEAM_ZOOM`. **The detail maximum is a floor,
not an equality**, and there is deliberately no client-side maximum at all: an
operator may ship a shallower profile than the pipeline can build, and an
equality would reject it and, by the silence rule below, say nothing.

**A 404 on the detail archive is silent; a timeout is not.** A global-only build
is a supported shipping profile, so a missing archive is a configuration and
warns nobody. A transient failure is the one case where the two look identical on
screen: the style is built once inside the map's creation effect and the map is
created once and then moved, so a timed-out probe yields a panel with no detail
source for the whole of its life, rendering exactly like a correct global-only
install. It is therefore logged and not latched, and the next map retries.

**The hover card carries no detail source at all.** Preview is built
`interactive: false` at a zoom `zoomForSpan` already clamped, so it can never
request an OSM tile; `a preview camera cannot reach the seam` drives that across
every latitude and width the surfaces use. Carrying the source anyway would print
an OSM credit on a card with no OSM on it.

**Attribution is now a license condition, which reverses a documented decision.**
Natural Earth is public domain, so its credit was a courtesy and was dropped from
all three surfaces; OpenStreetMap is ODbL and the OpenMapTiles schema is CC-BY,
and both require credit. `OSM_CREDIT` in `maplibre.ts` is written once and read
by both the style's own `attribution` field and the line the component renders,
so the two cannot disagree about what was credited.

It is **a text line in the caption row rather than a MapLibre control**, for
three reasons and the third decides it: all four corners are already taken
(navigation top-right, scale bottom-right, zoom readout bottom-left) and control
positions are fixed anchors; a *compact* `AttributionControl` is a button whose
text appears only on click, which at a 300 px panel width is the only form that
fits and is a poor reading of "reasonably calculated to make people aware"; and
`LocationMap` already has a caption row, which is where "Open larger" lives. The
row is `justifyContent: flex-end` with `marginRight: auto` on the credit, so the
credit sits left and the link right, and either one alone still lands where it
always has. `attributionControl: false` and the scale bar's corner are unchanged.

`public/map/LICENSE-OSM.txt` ships beside the packages and `make bundle` refuses
a bundle carrying a package without it, exactly as it does for the fonts. ODbL
also makes a filtered regional extract a Derivative Database, so that file names
the source of the derivation rather than leaving it to be reconstructed.

**Regions began as a build-time list with no runtime package system, and that
did not survive.** The argument for it was that the feature stayed small, but an
area an operator cannot add without a plugin release is an area they cannot add,
so there is a package system now: `server/packages.go` discovers and validates
them, `/api/v1/packages` and the System Console control install and remove one,
and `LocationMapPackagesDir` is where they live. The rest of this section is that
design, and the generator's own decisions are in `build/maposm/README.md`.

One of the three things that file left unresolved is still open, and it is the
one that matters most: planetiler's ~1.2 GB of auxiliary datasets are pinned by
nothing, so an archive is not byte reproducible from pinned sources alone. The
other two are closed. A region spanning more than one Geofabrik extract is
merged rather than cut, so the buffered `osmium extract` that was "not built
yet" is not wanted at all, for the reason under "A multi-country region is
merged, never cut" below; and the pilot is no longer a single island, since
`indopacom-guam` ships beside Hawaii and the eleven release areas are
multi-country.

**Areas are separate files, dropped in or uploaded.** Each region is its own
archive named `<command>-<area>.pmtiles`, such as `indopacom-hawaii.pmtiles`,
and an operator adds one by copying it into `LocationMapPackagesDir` or
uploading it in the System Console. That reversed an earlier decision to merge
every region into one archive, and the reversal buys back the thing that
decision had written off: a merged archive declares one rectangular bounds
covering everything between its regions, so coverage was unknowable and stale,
partial, uncovered and truncated all rendered alike. Separate files each carry a
tight bbox, so MapLibre skips out-of-area requests and the plugin can say which
areas it has.

**The storage is a real directory, and that is forced rather than chosen.**
`plugin.API.ReadFile` and `GetFile` both return the whole file, and PMTiles is
read by byte range, so an archive stored through the plugin API would be pulled
into memory for every tile a reader scrolls past. `os.Open` with
`http.ServeContent` is the only reader that answers a range without doing that.
Two consequences follow and are operational rather than defects: **in a cluster
every node needs the directory**, and an **upload reaches only the node that
served it**, so shared storage or a copy per node is required either way. The
uploader is a convenience for modest areas; a large one is copied, because an
upload crosses Mattermost's own request limits and whatever proxy is in front of
it.

**An upload is written beside its target and renamed, and validated before the
rename.** Discovery lists the directory on every pass, so a file written
directly under its final name would be found, validated, rejected and logged
while the upload was still in progress. Writing to a temporary name in the same
directory and renaming makes the archive appear whole or not at all. Validation
runs on the temporary file, before the rename, so a file that is not an archive
is refused with a reason the uploader can show rather than accepted and then
silently skipped by the next discovery, which is where every other bad package
ends up.

**The name is a whitelist in three languages.** `build.sh` writes it,
`packages.go` serves it and `basemap.ts` requests it, and it reaches both a URL
and a filesystem path, so it is matched against `^[a-z0-9]+(-[a-z0-9]+)+$`
rather than cleaned. No dot, slash or separator survives that, which is what
makes traversal impossible rather than merely unlikely.

**The dropped-in directory wins a name collision with a bundled package**, so an
operator can replace a shipped area with a newer build without waiting for a
release. That is most of what the directory is for, and the other ordering would
make a shipped package permanently unreplaceable.

**A package that fails validation is dropped and logged, never served.** The
four questions `basemap.ts` asks on arrival are asked again at discovery,
because a client cannot report what it found: a bad archive draws nothing, and
the reader sees an area that is simply missing. The log line is the only thing
that will ever explain it.

**Discovery runs before `OnActivate` has handed the plugin an API.** `ServeHTTP`
and the page renderers are wired the moment the plugin loads, so `p.API` is
reachably nil. With no API there is no bundle to find and nowhere to send a
warning, and the honest answer is the configured directory alone; the warning a
rejected file would have produced is dropped rather than taken as a nil
dereference that stops the process.

**One bad package does not take the others off the map.** `loadPackages` drops
what does not answer and keeps the rest, so an install with six areas and one
half-copied file draws five.

**The package list is AWAITED before the style is built**, not read from a hook,
and that is a defect this phase shipped and then fixed. The map is created once
and moved thereafter, so a list arriving after creation never reached a style
again: the panel drew the global tier for the whole of its life on an install
that had detail areas, which looks exactly like an install that has none.

**The style carries one source and one layer set per package**, ids suffixed
with the area name. That is why the Go layer-agreement test cannot scrape
`source: 'detail'` any more and reads `DETAIL_SOURCE_LAYERS` from `maplibre.ts`
instead, with a TypeScript test holding the built style to that same list. The
style, the constant and every committed archive are one chain.

**The pages get the list in their shell**, as `data-packages`, because
`/api/v1/packages` needs a session they do not have. Its absence reads as NONE,
which is the opposite of how `data-maps` reads its own absence and is right for
the opposite reason: there, failing to draw a map is the silent costly
direction, and here a name invented from nothing would be a request for an
archive that does not exist.

**What Phase 1 ships is `indopacom-hawaii`**, z10-14, 6,808,975 bytes, which is what turns
every assertion above from one that skips into one that runs. The pilot
measurement, the per-zoom curve that decides how deep a bigger profile can go,
and the extrapolation to the thirteen requested areas of responsibility are all
in `build/maposm/README.md`.

**The roster is thirteen regions and two of them are committed.**
`build/maposm/regions.txt` carries `indopacom-hawaii` plus the twelve areas of
responsibility: Korea, Taiwan, Japan, the Philippines, the South China Sea,
Guam, the Persian Gulf, the Levant, the Red Sea, Ukraine, the Baltics and the
Horn of Africa. Hawaii stays the reference package because it is small enough to
commit and is what proves the whole lifecycle of build, package, transfer,
install and render. **`indopacom-guam` is bundled beside it** at 976,239 bytes,
which buys Andersen and the CNMI divert fields and makes the shipped plugin
exercise the multi-package path rather than the single-package one.

The other eleven **cannot ship in the bundle and are not in git**.
`TestArchivesFitTheBundleTogether` holds `world.pmtiles` plus every committed
package under 64 MiB, leaving about 15.5 MB once Hawaii and Guam are in, and the
eleven release assets are 2.44 GB together at z10-14. They build into the gitignored `build/maposm/out/`
and are attached to a plugin release by `make map-publish`, from where an
operator drops them into `LocationMapPackagesDir`. A reserved `bundled` profile
token in `regions.txt` is what routes an archive into `public/map/packages/`,
and `TestOnlyBundledRegionsAreCommitted` fails on any committed archive whose row
lacks it. The byte budget alone is not that guard: one accidental region fits
under it, and a guard that catches only the second mistake is not one.

**A multi-country region is merged, never cut**, and that reverses the reading
this file's generator notes started from. Seven of the twelve span more than one
Geofabrik extract, so `osmium merge` concatenates them and planetiler bounds the
output as it already did. A buffered `osmium extract` is the intuitive answer and
is the wrong one: `--strategy=smart` keeps a way only when one of its nodes falls
inside the box, and a way that spans the box with no vertex inside is dropped
whole. Desert administrative boundaries are mapped as near-straight ways with
vertices hundreds of kilometers apart, `filter.json` renders `admin_level <= 4`,
and a 0.25 degree buffer is 28 km, so cutting would erase a national border
through the middle of `centcom-red-sea` and it would look identical to a correct
build. The full argument, including why the performance case does not rescue it,
is in `build/maposm/README.md`.

Merge carries one constraint of its own: it is only correct across files cut from
the same planet snapshot, because a border way edited between two cuts appears
twice with two versions and both survive. `build.sh` refuses a region whose
extracts do not share a date suffix rather than leaving that to be discovered on
screen.

**Three regions are measured and they correct the size projection upward.** The
pilot is 241 bytes per km² of land, Korea 827 and Taiwan 1,590, so the `~1 GB`
figure this note carried for the full roster is a floor rather than a middle
estimate. Hawaii is the least representative region on the list rather than a
typical one: an island chain with a single metro area is not what an area of
responsibility looks like.

That changes nothing about what ships, because only the pilot is in the bundle
and a release asset's size is a download rather than a ceiling. Japan is the
largest region on the roster at 477,543,062 bytes, or 455.4 MiB, which is 56.6
MiB **under** the 512 MiB upload ceiling in `server/packages.go`, so every area
installs through the System Console uploader and none needs the copy-by-hand
path. The per-region numbers are in `build/maposm/README.md`.

### The page content policy

`Page.Capability` is how much of `default-src 'none'` a page gives back.
`PageStatic`, the zero value, gives back nothing and is what every page should
want. `PageMapping` is the only other one, and it exists for one thing: the map
pages run MapLibre, which is a real script file with a worker beside it, fetches
the basemap, and draws through a canvas.

```text
PageStatic   default-src 'none'; style-src 'unsafe-inline'; script-src 'none'|'sha256-...'
PageMapping  ... plus 'self' on script-src,
             worker-src 'self'; img-src data:; connect-src 'self'
```

Each of those is the narrowest form that works, measured rather than copied from
the spec. `style-src` keeps only `'unsafe-inline'`, because MapLibre's stylesheet
arrives through `style-loader` as an injected `<style>` and nothing emits a
`<link>`. `img-src` keeps only `data:`, which is the zoom control's glyphs: the
map draws through WebGL rather than `<img>`, and the style has no sprite, no
glyphs and no raster source. Both were wider for a while on the strength of what
MapLibre might need.

**This is a real reduction in a defense, on a route that echoes author text to
anybody who can write its query string, and it was taken deliberately.** Under `PageStatic` an escaping mistake in
the author's own text was inert markup: it could not execute, because the only
script allowed was the one named by digest, and it could not exfiltrate, because
`img-src` and `connect-src` were **absent** and their absence is what blocks
`<image>`, `<feImage>` and CSS `url()`. Under `PageMapping` an injected
`<script src>` to anything else on this origin runs, and an injected image URL
is a channel. **Escaping is now the only defense**, so a map page may never
interpolate a request value into script or markup without it.

What did not change: the error page and every DTG page stay on `PageStatic`, and
`font-src` stays absent because nothing loads a font.
`TestPageCapabilityDecidesTheWholePolicy` pins both policies as whole strings,
so widening either is a visible diff rather than a directive appearing in a
builder.

The digest is still kept **beside** `'self'` rather than replaced by it, for a
page that has an inline script. No location page does any more, so
`script-src 'self'` is the whole story there; `'unsafe-inline'` stays absent,
which is what keeps injected markup from executing whatever it is spelled as.

`ScriptSrc` is refused unless the page declared `PageMapping`, so a page cannot
name a script its own policy forbids, and it is refused unless it is
**relative**. Absolute would be a same-origin claim nothing checked, and
relative is also what makes a subpath install work: the page renderers are pure
functions of a query string and cannot see `SiteURL`, so `/map` writes
`./public/app/page.js` and `/decorate/<type>`, one level down, writes
`../public/app/page.js`. A test asserts the two differ by exactly one level,
because getting it wrong is silent: the page renders its shell and the bundle
404s, so the reader gets an empty document.

### The page bundle

`webapp/src/page/` is the pages' entry point, built by a **second webpack
configuration** into `public/app/page.js`. Two configurations rather than two
entries, because two things have to differ at once: the output directory, and
`externals`. Mattermost hands the plugin bundle React and Redux as globals, and
a page served from `/decorate` or `/map` has no Mattermost around it, so this
build carries its own React. That is what the second copy costs, and it is the
whole reason the pages could not simply import the panel before.

`publicPath` is `'auto'`, so lazy chunks and the MapLibre worker resolve against
the script's own URL. The page renderers are pure functions of a query string
and cannot see `SiteURL`, so deriving the base from where the file actually is
beats anything they could pass in. `applyBasename` does the same job for
everything that goes through `pluginBaseUrl`, reading the subpath back out of
the page's own address: that one assignment is what makes the basemap fetch, the
documentation link and the link between the two pages work on a subpath install
without a single URL being passed in from Go.

**The shell carries the parameters and the conversion, and nothing else.**
`renderRoot` writes `data-f`, `data-v`, `data-r` and `data-conversion` onto one
empty div. The conversion goes through `Convert`, the same function
`/api/v1/convert` calls, which is what stops a page and the sidebar disagreeing
about a coordinate: a public page has no session and cannot call that route, so
it is handed the answer instead of a second route being opened for it.

**A disagreement about the grammar degrades rather than blanking the page.**
`fromParams` runs on the server's own parameters, and the webapp keeps only a
copy of the canonical shapes, so it can refuse a token Go issued. That already
happened once, when the band class was widened in Go and not here. On the
sidebar the click handler simply stood aside and the browser opened the page;
on a page there is nothing to fall through to. So a refusal now logs and falls
back to a payload with no local coordinate, where every row comes from the
conversion the server already worked out.

**An unrecognized format id degrades the same way**, and refusing it was the
coarser half of the same drift behaving oppositely: `readPageData` returned null,
the entry point rendered nothing, and the reader got a wholly blank document with
the server's own conversion sitting unread three attributes away. An **empty**
id still refuses, since that means the shell was not written by this plugin.
`TestWebappFormatListMatches` pins the two format lists so the drift is caught in
Go rather than on somebody's phone.

Two things follow from an arbitrary string being a legal `format` on that path,
and both were live defects. `CANONICAL` is **null-prototype**, because
`CANONICAL['toString']` on an ordinary literal resolves up the chain to a
function, which is truthy and has no `.exec`, so `?.` sails through and the call
throws. And `gridResolutionMeters` refuses any id that is not `mgrs` rather than
reading the MGRS pattern for everything non-UTM: a token whose format this build
does not know, whose canonical happens to match the MGRS shape, was rendered as
`1 m grid, at center` with a 1 m cell drawn around it, which is a resolution
claimed from a grammar the page had just said it does not have.

**One palette, in `mapColors()`, and nothing else has a copy.** It used to be
declared again as `--map-*` custom properties in `pageStyles` for the Go
renderer. What `pageStyles` declares now is **Mattermost's own** theme
variables, `--center-channel-color`, `--center-channel-bg` and `--link-color`,
in both themes, which is what lets the panel's components style themselves
unchanged on a page that has no Mattermost around it.
`TestMapPaletteCarriesItsContrast` reads `maplibre.ts` and holds the measured
pairs to WCAG 1.4.11's 3:1 for non-text; the first palette was picked by eye and
sat at 1.46:1 in dark and 1.28:1 in light, which filling a window read as a
near-uniform slab with a dot on it.

**The map is drawn dark in both themes**, because the dark palette reads better
as a map: the land/water edge carries more of the frame, and the pin and cell sit
on a ground that is not competing with the panel around them. `mapColors` used to
sniff `--center-channel-bg` for lightness so the map followed `_theme` for free,
and it no longer does. **The cost, accepted deliberately: a light Mattermost gets
a dark map inside a light panel, and a light page gets a dark map under a light
table.** Everything around the map still follows the theme; only the map does
not.

`ALWAYS_DARK` in `maplibre.ts` is the whole of it, and flipping that one constant
restores theme-following everywhere. It is annotated `boolean` rather than left to
infer `true`, so the other branch stays live code to the type checker instead of
becoming unreachable.

**The light palette is kept, not deleted, and is held to both halves of still
being real.** `palette(false)` keeps it reachable; the Go contrast test proves
the hex values in the file are still legible pairs, and `the unused light palette
is still whole` in `style.spec.ts` proves something still returns them and that
they differ from the dark ones in every field. A palette nothing draws today is
exactly the kind of thing that rots into one that is wrong the day it is drawn
again, and those two tests are what stop it. `the palette does not follow the
theme` pins the decision itself, and it is meaningful because the unit
environment has no document and therefore sniffs as light: it is the case that
used to return the light palette.

**The pages have no inline script at all.** The digest that used to pin the copy
controls is gone because there is nothing left to pin, and the property is
stronger for it: `'unsafe-inline'` is absent, so an escaping mistake in the
author's own text cannot become a running script however it is spelled.

MapLibre is no longer vendored. It was copied into `public/map/vendor/` for the
hand-written page module, and that whole directory, the `map-vendor` target and
the two tests guarding the copy went with it: webpack resolves it from
`node_modules` for both builds now. It still ships twice, once per bundle, but
from one source rather than one source and one copy.

### What the pages gave up

**The readings no longer render without JavaScript.** They were server HTML, so
they appeared instantly and survived a failed script; now the page is a shell
until the bundle runs. That is a real regression on the page the mobile app
opens, and it was accepted deliberately in exchange for one implementation of
the table.

What softens it: the shell is about 4 KB and carries every value already, so
nothing waits on a second request; the bundle is one cacheable response for
every coordinate a reader ever opens; and the readings that used to be
Go-rendered are still Go-*computed*, in the `Conversion` the shell carries.

**WebGL2 is required to see a map**, on every surface including the standalone
page, where the SVG renderer always drew one. Every reading still renders, and
the note says which of the two failures it is: reporting a load failure as a
missing capability sends the reader, and whoever they report it to, looking at
the wrong thing.

### What the map states, and what it does not

**A pin is never drawn at a position the projection cannot represent.** Web
Mercator caps at ±85.0511287798066 while the grammars validate latitude to 90,
so `89.9000, 12.0000` is a decoratable token today and clamping it would put the
pin 550 km from what the author wrote. Past the limit the map is omitted and a
line says so. `mapNorthOfMercator` and `mapSouthOfMercator` are those two
strings.

**The cell is always drawn, and its SIZE carries the resolution.** A coordinate
is drawn as a rectangle the size of the token's own resolution, plus a dot. Two
things this gets right that a radius does not: `resolutionAt` returns a cell
**edge**, so a radius equal to it draws a footprint twice the size of the square
the token names; and a ring around a point reads as a range ring to this
audience, and contradicts the page's own note that a grid reference names a
square. Each axis uses `axisResolutionDegrees`, so a mixed-precision pair like
`21.3353N,157.9W` is not squared off to its coarser half. For MGRS and UTM the
rectangle **bounds** the grid square rather than being it: an MGRS square is
axis-aligned in UTM space and rotated by the grid convergence, which is
invisible at this scale.

There is no minimum size and no threshold below which the cell is dropped. These
surfaces zoom, so there is no one scale to test against: a meter-wide cell is
invisible until the reader zooms far enough to see it, which is more honest than
a number guessing on their behalf.

There was a 6 px floor in `drawableCell` contradicting exactly that, and the way
it was wrong is the point rather than the size of it. `applyView` runs on a
change of selection and on **Reset view**, and at the time nothing listened for
`zoom`, so the floor was measured once at the opening camera and never again: a
square dropped there stayed dropped however far the reader zoomed in. (There is
one `zoom` listener now, for the zoom readout, and it is not a way to
reintroduce a threshold: it reports the camera's zoom and decides nothing about
what is drawn.) `maxZoom` was 6 at the
time, so the visible cost was small (a 10 km cell reaches only about 10 px at that zoom, which
the pin covers anyway), but a threshold evaluated at a scale the reader has left
answers a question nobody asked.

**Zoom follows a target ground span, not a zoom level**, because Mercator scale
is 1/cos(lat) and a constant zoom is not a constant answer: the same 320 px is
about 2,940 km at the equator and about 1,000 km at 70°N. `zoomForSpan` is in
`span.ts` and every surface calls it. It was briefly written twice, once in Go
and once for a page module, and neither copy exists now: nothing in Go computes
map geometry at all, and `mapcell.go` went with it.

**The label is escaped specifically.** It is a visually-hidden `<span>` beside
the canvas rather than an `aria-label` on the container, so React's own text-node
escaping is what does it, and `renders a hostile region as text rather than as
markup` in `LocationMap.pw.tsx` holds it. Its content is a country name from
generated data, so nothing from a request reaches it today, but the escaping is
what makes that a defense rather than an accident. It was described here as an
`aria-label` held by a Go test long after it was neither, which is the state a
guard rots into when the only thing asserting it is a sentence.

### Two zoom numbers, and why they are those numbers

**`TARGET_SPAN_METERS` decides what a reader sees first**, and it is 400 km. It
was 2,400 km, which opened the panel at about z3.4 at the equator: two and a half
zoom levels below the map's own ceiling, with a one meter grid reference framed
exactly like a whole-degree one. It is pinned by no test in either language, and
it is the single constant with the largest effect on whether the map feels like
it is showing you anything.

**`MAX_ZOOM` is 9, and it was 8, and the arithmetic did not change.** 1:10m data
carries roughly 5 km of positional accuracy, and a 512 px tile puts 78271.5/2^z
meters in a pixel, so that error is about 16 px at z8, 33 px at z9 and 65 px at
z10. At 16 px a coastline reads as generalised; at 65 px it reads as fact to a
reader with no way to tell, which for an audience acting on grid references is
the wrong way to be wrong. z9 sits between those, and taking it was a deliberate
trade rather than a revision of the measurement.

**What z9 buys is narrow, and is worth stating so it is not re-derived as
generous.** The sources are the same 10m files, so z9 carries no geometry z8 did
not, and vector tiles magnify without blurring, so this is not the raster
sharpness argument. It buys two things: halved coordinate quantization inside a
tile, since a 4096-unit extent spans about 38 m at the equator at z8 and about
19 m at z9; and more room for the collision index to place labels. It buys no
accuracy, and 33 px of possible error is what it costs.

The archive is built to exactly this depth and `TestArchiveDepthMatchesTheData`
holds the two together, so neither can move alone. `MAXZ` in
`build/maptiles/build.sh` is the pipeline's half, written once so the depth
cannot move for one layer and miss another.

**The camera is a second number, and it deliberately runs past the data.**
`DATA_MAX_ZOOM` is 9 and `MAX_ZOOM` is 17. They were one constant, and that made
a cell impossible to inspect at the resolution its own token carried: the
rectangle around a pin says how precisely the text located anything **by its
size**, and at z9 a 10 m grid reference is about a third of a pixel. A reader
could see that a cell existed and never how small it was, which is the one thing
it is drawn to say.

17 is where a 10 m cell reaches about 20 px, which covers the fine end of what
the grammars actually produce: 10 m MGRS, and four decimal places of a degree
(about 11 m). A 1 m grid reference is still about 2 px. Going deeper was
declined: it needs z20, where the basemap is magnified 2048 times and is a flat
color with a straight line for a coastline.

Past `DATA_MAX_ZOOM` MapLibre overzooms. For vector tiles that magnifies
**without blurring**, so lines stay crisp and only their generalisation is
wrong: the failure the single ceiling existed to prevent is now invisible rather
than impossible.

**Nothing on the map states that in words.** There was a notice reading "Basemap
detail ends here", drawn past `DATA_MAX_ZOOM` and repeated in the accessible
label, and it was removed. Record this as the state of things rather than as an
oversight: a reader past the ceiling is looking at a coastline that may be five
kilometers from where it is, drawn at street scale, and the only thing saying so
is the **zoom readout**, which requires knowing that the data stops at 9. If
that turns out to be too little in practice, the notice is the thing to bring
back.

What does still hold: **`zoomForSpan` clamps to `DATA_MAX_ZOOM`**, so nothing
ever *opens* into overzoom. It is a gesture a reader makes, never a default they
are given, which is what keeps the unannounced magnification something they
chose. `the camera may overzoom and the opening view may not` in `span.spec.ts`
pins both halves.

**The zoom is on the map**, bottom-left, as `z6.3`, and with the notice gone it
is the only indication that the camera has left the data. One decimal, because
the wheel and a trackpad pinch are continuous and a whole number would sit still
through most of a gesture and read as broken. It is seeded at construction as
well as read from the `zoom` event, since building a map at a zoom fires no
event and the readout would otherwise be blank until the reader's first gesture.

The bottom edge is a row: the readout at the left, MapLibre's scale bar at the
right.

**A line width is in screen pixels, so every width interpolation has to reach the
ceiling too.** Without a z9 stop the whole road network and every river holds its
z8 width while the map doubles around it, which reads as the network thinning out
exactly where a reader has zoomed in to see it. `roads` and `rivers` carry that
stop; the deliberately faint context strokes (`railroads`, `admin-1`, `borders`)
are constant widths and stay that way.

Simplification follows the same logic in reverse. Douglas-Peucker tolerance
scales as 1/2^z, so full detail costs 81% more vertices at z5 and 11% at z8: the
zooms where shape is visible are the zooms where keeping it is cheap. The
pipeline uses tippecanoe's default at z7 and above, and four times that below.

### The boundary classification, which is unresolved

Natural Earth classifies its boundary lines, and this plugin ships none of that
classification. Among the 515 admin-0 lines are ten `Line of control (please
verify)` (the Korean DMZ, the Cyprus buffer zone, UNDOF, the 1974 ceasefire
lines), thirty-nine `Disputed`, thirteen `Indefinite` and four `Indeterminant
frontier`. All of them draw in the same stroke as the France-Germany border, and
adding province boundaries doubles the question.

**Stripping the field is not neutrality here.** Silence about the attribute is an
assertion that a ceasefire line and a settled border are the same kind of thing,
which is a stronger claim than the one the `FCLASS_*` decision refused to make.
Three coherent answers exist and one has to be chosen deliberately: draw them
alike as now, drop the contested features so the map is simply silent where
boundaries are contested, or keep one `disputed` flag and dash those lines the
way most basemaps do. The third depicts rather than determines but uses Natural
Earth's classification to do it. This is recorded rather than settled.

**`glyphs` and `sprite` are the only two style fields MapLibre resolves as
URLs.** There is still no `sprite`, and `glyphs` points at font ranges bundled
under `public/map/fonts/`, so nothing reaches off-origin. The style is asserted
against that: every URL in it must resolve under this plugin's own base, which
is a stronger statement than the "there are no URLs" it replaced, because it
names where bytes may come from rather than which two fields are forbidden.

There were no `symbol` layers at all until the country labels arrived, on the
argument that an air-gapped install has nowhere for glyphs to point. Bundling
them answers that. **It also costs no CSP**: glyph ranges load over fetch on the
main thread, which `connect-src 'self'` already grants, so the page policy is
unchanged and `font-src` stays absent. A sprite would cost one, because its
image half loads under `img-src`, which is why there still is not one.

The fonts are SIL OFL 1.1, and SDF ranges generated from a TTF are a Modified
Version, so `public/map/fonts/LICENSE.txt` ships beside them because the license
requires the notice to travel with them. That is a redistribution condition
rather than a courtesy, which is what makes it unlike the Natural Earth credit
this plugin deliberately dropped. Noto declares no Reserved Font Name, which is
what permits the generated ranges to keep the `NotoSans-Regular` name.

**WebGL2 is now required to see a map at all**, on every surface including the
standalone page the mobile app opens, where the SVG renderer always drew one.
That is the second real cost of consolidating, and it is why the note says which
of the two failures it is: reporting a load failure as a missing capability
sends the reader, and whoever they report it to, looking at the wrong thing.
Every reading in the table still renders, because the conversions and the Region
row are worked out on the server and do not depend on it.

### Naming the country

Derived server-side by point-in-polygon against `mapdata.Countries`, with a
per-feature bounding box as a prefilter, and served as a **string** over the
wire, which is what `/api/v1/convert` is for.

**It is no longer a row.** It was one, on the grounds that an unlabelled 110m
coastline at 300 px identifies Italy and does not identify Chad from Niger, and
that a row answers that with the map hidden, with no WebGL, and with the basemap
unavailable. Retiring it gives up exactly that: the country now reaches a reader
**only through the map's accessible label**, on every surface. It was also
printed under the picture on the map page until that bar was cut back to the
author's text. No map, no country, and with the map drawn, no country for anyone
reading it with their eyes. The value is still computed and still travels in the
`Conversion`, so bringing the row back is one entry in `Rows` and one in `ROWS`.

**The field is `ADMIN` from `ne_110m_admin_0_countries`**, the de facto
administering entity, not `SOVEREIGNT` (the claimed sovereign) and not `NAME`.
That choice, not the citation, is the sovereignty decision, and it is a policy
call rather than a technical one. The value carries its own citation,
`United States of America (Natural Earth 110m)`, so it reads as a basemap lookup
rather than a determination.

**A position in no polygon yields nothing** and never guesses at a nearest
country. On a shared border the **lowest feature index wins**, and the generator
sorts by name so that answer cannot change when the source is regenerated.
Natural Earth splits geometries at the antimeridian, which is why Fiji is three
polygons; a fixture pins each side.

The 2-decimal rounding used for display **must not reach `admin.go`**. At 1.1 km
quantization adjacent countries stop sharing edges, so borders develop overlaps
and gaps, and a gap would then be misread as the intended "no answer".

**The country never goes through `remote()`.** That helper turns any empty value
into `converting…` or `unavailable`, which over open ocean would report an
outage where the honest answer is that there is no country. It reached the map
unwrapped while it was a row, and still does.

### What `Conversion` may carry

Exactly the rows the panel cannot work out for itself. The webapp has its own
renderer in `format.ts` and uses it for every textual grammar; what it does not
have is a projection. So the two grid rows are on the wire because they need
one, the three coordinate rows because a grid token has no `Coordinate` for
`format.ts` to render from, and the country because the polygons are Go-only.
The country is the one field on the wire that is not a row: it reaches the map's
accessible label and nothing else.

`Lat` and `Lon` are the one pair of numbers, and they are there because a map
needs a position rather than a rendering of one: the resolution rule reaches the
map through the drawn cell, not through digit counts. The webapp refuses a
non-finite or out-of-range value on arrival (`asConversion`), and it must never
gate on truthiness, since `0, 0` is a position.

Everything else a panel shows, it computes. Adding a field nothing reads is how
a payload starts drifting from its consumer, and `webapp_sync_test.go` holds the
two definitions to the same names, **the same types** and the same order: while
every field was a string, name agreement implied type agreement, and with
numbers on the wire a TypeScript `lat: string` against a Go `float64`
type-checks, ships, and puts the pin at `NaN`.

### The map page

`/map` is a route of its own, a sibling of `/decorate` rather than a mode of it.
The decorator route answers "what is this token" and every page under it opens
onto a table; this answers "where is it" and gives the window to one picture,
which is what a reader following **Open larger** from a 300 px sidebar is asking
for. It validates through `validateParams`, the same gate, so a link one page
refuses the other cannot render.

**Both pages are a 4 KB shell now.** They were about 110 KB and 130 KB when the
basemap was inlined and the table was Go-rendered. The basemap, the bundle and
the MapLibre chunk are each one cacheable response shared by every coordinate a
reader opens, instead of a fresh copy of the world inside every page on
`Cache-Control: private, max-age=300`.

**One set of controls, because there is one map component.** They cannot
disagree about what the buttons are called: `LocationMap` writes them once, and
the page passes `fill` to give the window to the picture and omit the "Open
larger" link that would point at itself. The label is **Reset view**
everywhere. Zoom is MapLibre's own `NavigationControl` on all three and the
scale bar sits bottom-right on all three, because bottom-left collides with the
hint.

**Beneath the picture is the author's own text and the way back, and nothing
else.** The bar also carried the canonical token, the position in decimal
degrees, the region and the basemap credit. All four are readings, and
**All readings** is one link away and is nothing but readings, each rendered
with its label beside it; four of them crammed unlabeled under a picture is a
worse table rather than a summary. What is left is the one line saying which
text this map was drawn for, through the same `vouchedText` the table's leading
row uses, so the two surfaces cannot disagree about which text is the author's.

**No surface prints the basemap credit.** "Natural Earth 110m" sat under the map
on the panel and the readings page, and in this bar on the map page, and it is
gone from all three. Natural Earth is public domain, so the credit was a
courtesy rather than a requirement, and the plugin still names the basemap where
a reader would go looking for it: `public/help/panel.html` says what it is and
how coarse it is. The **region's own value keeps its citation**, which is a
different thing and must not be removed with it: that says where the country
came from, and it is what stops a border lookup reading as a determination.

`LocationMap`'s caption is therefore the "Open larger" link alone, and it is
rendered only when there is a link to put in it, so the panel does not carry an
empty strip under the frame. Its `justifyContent` moved from `space-between` to
`flex-end` at the same time: with one child, space-between falls back to the
left edge, which is not where that link has ever been.

### One map instance, reused across selections

The panel keeps its map and moves it, because tearing down a WebGL context per
coordinate walks the browser's cap of about sixteen and evicts the map the
reader is looking at. Everything below follows from that one decision, and each
of these was a bug before it was a rule.

**A source decided at construction is a source the next selection cannot have.**
`buildStyle` used to add the `accuracy` source and its layers only when the
opening position stated a CE. An event stating accuracy after one that stated
none therefore drew no ring at all, for the life of the panel, while the
readings beside it said "9.5 m circular". `geometry` had already been made
unconditional for exactly this reason; `accuracy` now is too. An empty
collection costs nothing, which is cheaper than being wrong on the second
selection. `withMarker` is the remaining conditional and is safe only because it
is fixed per surface: a Cursor on Target map always states markers and a
location map never does.

**A marker image is registered per color, so registration belongs with the
draw.** `addMarkerImages` ran once in the `load` handler. A second event whose
affiliation differed from the first named an image that was never added, and the
symbol layer drew nothing for it: not a fallback dot, nothing. It runs from
`applyView` now, where the features are written, and is idempotent through
`hasImage`/`updateImage`.

**The redraw is keyed on what the overlays are, not on the objects carrying
them.** `overlayDigest` is that key. Two bugs met here. `markers` was read
through a ref and named in no dependency, so a marker set that changed over an
unchanged primary position never redrew, and the panel listed one event while
the map drew three. `geometry` was named as a raw dependency and the Cursor on
Target card builds its geometry inline, so a fresh object arrived every render
and `applyView` re-framed the camera, throwing away any pan or zoom the reader
had made. Identity was wrong in both directions at once; a structural key is
right in both.

### The panel map

The sidebar draws the same coordinate with the same library, which is the one
dependency this plugin takes on for a feature. It costs about 250 KB gzipped and
22 packages on the npm SBOM, permanently on the Grype gate.

**Suppression is not an available mitigation for it.** `.grype.yaml` allows
suppression for a dev-only transitive dependency or one Mattermost externalizes,
and MapLibre is neither: it ships and it runs in the reader's browser. The
process for an advisory against it is upgrade or pin.

**The library is dynamically imported**, so it costs nothing until a reader opens
a coordinate. That is a bytes-over-the-wire property and nothing else: it changes
neither the SBOM surface nor reachability. Types come in through `import type`,
which TypeScript erases, so the entry bundle contains no reference to `maplibre`
at all and the 950 KB chunk stays lazy. Both the style and the map handle are
typed as MapLibre's own `StyleSpecification` and `Map` rather than cast, so a
typo in a layer id or a paint property is a compile error rather than a runtime
failure that presents as a map stuck on "Loading map…".

**Chunks need no new route.** `plugin/environment.go` copies the whole directory
containing `bundle_path` into Mattermost's static plugin directory and renames
only `main.js`, so a sibling chunk in `webapp/dist/` is already served at
`/static/plugins/<id>/`. `plugin.json`, the Makefile and `.gitignore` are all
untouched; the only change is `chunkFilename` with a contenthash and a validated
`__webpack_public_path__` set in `index.tsx` and nowhere else. It has to be there
rather than in `plugin_url.ts`, because the component tests load that import
graph through Vite as strict-mode ESM, where a module-scope assignment to a
webpack free variable is a `ReferenceError`.

**The worker is served, not laundered through a blob.** MapLibre builds its
worker from `URL.createObjectURL` by default, which needs `worker-src blob:` in
whatever CSP the host serves under. It is emitted as a hashed asset and handed
to `setWorkerUrl` instead, by `maplibre.ts`, which both builds share. The
stylesheet is imported in the same module, without which the zoom control's
glyphs, which are CSS background images, and the control container's corner
placement are simply absent, leaving invisible focusable elements.

**One map, created once and moved.** Rebuilding per coordinate meant a fresh
WebGL context and a re-tessellation of the whole basemap on every click, against
the browser's cap of roughly sixteen live contexts, and the panel stays mounted
across a change of selection so those clicks arrive in a run. Creation and
movement are separate effects; the position is read through a ref at apply time,
so the asynchronous `load` handler positions the map at whatever the reader has
selected now rather than at whatever was selected when the load started. The
pages need none of this: a page is one coordinate and is thrown away.

That split introduces the one hazard rebuilding used to hide for free: a **stale
pin**. Clicking a grid coordinate while an earlier one is drawn would leave the
previous position on screen beside the new one's readings until the conversion
lands, and permanently if it never does, so both overlay sources are cleared
whenever the position is unknown or beyond the projection.

Camera moves are `jumpTo`, never `flyTo`. A world-crossing animation on every
click is a vestibular risk and says nothing in a box this size, which is also why
`prefers-reduced-motion` needs no special case. Rotation is disabled, because a
rotatable map with no compass means a reader misreads every bearing taken off it.

**The wheel zooms, and the cost is that it does not scroll.** The map sits at the
top of a panel that scrolls, so a wheel event over it is consumed rather than
passed to the sidebar; a reader reaching the readings below has to move the
pointer off the map first. `cooperativeGestures` is the one-line alternative and
trades a modifier key for that, which is the trade Google's map embeds make.

**`maxZoom` belongs on the map, not only on the arithmetic.** `zoomForSpan`
clamps the opening view to `DATA_MAX_ZOOM`, but the controls and the wheel let a
reader leave that range, so the camera's own ceiling is `MAX_ZOOM` and the two
are different numbers on purpose (see "Two zoom numbers"). Between them the
basemap is a coastline generalised by about five kilometers drawn at street
scale, and the zoom readout is the only thing that hints at it. **Reset view**
restores the opening view, because once a reader can zoom and pan there is
otherwise no way back to the pin.

**The probe is memoised and releases its context.** `hasWebGL2` is called on
every creation attempt, and an unreleased probe context is a real driver
allocation: probing afresh each time walks the same sixteen-context cap and
evicts the oldest, which is the map the reader is looking at, silently and with
no placeholder.

**The note is derived, not assigned.** `positionNote` is one expression over
`beyond`, `known`, `pending` and `loaded`, and `failure` is separate so the two
cannot overwrite each other. It was written from three effects and two event
handlers, and two of them disagreed: the `load` handler set it to null while
`applyView` was clearing the pin for a position it could not draw, leaving a map
with no pin and nothing saying why.

**Readiness belongs to the map, not to the effect run that made it.** The `load`
handler is guarded on `map.current !== instance`, never on the creation effect's
`live` flag. It was guarded on `live`, and the map is stored on a ref and removed
only on unmount, so it outlives that run: a reader who clicked a second
coordinate while the first was still loading left `ready` false **forever**,
`applyView` no-opped from then on, and the panel sat on "Loading map…" until the
page was reloaded. The first click is the slowest one, since it pulls the 950 KB
chunk, so it is the click most likely to be interrupted.

**An error after the map is usable is ignored.** `setFailure` runs only while
`ready.current` is false. A one-way latch replaced a working map with a notice
saying it could not be loaded.

**The basemap distinguishes a broken deploy from a bad minute.** Definitive, and
therefore remembered so that no further request is made: a non-2xx response, a
body too short to be a header, wrong magic bytes, a spec version other than 3, a
tile type other than MVT, a `maxzoom` shallower than the camera's ceiling, and a
200 whose `Content-Length` shows the server ignored the range request. That last
one looks healthy and is not: the pmtiles reader applies the same test to every
tile and throws, so accepting it would pass a deploy whose every tile then
fails.

A timeout or a network throw is **not** remembered. Latching on those means one
stalled fetch tells a reader the map is broken for the rest of the session, with
a reload the only way back, and a request can stall on any network rather than
only a bad one. `loadMapLibre` makes the same split for the library chunk, and
bounds it with the same 10 seconds, because a chunk fetch that **hangs** never
rejects: clearing the memo in the catch alone left every retry joining one
pending promise, which is the identical failure one step removed.

**That retry has to be reachable from the component, and twice it was not.**
Both failures were the same shape and neither was visible from `basemap.ts`:

- `LocationMap`'s creation effect kept the position out of its deps, so after a
  failure it never ran again and `loadBasemap()` was never called again. It also
  failed asymmetrically, which is the tell: `known` flips per selection for a
  grid token and stays true across every textual one, so grid links retried and
  lat/lon links did not.
- A map that was **built and then errored before `load`** stayed in the ref, so
  `map.current` was truthy forever and every later attempt returned at the
  guard, leaving a frozen map under a stale note with `applyView` no-opping on
  `!ready.current`. That is the likelier failure, since `loadBasemap` reads only
  127 bytes and every tile fetch happens after construction. The error handler
  therefore **releases the instance** rather than only reporting, and is
  identity-guarded like the `load` handler beside it.

A verdict belongs to the coordinate that produced it, so a change of position
retires it, in an effect of its own ahead of creation rather than inside it:
inside, the clear sat behind the `map.current` guard and never ran in either
case above. `NO_WEBGL` is kept rather than cleared, since `hasWebGL2` is
memoised precisely because that answer cannot change mid-session.

Both halves are pinned, and pinned by tests that were **checked against a
reverted fix**: an earlier version of the second one released the held worker,
which let the original instance load and clear its own verdict, so it passed
whether or not the dead map was ever let go.

The whole-file SHA-256 the webapp used to compute is gone with the GeoJSON
basemap it checked: only a 127-byte header is read now. That check moved to
where the whole file is in hand, `make bundle`, which is also where it belongs,
because `crypto.subtle` is undefined on a plain-HTTP origin and so it silently
passed on exactly the installs it existed for. That is the posture the copy buttons
already take, and it is why `map-data-check` runs in `make test`: drift is
invisible on HTTP and fatal on HTTPS. The page module does **not** check the
digest, because it has no bundled constant to check against: the page and the
basemap ship together and are served by the same plugin.

**The map is hideable and is not a row.** `MAP_ID` and `SectionMap` carry it, and
`HideableID` is a separate union rather than a widened `RowID`, because the panel
declares its values as an exhaustive `Record<RowID, string>` and a key with no
string value does not type-check there. The webapp's read path is deliberately
forgiving, so a regression that narrows it back drops the id silently: the reader
unticks Map, saves, and it is back after a reload with nothing logged.

### Turning maps off

Four admin switches, in the **Maps** section of the manifest: `EnableLocationMap`
over `EnableLocationMapPanel` (the sidebar and the hover card),
`EnableLocationMapInline` (under a coordinate-only post) and
`EnableLocationMapPage` (`/map`). All four ship on, and all four are ANDed with
`EnableLocation`, since a map is only ever drawn for a coordinate this plugin
decorated. `Plugin.locationMaps` does that AND, beside `locationFormats`, and
hands the result to `location.Decorator.Maps`.

**Off means nothing is FETCHED, not merely nothing is drawn.** That is the whole
point: the pmtiles archive is 43 MB, the glyph ranges are beside it and the
MapLibre chunk is about 950 KB, which together are the largest thing this plugin
transfers. Gating the four `<LocationMap>` mount sites is what achieves it, so
`loadMapLibre` and `loadBasemap` need no switch of their own: they are reached
only from that component.

**What it cannot do is make the archive unreachable.** Mattermost serves the
bundle's `public/` directory itself, before `ServeHTTP` sees the request, so the
switch stops clients *asking* and the file ships either way. A build that omits
`public/map/` is a separate piece of work and was declined as out of scope.

**These are the only switches read at render rather than at decoration**, and the
`Formats` doc comment names `Maps` as the exception so the two rules cannot be
confused. A format governs text already written permanently into a message; a map
is drawn afresh each time somebody looks.

Per surface, the mechanism differs, and each one is chosen rather than uniform:

- **`/map` answers 404**, gated in `ServeHTTP` with its own code
  (`errcode.HTTPMapDisabled`, TF-12004). The only route a switch removes
  outright, because that page **is** the map: there is no reduced version worth
  serving, where `/decorate/location` still has every reading.
- **The post is not stamped.** `location.Decorator.PostType()` answers `""` when
  the inline map is off, and `decorators.StandalonePostType` already reads `""`
  as "this decorator renders no post body", so `stampStandalonePost` needs no
  change and the stamp is never written rather than written and ignored. That
  distinction is the substance of the switch: the stamp is what costs the post
  its Elasticsearch and OpenSearch matches, and `Post.Type` survives every edit
  with no `MessageWillBeUpdated` hook to clear one. **Posts stamped while it was
  on keep their type forever**, and render as the link alone.
- **The hover card disappears entirely**, and costs nothing while it does.
  `LocationHover` is split into an outer gate and an inner card so `useConversion`
  is never reached, the same shape `LocationInline` uses: a hook that runs is a
  request that goes out, and hooks cannot be called conditionally, so gating
  inside one component left a maps-off install firing a conversion per hovered
  token for a card nobody would see. The gate returns `null`, and the
  framework's `DecoratorTooltip` now carries `HOVER_CARD_CLASS` with a
  `:empty { display: none }` rule in `buildDecoratorStyles()`, so a Hover that
  renders nothing gets no chrome instead of an empty bordered box. That rule
  cannot be an inline style, since `:empty` is a selector and every other style
  in this plugin is an inline attribute; the class name is written once in
  `registry.ts` so the element and the rule cannot drift apart. It is what lets a
  decorator decline a card **at render** rather than only by declining to declare
  a `Hover` at all.
- **`Customize` stops offering the tick box** for a surface that is off, because
  one that changes nothing a reader can see is worse than an absent one. What is
  *stored* is untouched: the read path already ignores an id nothing renders, so
  the switch coming back returns each reader to the choice they had made.

**The webapp learns this from `GET /api/v1/features`**, which is the only channel
from plugin configuration to the webapp and had to be built: Mattermost hands
plugin settings to system admins alone, the store given to `initialize()` is used
only to dispatch RHS actions, and there is no reducer. It answers **per surface**
rather than per setting, so the parent AND stays in Go and the two sides cannot
reach different conclusions from the same switches.
`webapp/src/features/` mirrors `preferences/`: module-state cache, one in-flight
promise shared by every caller, `useFeatures()`, and the same `CACHE_TTL_MS`
rather than a second cross-language number. `TestWebappFeatureShapeMatches` pins
the payload, and it is a worse seam to get wrong than the `Conversion` one:
every field is read as a boolean, so a drifted name reads `undefined`, which is
falsy, and the symptom is a map that silently stops drawing on an install that
never touched the switch.

**The two defaults in that store are opposite, and the pair is the design.**
`NO_FEATURES` while the answer is in flight, because assuming maps are wanted and
correcting afterwards would pull the archive once per tab on exactly the installs
the switch exists for; `ALL_FEATURES` on a failed **first** read, because nothing
may fail the panel into permanently hiding a feature the admin is paying for.
"Not answered yet" is a moment and resolves itself; "could not be answered" could
last. A failed read does not stamp the clock, so the next mount retries. A
payload that does not match the shape throws rather than defaulting a field,
since `Boolean(undefined)` is a confident `false` and would report a rename as an
admin decision.

**A failed REFRESH keeps the last good answer**, which is a third case and not a
detail of the second. This store was written from `preferences/store.ts` without
its fix and reproduced the defect that file documents: the cache lapses every
`CACHE_TTL_MS`, so on a maps-off install one failed refresh flipped every surface
back on and started the archive downloading, and because a failure does not stamp
the clock it flapped for as long as the server was unwell. `loadedAt === null` is
the test for "has a good answer ever landed".

**The read is bounded at ten seconds**, the same bound `basemap.ts` and
`loadMapLibre` put on theirs and for the same reason: a fetch that *stalls* never
rejects, so `inflight` is never cleared and every later caller joins one pending
promise. Here that fails **closed**, which is the one direction this store must
never fail in: every map off, on every surface, indistinguishable from an admin
having switched them off.

**`/api/v1/features` answers 503 while the configuration has not loaded**, rather
than reporting the zero value. Everywhere else "not loaded" reads as "every
switch off" and that is the safe direction, because the answer is discarded
immediately. Here it is handed to a client that stamps its cache on a 200 and
keeps it for half an hour, so an unloaded configuration would be cached as an
admin decision. `configurationLoaded()` is what tells the two apart, and the
window is not only a startup race: `OnConfigurationChange` returns before
`setConfiguration` when the load fails.

**The pages read it out of the shell instead**, as `data-maps`, a comma list of
the surfaces that are on. A page is handed everything it needs in one document,
and `/api/v1/features` needs a session the route does not otherwise require.
The attribute name, the two tokens and the separator are all held to the
webapp's reader by `TestWebappMapSurfaceAttributeMatches` and its sibling, which
is the guard this seam was missing: each side pinned its own copy, so renaming
one together with its own test left both suites green while the standalone
readings page silently stopped drawing a map.

Written **unconditionally, empty included**: an absent attribute has to mean "a
shell older than this bundle", which `mapsFrom` reads as every surface on, so if
"off" were also spelled as absence a maps-off install would keep drawing. Read
against a closed set, so a surface a later server knows about is ignored rather
than drawn, and `inline` is never on the wire there because no page has posts in
it.

Deliberately not done: tightening `PageMapping` back toward `PageStatic` on a
page that now draws no map. It would still need `script-src 'self'` for the page
bundle, so only `worker-src`, `img-src` and `connect-src` could go, and that is
worth its own change with its own whole-policy test.

### The map under a post

**Two surfaces reach this now**, not one: a coordinate-only post, and a Cursor
on Target card. Both read `features.mapInline`, both respect `INLINE_ID`, and
both pay the `Post.Type` costs below. The CoT card additionally passes
`accuracyMeters`, which draws the event's stated circular error as a geodesic
polygon around the pin; `LocationMap` takes it as an optional prop and every
location surface omits it, so a coordinate's own precision keeps being carried by
the cell rather than by a circle. See [`cot.md`](cot.md), "The CE circle", for
why the vertices are geodesic and why an unstated accuracy draws nothing.

When a posted message is **only** a coordinate, the server stamps the post with a
custom type and props, and the webapp renders that post's body itself: the link
exactly as before, plus the map under it. Everything else is unchanged, which is
what makes this additive rather than a second rendering path.

**The verdict is the tagger's, not a regex over the decorated text.**
`DecorateWithResult` returns a `Result` beside the string, and `SoleToken` is
`len(accepted) == 1` **and** that candidate's `match` covering the message apart
from surrounding whitespace. Both halves are needed: `see MGRS: 4QFJ0906059620`
also accepts exactly one candidate. It is `match` rather than `replace`, because
a pattern may match more than it rewrites: the airfield pattern sets
`ReplaceGroup` so the `//` a USMTF set line ends with stays in the message, and
`replace` therefore stops short of it. Measuring `replace` would refuse
`DEPLOC:PHNL//`, which is the ordinary military spelling and the case this
exists for. Guards are zero-width by construction, so `match` is the token, or
the label and the token, and nothing else. The verdict is taken **before** `applyReplacements`, which
re-sorts `accepted` in place.

Recovering this from the decorated string instead would mean undoing
`labelEscaper` and `buildURL`'s percent-encoding to learn something the tagger
had in hand two frames earlier, and it still could not tell a lone token from one
with a moniker in front of it.

**A decorator opts in through an optional Go interface**, `PostRenderer`, type
asserted rather than added to `Decorator`. That is the Go analogue of the
webapp's optional `Hover`, and it leaves the five existing implementations
untouched. DTG declares nothing, so a lone date-time group stays an ordinary
post and pays none of the costs below.

**One custom type per decorator, not one for the plugin.** That is what buys the
fallback: `post_message_view.tsx` checks `Object.hasOwn(pluginPostTypes, postType)`
with no `else` and falls straight through to `PostMarkdown`, so a type an older
bundle does not know, or any type at all once the plugin is disabled, renders as
ordinary markdown. A plugin-wide type would instead route to a component that
then has to render the message as literal text.

**`Posts.Type` is `VARCHAR(26)` and `Post.IsValid` checks the `custom_` prefix
but never the length.** An over-long type is therefore not a bad render, it is a
database error at save time, which is the author unable to post at all: exactly
the failure `decoratePost` is arranged to avoid. `StandalonePostType` refuses
anything over `PostTypeMaxLen` and the decorator simply gets no inline rendering,
which is why the type is `custom_tf_location` and not the spelled-out form.

The stamp is guarded on `post.Type == ""`, because another integration's custom
type is real mission content, and it uses `AddProp` rather than a map assignment
so their props survive. It happens only on the path that keeps `decorated`,
which is the one branch where "we found a standalone token" and "the stored
message is decorated" can diverge.

**What setting `Post.Type` costs, measured against Mattermost master rather than
assumed.** All three were checked and accepted deliberately:

- **Elasticsearch and OpenSearch exclude the post from search entirely.** Their
  `SearchPosts` builds an **allowlist** of `type: default` and
  `type: slack_attachment`, not a `system_` denylist, so a `custom_*` post is
  indexed and then never matches. That takes Recent Mentions with it. Installs on
  Postgres are unaffected, since `post_store.go` filters `system_%` only. This is
  the largest cost in the feature and it lands on exactly the posts it is for.
  **`EnableLocationMapInline` is how an install gets out of it**: with that off
  the stamp is never written, so those posts stay ordinary and searchable. See
  Mapping, "Turning maps off".
- **Auto-translation is off**, at six call sites that all test `post.type === ''`.
- **Link previews, image embeds and message attachments are dropped**, because
  `message_with_additional_content.tsx` computes `hasPlugin` from the raw
  `post.type` and skips `PostBodyAdditionalContent` when a plugin owns the body.
  "Message attachments" here is Mattermost's own term for slack-style
  `props.attachments`, which is what that component renders.

  **The FILE attachment list is not among them**, and reading this bullet as
  though it were is what put a "Download <name>" link on the Cursor on Target
  card. Files are drawn by `post_body` itself, outside
  `PostBodyAdditionalContent`, so a stamped post keeps them: observed on a
  running server, where the card's link sat directly above Mattermost's own
  attachment for the same file. Nothing else in this list changes.

`Props["type"]` was the alternative: `post_message_view.tsx` reads
`post.props?.type ?? post.type` while all three of the above key on `post.type`,
so dispatching through props alone keeps search, translation and embeds. It was
declined in favour of a post that declares itself in its own `Type`. Record that
as a decision rather than an oversight; the two differ by a few lines in
`stampStandalonePost`.

**The plugin owns the post body, so a render throw costs the message.**
Mattermost wraps a registered post-type component in `PluggableErrorBoundary`,
which replaces the whole body with "An error occurred in the … plugin." So the
link renders **outside** `components/ErrorBoundary.tsx` and only the map renders
inside it. That one nesting is what keeps a map bug from destroying what somebody
wrote.

**Props and message must agree, or nothing is drawn.** `Post.Type` survives an
edit unconditionally (`model.PostPatch` has no `Type` field) and `Props` may not,
since they are replaced by whatever the client sends and are absent from
`PreserveIdentityPropsFrom`'s rescue list. So a stamped post can arrive whose
message no longer names its payload. `DecoratorPostBody` therefore requires the
props and the message's sole decorator link to name the same `(f, v)` and falls
back to the message text otherwise. **The cost, accepted deliberately: markdown
in an edited message renders literally**, because a plugin has no access to
Mattermost's renderer. Adding `MessageWillBeUpdated` to clear a stale stamp was
declined, and `TestPluginHasNoMessageWillBeUpdatedHook` still stands.

Two more things a reader will meet before they report them as bugs. A reader
with post formatting turned off gets `<span>{post.message}</span>` and never
reaches the component at all. And `ShowMore` still wraps the body at 600px:
`FULL_HEIGHT_POST_TYPES` is a one-entry allowlist for `custom_spillage_report`,
not a `custom_` prefix rule.

**The webapp half mirrors the hover exactly.** `Decorator<T>` gains `postType`
and `Inline`, `index.tsx` registers one body component per declared type, and
nothing in the bootstrap changes when a decorator adds one. `Inline` differs from
`Hover` in one way worth knowing: returning `null` here means **no view at all**,
where `Tooltip.tsx` has already built its chrome and `null` there is an empty
bordered box.

The body renders the label as text and the token as a plain
`<a href={href}>{label}</a>`. Two of the three things a decorated link normally
does come back for nothing, because neither is tied to Mattermost's renderer: the
teal chip is the stylesheet's `a[href^=…]` rule, and the sidebar is the
document-level capture click handler. **The hover card does not survive, which is now verified rather than expected.**
`registerLinkTooltipComponent` is wired into Mattermost's own markdown link
rendering, and it only ever offers a link that renderer drew, so an anchor a
plugin draws inside its own post body is never offered and gets no card.

It costs little here, because the hover is the map and the map is already on
screen underneath. It cost more on the Cursor on Target card, whose linked times
carry a countdown that is deliberately NOT on screen, so that surface renders the
card itself through `decorators/HoverLink.tsx`. See `cot.md`.

The qualifying test the component runs on the message is
`soleDecoratorLink` in `decorators/inline.ts`, and its label is matched as a
**shape** (`^[A-Za-z][A-Za-z0-9]{1,15}[ \t]*:[ \t]*`) rather than as the USMTF
moniker list. That list is Go-only grammar and mirroring it would be a third
cross-language duplication that buys nothing: the server already decided this is
a coordinate and the link is the proof, and `Target: <coord>` is as much a
coordinate-only message as `LATM: <coord>`. The destination pattern is
`[^()\s]+`, which is correct **because** `buildURL` percent-encodes `(` and `)`
in the query; the one honest limit is a subpath install whose path contains a
paren, and such a link is already broken by CommonMark on every client.

**The map is mounted only while its post is near the screen.** Browsers cap live
WebGL contexts at roughly sixteen, shared with the panel and any hover card, and
a channel of coordinate-only posts is exactly the shape that stresses that.
`useNearViewport` in `map/near_viewport.ts` owns it:

- `NEAR_VIEWPORT_MARGIN` is `300px 0px`. A map plus its post is roughly 420px and
  a tall channel about 900px, so `(900 + 600) / 420` is about four live at once
  in the worst case. A wider margin pre-warms maps the reader may never reach.
- `RELEASE_AFTER_MS` is 2000, and the hysteresis is in **time** rather than in a
  second observer with a wider margin: a reader parked on the boundary would
  otherwise build and tear down a context on every small movement.
- It returns true forever where `IntersectionObserver` is missing, the same
  posture `LocationMap` takes for a missing `ResizeObserver`, so no host is left
  on a permanent placeholder because it lacked an optimisation.

While the map is down, the box **reserves `MAP_HEIGHT`** so the channel does not
jump as the reader scrolls past. That is why `MAP_HEIGHT` is exported;
passing `fill` instead would also suppress the "Open larger" caption.

The preference is read in the **outer** component so the inner one never mounts,
and `useConversion` lives in the **inner** one. Mattermost renders on the order
of thirty posts at a time, so outside the gate every qualifying post in the
rendered window would fetch whether or not the reader ever sees it.

`INLINE_MAX_WIDTH_PX` is 640, which at the tall end of `MAP_HEIGHT` is 16:9. It
matters because `zoomForSpan(lat, widthPx)` holds `TARGET_SPAN_METERS` across the
**width**: uncapped, a 2000px center channel would open roughly 1.6 zoom levels
deeper than the panel does for the same coordinate.

The map is the panel's, not the hover's: controls, gestures, zoom readout and
"Open larger". **The wheel therefore zooms it rather than scrolling the
channel**, and unlike the panel this is a column a reader scrolls through rather
than one box they chose to open. `cooperativeGestures` is the one-line
alternative if it bites. Compact display draws the link and no map, since compact
is a density choice. There is no `isRHS` suppression today, though this hook does
supply the signal, so it is one line if a map in a thread proves noisy.

A `rejected` conversion draws **nothing** rather than the hover's "Not a
coordinate": the post's own link is still on screen and the panel is one click
away, so a refusal banner under somebody's message is loud out of proportion to a
hand-edited link.

**Drawing inside a post body means inheriting the host's CSS**, which the panel
and the pages do not. The zoom readout is a `<p>` positioned with `left` and
`bottom` and no width, and an absolutely positioned box shrinks to fit only while
nothing gives it one: Mattermost's paragraph styling does, so the readout
stretched the whole width of the map inline and nowhere else. `width:
fit-content` is the fix and it belongs on the shared component rather than on the
inline surface, since it is what that element always meant. `stays its own width
under a host that stretches paragraphs` in `LocationMap.pw.tsx` injects the
hostile rule rather than waiting for it, because the component environment has no
Mattermost CSS and the test would otherwise pass either way. Anything else drawn
over the map is exposed the same way.

The reader hides it with `INLINE_ID` / `SectionInline`, its own id beside the
panel map's, and everything the panel map's id says applies here too.

## Framing a block of events

`fitBounds` frames every marker rather than opening on the first, and the
padding it is given is **asymmetric**, from `fitPadding` in `span.ts`.

It used to be one uniform 32px, and every corner of this map has chrome in it:
the Reset button top left, MapLibre's zoom buttons top right, the zoom readout
bottom left, and the scale bar bottom right. A number small enough not to waste
the canvas is smaller than the tallest of them, so a block of events opened with
markers underneath the controls.

**Corner chrome only has to be cleared on one axis, and which one is chosen by
cost.** The zoom buttons are 29 wide and 58 tall, so clearing them sideways
costs 12% of a 320px width where clearing them downwards costs 34% of a 200px
height. The Reset button and the readout are the other way round, wide and
short, so they are cleared vertically. That is why the four numbers are not
symmetrical and must not be tidied into one.

Each carries half a marker on top of the control's own reach, because a
crosshair is `MARKER_SIZE` across and drawn centered on its point. That is not a
rounding allowance: at 40px the top edge of a marker landed exactly on the
bottom edge of the Reset button.

`MAX_PADDING_SHARE` is a half rather than something more comfortable. The panel
map is 200px tall at its minimum and the chrome plus its marker allowance wants
88 of them, so a lower ceiling would scale the padding back down into the
controls it exists to clear. Past the share both edges of an axis scale
**together**, keeping the wide edge wide; clipping each to a ceiling separately
would flatten the asymmetry that is the whole point.

### Why the browser test mounts at 360px

`no marker opens underneath a control` projects each marker onto the canvas and
checks it against the rectangles the chrome occupies, rather than asserting a
zoom number: a zoom assertion passes for a padding that zoomed out and still put
a marker under the scale bar, which was the actual complaint.

It mounts inside a fixed 360px wrapper because **left to fill the component-test
page the canvas is about 1264px wide**, where the four corners are so far apart
that nothing can reach the chrome and the test passes against any padding at
all. It has to check the marker's extent rather than its center for the same
reason: a marker whose center clears the scale bar by 4px still has its bottom
third under it. Both of those were found by reverting the fix and watching the
test pass.

## The location pin is violet, and deliberately not red

These maps draw two different kinds of thing, and only one of them is making a
claim. A Cursor on Target marker's color says what a track IS: red is hostile
and suspect, blue is friend, green is neutral, amber is unknown and pending. A
location is a place somebody typed into a message. It has no affiliation, and a
pin wearing one of those hues asserts something about a coordinate that nothing
in the coordinate said, in the most loaded color available.

The pin was `#c92a2a` / `#ff6b6b`. It is now `#8d0da0` / `#e070e0`, an orchid.

**Purple through magenta is the only window there is.** Holding 45 degrees of
hue from all four affiliations leaves **252 to 321 degrees and nothing else**:
the gaps between red and amber, amber and green, and green and blue are each
too narrow for anything to stand in. That is worth stating because it looks like
a preference and is not one, and because it means a future request for a
different-looking pin has only this window to choose from.

The pin's contrast contract is on its **edge**, which is what
`TestMapPaletteCarriesItsContrast` gates, and that is what frees the fill to be
chosen for what it means rather than for its own contrast.

### Why not simply a different hue

The reason this pin was ever changed is that red claims something. The reason
this PARTICULAR hue was chosen is a second question, and it was answered by
measurement rather than by eye.

The pin's real neighbor is the **cell outline**, which is blue and is drawn on
the same map on every location panel. Under deuteranopia the whole 252-321
window collapses toward blue, so a sweep of that window (hue by saturation by
lightness) found the best achievable separation from the cell was about 12
against the violet's 10.4. **Hue is not the lever**; nothing in the legal window
meaningfully fixes that particular collision.

What the choice was made on instead is the **worst case against everything the
map draws** (labels, roads, railways, urban fill, admin lines, the cell, the
airfield, the land): 21.8 normal and 12.1 deuteranopic, where the violet was
17.1 and 10.4. It costs land contrast, 2.16 against the violet's 2.58, which the
edge-based contract absorbs.

**Achromatic was tried and rejected.** White scored best on every axis that was
being optimised, was uniquely stable across all three CVD types, and could not
be confused with an affiliation at all since it has no hue. It is disqualified
by a neighbor the earlier analysis had not looked at: it sits **5.5 from the
map's own label color**, so a white pin reads as a place name. It would also
have made the hue test misfire, since `hueOf('#ffffff')` is 0 and 0 is hostile
red's hue.

The light half of the pair is dormant while `ALWAYS_DARK` is true. It was still
chosen by the same measurement rather than by eye-matching the dark one, because
the two themes do not behave alike: at this hue the light value had to be pushed
to `#8d0da0` to beat the violet it replaced under deuteranopia at all.

### The test compares hue, not hex

`the location pin wears no affiliation color` in `style.spec.ts` reads the
built style's pin fill and holds it 45 degrees of hue away from every color
`affiliationColor` returns.

Equality would not have caught this. The old pin was `#c92a2a` and hostile is
`#c0392b`: different values, six degrees apart, both unmistakably red. An
equality check calls that a pass and the reader still sees a hostile marker over
every coordinate anybody types. What a reader perceives is the hue, so the hue
is what is asserted.

The affiliation colors are read out of `affiliationColor` rather than copied
into the test, so an affiliation added in a violet hue fails here rather than
shipping.

## The geometry layer

Cursor on Target events that describe a shape rather than a point draw it, and
the mechanism is the accuracy circle's, one level generalised.

`accuracyLayers` and `accuracyFeature` were already the pattern: a `geojson`
source added only when the surface asks for it, a fill and a line layer over it,
and a ring built from `DEGREE_METERS` so the shape keeps its meters at every
zoom. Geometry adds a second such source and reuses the ring.

**An ellipse is that ring with two radii and a rotation**, so the ring math is
extracted and both call it: `accuracyFeature` is now one line,
`ellipseFeature(lat, lon, meters, meters, 0)`. It keeps its signature and its
behavior, which `accuracy.spec.ts` measures against real meters and
`the accuracy circle is the ring with equal axes` pins directly.

**The bearing is clockwise from north, so the rotation is the transpose of the
usual one.** The first version applied a counter-clockwise matrix, which is
right at 0 and at 90 and mirrored everywhere else: a shape stated at 45 was
drawn at 315. Two things let that ship. Every test used an angle of zero, where
the rotation cancels; and the one non-zero fixture used -45, whose mirror is
plausible on sight. `the major axis lies along the bearing the event stated`
sweeps six bearings now, and the geodesy test moved off the equator, where the
cos(lat) division is a no-op and therefore proves nothing.

**Nothing author-derived reaches `fitBounds` outside the projection.** MapLibre
throws on a latitude past 90 rather than clamping, and the throw is swallowed by
its own render loop from the load handler, so the reader gets a blank map with
no note. The union is clamped to the Mercator limit before it is used.

**Superseded, kept for the argument.** What follows described `geometryColor`,
a scalar prop deleted when the paint became data-driven; the current shape is
"simplestyle, and the two gates it passes" in `geojson.md` and the module table
below. `geometryColor` repainted
`geometry-outline` and `geometry-fill` from `applyView`, always to an explicit
value rather than only when one is given: the panel reuses one map across
selections, so leaving the previous paint in place drew the next event's shape in
the last one's color. The layer defaults in `geometryLayers` are unchanged, which
is what keeps `style.spec.ts` and the archive guard out of it. The reasoning for
who may state a color, and why the marker may not take one, is Cursor on Target's
and lives in [`cot.md`](cot.md).

**Both overlay sources are built on every surface.** The geometry source used to
be built only when the first render asked for one, and the sidebar reuses a
single map across selections: opening a shapeless event and then a shape drew
nothing, forever, because `getSource('geometry')` stayed undefined and `setData`
was an optional chain that no-opped. An empty collection costs nothing.

**The frame is the union of the markers and the geometry.** `frameBounds`'s
`nothingToFrame` guard answers
only about markers and returns nothing below two of them, which is right for a
set of pins and wrong for one polygon: a shape whose extent is larger than its
`<point>` would open half off screen, or at a zoom picked for a point that
happens to sit inside it. Geometry contributes its bounds to the same box the
markers do.

### The antimeridian

A shape crossing 180 arrives as a jump from 179 to -179, and read literally that
is 358 degrees the wrong way round. Both consumers got it wrong in the same way:
the outline was drawn straight back across the whole map, and the camera framed
the planet instead of the two degrees the shape occupies. For a plugin whose
audience works either side of the date line, that is the common case rather than
the exotic one.

**The fix is to keep the longitudes continuous rather than to wrap them.**
`unwrapLongitudes` takes each step the short way, which can carry a longitude
past 180, and that is the point: MapLibre draws geometry beyond the meridian in
the adjacent world copy, so the shape stays one unbroken outline, and
`fitBounds` reads 179 to 181 as the two degrees it is.

Wrapping into range was the other candidate and is worse in both places. It
leaves the drawn line with a full-width stride at the crossing, and it makes the
bounding box the complement of the shape.

**So longitude is deliberately not clamped.** Latitude is, because `fitBounds`
throws past 90 rather than clamping for us, but clamping longitude to 180 would
collapse a crossing shape's box to nothing, which is the bug wearing a guard's
clothes.

The one thing continuity cannot express is a route that circles the globe: once
unwrapped its span exceeds 360, and a span wider than the world cannot be framed
more tightly than the world. `spansTheWorld` says so and the camera opens on the
whole of it, rather than on a box the projection has no way to honor.

**Vertices are not markers.** They are drawn as a line and a fill and never as
crosshairs. Feeding them through `markers` would have been less code and would
have put a reticle on every corner of a polygon, which says that each corner is a
position somebody reported.

Geometry draws on the card and in the panel both. A drawn shape whose shape is
not drawn is a card that has said nothing.

## The map page, addressed by a post

`/map` was addressed only by a coordinate: `?f=<format>&v=<canonical>`, with
every reading re-derived from the token, which is what "a link may never
disagree with itself" means on that route.

A block of Cursor on Target events and a GeoJSON document have no canonical
token. There is no coordinate that names an overlay, so "Open larger" was not
offered at all: `CotMap` passed a `pageHref` only for a post with exactly one
drawable event, and `GeoJsonMap` never passed one. To a reader that read as the
control being broken on precisely the posts where a bigger map is worth most.

`?post=<id>` is the second address. The invariant survives it: nothing derived
travels in the URL there either, and the whole overlay is re-read from stored
props at render.

### `drawsNothing` is the single authority on an empty overlay

Both canvases answer a payload that draws nothing with `null`: `unplaceable`,
which is Go's refusal to place a document whose axis order it could not confirm,
and an event or feature set that yields no marker and no shape. The overlay page
rendered that `null` inside a framed window with a populated label bar, so a
document that drew nothing appeared as an empty basemap captioned "1 event".

The test lives in one exported `drawsNothing` per format, which the canvas and
the page both call. It was tempting to restate the condition in the page, since
it already had the counts for the label. That is how the defect arose in the
first place: the page renders the canvas directly and inherits none of the
wrapper's gates, so a second copy of the test in the caller is exactly how the
two come apart again.

### One event keeps its coordinate address

"Open larger" chooses between the two addresses rather than always using the
post. A single drawable event still links to `?f=&v=`, because that page carries
the token and a way through to every reading of it, which the post form cannot
offer. A block has no SINGLE coordinate to be addressed by. It has one per
drawable event, which is what its markers are built from, and no one of them
names the block: linking the first event's page would frame that position and
say nothing about the rest, the same argument that keeps the accuracy ring off a
block map. A card with no post id, which is what a harness builds, has nothing
to address and so offers no link.

### The overlay page is a mode of `/map`, not a route of its own

`/map` is a route of its own rather than a mode of `/decorate`, and the reason
is above. The overlay goes the other way: it is the same window given to the
same picture over the same basemap, behind the same admin switch, and a reader
arrives at it from the same "Open larger". Only what is drawn differs, which is
why the mode is a shell attribute rather than a path.

`RenderOverlayPage` therefore takes `kind` and `blob` as opaque strings. By the
time it is called, `ServeHTTP` has already decided that the post is one this
plugin stamped, that this reader may see it, and that its card has not stood
down. The renderer's whole job is the shell around what it was handed, and it
must not re-derive any of those three: a second answer to "may this reader see
it" is how the two come to disagree.

### The shell carries the props blob, not markers and shapes

The obvious alternative was to distil the post into markers and shapes in Go and
put those in the shell. That would be a second answer to "what does this document
draw", and the two would part company the first time either map changed.

So the shell carries the format's own props blob verbatim, and the bundle hands
it to the same `fromProps` the card in the channel calls and renders the same
canvas. `CotMapCanvas` and `GeoJsonMapCanvas` are exported for that: the canvas
is the part that consults nothing, and the wrapper around it is where the
reader's sections and the admin's inline switch live. The page IS the map, so
reaching it is the decision and neither of those applies.

`data-overlay-kind` is the post type, which is already pinned to the webapp's
copy by `TestWebappCotPostTypeMatches` and `TestWebappGeoJSONPostTypeMatches`, so
this mode introduced no new vocabulary that could drift.

### The map's modules, and why the file was split

`LocationMap.tsx` reached 1511 lines by accretion: the pin, the accuracy ring,
the cell, the outline, the ringed shapes, the simplestyle gate, the framing math
and the accessible label all landed in one file. It is now the component and its
lifecycle, and four modules beside it:

| Module | Holds |
|---|---|
| `paint.ts` | `fillOf`, `numberWithin`, `styleOf`, `paintGeometry`, `MapShape`. The gate an author-supplied color, width or opacity passes before it is paint |
| `overlay.ts` | every `drawable*` builder, `markerFeatures`, `overlayDigest`, `MapMarker`, `MapEllipse`, `addMarkerImages`. What gets drawn |
| `bounds.ts` | `overlayBounds`, `frameBounds`, `unionOf`, `openingAnchor`. Where the camera goes |
| `label.ts` | `label`, `positionNote`, and the strings a map says when it cannot draw |
| `use_map_instance.ts` | `MapProps`, and the hook that owns the MapLibre instance, `applyView` and every effect |

**Not `style.ts`.** `style.spec.ts` already means the MapLibre *style
specification* built by `buildStyle` in `maplibre.ts`. The simplestyle gate is
`paint.ts`, after what it guards.

The split needed one type change rather than being a pure move: 21 helper
signatures were typed as `Props['markers']` and its siblings, so a helper in its
own module could not describe its own arguments without importing `Props` back
from the component. `MapMarker` is now a named export beside `MapShape` and
`MapEllipse`, and the dependency runs one way: `paint` → `overlay` → `bounds`,
with `use_map_instance` above those three and `LocationMap` above it.

Three `_ForTesting` wrappers went with it. `_frameBoundsForTesting`,
`_drawableCellForTesting` and `_positionNoteForTesting` existed only because the
functions were private to a component module; two forwarded unchanged and one
reordered arguments, which is why `geometry.spec.ts` had been calling
`frameBounds` with `geometries` last when the real signature takes it third.
The specs now call the real functions.

### `useMapInstance` owns the map; the component presents it

The lifecycle came out too: creation, the readiness deadline, the camera and
overlay writes, and teardown are `use_map_instance.ts`, which returns the
container to attach, the instance, `applyView`, and what to tell the reader.
`LocationMap.tsx` is 184 lines of presentation.

`MapProps` moved with it, because it is the map's contract rather than the
component's: the hook consumes almost all of it and the component forwards it
whole. `MAP_HEIGHT` is re-exported from `LocationMap` so the three surfaces that
size themselves against it keep their import path.

The seam was chosen, not taken. Passing the eleven refs the effects share with
`applyView` into a hook would have moved lines without separating anything;
moving `applyView` in as well is what let the hook own `map` and `ready`
outright, so the boundary is "give me what to draw" rather than "hold these for
me".

### One overlay path, and `extentOnly` derived

There were two ways to hand this map a shape: the singular `geometry` with its
`geometryColor`, and the plural `geometries`. Each had exactly one production
caller. `MapShape` is strictly the more general of the two, so the `outline`
variant folded into `geometries` and `outlineFeature` went with it: it was
`shapesFeature` for one single-ring shape.

`MapEllipse` is what remains of the singular slot, and it is not a simplification
that it stayed. An ellipse is placed by the map's own anchor rather than by its
own vertices, so a plural array of them would stack every one on a single point.

`extentOnly` is now derived: `lat === null && (markers or geometries)`. It was an
explicit prop to stop a null position meaning two things, and the case that
argument was protecting is a null with **nothing** to draw, which must still read
as unavailable. That clause is what keeps it true, and a mutation test proves it:
drop it and "an unknown position still reads as unavailable" fails.

One trap found on the way. `soleOutline` decides which event in a block draws,
and folding the color into the shape it inspects made that decision read
`event.detail.colorArgb`, which a caller counting outlines need not have.
`outlineOf` is colorless for that reason and `shapeFor` adds the color at the
render; the split is not stylistic.

### The write path, collapsed for real this time

Phase 3 unified the MARKER half of `applyView`'s fork and left the shape half,
while the commit said the path was one. The extent-only branch kept its own
`shapesFeature` call, which meant a second `styleOf` call site on the branch
GeoJSON always takes, and it silently dropped the ellipse.

`drawableOverlay` now takes a nullable `lat`/`lon` and returns the plural
collection when either is null, so both branches call it. The check that the
duplication is gone is that `shapesFeature` and `styleOf` are no longer imported
by `use_map_instance.ts` at all.

It was found by mutation rather than by reading: deleting `style: styleOf(shape)`
from the inline copy left all 806 tests green, because every style test mounted a
POSITIONED map and GeoJSON is never positioned. The test that closes it is
"a stated style survives the extent-only write path", the sibling of the
marker-size one.

### A verdict outlives its coordinate only where the coordinate changes

The failure-clearing effect keyed on `[lat, lon]`. An extent-only surface passes
a literal null for both, so on GeoJSON it ran once at mount and never again: one
transient basemap failure latched "The map could not be loaded" for every later
document, while the creation effect beside it retried on `overlayKey`. It keys on
`overlayKey` too now.

### Two channel permissions, not one

`overlayForPost` requires `read_channel` AND `read_channel_content`. It shipped
with only the first, which is "may see that this channel exists"; the second is
what Mattermost's own post reads gate on, and the two are separately grantable
through a custom scheme or channel moderation. This route returns the whole
stamped document, `src` included, so the weaker permission alone would have
served a post body out of a channel a reader may see but not read.

The permission check also runs before `DeleteAt` is read. It shipped the other
way round, which leaked nothing (both answers are the same 404) but contradicted
the ordering this file claims.

### Every refusal is one 404 and one code

Any reader with a session can put any id on this route. `TF-12008` is therefore
the single answer for a post that does not exist, a post in a channel this
reader may not read, a post this plugin never stamped, and a stamped post whose
card has stood down. Finer codes would answer, one id at a time, whether a given
post exists and what kind of thing it holds.

The permission check runs before anything is read out of the post, so behavior
cannot leak what is in a channel the reader cannot see. Only the plugin's own
props key is encoded into the page, never the whole props map: everything else
on a post belongs to Mattermost or to another plugin, and this page republishes
what it is given.

### The card's stand-downs are restated on the page

`CotPostBody` and `GeoJsonPostBody` refuse an edited post, and a file source
whose file is no longer attached, because `Post.Type` survives an edit and Props
may not. The page applies both. A page that drew what the card had already
refused to draw would be the one surface still claiming something no reader can
check.

### The overlay page carries no link out of itself

It shipped with a "Back to the post" permalink, on the argument that "Open
larger" opens a new tab so the browser's Back is not the way back. That was
removed.

It cost two API calls, `GetChannel` and `GetTeam`, on every page load, to build
a second route to the post the reader had arrived from and still has open in the
tab behind them. It could not be built at all for a direct or group message,
whose permalink needs a team name that does not exist, so the one page that
could not offer it was also the one where a reader is least likely to find the
post again by other means. A link that is absent exactly where it would help
most is not a way back.

Removing it took the two API calls with it, which is why `fakeAPI` stubs neither
any more: an unstubbed call panics there, so a future `GetChannel` on this path
cannot be added without the test suite saying so.

The coordinate map page keeps its own "All readings" link. That is not a way
back, it is the way to the other half of what the plugin knows about that token,
and there is no equivalent for an overlay.

A single Cursor on Target event still addresses the page by its coordinate
rather than by its post. That page carries the token and a way through to every
reading of it, which a post id cannot offer.

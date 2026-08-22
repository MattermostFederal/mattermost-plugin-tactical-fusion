# The OpenStreetMap detail tier

Builds one `.pmtiles` archive per region, which every map surface draws **above
the seam**. Below the seam the Natural Earth archive from `build/maptiles/` is
what draws; see [`docs/design/mapping.md`](../../docs/design/mapping.md) for the
seam itself.

## Running it

```
make osm-sources
make map-osm PROFILE=pilot
```

`PROFILE` is any token in a row's profiles column, so a single region
(`PROFILE=korea`), a theatre (`PROFILE=centcom`), the committed pilot
(`PROFILE=bundled`) or everything (`PROFILE=dod`).

The first fetches the OpenStreetMap extracts and verifies them against
`sources.lock`. The second builds a container pinning planetiler-openmaptiles
and tippecanoe, and runs `build.sh` inside it.

Like `make map-tiles`, this is **a prerequisite of nothing**: it is never
reached by `make test` and never runs in CI. Wiring it into CI would put a
multi-gigabyte download and hours of tiling in the release path, which is why
the pilot is committed and the rest are uploaded by hand.

## This is a sibling of build/maptiles, not an extension of it

Different source, different toolchain, different output, and different licence
obligations. There is no line of `build/maptiles/build.sh` a planetiler run
would reuse, and sharing a directory would put an ODbL pipeline and a
public-domain one behind one README and one `sources.lock` when the licence text
has to travel with exactly one of them.

## What it reads

The OpenStreetMap extracts named in `regions.txt`, fetched from Geofabrik and
pinned in `sources.lock` by SHA-256 **and by the date they were cut**. Both
halves matter: Geofabrik keeps daily files for roughly ninety days, so a digest
alone becomes unfetchable and the date is what says whether an internal mirror
holds the right one. `fetch-sources.sh` resolves `-latest` to its dated URL and
records that, so a rebuild asks for the same day's data rather than today's.

**Three auxiliary datasets are NOT pinned**, and this is the one place this
pipeline is weaker than `build/maptiles`. The OpenMapTiles profile also reads
Natural Earth, OSM water polygons and lake centrelines, about 1.2 GB in total,
which planetiler downloads itself into `build/maposm/cache/`. They are pinned by
nothing but planetiler's own URLs. Closing that is worth doing before this tier
ships beyond a pilot.

## What it writes

**One archive per region**, into `public/map/packages/`, named for the region in
`regions.txt`: `indopacom-hawaii.pmtiles`. Not one merged archive. The name
reaches a URL and a filesystem path on the server, so `build.sh` refuses one
that is not `<command>-<area>`, lower case, alphanumerics and single hyphens.

Separate files are what make coverage knowable. A PMTiles header carries one
rectangular bounds, so a merged archive spanning several regions declares nearly
the whole world and filters nothing; one file per area carries a tight bbox, so
MapLibre skips out-of-area requests and the plugin can list what it has. An
operator can also drop a single area into a directory, which is the whole point
of the naming convention.

Each archive is z10 through z14 and carries nine OpenMapTiles layers.

`MINZ` and `MAXZ` at the top of `build.sh` are the depth, written once.
`MINZ` must equal `SEAM_ZOOM` in
`webapp/src/decorators/location/map/span.ts`, and
`TestDetailPackagesStartAtTheSeam` holds all three together: the constant, the
generator and the archive's own header. A gap there draws neither tier and an
overlap draws both, kilometres apart.

| Layer | What it carries |
|---|---|
| `water`, `waterway`, `water_name` | coastline, lakes, rivers, and their names |
| `transportation` | motorway, trunk, primary, secondary, rail, and tertiary from z12 |
| `transportation_name` | road names and refs |
| `place` | cities, towns, villages, hamlets, suburbs |
| `boundary` | admin levels 2 and 4, carrying the `disputed` flag |
| `aeroway`, `aerodrome_label` | runways, taxiways, aerodromes and their names |

Deliberately absent: `building`, `housenumber`, `poi`, `landcover`, `landuse`,
`park`, `mountain_peak`. This basemap is for operational context rather than
consumer navigation, and those layers are most of the bytes.

## What in here is a decision, not a mechanic

**One region, one file.** Each is a planetiler run bounded by its bbox followed
by a `tile-join` pass that applies the class filter. `tile-join` is still doing
the work it did when everything was merged; it just writes one archive per
region now.

This replaced a merged archive, and the reason is worth keeping: a merged one
declared a bounds covering everything between its regions, so **stale, partial,
uncovered and truncated all rendered identically** as bare land and water. Per
region, each header carries a tight bbox and the plugin can list what it has.
The cost is a runtime package system, which is what the merged archive was
buying: discovery, a list on the wire, one style source per area, a route and an
upload path.

**The bbox is the TILE extent, not the extract extent.** Planetiler reads the
whole extract and emits tiles only inside the box, so geometry crossing the edge
is still rendered correctly into the edge tiles: the truncation problem comes
from cutting the *extract*, not from bounding the *output*.

**A region spanning several countries is therefore merged, never cut.** Where a
region names more than one extract, `osmium merge` concatenates them and
planetiler bounds the output as before. An earlier reading of this file called
for a buffered `osmium extract` ahead of planetiler; that is now deliberately
**not** done, and the reason matters because the earlier reading is the
intuitive one.

`osmium extract --strategy=smart` keeps a way when any of its nodes falls inside
the buffered box. A way that *spans* the box with no vertex inside it is dropped
whole. In dense terrain that is theoretical; in CENTCOM it is not, because
desert administrative boundaries are mapped as near-straight ways with vertices
hundreds of kilometres apart, and `filter.json` renders `admin_level <= 4`. A
0.25 degree buffer is 28 km and does not save a 400 km span, so cutting would
put a missing national border through the middle of `centcom-red-sea` and it
would look exactly like a correct build. The performance argument does not
rescue it either: osmium's node bitmap is sized by the highest OSM node ID
rather than by the extract, so `smart` costs 3 to 4 GB of RSS whatever the box.

If a build ever does need less input, cut by **whole extract files** in
`regions.txt` rather than by geometry. A missing country is obvious on screen; a
missing 400 km boundary way is not.

**The merge decision is confirmed on both regions it was made for**, using
planetiler's own `--output_layerstats` to count `boundary` features per tile.

| Border | Transect | z12 columns with data | Gaps |
|---|---|---|---|
| Saudi-UAE, Empty Quarter | 51.5E to 56.2E | 48 of 48 | none |
| **Egypt-Sudan, the 22N line** | 32.0E to 36.9E | 50 of 50 | **none** |

Both are the near-straight, distant-vertex geometry the argument above is about,
and the second is the example that argument names. A buffered `osmium extract`
is what would have deleted those ways, and the merged build keeps them
continuous end to end.

**Merge requires one snapshot.** `osmium merge` is a streaming k-way merge over
inputs sorted by type, id and version, and it deduplicates objects appearing in
more than one file, which is what handles Geofabrik's border-strip overlaps.
What it cannot do is reconcile files cut on different days: a border way edited
between two cuts appears twice with two versions and both survive. `build.sh`
therefore refuses a region whose extracts do not share a date suffix, and the
fix is to re-pin the whole set together with
`UPDATE_LOCK=1 make osm-sources PROFILE=<name>`.

**Geofabrik publishes regions through the day, so a set can straddle a
rollover.** That is not hypothetical: re-pinning CENTCOM mid-afternoon moved
Syria to the new cut while Egypt, Jordan, Lebanon and Israel/Palestine were still
on the old one, leaving two regions unbuildable until upstream caught up. The
guard therefore takes `ALLOW_MIXED_DATES=1`, which downgrades it to a warning
naming the dates.

**`PIN_DATE` is the better answer, and is how the roster was made uniform.**
Rather than wait for `-latest` to converge, pin the set to a cut where every
member exists: Geofabrik keeps dailies for about ninety days, so an older date is
usually available for all of them when the newest is available for only some.

```
PIN_DATE=260820 UPDATE_LOCK=1 make osm-sources PROFILE=centcom
```

Every region on this roster is pinned to 260820 that way. `ALLOW_MIXED_DATES=1`
remains for the case where no single cut covers the set.

**The lock is authoritative, not the directory.** `build.sh` resolves each
extract to the exact filename `sources.lock` pins rather than globbing
`<leaf>-*.osm.pbf` and taking the first. It globbed until a rollover left two
dates on disk for the same country, at which point the glob picked the older file
while the lock named the newer one, and the build would have silently disagreed
with its own pins. `fetch-sources.sh` now also deletes superseded files after
re-pinning, so the directory cannot drift from the lock in the first place.

`osmium cat` would be the obvious cheaper alternative and is wrong: it would
emit nodes(a) ways(a) nodes(b) ways(b), and planetiler needs every node before
the ways referencing it.

**Road classes are filtered at `tile-join`, not by planetiler.**
`planetiler-openmaptiles` offers `--only_layers` and `--exclude_layers`, which is
whole layers and no finer, so restricting `transportation` to the classes this
plugin draws needs either a forked profile or a filter pass. `tile-join` takes
`-j`, a per-layer filter in the Mapbox GL Style filter syntax, and it is already
reading every tile to do the merge, so the filter is free there and costs no
fork. `filter.json` is that filter, and its `$zoom` clause is what holds tertiary
roads back to z12.

Measured on the pilot: the filter takes the archive from 11,329,212 bytes to
6,808,873, which is 60% of it.

**Military classification is not shipped.** OpenStreetMap tags some aerodromes
`military=airfield` and carries `landuse=military` polygons. Neither is built
into this archive, for the same reason `build/maptiles` drops Natural Earth's
equivalent field: it would be a viewpoint, and an unreliable one. Drawing
aerodromes is not the same as classifying them.

## The map schema, and why areas outlive a plugin upgrade

Map areas are large and awkward to move, so an operator upgrading the plugin
should not have to move them. They do not have to: nothing version gates a
package. The `?v=` in its URL is the plugin version and is cache busting only,
the server ignores the query string, and an area built by any version is served
by any other. Depth is a floor rather than a match, so an older z10-12 area
still draws against a build that now makes z10-14; it simply runs out of detail
sooner.

What is NOT portable is a change to the shape of the data itself. Two would do
it: the seam moving off z10, and a layer joining or leaving
`DETAIL_SOURCE_LAYERS`. The first is caught already, since the header check
requires `minzoom == 10` exactly, but it reports a well formed archive as one
that "is not the archive it claims to be", which sends an operator looking for a
corrupt file. The second is not caught at all and simply draws nothing.

So `build.sh` stamps the schema an archive was built for into its PMTiles
metadata name, `tactical-fusion-map/<n>`, and the server reads it back:

```
indopacom-guam  z10-14  schema 1  -Xmx4g  1783446 -> 976239 bytes
```

**Bump `MAP_SCHEMA` when an older archive becomes wrong rather than merely
shallower**, which is those two changes and not much else. A class filter tweak
is cosmetic drift and does not qualify. `mapSchemaVersion` in `server/packages.go`
is the other half of the pair and `TestMapSchemaMatchesTheGenerator` holds them
together.

**An archive with no stamp is schema 1.** Every area published before this
existed is unstamped, and requiring the stamp would have rejected all of them,
which is the failure this exists to prevent. The stamp only starts mattering at
the first bump.

A mismatch is `TF-18008` and says which side is behind: an older archive is
re-downloaded for that area, a newer one means the plugin is behind the archive.
That is deliberately distinct from `TF-18002`, which means the file is broken.

## The regions, and where each one lands

`regions.txt` is the roster. Thirteen areas of responsibility, one archive each,
all z10-14.

**Two regions are committed: `indopacom-hawaii` and `indopacom-guam`.** They
carry the reserved `bundled` profile, which is the only thing that routes an
archive into `public/map/packages/`. Every other row lands in
`build/maposm/out/`, which is gitignored, and ships as a **release asset** an
operator downloads and drops into `LocationMapPackagesDir`.

That split is a budget, not a preference. `world.pmtiles` plus the committed
packages are held under 64 MiB by `TestArchivesFitTheBundleTogether`:

| | bytes |
|---|---:|
| `world.pmtiles` | 43,074,410 |
| `indopacom-hawaii.pmtiles` | 6,808,873 |
| `indopacom-guam.pmtiles` | 976,218 |
| **used** | **50,859,501** |
| **free** | **16,249,363** |

Guam earns its place at under a megabyte: it covers Andersen and the CNMI divert
fields, and it makes the shipped plugin exercise the **multi-package** path
rather than the single-package one, so an install with no downloads still proves
that per-package sources and layer sets compose. The eleven remaining regions are
2.4 GB together and could never fit.

`TestOnlyBundledRegionsAreCommitted` is what enforces the split, because the byte
budget alone would let one accidental region through and a guard that catches
only the second mistake is not a guard.

Every region is built and measured from **one Geofabrik snapshot, 260820**, the
pilot's 260818 aside. The eleven release assets total **2,442,966,520 bytes**,
which is 2.44 GB; Hawaii and Guam ship in the plugin instead.

| Region | Extracts | Input | Built |
|---|---|---:|---:|
| `indopacom-hawaii` | `north-america/us/hawaii` | 27 MB | **6,808,873** (bundled) |
| `indopacom-korea` | `asia/south-korea`, `asia/north-korea` | 377 MB | **182,564,763** |
| `indopacom-taiwan` | `asia/taiwan` | 326 MB | **57,553,192** |
| `indopacom-japan` | `asia/japan` | 2,500 MB | **477,543,062** |
| `indopacom-philippines` | `asia/philippines` | 604 MB | **160,885,873** |
| `indopacom-scs` | `asia/china`, `asia/vietnam`, `asia/malaysia-singapore-brunei`, `asia/philippines`, `asia/indonesia` | 4,491 MB | **268,095,846** |
| `indopacom-guam` | `australia-oceania/american-oceania` | 5 MB | **976,218** (bundled) |
| `centcom-persian-gulf` | `asia/gcc-states`, `asia/iran`, `asia/iraq` | 571 MB | **125,582,695** |
| `centcom-levant` | `asia/israel-and-palestine`, `asia/lebanon`, `asia/syria`, `asia/jordan` | 284 MB | **94,109,251** |
| `centcom-red-sea` | `africa/egypt`, `africa/sudan`, `africa/eritrea`, `asia/yemen`, `asia/gcc-states`, `asia/jordan`, `asia/israel-and-palestine` | 857 MB | **168,252,760** |
| `eucom-ukraine` | `europe/ukraine` | 874 MB | **351,774,983** |
| `eucom-baltics` | `europe/estonia`, `europe/latvia`, `europe/lithuania`, `europe/poland`, `europe/belarus`, `russia/kaliningrad` | 2,947 MB | **336,976,212** |
| `africom-horn` | `africa/somalia`, `africa/ethiopia`, `africa/eritrea`, `africa/djibouti`, `africa/kenya`, `asia/yemen` | 734 MB | **219,627,883** |

**`eucom-baltics` carries the Suwalki approaches, not just the three states.**
The first build was Estonia, Latvia and Lithuania alone, and the z12 layerstats
showed road features stopping dead at the Lithuanian border: 31 to 151 per tile
band on the Lithuanian side of 22.9E and **exactly zero** on the Polish side
below 54.3N, with 8 features in the whole of Kaliningrad. The Suwalki corridor is
most of why the Baltics are on this roster, and only its Lithuanian half was
drawn.

`europe/poland`, `europe/belarus` and `russia/kaliningrad` were therefore added,
and the box widened from `20.8,53.8` to `19.5,53.0` to reach Kaliningrad city at
20.5E and Bialystok at 53.1N, which the old west and south edges cut off. Adding
the extracts without moving the box would have changed almost nothing on screen.

| Area | Before | After |
|---|---:|---:|
| NE Poland and Bialystok | 0 | 23,492 |
| Kaliningrad oblast | 8 | 9,512 |
| Western Belarus | 138 | 24,776 |

The cost is the largest input on the roster after `indopacom-scs`: 2,947 MB,
almost all of it Poland, because Poland extends far west of the box and merge
never cuts. The archive grew 1.6x to 336,976,212 bytes, still 190 MiB clear of
the upload ceiling.

**No two boxes overlap, and that took a deliberate split.** The client adds one
vector source and one layer set per package, so where two packages hold data for
the same tile both draw it. Two pairs originally overlapped badly and were
resolved by moving the seams rather than by dropping coverage:

| Pair | Was | Now |
|---|---|---|
| `centcom-red-sea` and `africom-horn` | overlapped `32.9,11.0,45.0,18.1`, all of Eritrea, Djibouti and northern Ethiopia | split at **15.0N**. Horn takes Bab-el-Mandeb, Djibouti and southern Eritrea; red-sea takes the corridor north of it |
| `centcom-red-sea` and `centcom-levant` | overlapped `33.9,29.0,42.5,30.2` | split at **30.2N**. Red-sea takes the Gulf of Aqaba, so `asia/jordan` and `asia/israel-and-palestine` joined its extracts for Aqaba and Eilat |

Moving a seam moves extracts with it. Red-sea dropped `africa/djibouti`, which is
now wholly inside horn, and horn gained `asia/yemen` for the Aden and Mukalla
coast that fell to it along with Bab-el-Mandeb.

**A shared edge still costs one tile row**, and that is unavoidable: planetiler
emits every tile the box intersects, so two boxes meeting at a latitude both
carry the tile row straddling it. Measured at z12, the levant/red-sea seam is 63
to 67 tiles and about 550 road features on each side, against **11,874 features**
in the overlap it replaced. A twentyfold reduction, not elimination.

**`indopacom-korea` and `indopacom-japan` overlap as boxes and do not
double-draw.** Japan's box contains Korea's entirely, but the extract lists are
territorially disjoint, so Japan's archive holds zero features over Seoul, Daegu
or Pyongyang and Korea's holds 11 over Tsushima. Box overlap alone is not the
defect; overlapping **extracts** are. The only cost is that Japan's header claims
bounds it has no tiles for, which the pmtiles client resolves from its own
directory without issuing requests.

**Northern Kenya was covered by nothing** and now is: `africa/kenya` joined the
horn merge, taking that area from 47 road features at z12 to 12,557, and Nairobi
from nothing to 4,833.

`indopacom-scs` is deliberately the **last row in the file** rather than sitting
with the other INDOPACOM areas. `build.sh` walks `regions.txt` in order, and SCS
is by far the heaviest region: five extracts totalling about 4.5 GB, against 2.5
GB for the next largest. Anywhere earlier and `make map-osm PROFILE=indopacom`
or `PROFILE=dod` spends hours on it before producing a single cheap archive.

Guam has no Geofabrik extract of its own. It lives inside
`australia-oceania/american-oceania`, which carries the Northern Marianas too,
so Saipan, Tinian and Rota come along for 5 MB. Asking for
`australia-oceania/guam` gets a **302 to the Geofabrik home page and an HTML
body under HTTP 200**, which is what `fetch-sources.sh`'s `OSMHeader` check
exists to catch.

`indopacom-scs` stops at 116.8E, where `indopacom-philippines` begins. The boxes
are deliberately adjacent rather than overlapping: two packages covering the same
ground ship those tiles twice and the client draws both, one over the other.
`centcom-red-sea` and `centcom-persian-gulf` are kept apart for the same reason,
though both draw on `asia/gcc-states`.

## The density curve, which corrects the extrapolation

**All thirteen regions are measured.** The spread between them is the finding.

| Region | Bytes | Land | Bytes per km² | vs pilot |
|---|---:|---:|---:|---:|
| `africom-horn` | 219,627,883 | ~2,470,000 km² | ~89 | 0.4x |
| `indopacom-hawaii` | 6,808,873 | 28,311 km² | 241 | 1.0x |
| `centcom-red-sea` | 168,252,760 | ~800,000 km² | ~210 | 0.9x |
| `centcom-persian-gulf` | 125,582,695 | ~500,000 km² | ~251 | 1.0x |
| `centcom-levant` | 94,109,251 | ~300,000 km² | ~314 | 1.3x |
| `indopacom-scs` | 268,095,846 | ~570,000 km² | ~470 | 2.0x |
| `indopacom-philippines` | 160,885,873 | 300,000 km² | 536 | 2.2x |
| `eucom-ukraine` | 351,774,983 | 603,548 km² | 583 | 2.4x |
| `indopacom-korea` | 182,564,763 | 220,750 km² | 827 | 3.4x |
| `indopacom-guam` | 976,218 | 1,008 km² | 968 | 4.0x |
| `eucom-baltics` | 336,976,212 | ~330,000 km² | ~1,021 | 4.2x |
| `indopacom-japan` | 477,543,062 | 377,975 km² | 1,263 | 5.2x |
| `indopacom-taiwan` | 57,553,192 | 36,197 km² | **1,590** | **6.6x** |

Hawaii was the only density figure this pipeline had, and it is the **least**
representative of the roster rather than a middle case. An island chain with one
metro area is not what an area of responsibility looks like.

**The `~1 GB` extrapolation for the thirteen areas is therefore a floor.** At
Korea's density the same 4.5M km² would be 3.7 GB; at Taiwan's, far more. The
true total sits below both, since the Horn of Africa and the four mostly water
regions are sparser than Hawaii while Japan, Korea and Taiwan are the outliers
pulling upward. Nothing here should be planned against 1 GB.

The class filter keeps 55 to 78% of the raw archive across all thirteen, so the
variation is real map density rather than anything the filter does. The top of
that range is `centcom-red-sea`, and it says something: in a mostly water, mostly
desert region the bytes are coastline and water, which the class filter does not
touch, rather than the road classes it prunes.

**Every archive so far carries all nine layers**, including Guam's at 976,208
bytes. The concern that a small or mostly water region would omit one entirely,
which `TestArchiveCarriesEveryLayerTheStyleDraws` requires in both directions and
which no test can catch for a release asset, has not materialised at a thousandth
of Japan's size.

### `indopacom-japan` is the largest region and it fits

Japan is **477,543,062 bytes, or 455.4 MiB**, which is **56.6 MiB under the 512
MiB upload ceiling** in `server/packages.go`. It installs through the System
Console uploader like any other area and needs no special handling. It is the largest region on the roster by input and by output; `eucom-ukraine`
is second at 351,774,983 bytes and still 176 MiB clear, so nothing else here
threatens that ceiling.

Had it landed above, the file would have been copied into
`LocationMapPackagesDir` by hand, which is already the documented path for a
large area, or built one level shallower. Neither is needed.

## What a full build costs

Twenty-nine unique extracts, about **10 GB** in `source/`. Four of them serve
two regions each and `fetch-sources.sh` deduplicates them, so the per-region
column above sums higher than what is downloaded.

| | |
|---|---:|
| `source/` | ~10 GB |
| `work/*.osm.pbf`, the merged regions | ~6 GB |
| planetiler `--tmpdir`, peak | ~5 GB |
| `work/*.raw.pmtiles` and `out/` | ~2 GB |
| `cache/`, planetiler's auxiliary datasets | 1.4 GB |

**Budget 50 GB free.** The merged PBFs are the cheap thing to drop:
`rm build/maposm/work/*.osm.pbf` costs only re-merge time on the next run.

**Heap comes from the input size**: under 256 MB of input gets 4g, under 1 GB
gets 6g, and the rest get 8g. These are preferences rather than requirements, and
they are lower than they first were because the first Japan build disproved the
original numbers.

**Measured**: Japan's 2.5 GB extract, 317M nodes and 45.5M ways, ran to
completion in 2m19s at `-Xmx6g` with **3 seconds of GC**. Planetiler keeps node
locations off-heap in an mmap, so heap need does not scale with input the way the
first ladder assumed. It asked for 12g on this input and refused a build that
works comfortably at half that.

**A derived heap is therefore clamped to what the container has, not enforced.**
`build.sh` reads the cgroup limit, and where the preference does not fit it drops
to what does and says so:

```
  indopacom-japan prefers -Xmx8g; this container has 7835 MiB, using -Xmx6g
```

It refuses only when the container cannot supply the 4g floor, or when an
explicit `JAVA_HEAP` cannot be met. That keeps the check's real value, which is
failing in seconds rather than being OOM-killed an hour into a build with a
message that says nothing about memory, without turning a conservative guess into
a wall.

## Downloads are atomic

`fetch-sources.sh` writes to `<name>.osm.pbf.part` and renames only on success,
and sweeps stale `.part` files on startup. Before that, an interrupted download
left a truncated file that the next run treated as complete: the `OSMHeader`
check passes on a partial PBF, so its digest would have been pinned as the
region's source and every later verification would have agreed with it. Poland is
2 GB, and that is exactly where a connection drops.

## Testing the whole roster locally

`make deploy` installs the plugin into the docker-compose stack and then runs
`make docker-packages`, which copies everything in `build/maposm/out/` into
`docker/mattermost/data/map-packages/` and points `LocationMapPackagesDir` at
`/mattermost/data/map-packages`. That directory is bind-mounted, so the copy is a
host-side file copy and the server sees it immediately.

The bundled regions are deliberately **not** copied. A drop-in file wins a name
collision with a bundled one, so copying them would shadow the bundled path and
stop testing it. A development install therefore exercises both: Hawaii and Guam
from inside the plugin, everything else from the directory an operator would use.

Only files whose source is newer are copied, so the first deploy moves 2.4 GB and
every later one prints `packages already current`. The setting is written with
`mmctl --local config patch`, because `mmctl config set` cannot address a plugin
id containing dots.

Verify the server is serving them by asking for a header:

```
curl -s -D - -o /dev/null -r 0-126 \
  http://localhost:8065/plugins/<plugin-id>/packages/indopacom-japan.pmtiles
```

A `206` with a `Content-Range` naming the full archive size is the route, the
discovery and the byte-range reader all working.

## Publishing the release-asset regions

The archives in `build/maposm/out/` are attached to an existing plugin release
by hand:

```
make map-publish TAG=v0.3.0
```

That writes `PACKAGES.sha256` beside them and runs `gh release upload`. It is
manual because the release workflow can neither download 10 GB of extracts nor
spend the hours of tiling, and `release.yml`'s `files:` list must not be pointed
at these.

## Two things you will see on a clean run

`tile-join` prints `Warning: attribute not found for comparison:
["<=","admin_level",4]` once. It is benign: some `boundary` features carry no
`admin_level`, and dropping them is what the filter is for. Noted here so nobody
chases it.

The build is **byte-reproducible** from the pinned sources. Rebuilding the pilot
reproduces the committed archive exactly:

```
d29847a48ce6e1cfa7ed3affbd0751e29ecd6810b745eb711ac1abc7a5f1f518  public/map/packages/indopacom-hawaii.pmtiles
```

An earlier hash here, `960e3a27...` over 6,808,864 bytes, belonged to the merged
`detail.pmtiles` that per-region packages replaced. Planetiler's output is
identical either way at 11,329,212 bytes; the nine byte difference is metadata
`tile-join` writes for the archive it is producing, so the merged figure cannot
be reproduced by this pipeline and is not a regression.

Planetiler's scratch (`--tmpdir`, `--tile_weights`) is pointed into `work/` and
`cache/`, both gitignored. Left at its defaults it writes `data/` into the
repository root, which is what it did on the first run here.

## The pilot measurement

Hawaii, main islands, z10-14, from `hawaii-260818.osm.pbf`.

| Stage | Bytes |
|---|---:|
| planetiler output | 11,329,212 |
| after the `tile-join` class filter | **6,808,873** |

Per zoom, from planetiler's own `--output_layerstats` (uncompressed layer bytes,
before the class filter):

| Zoom | Bytes | Share | Tiles |
|---|---:|---:|---:|
| z10 | 309,281 | 1.7% | 234 |
| z11 | 616,843 | 3.3% | 840 |
| z12 | 1,608,614 | 8.7% | 3,234 |
| z13 | 4,561,215 | 24.6% | 12,810 |
| z14 | 11,413,882 | 61.7% | 50,447 |

**The zoom curve is the number that decides everything downstream.** z10-12 is
13.7% of a z10-14 build and z10-11 is 5.0%, so the depth is worth roughly seven
times what the region count is. An earlier estimate put z10-12 at about 6.7%,
which was pessimistic by two: tile counts quadruple per level but per-tile bytes
shrink, so the two effects partly cancel.

Per layer, before the class filter:

| Layer | Bytes | Features |
|---|---:|---:|
| `transportation` | 5,435,445 | 63,853 |
| `water` | 4,573,613 | 72,518 |
| `transportation_name` | 2,987,088 | 30,039 |
| `waterway` | 2,209,983 | 21,502 |
| `place` | 1,610,115 | 18,597 |
| `water_name` | 715,593 | 8,491 |
| `boundary` | 701,333 | 4,715 |
| `aeroway` | 195,800 | 4,104 |
| `aerodrome_label` | 80,865 | 440 |

Extrapolating from Hawaii's 28,311 km² of land gives about 240 bytes per km²,
which over the roughly 4.5M km² of the thirteen requested areas of
responsibility is **on the order of 1 GB at z10-14**. Hawaii is more completely
mapped than the Horn of Africa and less than Japan, so treat that as a middle
figure with a wide spread, and **measure a dense region before spending it**:
the plan's Phase 2 names Taiwan for exactly this.

Against roughly 30 MB of plugin-bundle headroom, that puts z10-14 for the full
set out by a factor of thirty, z10-12 out by three to eight, and z10-11 at the
boundary. Nothing about that is fixable by tuning this generator.

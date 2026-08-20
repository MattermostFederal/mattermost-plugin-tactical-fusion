# Mapping

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

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
the map that is neither basemap grey nor pin red, and they are held to the same
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
representations of one basemap rendering two different worlds, one labelled and
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

### The page content policy

`Page.Capability` is how much of `default-src 'none'` a page gives back.
`PageStatic`, the zero value, gives back nothing and is what every page should
want. `PageMapping` is the only other one, and it exists for one thing: the map
pages run MapLibre, which is a real script file with a worker beside it, fetches
the basemap, and draws through a canvas.

```
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

**This is a real reduction in a defence, on a route that echoes author text to
anybody who can write its query string, and it was taken deliberately.** Under `PageStatic` an escaping mistake in
the author's own text was inert markup: it could not execute, because the only
script allowed was the one named by digest, and it could not exfiltrate, because
`img-src` and `connect-src` were **absent** and their absence is what blocks
`<image>`, `<feImage>` and CSS `url()`. Under `PageMapping` an injected
`<script src>` to anything else on this origin runs, and an injected image URL
is a channel. **Escaping is now the only defence**, so a map page may never
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
`34.0561N,118.2W` is not squared off to its coarser half. For MGRS and UTM the
rectangle **bounds** the grid square rather than being it: an MGRS square is
axis-aligned in UTM space and rotated by the grid convergence, which is
invisible at this scale.

There is no minimum size and no threshold below which the cell is dropped. These
surfaces zoom, so there is no one scale to test against: a metre-wide cell is
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
what makes that a defence rather than an accident. It was described here as an
`aria-label` held by a Go test long after it was neither, which is the state a
guard rots into when the only thing asserting it is a sentence.

### Two zoom numbers, and why they are those numbers

**`TARGET_SPAN_METERS` decides what a reader sees first**, and it is 400 km. It
was 2,400 km, which opened the panel at about z3.4 at the equator: two and a half
zoom levels below the map's own ceiling, with a one metre grid reference framed
exactly like a whole-degree one. It is pinned by no test in either language, and
it is the single constant with the largest effect on whether the map feels like
it is showing you anything.

**`MAX_ZOOM` is 9, and it was 8, and the arithmetic did not change.** 1:10m data
carries roughly 5 km of positional accuracy, and a 512 px tile puts 78271.5/2^z
metres in a pixel, so that error is about 16 px at z8, 33 px at z9 and 65 px at
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
colour with a straight line for a coastline.

Past `DATA_MAX_ZOOM` MapLibre overzooms. For vector tiles that magnifies
**without blurring**, so lines stay crisp and only their generalisation is
wrong: the failure the single ceiling existed to prevent is now invisible rather
than impossible.

**Nothing on the map states that in words.** There was a notice reading "Basemap
detail ends here", drawn past `DATA_MAX_ZOOM` and repeated in the accessible
label, and it was removed. Record this as the state of things rather than as an
oversight: a reader past the ceiling is looking at a coastline that may be five
kilometres from where it is, drawn at street scale, and the only thing saying so
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
Version, so `public/map/fonts/LICENSE.txt` ships beside them because the licence
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
basemap is a coastline generalised by about five kilometres drawn at street
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

When a posted message is **only** a coordinate, the server stamps the post with a
custom type and props, and the webapp renders that post's body itself: the link
exactly as before, plus the map under it. Everything else is unchanged, which is
what makes this additive rather than a second rendering path.

**The verdict is the tagger's, not a regex over the decorated text.**
`DecorateWithResult` returns a `Result` beside the string, and `SoleToken` is
`len(accepted) == 1` **and** that candidate's `match` covering the message apart
from surrounding whitespace. Both halves are needed: `see MGRS: 18SUJ2347806483`
also accepts exactly one candidate. It is `match` rather than `replace`, because
a pattern may match more than it rewrites: the airfield pattern sets
`ReplaceGroup` so the `//` a USMTF set line ends with stays in the message, and
`replace` therefore stops short of it. Measuring `replace` would refuse
`DEPLOC:KIND//`, which is the ordinary military spelling and the case this
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
document-level capture click handler. **The hover card is expected not to
survive, and that is unverified.** `registerLinkTooltipComponent` is wired into
Mattermost's own markdown link rendering and this anchor does not go through it.
It costs little worth having, since the hover is the map and the map is already
on screen underneath, but confirm it on a running server and record the answer
here.

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
**width**: uncapped, a 2000px centre channel would open roughly 1.6 zoom levels
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


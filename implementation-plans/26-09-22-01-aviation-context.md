# Aviation context: airfields, reports and frequencies

## Context

Tactical Fusion already recognizes an ICAO airfield code behind a USMTF label
(`DEPLOC:PHNL`) and renders the field's name, place, type, elevation and IATA
code, with a map in the sidebar. Everything else an air operations channel
types about airfields is still plain text: an IATA code, a runway, a tower
frequency, a pasted METAR or TAF, a NOTAM. Readers decode those by hand or
leave the channel to look them up, and on an air-gapped network there is
nowhere to go.

This plan extends the airfield decorator with the rest of the airfield (IATA
codes, a military designator, runways, frequencies, runway lines on the map),
adds a route map under a message that names several airfields, adds a new
decorator that decodes pasted METAR, SPECI, TAF and NOTAM reports into plain
language, and adds a small label-only decorator for radio frequencies. Every
dataset ships inside the plugin, and nothing fetches from the network.

**Scope decisions already taken with the user:**

- Aircraft and platform decorators (tail numbers, call signs, aircraft type
  data) are out. No aircraft dataset ships.
- "Current weather" is decoding what somebody pasted. No live fetch, no
  weather source URL, no outbound HTTP. That is a possible later plan.
- "Plot on the tactical map" is per-link maps plus a multi-airfield post map.
  No thread or channel aggregate map.

## Current state

- `server/decorators/airport/` is the ICAO decorator: label-only grammar
  (`grammar.go:12-75`), validation by lookup (`airport.go:131`), one query
  parameter `v` (`airport.go:19`), a `PageStatic` page (`page.go`), a markdown
  table under a code-only message (`markdown.go`), and `/api/v1/airport`
  (`server/api.go:310`). The embedded database is 19,012 rows of the DataHub
  `airport-codes` file (`data/README.md`), which carries no runways and no
  frequencies.
- The panel map is the location decorator's `LocationMap`
  (`webapp/src/decorators/airport/AirportPanel.tsx:182-212`), which already
  draws many markers and many shapes: `markers?: MapMarker[]` and
  `geometries?: MapShape[]` in `map/use_map_instance.ts:149,185`, framed by
  `frameBounds` (`map/bounds.ts:102`), as the GeoJSON and CoT maps do
  (`webapp/src/geojson/GeoJsonMap.tsx:262-296`, `webapp/src/cot/CotMap.tsx:269-282`).
- The map page has two addresses: `/map?f=&v=` for a coordinate and
  `/map?post=<id>` for a stamped overlay (`server/http.go:105-114`,
  `server/mappost.go`, `location.RenderOverlayPage` at `mappage.go:53`),
  and the webapp picks the drawing by post type in
  `webapp/src/page/OverlayPageView.tsx:65`.
- Two formats stamp a post with a custom type and decoded props: CoT
  (`server/hooks_cot.go`) then GeoJSON (`server/hooks_geojson.go`), sharing
  `stripStampedTypes` and `commitStamped` in `server/hooks_stamp.go`. The
  location decorator stamps a sole-token post through `PostRenderer`
  (`server/hooks.go:186`), with the links kept.
- `Result` reports one token only (`server/decorators/tagger.go:173-187`,
  `soleTokenResult` at `:235`).
- The DTG decorator resolves a `DDHHMM` group against a reference time and
  carries the instant as `t`, which the page requires to round-trip
  (`server/decorators/dtg/dtg.go:182-201`, `validateParams` at `:284`).
- Nothing in the tree mentions METAR, TAF or NOTAM. `IATA` is a displayed
  field and `troubleshooting.html:255` says the plugin does not read it.
  `implementation-plans/26-08-18-01-airport-icao-decorator.md:120-121` put
  IATA, runways, frequencies and NOTAMs out of scope.

## Phase strategy

| Phase | Delivers | Framework change |
|---|---|---|
| 1. Airfield enrichment | Three embedded CSVs from OurAirports, `IATA:` grammar, military designator, runways and frequencies on every airfield surface, runway lines on the panel map, `/map?airport=` | None |
| 2. Route map | A message that is nothing but airfield references is stamped and draws every airfield on one map with legs | `Result` reports every token; a second optional stamping interface |
| 3. Aviation reports | METAR, SPECI, TAF, NOTAM decoded: a decorator link for a single-line report, a stamped card for a multi-line or fenced one | A third stamper beside CoT and GeoJSON |
| 4. Frequencies | `FREQ:` decorator with band and use | None |

Phases ship in order, each a separate PR with its own `feat:` subject. Phase 1
is the largest and the most valuable. Each later phase depends only on Phase 1.

## Design principles

| Concern | Approach | Reference |
|---|---|---|
| A false positive is permanent corruption | Every new grammar is label-only or self-labeling. `IATA:` needs its label; `FREQ:` needs its label; a report is recognized by its own header (`METAR`, `TAF`, `!JFK 09/123`, or `KJFK 221651Z` followed by a wind group) | `docs/design/airfields.md:67-129` |
| Validation by lookup where a lookup exists | `IATA:HNL` declines unless the build holds `HNL`; a report station is decoded whether or not it is in the database, because the report is the thing being read | `airport.go:131` |
| A link may never disagree with itself | The airport URL carries `v` or `i`, never both; a report URL carries the report and its instant, and the page requires the instant to reproduce the report's time group | `dtg.go:284-360` |
| Render in Go, once | Every new payload carries rendered strings. No `format.ts` and no paired fixture table | `format.go:30`, `docs/design/airfields.md:271-275` |
| Keep the post ordinary when a link can hold it | A single-line report is a link, search-safe. Only what a link cannot hold (more than one line, a fence) is stamped | `docs/design/airfields.md:481-501` |
| Recognition is narrow where a stamp is permanent | The report stamp reads a fence labeled `metar`, `speci`, `taf` or `notam`, or a sole message whose first line is a report header. Never a bare unlabeled fence | `docs/design/geojson.md`, "Recognition" |
| Maps read the map switches, formats read theirs | The route map and the report card read `EnableLocationMapInline` for the picture; each stamp has its own opt-out because setting `Post.Type` costs search matches | `CLAUDE.md`, "Setting `Post.Type` costs" |
| Air-gapped by construction | Every dataset is embedded with `//go:embed`, generated by a stdlib-only program that is not a `make test` prerequisite, with provenance and SHA-256 in a README beside it | `data/README.md`, `Makefile:266-272` |
| Nothing fetches | No outbound HTTP anywhere in this plan | verified: no `http.Get` in the tree |

## Reference patterns

- Decorator contract and the two optional interfaces: `server/decorators/decorator.go:133`, `:170`, `:205`.
- Label consumed, terminator matched but not linked: `server/decorators/airport/airport.go:93-121`, `grammar.go:37,75`.
- Boundary guard: `server/decorators/boundary.go:26`. Never hand-write a third.
- Page shell and capabilities: `server/decorators/page.go:13-65`; the airfield page is `PageStatic` with no script (`airport/page.go`).
- Discriminated API shape and its sync test: `server/api.go:84-124`, `server/airport_sync_test.go:19-169`.
- Webapp client with TTL cache, ten-second bound, `failed` never cached: `webapp/src/decorators/airport/airport.ts`.
- Panel map mounted outside the status branch: `AirportPanel.tsx:228-255`.
- Multi-marker, extent-only map: `webapp/src/geojson/GeoJsonMap.tsx:111-142,262-296`.
- Overlay page by kind: `webapp/src/page/OverlayPageView.tsx:65`, `server/mappost.go:44`, `location.RenderOverlayPage`.
- Stamper shape: `server/hooks_geojson.go:36-80` (recover first, strip, switch, source, parse, ladder, commit).
- Example sets keyed by `Type()`: `server/command_examples.go:30-81`; `TestExamplesCoverEveryRegisteredDecorator`.
- Settings sections: `plugin.json:188-207`; counts held by `TestTheStatedSettingCountsMatchTheManifest` (`server/help_docs_test.go:523`).
- Error code allocation: `server/errcode/codes.go:11-21`; next free: hooks 11016, http 12009, api 13010, decorators 17003.
- Help page inventory: `server/help_docs_test.go:31-46`; per-decorator page slots: `public/help/airfields.html:58-266`.
- Bridge types and switch arms: `bridgeclient/types.go:16-28`, `server/bridge.go:199-240`.

## Requirements

- [ ] `IATA:HNL` decorates to a link that opens Honolulu; `IATA:ZZZ` and `iata:hnl` decline; a bare `HNL` never decorates.
- [ ] Every airfield surface (hover, panel, page, table, API) shows runways and frequencies where the data has them, and says nothing where it has none.
- [ ] A military designator in the airfield's name (`AFB`, `AB`, `NAS`, `MCAS`, `AAF`, `RAF`, `ANGB`, `Air Base`, `Air Force Base`, `Naval Air Station`, `Army Airfield`, `Joint Base`) is shown as a row naming the designator, never as a bare "military" claim.
- [ ] The panel map draws the airfield's runways as lines when both ends have coordinates, and `/map?airport=PHNL` opens the same picture full-window.
- [ ] A message that is nothing but two or more labeled airfield references draws all of them on one map under the post, numbered in message order with legs between consecutive airfields, and `/map?post=` draws the same. The links stay.
- [ ] A single-line METAR, SPECI, TAF or FAA-format NOTAM in a message becomes a link. Hover shows a one-line plain-language summary; the panel and page show every decoded group, list what was not decoded verbatim, and draw the station's airfield on the map when the build holds it.
- [ ] A multi-line TAF or ICAO-format NOTAM that is the whole message, or any report in a fence labeled `metar`, `speci`, `taf` or `notam`, is stamped: the message stays verbatim, a card renders the decode and the map, the sidebar opens on it, and `/map?post=` draws the station.
- [ ] A report's day-of-month is resolved against the post's reference time the way the DTG short form is, the UI says the month and year were inferred, and a hand-edited instant is refused.
- [ ] `FREQ:121.5`, `FREQ:243.0`, `FREQ:118.300 MHZ` and `FREQ:8992 KHZ` decorate; hover and panel say the band and any known allocation; `FREQ:` in lower case declines.
- [ ] Every new switch is in a named section, defaults on, and governs decoration only. `/map?airport=` and the two cards read the map switches.
- [ ] The plugin builds and tests on a clean, air-gapped checkout with no generator run first.
- [ ] `make check-style && make test && make sbom-audit` pass with no new runtime dependency.

## Out of scope

- Tail numbers, call signs, aircraft type designators, any aircraft dataset.
- Live weather, NOTAM feeds, any outbound HTTP, any weather source setting.
- Thread-wide or channel-wide aggregate maps; a map mixing airfields and coordinates from one message (the route stamp requires every token to be an airfield).
- Navaids, airspace, SIDs and STARs, approach plates.
- A bare three-letter IATA grammar (the collision rate is worse than four letters) and a bare frequency grammar (`121.5` is any decimal).
- ICAO-format NOTAMs on one line inside prose (they are multi-line by construction; the stamp reads them).
- SNOWTAM, ASHTAM, PIREP, SIGMET, AIRMET, volcanic ash advisories. The decoder is built so a later plan can add a kind without touching the surfaces.
- Reports as file attachments (`.txt`). The two source kinds are the message and a fence.

## Technical approach

### Phase 1: airfield enrichment

#### 1.1 The data: three files from one origin

`build/airportdata/main.go` currently reads the DataHub repackaging, which has
no runways and no frequencies. It changes to read OurAirports' own three files,
all public domain, placed under the gitignored `build/airportdata/source/`:

| Upstream | Rows kept | Output |
|---|---|---|
| `airports.csv` | `ident` matching `^[A-Z]{4}$`, minus `ZZZZ` | `server/decorators/airport/data/airports.csv` |
| `runways.csv` | `airport_ident` in the kept set | `server/decorators/airport/data/runways.csv` |
| `airport-frequencies.csv` | `airport_ident` in the kept set | `server/decorators/airport/data/frequencies.csv` |

`airports.csv` keeps its ten columns and adds one: `military`, the designator
the generator matched in the name, or empty. The match is a fixed list of
whole-word designators (above), measured against the data in the design note
and pinned by `TestMilitaryDesignatorsAreWholeWords`. OurAirports has no
military flag, and a row that says "Military designator: AFB" is the author's
own text rather than a claim about the field's operator.

`runways.csv` columns: `ident, le_ident, he_ident, length_ft, width_ft,
surface, lighted, closed, le_lat, le_lon, he_lat, he_lon, le_heading`. Ends are
rounded to four decimals like the reference point, and a runway with one end
missing keeps its rows and gets no line. `surface` is normalized through a table
in the generator (`ASP`, `ASPH`, `asphalt` to `Asphalt`; `CON`, `CONC` to
`Concrete`; `TURF`, `GRS`, `grass` to `Turf`; `GRE`, `GRVL` to `Gravel`;
`DIRT`, `SAND`, `WATER`, `PEM`) and an unknown value is kept as written if it
passes the whitelist and is at most 24 runes, else dropped. The table and the
residue count go in the design note.

`frequencies.csv` columns: `ident, type, description, mhz`. `mhz` is written to
three decimals, the aviation channel convention; the source carries one to
three. Rows are sorted by ident, then type, then value, so a regeneration
diffs cleanly.

The generator refuses a runway or frequency whose `ident` is not in the kept
set, a duplicate IATA code (see 1.2), an end coordinate outside its range, a
non-finite number, a line break in any field, and more than 32 runways or 48
frequencies on one airfield. The caps are measured maxima plus headroom and are
what bounds the panel, the page and the table.

`data.go` gains `//go:embed` for the two new files, `Runway` and `Frequency`
structs, `RunwaysOf(ident)` and `FrequenciesOf(ident)` reading pre-grouped
slices built at init, and the rune whitelist is measured again over the new
text columns (`TestTheTextWhitelistCoversEveryTextColumn`). `data/README.md`
records three upstream SHA-256s, sizes, the retrieval date, the surface table
and the designator list. `make airport-data` stays out of `make test`.

Regenerating `airports.csv` from the direct file rather than the DataHub
repackaging will churn some rows (different snapshot date). `TestEveryAirfieldIsUsable`,
`TestEveryAirfieldConverts`, `TestEveryAirfieldLinksToAPageThatRenders` and the
table sweeps hold the new file to the same bar; the count in prose (19,012) is
updated wherever `TestManyIdentsAreOrdinaryWords` logs it.

#### 1.2 The `IATA:` grammar

A second `Pattern` in `airport.Patterns()`, on when `Formats.IATA` is:

```
((?:IATA)[ \t]*:([A-Z]{3}))(?://)?
```

Same `ReplaceGroup`, `Extract`, `Boundary` and `[ \t]*:` separator as the ICAO
pattern, for the same reasons. Upper case only. `Parse` distinguishes the two
by length: four letters looks up the ident and answers `v`; three letters looks
up the IATA index and answers `i`. The bridge's `link('airport', 'HNL')` then
works with no label, which is what `bridge.md` asks of `Parse`.

The IATA index is built at init from `iata_code`. A duplicate IATA code in the
upstream (OurAirports has a handful) fails the generator, which drops the code
from every row carrying it and records the list in the README; the index
therefore never has to choose. `TestEveryIATACodeNamesOneAirfield` pins it.

The URL carries `i=HNL` and nothing else. `v` and `i` together, or neither,
is a 400 on the page (`AirportPageParamsConflict`, 17003) and on the API
(`APIAirportParamsConflict`, 13010), the same rule DTG's `z`/`o` pair follows.
The page, the API and `Describe` resolve `i` through the index and render the
same `Details` the ICAO path renders; `Details` gains `IATA` already and needs
nothing new for this. A dropped IATA code answers `found: false` at 200 with the
existing note, never 404.

Webapp: `IATA = /^[A-Z]{3}$/` beside `IDENT`, pinned by a sync test; `fromParams`
reads exactly one of `v`, `i` into `AirportPayload {key: 'icao' | 'iata'; code}`;
`endpoint` writes the matching parameter; `fetchAirport`'s echo check becomes
"the answer's `ident` equals the code asked for, or its `airport.iata` does".
The panel header shows the ICAO ident under the name as today, with the IATA row
present already.

#### 1.3 `Details`, the API and the three surfaces

`Details` gains:

```go
Military    string
Runways     []Runway     // Designation, Length, Width, Surface, Lighted, Closed string; Ends [2]Coordinate (Format, Token) when both ends are known
Frequencies []Frequency  // Type, Description, MHz string
```

`DescribeFields` includes runways and frequencies (map reads, cheap enough for
the post path, and the table needs them); `Describe` also builds each runway
end through the same `position` gate the reference point uses, so an end the
location grammar would refuse carries no `Ends` and draws no line.

`airportDetails` on the wire gains `military`, `runways` and `frequencies`,
with `airportRunway {designation, length, width, surface, lighted, closed,
ends?: [coordinate, coordinate]}` and `airportFrequency {type, description,
mhz}`. The `webappInterface` scraper in `airport_sync_test.go:117-135` reads
`name: type;` lines only; it is extended to read `name: Type[];` and
`name?: [Type, Type];` and to recurse into the two new interfaces, and
`TestWebappAirportShapeMatches` keeps holding names, types and order.

**Panel** (`AirportPanel.tsx`): a `Use` row (plain, no copy) when `military` is
non-empty; a Runways section (a small table: designation, length by width in
feet, surface, "lighted" or "unlit", "closed" when closed); a Frequencies
section (type, MHz with a copy button, description). Both sections render
nothing when empty. The map gains `geometries` built from `ends` through
`location.fromParams`, one open two-vertex `MapShape` per runway, in the
airfield chip color; the accessible label counts them ("with 4 runways
drawn").

**Page** (`page.go`): the same rows and two sections, still `PageStatic` with no
script; runway ends link nowhere (the map page does that).

**Table** (`markdown.go`): a Runways row joining `08L/26R 12,300 x 150 ft
asphalt` with `; `, and a Frequencies row joining `TWR 118.300`. The expansion
already falls back to the plain link when it would not fit `safePostRunes`;
`TestTheLargestAirfieldTableFitsTheFloor` measures the largest and
`TestEveryAirfieldTableIsWellFormed` keeps sweeping all rows. `mdCell` is
applied to every new value; the whitelist test covers the new columns.

**Hover** unchanged: name and place, one line.

#### 1.4 `/map?airport=<ident>`

A third address on `/map`, checked after `post` and before the coordinate
fallback in `server/http.go:105-114`. `serveAirportMapPage` validates the shape
(`MatchesIdentShape`), runs `Describe`, and answers 404 `HTTPMapAirportUnavailable`
(12009) for anything else, one refusal like `?post=`. It renders through the
existing `location.RenderOverlayPage(w, params, packages, kind, blob)` with
`kind = airport.MapKind` (`"airport"`) and `blob` the JSON of
`{ident, name, coordinate, runways: [{designation, ends}]}` built in Go.
No new capability, no new CSP.

Webapp: `page/OverlayPageView.tsx:drawingFor` gains the `airport` kind, read
by a new `airportMapFromBlob` in `webapp/src/decorators/airport/map.ts` that
validates the blob the way `asAirport` validates the API answer, and draws
`AirportMapCanvas` (marker plus runway lines, `fill`, `openAt`). The panel's
"Open larger" points at `/map?airport=PHNL` through a new `airportMapPageHref`
in `map/view.ts`, with `withTheme` and the camera fragment, when
`features.mapPage` is on.

#### 1.5 Switches

`EnableAirportIATA` joins the Airfields section, default on, ANDed with
`EnableAirport` in `airportFormats()`. `Formats` gains `IATA bool`. Nothing
new for runways, frequencies or the map: they are rows of the same airfield
and the panel map already reads `mapPanel`.

### Phase 2: the route map under a multi-airfield message

The tagger reports one token. This phase makes it report every token so a
decorator can act on a message that is nothing but its tokens.

**`Result`** (`tagger.go:173-187`) gains:

```go
type Token struct { Type string; Params url.Values; Trail string }

Tokens   []Token   // every accepted token, in message order
Covers   bool      // the tokens and whitespace alone make up the message
OnlyType string    // the one type every token has, or ""
```

`soleTokenResult` becomes `resultOf(message, accepted)`. It walks a **clone**
of `accepted` sorted by `match.start`, because `applyReplacements`
(`tagger.go:644`) re-sorts the original in place and the verdict is taken
before it; between consecutive `match` spans only `tokenSurroundingSpace` may
appear, `max(cursor, ...)` tolerates the overlapping matches `resolveOverlaps`
permits, and each token's `Trail` is `message[replace.end:match.end]`.
`SoleToken` is then `Covers && len(Tokens) == 1` with `Type`, `Params` and
`Trail` copied from the one token, so every existing consumer and every test in
`tagger_internal_test.go:201-249` is unchanged. Separators are whitespace only,
exactly `SoleToken`'s rule: `DEPLOC:PHIK ARRLOC:PGUA//` and one per line cover;
`DEPLOC:PHIK, ARRLOC:PGUA` does not (the comma) and is decorated without a
stamp. Recorded as a limit in `decorators.md` rather than widened quietly.

**A second optional interface** in `decorator.go`, beside `PostRenderer`:

```go
type MultiPostRenderer interface {
    MultiPost() (postType, propsKey string)
    MultiPostProps(tokens []Token) (map[string]any, bool)
}
func MultiPostType(d Decorator) (postType, propsKey string)   // same prefix and PostTypeMaxLen gate as StandalonePostType
func MultiPostProps(d Decorator, tokens []Token) (map[string]any, bool)
```

The decorator builds the blob, unlike `StandalonePostProps`, because the blob
carries a per-airfield lookup only `airport` can do. Location keeps
`PostRenderer`, airport implements only `MultiPostRenderer`, DTG neither.

**The hook**: `hooks.go:179-181` becomes `stampDecoratedPost(updated, found)`,
still inside the `if message == decorated` gate and inside `decorateMessage`'s
own recover, so no new recover: when `SoleToken`, the existing
`stampStandalonePost`; when the post is still untyped and `Covers` with a
non-empty `OnlyType`, `stampMultiTokenPost`, which asks `MultiPostType`, then
`MultiPostProps`, then commits through the existing `commitStamped` with one
rung and its own codes (`HooksAirfieldsPropsUnmeasurable`, `HooksAirfieldsPropsTooLarge`).
`commitStamped` leaves the clone untouched on failure, so a route that does not
fit is posted with its links and no map. Outcomes: one airfield with
`EnableAirportTable` on is expanded and never stamped (the table wins, as
today); one airfield with the table off is stamped with a one-entry route (a
marker, no leg), which is the location inline-map precedent; two or more are
stamped; an airfield beside a DTG is `OnlyType == ""` and not stamped; two
coordinates are not stamped because location declares no `MultiPostRenderer`
(`TestDecoratePostDoesNotStampTwoCoordinates` stays green).

**The airport side**, new `server/decorators/airport/route.go`:

```go
PostType          = decorators.PostTypePrefix + "tf_airfields"   // 19 bytes
PropsKey          = "tactical_fusion_airfields"
PropsVersion      = 1
MaxRouteAirfields = 64
```

Its own props key, never the shared `tactical_fusion` key: that key is
location's and the strip deliberately leaves it alone (`hooks_stamp.go:16-21`).
`stampedTypes` gains `{airport.PostType, airport.PropsKey}`, which is required
in any case because `stampedPropsKey` (`mappost.go:136-144`) is how
`/map?post=` finds a blob; a forged `custom_tf_airfields` post is therefore
stripped in `cotStamp` and re-decided by the tagger, which is stronger than
location's posture, and `geojson.md`'s "the strip clears every key" section
records why airfields is in the table and `custom_tf_location` still is not.

The blob is `{version, airfields: [{ident, name, format, value}...]}` in
message order: identity plus the location pair, no region, place or
elevation. The pair is what every stamped blob carries for a position
(`cot.go:452-459`, `geojson.go:189-190`), it must be built in Go (float
formatting), and it must be built **cheaply** on the post path: `FormatFloat`
then `location.Parse` and `Canonical()` as CoT does, never `location.Convert`,
which runs the polygon lookup `DescribeFields` exists to avoid. Past
`MaxRouteAirfields` the stamp is refused rather than truncated (the
`maxCotEvents` argument), and the whole props map is still measured by
`commitStamped`. The panel keeps looking each airfield up live; the name in
props is frozen like every stamp.

**Webapp**: `AirfieldsPostBody.tsx` registered directly in `index.tsx` beside
the CoT and GeoJSON bodies, unconditionally, so a stamped post keeps rendering
with the switch off. `inline.ts` gains `decoratorLinks(message)`, the global
form of `soleDecoratorLink`; the body requires the ordered list of link `v`
values to equal the props idents (the multi-link form of `agrees()` in
`PostBody.tsx:24-26`) and otherwise renders the message as plain text, which is
what stands the card down after an edit. Text segments with an `<a>` per link
render outside the `ErrorBoundary`, the map inside it; `compactDisplay` draws
no map. `AirfieldsMap.tsx` exports `AirfieldsMapCanvas` (one numbered marker
per placeable entry through `location.fromParams`; one open `MapShape` per run
of consecutive placeable entries, so a leg never crosses an entry that failed to
place; extent-only `LocationMap` after `GeoJsonMap.tsx:279-295`),
`drawsNothing`, and a wrapper reading `features.mapInline`, `INLINE_ID` and
`useNearViewport` after `GeoJsonMap.tsx:323-359`. The legend's numbers open
the airfield panel through `setSelection`. `drawingFor` gains the kind for
`/map?post=` with a label like "2 airfields, PHIK to PGUA". The reader refuses
a blob over the cap rather than slicing it.

**Switch**: `EnableAirportRoute` in the Airfields section, default on, with
the Elasticsearch and OpenSearch warning `EnableCot` carries. `Formats` gains
`Route`, and `airportFormats()` sets it to `EnableAirport && EnableAirportRoute
&& p.locationMaps().Inline`, so `EnableLocation`, `EnableLocationMap` and
`EnableLocationMapInline` gate it exactly as they gate location's stamp.
`MultiPost()` answers `"", ""` when `Route` is off. Read at decoration; the
webapp reads `features.mapInline` at render, location's split.

Legs are straight lines between consecutive airfields in message order and the
card says so. A great-circle leg is a claim about a route nobody stated; message
order is what the author wrote.

### Phase 3: aviation reports

#### 3.1 The package

`server/avreport/` holds the decoder and the decorator together (the decorator
is thin and the decoder is the work): `parse.go` (the four kinds), `metar.go`,
`taf.go`, `notam.go`, `vocab.go` (the WMO 4678 weather table, sky, units),
`contractions.go` with `data/contractions.csv` (the FAA contractions list,
public domain, provenance in `data/README.md`), `qcodes.go` with
`data/qcodes.csv` (the NOTAM Q-code subject and condition tables as the FAA
publishes them), `render.go` (`Describe` to rendered strings), `decorator.go`,
`page.go`, `props.go`. `Type = "avreport"`.

**Kinds**: `metar` (covers `SPECI`), `taf`, `notam`. Each has a header
recognizer and a body decoder. The decoder is total: every group it does not
understand is kept verbatim in `Unknown` and shown under "Not decoded", so a
report is never refused for a group the vocabulary lacks. It is refused only
when the header does not parse.

**Bounds**: a report is at most 2,048 runes and 64 lines; `Parse` and the
stamper both refuse past that. No regex runs unbounded over a message: the
single-line patterns stop at end of line.

**The instant**: the `DDHHMM` group takes its month and year from the
reference time, exactly as the DTG short form does (`dtg/parse.go:80-85`:
`ref.UTC()`'s month and year, `AssumedMonth` and `AssumedYear` set, no
nearest-month arithmetic), and the decode marks it inferred so every surface
says so. `dtg` exports a small `ResolveDayTime(day, hour, minute int, ref
time.Time) (time.Time, bool)` for it rather than a second copy of the rule,
and `TestAReportAndAShortDTGResolveTheSameInstant` holds the two together. A
TAF's validity `2218/2324` and a NOTAM's `2609221200-2609232359` carry their
own dates and need no inference.

#### 3.2 The decorator link (single-line reports)

Three patterns, each one line, each ending at `[ \t]*=?` with the `=` matched
but not linked (the `//` trick, `ReplaceGroup`):

| Kind | Header, upper case only |
|---|---|
| METAR, SPECI | `(?:METAR\|SPECI)[ \t]+(?:COR[ \t]+)?[A-Z][A-Z0-9]{3}[ \t]+\d{6}Z[ \t]+` then the rest of the line; or the bare form `[A-Z][A-Z0-9]{3}[ \t]+\d{6}Z[ \t]+(?:AUTO[ \t]+\|COR[ \t]+)?(?:\d{3}\|VRB)\d{2,3}(?:G\d{2,3})?(?:KT\|MPS)` then the rest |
| TAF | `TAF[ \t]+(?:AMD[ \t]+\|COR[ \t]+)?[A-Z][A-Z0-9]{3}[ \t]+\d{6}Z[ \t]+(?:\d{4}/\d{4}\|NIL\|CNL)` then the rest |
| NOTAM (FAA domestic) | `![A-Z]{3}[ \t]+\d{2}/\d{3,4}[ \t]+[A-Z]{3,4}[ \t]+` then the rest |

The bare METAR form is admitted because the wind group makes it as specific
as a keyword: four letters, six digits, `Z`, and a wind group ending `KT` or
`MPS` do not occur together in prose. The DTG decorator also matches the
`221651Z` inside; the report span is longer, so `resolveOverlaps` gives it the
match, and when the report's `Parse` declines the DTG keeps it as today.
`TestAReportOutranksTheTimeGroupInsideIt` pins the first half and
`TestAGarbledReportLeavesTheTimeGroupToDTG` the second.

`Boundary` is `decorators.BoundaryOK`. The pattern is anchored at a line start
by the guard (`before` is `0` or `\n`), so `RMK METAR` inside another line
cannot start a second report. A pattern never crosses a line, so a multi-line
TAF pasted inside prose links nothing: its first line alone would carry a
truncated `v`, and the stamp (3.4) reads it only when it is the whole message
or fenced.

`Parse` decodes the whole report (cheap: a few hundred bytes, no allocation
beyond the groups), and answers `v` (the report text exactly as matched,
without the `=`) and `t` (the resolved instant in milliseconds). The link label
is the report text; a 150-character METAR becomes about a 500-character link,
which the existing `safePostRunes` check bounds.

`RenderPage` re-parses `v`, requires `t` to reproduce the report's day, hour
and minute and to lie within DTG's 1970 to 2200 window, else 400
(`AvReportPageInvalid`, 17004), and renders under `PageStatic` with no
script: kind and station heading, the report verbatim in a `<pre>`, the
decoded rows, TAF groups as sub-tables, "Not decoded" and "Remarks", and the
station's airfield name as an `<a>` to the airfield page and its position as
an `<a>` to the coordinate page when the build holds the station.

`/api/v1/avreport?v=&t=` answers the same rendered shape as JSON
(`avreportResponse`), with the same 400 rule (`APIAvReportInvalid`, 13011),
`Cache-Control: private, max-age=300`, no switch consulted. Its body is
exactly the props blob the stamp writes, so there is one wire shape and one
TypeScript reader (`avreport/types.ts`, `fromWire`), held to Go by
`TestWebappAvReportShapeMatches`, walked rather than scraped the way
`TestWebappGeoJSONShapeMatches` is.

#### 3.3 The rendered shape

```
kind          "METAR" | "SPECI" | "TAF" | "NOTAM"
station       "KJFK"
stationName   "John F Kennedy International Airport" or ""
observed      "22 Sep 2026 16:51Z"   (inferred month and year flagged: inferredDate bool)
summary       one line, the hover
flags         ["AUTO", "COR", "AMD", "NIL", "CNL"] subset
rows          [{label, value}]      METAR body and NOTAM fields, in report order
groups        [{period, rows}]       TAF: base forecast then FM, TEMPO, BECMG, PROB groups
remarks       [{label, value}]       decoded RMK groups
unknown       [string]               groups kept verbatim
coordinate    {format, value, region} or absent   the station's airfield, or a NOTAM Q-line center
radiusNm      "5" or ""              NOTAM Q-line radius
raw           the report text
```

Vocabulary decoded in Phase 3: wind (direction, speed, gusts, variable,
calm, `VRB`, `MPS`), visibility (`P6SM`, `M1/4SM`, `9999`, meters, `CAVOK`),
RVR, present weather (intensity, descriptor, phenomena), sky (`SKC`, `CLR`,
`NSC`, `FEW`, `SCT`, `BKN`, `OVC`, `VV`, `CB`, `TCU`), temperature and dew
point (`M` prefix), altimeter (`A` inHg, `Q` hPa), trend (`NOSIG`, `TEMPO`,
`BECMG`), wind shear (`WS`), TAF temperature extremes (`TX`, `TN`), remarks
`AO1`, `AO2`, `SLP`, `T` group, `PK WND`, precipitation begin and end. NOTAM:
FAA location, number, keyword class (`RWY`, `TWY`, `APRON`, `NAV`, `COM`,
`SVC`, `AIRSPACE`, `OBST`, `AD`), effective and expiry (`YYMMDDHHMM`, `PERM`,
`EST`), body contractions expanded from the FAA list with the original kept;
ICAO `Q)` line (FIR, Q-code subject and condition, traffic, purpose, scope,
lower and upper limits, center and radius), `A)` through `G)`.

Everything decoded is from a citable public source (WMO No. 306 code tables
as reproduced in the FAA Aviation Weather Handbook, FAA Order JO 7340.2
contractions, the FAA NOTAM manual's Q-code table). Anything that is not goes
in `docs/design/unverified.md` rather than in the decode.

#### 3.4 The stamp (multi-line and fenced reports)

`server/hooks_avreport.go` is the third stamper, called after CoT and GeoJSON
in `decoratePost` (`hooks.go:83-96`), format-major as `geojson.md` argues.
Fence sources are disjoint by label (`cot`/`xml`, `geojson`/`json`,
`metar|speci|taf|notam`) and bare sources by shape (`<event`, a brace pair,
plain text), so the only collision is an **attachment** beside a visible
report: with reports last, a `.cot` or `.geojson` file would win over a pasted
TAF. It is closed where `geojson.md:56-70` closed the same collision:
`cotFileSource` and `geoJSONFileSource` both refuse when `messageShowsReport(post)`,
defined after `messageShowsGeoJSON` (a labeled report fence suppresses whether
or not it decodes; a bare multi-line report suppresses only if it decodes).
`TestTheVisibleReportBeatsAnAttachmentAcrossFormats` pins it.

**Three callers justify one wrapper.** The recover, the strip and the
`enabled && post.Type == ""` gate are copied in `hooks_cot.go:16-43` and
`hooks_geojson.go:37-60`; a third copy is where drift starts. `geojson.md:79-88`
argues against a recover inside `commitStamped` because it would not span the
filestore call and has no `stripped` clone. A wrapper around the **whole**
stamper body meets both objections:

```go
func (p *Plugin) runStamper(post *model.Post, enabled bool, panicCode int, panicMessage string,
    recognize func(post *model.Post) (*model.Post, bool)) (result *model.Post, stamped bool)
```

It declares the recover first, owns `stripped` and returns it from the
recover, applies the gate, and calls `recognize`, which holds source-finding,
any filestore call, the parse and `commitStamped`. `cotStamp` and
`geoJSONStamp` keep their signatures and call sites; `panicOnFileInfo`
(`hooks_cot_test.go:510-535`) still lands inside the span and now tests the
one recover all three share. The result pair, the attachment gate and the
source kinds stay per format. `geojson.md`'s "not shared: the recover" is
rewritten to record the wrapper and why the old objection does not apply to it.

**Source rules** (`reportSource`): a fence through `SoleFencedBlock` whose info
string is one of the four labels, case-insensitive like `cotInfoString`; a
fenced single-line METAR **is** stamped, because a fence is a protected range
the decorator can never rewrite and the label is the author's statement. A
bare message: normalize CRLF, trim, and if `strings.Count(text, "\n") == 0`
it is not a source (the decorator links it, search-safe); if
`decorators.HasCodeSpan(message)` (a new exported helper over `codeRanges`) it
is not a source, for the reason `SoleElementSpan` refuses code; else
`avreport.Decode(text, ref)`. `Decode` decodes exactly **one** report, so a
bundle of METARs one per line is left to decoration: each line is
independently linkable and searchable, the opposite of CoT's "several events",
and the design note says why. The `avreport` decorator declares neither
`PostRenderer` nor `MultiPostRenderer`, pinned by `TestTheReportDecoratorDeclaresNoPostType`.
A multi-line TAF **inside prose** is neither a whole message nor a single-line
token, so it is neither linked nor stamped; `TestAMultiLineTAFInsideProseIsNeitherLinkedNorStamped`
holds it, because linking the first line alone would store a `v` that is a
truncated report.

**The blob**: `avreport.PostType = decorators.PostTypePrefix + "tf_avreport"`
(18 bytes), `PropsKey = "tactical_fusion_avreport"`, `PropsVersion = 1`,
`SourceMessage` and `SourceFence`, `MaxSourceBytes`, `MaxRows`, `MaxPeriods`.
Keys: `version, source, lead, trail, src, kind, station, station_name, format,
value, issued, issued_at, inferred, summary, flags, rows, periods, remarks,
unknown, radius_nm, rows_dropped`. The station pair comes from `airport.Lookup`
plus the cheap `location.Parse` path; a station outside the database carries
no pair and the card draws no map. Two rungs: the full blob, then
`PropsWithoutRows` (rows, periods, remarks and unknown dropped, `rows_dropped`
present, logged `HooksAvReportRowsDropped`); `src` is never dropped because
the webapp never reads `post.message` once stamped and the verbatim text is
the card. Past the last rung the post is left unstamped
(`HooksAvReportPropsTooLarge`). Refusals: `HooksAvReportPanic`,
`HooksAvReportUnreadable` (an ephemeral and a warn for a labeled fence only;
bare failures are silent, the `json`-fence argument),
`HooksAvReportPropsUnmeasurable`. `stampedTypes` gains the row.

**Caps on the webapp side**: `MAX_REPORT_ROWS` and `MAX_REPORT_PERIODS` equal
Go's, sliced in the reader as `cot/types.ts` slices, pinned by
`TestWebappAvReportCapsMatch`; the design note records that Go refuses past
the cap while the webapp shortens a list, acceptable for rows where it was not
for rings.

**Card** (`webapp/src/avreport/AvReportCard.tsx`, post body registered for the
type): the report verbatim as plain text, the summary line, the decoded rows,
TAF groups, and the station map when `features.mapInline` is on (marker at
the station or the Q-line center, an ellipse of `radiusNm` when present, the
same `LocationMap` mount the CoT card uses). Clicking the card opens the panel
(`registerPanel`, `showAvReport`), with a Customize list of hideable sections
(`rows`, `groups`, `remarks`, `unknown`, `map`) stored under a new `avreport`
preferences key, mirroring `geojson`.

**Panel for the link** reuses the same components from `/api/v1/avreport`,
with the five-state client copied from `airport.ts` (TTL cache, ten-second
bound, `failed` never cached, `rejected` on 400). The hover is the summary line
and returns `null` in every other state.

#### 3.5 Switches

A new **Aviation reports** section: `EnableAvReport` (parent),
`EnableAvReportMETAR` (METAR and SPECI), `EnableAvReportTAF`,
`EnableAvReportNOTAM`, and `EnableAvReportCard` (the stamp, with the search
warning). All default on. `avreportFormats()` ANDs the parent in Go. The map
in the card and panel reads `mapInline` and `mapPanel`.

### Phase 4: the frequency decorator

`server/decorators/frequency/`, `Type = "frequency"`, one pattern:

```
(FREQ[ \t]*:((?:\d{1,3}\.\d{1,3}|\d{4,5})(?:[ \t]*(?:MHZ|KHZ))?))(?://)?
```

Upper case only, label consumed, unit kept in the link label because it is
what the author wrote. `Parse` normalizes to kilohertz internally, refuses
anything outside 2 MHz to 1,300 MHz, and answers `v` as the author's token.
The page re-derives everything from `v` and refuses a token the pattern would
not have produced (`FrequencyPageInvalid`, 17005).

`Describe` renders: the frequency as written, in MHz to the resolution the
token carried, the band (HF aeronautical 2 to 30 MHz; VHF navigation 108 to
117.975; VHF air band 118 to 136.975 with the 25 kHz and 8.33 kHz channel
note; VHF marine 156 to 162.025; UHF military air band 225 to 400; distress
beacons 406 to 406.1; else "outside the aviation bands"), and a known
allocation when the value is one (121.5 aeronautical emergency, 243.0 UHF
emergency, 123.1 SAR on-scene, 122.75 and 123.45 air-to-air, 156.8 marine
channel 16). The tables are in `bands.go` with the ITU Radio Regulations and
FAA AIM citations in the design note.

Hover: band and allocation, one line. Panel: the rows with a copy button on
the frequency. Page: `PageStatic`. No API route (the page and panel derive
everything locally from `v`, the DTG pattern), no dataset, no map. Switch:
`EnableFrequency` in a new **Frequencies** section, default on.

### Admin settings

Eight new switches, two new sections: Airfields gains `EnableAirportIATA` and
`EnableAirportRoute`; **Aviation reports** has five; **Frequencies** has one.
The total becomes thirty-three switches across eight sections, and the four
prose sites `TestTheStatedSettingCountsMatchTheManifest` holds (`admin.html`,
`help.html`, `admin-settings.md`, `CLAUDE.md`) are updated in the same commit;
the number-word table in `help_docs_test.go:586` gains the entries it needs.
`defaultsOff` gets no entry: every new switch is label-only or self-labeling
and defaults on. `configuration.go` gains the eight fields, `plugin.go` the
three `Formats` builders.

### Error codes

| Code | Constant | Where |
|---|---|---|
| 11016 | `HooksAirfieldsPropsUnmeasurable` | route props will not marshal |
| 11017 | `HooksAirfieldsPropsTooLarge` | route stamp over budget |
| 11018 | `HooksAvReportPanic` | third stamper's recover, through `runStamper` |
| 11019 | `HooksAvReportUnreadable` | labeled fence that does not parse |
| 11020 | `HooksAvReportPropsUnmeasurable` | report props will not marshal |
| 11021 | `HooksAvReportPropsTooLarge` | ladder exhausted |
| 11022 | `HooksAvReportRowsDropped` | degraded rung committed |
| 12009 | `HTTPMapAirportUnavailable` | `/map?airport=` refusal |
| 13010 | `APIAirportParamsConflict` | both or neither of `v`, `i` |
| 13011 | `APIAvReportInvalid` | `/api/v1/avreport` |
| 17003 | `AirportPageParamsConflict` | airfield page |
| 17004 | `AvReportPageInvalid` | report page |
| 17005 | `FrequencyPageInvalid` | frequency page |

Each is four edits: the constant, `AllCodes`, the call site, the row in
`error-codes.html`.

### Bridge

Additive only. `bridgeclient/types.go` gains `TypeAvReport` and
`TypeFrequency`, the doc comment at `:72` lists them, `server/bridge.go`
`formatEnabled` gains arms (airport: `i` present means `IATA`; avreport by the
report's kind, read from the parsed params; frequency), `parsesWithEveryFormat`
and `bridgeInfo` learn the types, `bridgeclient/README.md` and
`public/help/integration.html` list them. `TestBridgeLinkOpensAPageThatRenders`
and `TestBridgeLinkIsTheLinkTheTaggerWrites` iterate the constants and cover
the new types with no change to their shape. `link` takes no label, so the
tokens are `HNL`, the report text, and `121.5`.
`TestBridgeClientTypesAreTheRegisteredDecorators` (`server/bridge_test.go:537-548`)
hard-codes three types and is updated with each new constant.

### Slash command

`exampleSets` gains rows and sets: airport rows `IATA:HNL` and a route line
`DEPLOC:PHIK ARRLOC:PGUA//` (decorates inline; the map needs a sole message,
which the note says); an `avreport` set with an invented METAR at `PHNL`, an
invented single-line TAF at `PGUA` and an invented FAA NOTAM `!HNL`; a
`frequency` set with `121.5`, `118.3` and `243.0`. Hawaii and Guam, as the
rule requires. `exampleSetOrder` gains the two types; the two coverage tests
enforce both directions, and `TestEveryCommandExampleIsDocumented` requires
each row on its help page.

### Help and design notes

Help: `airfields.html` gains sections for `IATA:`, the military designator,
runways and frequencies, the runway lines and `/map?airport=`, and the route
map (`#airfield-iata`, `#airfield-military`, `#airfield-runways`,
`#airfield-frequencies`, `#airfield-map`, `#airfield-route`). New
`reports.html` and `frequencies.html` follow the per-decorator slots
(`grammar`, `declined`, `panel`, `card` or `table`, `map`, `data`,
`settings`). `formats.html` index gets two rows and its heading stops saying
"four". `admin.html` gains the eight settings with `data-setting`; `panel.html`,
`commands.html`, `troubleshooting.html` (the IATA row at `:255` is rewritten;
rows for reports and frequencies added), `error-codes.html`, `integration.html`
and every page's nav block are updated; `helpPages` in `help_docs_test.go`
gains the two files.

Design notes: `docs/design/airfields.md` gains the data, IATA, designator,
runway, frequency, map page and route map rationale; new `docs/design/avreports.md`
and `docs/design/frequencies.md`; `decorators.md` records the `Result` change;
`mapping.md` the third `/map` address and the two new overlay kinds;
`admin-settings.md` and `help-and-errors.md` their counts; `cot.md` and
`geojson.md` a cross-reference to the third stamper; `unverified.md` anything
decoded from an uncited source. `CLAUDE.md`: the overview line, both
architecture tables, the design-notes table, the invariants ("Three formats
stamp"; "the five things that set `Post.Type`"; the route stamp beside the
location stamp), and the sync-point table.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Bare IATA grammar? | No. `IATA:` label, upper case, permanently | Three letters collide worse than four; the airfield note measured 343 word-shaped four-letter idents |
| Which OurAirports files? | The three direct files, replacing the DataHub repackaging | Runways and frequencies exist nowhere else; one origin, one license, one README |
| Military flag | The designator matched in the name, shown as text | OurAirports has no operator field; a designator is the author's data, "military" would be a claim |
| Runway surface | Normalized through a documented table, residue kept if clean | Upstream is free text in three cases and several spellings |
| Frequency precision | Three decimals | The channel convention; the source is a database, not an author's token |
| `i` and `v` | Exactly one, else 400 | The DTG `z`/`o` rule; a link may never disagree with itself |
| Route map props | Carry each airfield's `(f, v)` pair and name | The card and the map page draw with no fetch; the CoT precedent for parsed positions in props; the panel still looks up live |
| Legs | Straight, in message order, said so | A great-circle leg claims a route nobody stated |
| Route map at one airfield? | Only when the table is off | "Expanded or stamped, never both"; with the table off a one-entry route is the location inline-map precedent |
| Route props key | Its own, `tactical_fusion_airfields`, in the strip table | The shared key is location's and the strip leaves it alone; the map page finds a blob through the strip table |
| Three stampers' prologue | One `runStamper` wrapper around the whole body | Three copies of recover, strip and gate; the wrapper spans the filestore call and owns the stripped clone, which is what the old objection to sharing was about |
| A fenced one-line METAR | Stamped | A fence is a range the decorator can never rewrite, and the label is the author's statement |
| A bundle of METARs one per line | Links, never a stamp | Each line is independently linkable and searchable |
| Single-line report | A link, never a stamp | Search-safe; a link holds it |
| Multi-line report | A stamp, never a link | A link label spanning lines is a rendering claim this plugin has not verified, and a rewrite that breaks rendering is permanent |
| Bare `KJFK 221651Z ...` METAR | Admitted with the wind-group requirement | As specific as a keyword; the DTG short form made the same narrowing trade |
| Unknown groups | Kept verbatim under "Not decoded" | The report is never refused for vocabulary the build lacks |
| Report instant | Resolved like the DTG short form, flagged inferred, carried as `t`, required to round-trip | The DTG precedent, and the reference time is the only source of the month |
| Where the report decoder lives | `server/avreport/`, decorator and decoder together | The decoder is the work; the CoT and GeoJSON packages are the shape |
| Frequency grammar | `FREQ:` label only, unit kept in the label | `121.5` is any decimal; the unit is the author's text |
| Live weather | Out | Air-gapped target; no outbound HTTP anywhere in the plugin |

## Files to modify

### New

| File | Change |
|---|---|
| `server/decorators/airport/data/{runways,frequencies}.csv` | Generated, committed, embedded |
| `server/decorators/airport/{runways,frequencies,iata}.go` and tests | Parse, index, lookups, rendering |
| `server/decorators/airport/mapblob.go` | The `/map?airport=` blob |
| `server/decorators/airport/route.go` and test | Post type, props key, cap, `MultiPostRenderer` |
| `server/mapairport.go` | `serveAirportMapPage` |
| `server/hooks_avreport.go` and test | The third stamper, on `runStamper` |
| `server/avreport/**` and `data/{contractions,qcodes}.csv`, `data/README.md` | The decoder, decorator, page, props |
| `server/decorators/frequency/**` | The frequency decorator |
| `server/avreport_sync_test.go`, `server/frequency_sync_test.go` | Cross-language pins |
| `webapp/src/decorators/airport/{map.ts,AirportMapCanvas.tsx,AirfieldsPostBody.tsx,AirfieldsMap.tsx}` | Map blob reader, canvas, route body and map |
| `webapp/src/avreport/**` | Types, client, card, panel, hover, map, customize, harnesses |
| `webapp/src/decorators/frequency/**` | Decorator, panel, hover |
| `public/help/{reports,frequencies}.html` | Help |
| `docs/design/{avreports,frequencies}.md` | Rationale |

### Modified

| File | Change |
|---|---|
| `build/airportdata/main.go` | Three inputs, three outputs, designator match, surface table, IATA duplicate rule, caps |
| `server/decorators/airport/{airport,grammar,data,format,page,markdown}.go` | IATA pattern and `i`, `Details` growth, rows, sections |
| `server/decorators/decorator.go`, `tagger.go` | `Token`, `Result.Tokens`, `Covers`, `OnlyType`, `MultiPostRenderer`, `HasCodeSpan` |
| `server/hooks.go`, `hooks_stamp.go`, `hooks_cot.go`, `hooks_geojson.go` | `stampDecoratedPost`, the third stamper call, `runStamper`, `stampedTypes` rows, the report gate in both file sources |
| `webapp/src/decorators/inline.ts`, `src/index.tsx` | `decoratorLinks`; two body registrations and one panel |
| `server/http.go`, `mappost.go` | `?airport=` address |
| `server/api.go` | `i`, the extended `airportDetails`, `/api/v1/avreport` |
| `server/configuration.go`, `plugin.go`, `plugin.json` | Eight switches, two sections, three `Formats` builders, two registry arguments |
| `server/errcode/codes.go` | Ten codes |
| `server/bridge.go`, `bridgeclient/{types.go,README.md}` | Two types, switch arms |
| `server/command_examples.go` | Rows and two sets |
| `server/airport_sync_test.go` | Array-aware scraper |
| `server/help_docs_test.go` | Two pages, number words |
| `webapp/src/decorators/airport/{index.ts,types.ts,airport.ts,AirportPanel.tsx}` | `i`, new rows, runway lines, larger link |
| `webapp/src/decorators/index.ts`, `src/index.tsx` | Two decorators, two post bodies, one panel |
| `webapp/src/page/OverlayPageView.tsx` | Three kinds |
| `webapp/src/decorators/location/map/view.ts` | `airportMapPageHref` |
| `webapp/src/preferences/types.ts`, `server/preferences.go` | `avreport` section |
| `webapp/src/bridge/types.ts` | Nothing (types are the decorators' own strings) |
| `public/help/*.html` | Every page's nav, plus the pages named above |
| `docs/design/*.md`, `CLAUDE.md`, `README.md` | Counts, invariants, tables, the status paragraph |

## Tasks

### Phase 1

1. [ ] Fetch the three OurAirports files into `build/airportdata/source/`; rewrite the generator (three inputs, designator match, surface table, IATA duplicate rule, caps, three outputs); run it; commit the three CSVs and the README with three SHA-256s.
2. [ ] `data.go`: embed, `Runway`, `Frequency`, grouped lookups, IATA index; tests: usable, converts, whitelist over new columns, caps, one airfield per IATA, designators are whole words.
3. [ ] `grammar.go`/`airport.go`: the `IATA:` pattern, `Parse` by length, `Formats.IATA`; tagger tests (labeled decorates, bare declines, lower case declines, unknown declines, `IATA:HNL//`, end-of-line label, two on one line); corpus sweep of every IATA code behind the label in prose.
4. [ ] `format.go`: `Details` growth, runway ends through `position`, rendering helpers; `page.go` rows and sections; `markdown.go` rows; `TestTheLargestAirfieldTableFitsTheFloor`.
5. [ ] `airport.go` `RenderPage` and `api.go` `serveAirport`: `i` or `v`, conflict codes; API tests for every row of the rule table.
6. [ ] `/map?airport=`: `mapblob.go`, `mapairport.go`, `http.go` branch, 404 code, tests (unknown, malformed, switch off, page policy).
7. [ ] Webapp: `IATA`, payload key, endpoint, echo check, `types.ts` growth, panel rows and sections, runway lines, larger link; sync test scraper for arrays; `map.ts`, `AirportMapCanvas`, overlay kind; harness replies for runways and frequencies; component tests.
8. [ ] `EnableAirportIATA`; settings tests unmodified pass.
9. [ ] Bridge arm for `i`; examples row; help; design note; CLAUDE.md.
10. [ ] `make check-style && make test && make sbom-audit`.

### Phase 2

11. [ ] `Token`, `Result.Tokens`, `Covers`, `OnlyType`; `resultOf` on a sorted clone; tagger tests: every accepted token in message order, covers nothing but tokens and whitespace, does not cover prose or a comma between tokens, covers the consumed label and the trail, `OnlyType` empty for mixed types, `SoleToken` is covers with one token, the verdict survives `applyReplacements` reordering.
12. [ ] `MultiPostRenderer`, `MultiPostType`, `MultiPostProps`, `stampDecoratedPost` and `stampMultiTokenPost`; `airport/route.go` with the type, key, version and cap, the cheap pair, `Formats.Route`; the `stampedTypes` row; two codes; tests: route stamped on one line and one per line, links and trail kept, sole airfield stamped when the table is off and expanded when it is on, mixed tokens not stamped, inline map off or route switch off means no stamp, prose between means no stamp, past the cap decorated not stamped, measured against the budget, forged airfields blob stripped, forged type over a real route restamped, another integration's props kept, `TestRoutePostTypeFitsTheColumn`, `TestRoutePropsCarryThePairForEveryShippedAirfield`; `mappost_test.go` serves an airfields post and stands down for an edited one.
13. [ ] Webapp: `decoratorLinks` in `inline.ts`, `AirfieldsPostBody` (agreement check, links outside the boundary, map inside, compact draws no map), `AirfieldsMap` (legs between consecutive placeable entries only, `drawsNothing`, reader refuses past the cap), overlay kind, legend links to the panel, sync tests (post type, key, version, cap, shape walked), component tests, `EnableAirportRoute` and the four count sites; `decorators.md`, `airfields.md`, `mapping.md`, `geojson.md` (why airfields is in the strip table), CLAUDE.md invariants ("four things set `Post.Type`").

### Phase 3

14. [ ] `server/avreport/` decoder: header recognizers, the four bodies, vocabulary, contractions and Q-codes with provenance, `Unknown`, bounds; table-driven tests per group, plus a fixture per kind decoded whole and compared as a struct.
15. [ ] The instant: reuse `dtg`'s resolver (export what is needed), flag inferred, bounds.
16. [ ] Decorator: patterns, `Parse`, `RenderPage` with round-trip, `Formats`; tagger tests (each kind decorates, `=` stays, lower case declines, DTG outranked and left when garbled, protected inside a USMTF block, size fallback).
17. [ ] `/api/v1/avreport`; `runStamper` extracted and `cotStamp`/`geoJSONStamp` moved onto it with their tests unchanged; `decorators.HasCodeSpan`; `messageShowsReport` and the two attachment gates; `hooks_avreport.go` with the two rungs and five codes; `stampedTypes` row; hooks tests mirroring `hooks_geojson_test.go` (fenced METAR in three spellings, fenced TAF and NOTAM, bare multi-line TAF and NOTAM, sole single-line report linked not stamped, fenced single-line stamped, two METARs on two lines linked not stamped, message left exactly as written, silent when off, labeled fence failure tells its author, bare failure silent and unlogged, `json` and `xml` fences never a report, a report fence never CoT or GeoJSON, exclusive with decoration, forged type and sibling blob stripped, another integration's type and props left alone, never a half stamp, rows dropped before the card, past the last rung refused, unmeasurable props reported, panic survives without costing decoration, visible report beats an attachment across formats, labeled failure still suppresses the attachment, bare report never reaches into code, CRLF read as multi-line, trailing newline does not make a one-liner multi-line, every blob marshals, the decorator declares no post type); `mappost_test.go` for a report post and for a station with no position.
18. [ ] Webapp: types with `fromWire`, client, hover, panel, card, map with ellipse, customize and preferences section, overlay kind, post body and panel registration, sync tests (shape walked, post type, props key, section catalog, caps), harnesses and component tests.
19. [ ] Five switches, section, bridge type and arms, example set, help page, design note, `unverified.md`, CLAUDE.md.

### Phase 4

20. [ ] `server/decorators/frequency/`: grammar, `Parse`, bands, `Describe`, page; tests per band edge, allocation, resolution, refusal.
21. [ ] Webapp decorator, panel, hover; sync test on the type and band table; component tests.
22. [ ] `EnableFrequency`, section, bridge, example set, help page, design note, CLAUDE.md.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Regenerating from the direct OurAirports file changes idents the shipped tests or examples name (`PHIK`, `PHNL`, `PGUA`, `NZSP`) | The sweeps hold every row; the named fixtures are asserted present by `TestTheExampleSitsOnTheAirfieldItClaims` and fail loudly |
| The generator needs network in the implementing session | One-off fetch, recorded by SHA-256; if the session has no network, Phase 1 stops at the generator and says so |
| A runway or frequency text carries a rune the whitelist refuses | `parseAirfields` fails at init and in every test; the README records what was dropped |
| The largest airfield table crosses `safePostRunes` | The expansion already falls back to the link; the new test measures the largest |
| `Result.Tokens` changes `SoleToken` semantics | `SoleToken` is derived and every existing tagger test runs unchanged |
| A route stamp costs a route message its search matches | Its own switch with the same warning `EnableCot` carries; the links stay in the message either way |
| A METAR-shaped line with a typo leaves a DTG link in its middle | Today's behavior, unchanged; the help page says so |
| A report longer than the link budget | The size check drops the decoration; a sole multi-line report is stamped instead |
| Vocabulary is wrong for some emitter | Unknown groups are kept verbatim, never guessed; every table cites its source; the rest is in `unverified.md` |
| The report page echoes author text | `PageStatic`, no script, `html.EscapeString` on every value, the same as the airfield page |
| Two new webapp bundles' worth of code | Nothing new is loaded until a report or frequency link is on screen; the map is the same lazy MapLibre chunk |
| The help counts and the settings counts drift | Both are held by tests that read the manifest and the pages |

## UX summary

| Scenario | Behavior |
|---|---|
| `IATA:HNL` | `IATA:` consumed, `HNL` links to Honolulu |
| `iata:hnl`, bare `HNL`, `IATA:ZZZ` | Nothing |
| Click an airfield link | Panel: name, code, place, use, type, elevation, IATA, runways, frequencies, map with runway lines, Open larger |
| Open larger | `/map?airport=PHNL` full window, runways drawn |
| `DEPLOC:PHIK ARRLOC:PGUA//` as the whole message | Both links, and a map under the post with 1 and 2 and a leg |
| Same line inside a longer message | Both links, no map |
| `ICAO:PHNL` alone | The table, as today |
| `METAR KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012 RMK AO2=` in a message | The report links; hover "Wind 280 at 12 kt, 10 SM, few at 25,000 ft, 24/12, 30.12 inHg"; panel decodes every group and draws JFK |
| `KJFK 221651Z 28012KT ...` with no keyword | Same |
| A three-line TAF as the whole message | Verbatim text, decoded card with per-period groups, station map |
| A NOTAM in a fence labeled `notam` | Card with fields, contractions expanded, Q-line center and radius drawn |
| A one-line METAR as the whole message | A link, not a card |
| `FREQ:121.5` | Link; hover "VHF air band, aeronautical emergency" |
| `FREQ:1090` | Link; panel says outside the aviation bands the plugin names |
| Any switch off | New messages stop; existing links and cards keep working |

## Testing plan

**Go, unit**: every grammar in both directions with the corpus sweeps
(`-short` gated where they run the tagger tens of thousands of times); data
integrity over every row of every file; decode tables per group with expected
strings; page round-trip refusals; props shapes and ladders; the `Result`
generalization; every settings and help test unmodified.

**Go, integration**: `MessageWillBePosted` for every row of the UX table; the
API rule tables; `/map?airport=` and `/map?post=` for both new kinds; the
bridge for every type constant; the example command coverage tests.

**Cross-language**: the extended airport shape, the avreport shape walked,
the route props shape, the post types and props keys, the section catalog,
the IATA shape, the band table.

**TypeScript, unit**: `fromWire` readers refuse every malformed input; the
clients' cache behavior; band and label helpers.

**Playwright, component**: the airport panel with runways and frequencies,
the runway lines on the canvas node, the route body with and without
`mapInline`, the report card and panel in every state, the hover returning
nothing but a card when ready, the frequency panel.

**Manual, on `make deploy`**: the UX table end to end, the mobile page for
each new decorator, and `/map?airport=` on a subpath install.

## Acceptance criteria

- [ ] Every row of the UX table behaves as stated.
- [ ] The stored message keeps the author's text verbatim everywhere except the consumed label and the link syntax.
- [ ] Panel, hover, page, card and API agree on every value for the same link.
- [ ] A clean, air-gapped checkout builds and tests with no program run first.
- [ ] `make check-style && make test && make sbom-audit && make dist` pass; no new runtime dependency; no copyleft data.
- [ ] Every new error code, setting, page and example is documented, and the count tests pass.
- [ ] No prose comments in new or modified code; no em dashes anywhere; conventional `feat:` subjects per phase.

## Checklist

- [ ] Help pages, including every nav block and the `formats.html` index.
- [ ] `examples` rows and sets; `check` needs nothing.
- [ ] Design notes before comments come out; `unverified.md` for uncited vocabulary.
- [ ] `CLAUDE.md` overview, tables, invariants, sync points.
- [ ] `README.md` status paragraph names what shipped.

## Open questions

- **The military designator list.** It is a judgment about names. The plan
  ships the list above and measures it; an install cannot edit it. Say if a
  designator is missing or wrong.
- **The report kinds' default.** All on. If NOTAM decoding should ship off
  until the vocabulary has been seen against real traffic, say so and it joins
  `defaultsOff` with a reason.

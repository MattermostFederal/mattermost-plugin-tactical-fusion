# Location Parsing and Enrichment

## Overview

Add a second decorator, `location`, that recognizes geographic coordinates in
posted messages, normalizes every recognized grammar to one WGS84 latitude and
longitude, and renders a consistent panel regardless of which notation the
author typed.

This is also the first real test of the claim in `CLAUDE.md` that adding a
decorator is "one directory per side plus one line in `OnActivate`". It is not,
and the four places it breaks are named below rather than absorbed.

## Problem Statement

Geospatial data is the largest item on this plugin's list, and today a
coordinate posted in a channel is inert text. Worse, it is inert text in
whichever of nine mutually unreadable notations the author's tool happened to
emit: a JTAC pastes MGRS, a maritime watch pastes degrees decimal minutes, a
drone log pastes decimal degrees, and nobody in the channel can compare them
without leaving Mattermost.

The value is **making a coordinate readable in the notation the reader works
in**, which is a conversion problem rather than a mapping problem, and therefore
one that works air-gapped. Comparing two coordinates against each other
(distance, bearing) is a later phase and is not claimed here.

## Current State

The decorator framework shipped with exactly one decorator. Everything it
promises is real but has been exercised by a single implementation:

| Surface | Today |
|---------|-------|
| `server/decorators/decorator.go` | `Decorator`: `Type()`, `Patterns()`, `Parse(value, ref)`, `RenderPage(w, params)`. `Pattern` is a regexp plus an optional `Extract`. |
| `server/decorators/tagger.go` | Protected spans, longest-match overlap resolution, label escaping, root-relative URLs. Decorator-agnostic. |
| `server/decorators/page.go` | Shared shell. `Page` is `{Title, BodyHTML, Theme}`. The stylesheet declares `.countdown`, `.dtg`, `.described`, and a `@keyframes` pulse. CSP grants `script-src 'unsafe-inline'` to every page. |
| `server/hooks.go` | `MessageWillBePosted`, panic recovery, post-size guard, system-post skip. Decorator-agnostic. |
| `server/api.go` | One resource, `/api/v1/preferences`. `Cache-Control: no-store` and a `p.preferences == nil` readiness check both run before routing. |
| `server/command_examples.go` | Three sections, plus a footer keyed on `p.dtgFormats()`. |
| `server/errcode/codes.go` | One thousand-wide range per source file. `17000-17999` is already `server/decorators/`, framework and decorator pages. |
| `webapp/src/decorators/` | `Decorator<T>` with `fromParams`, `summary`, `style`, `Panel`, optional `Title` and `Hover`. |
| `plugin.json` | Four bool settings, all DTG. |
| Deps | Go: Mattermost public SDK, `golang-lru/v2`, `pkg/errors`. Webapp: react 19, redux 5. **No geodesy on either side.** |

### Current Gaps

- No coordinate grammar anywhere in the repo.
- No geodetic projection code. MGRS and UTM need transverse Mercator on the
  WGS84 ellipsoid; nothing available provides it.
- `Pattern` has no way to match more text than it replaces, so a grammar cannot
  express a boundary guard without deleting the guard character.
- `Page` has no way for a decorator to add CSS, and grants inline script to
  pages that carry none.
- `configuration_settings_test.go:88` (`TestEverySwitchDefaultsOn`) requires
  every bool setting to default to `true`, so a default-off switch is not
  available without weakening it.
- `webapp/src/components/rhs/RhsView.tsx:27` names date-time groups in the empty
  state, which is the only way back out of a panel.

---

## Prior art: mattermost-plugin-aocanywhere

The sibling plugin already handles geographic location for the same audience,
and `CLAUDE.md` records that this repo's tagger was ported from its
`webapp/src/enhanced_text/tagger.ts`. It is the strongest available evidence
about what these users actually type and about what the same team judged safe to
scan free text for. Paths below are relative to
`mattermost-plugin-aocanywhere/main`.

### The finding that should change your mind about bare detection

`webapp/src/enhanced_text/patterns.ts` is the direct ancestor of this plugin's
pattern list. It has **one** location pattern:

```ts
{type: 'location', regex: /LATM:(\d{4}[NS]\d{5}[EW])/g, extractValue: (m) => m[1]},
```

There is no bare coordinate pattern anywhere in that file. The only bare pattern
in the entire set is the fully qualified DTG (`patterns.ts:55`); everything else,
locations, ICAO codes, frequencies, telephone numbers, call signs, mission
numbers and tail numbers, is **tag-gated**. The same team, the same audience and
the same corruption risk produced a design in which a coordinate is decorated
only when the author labeled it.

This plan proposes bare signed decimal degrees on by default. That is a
deliberate divergence and it is recorded as such in the Decisions table rather
than smoothed over. The mitigation is that the strict rules in section 4 are far
narrower than a naive coordinate regex, and that the switch is per grammar so a
workspace can retreat to the aocanywhere posture in one click.

### The USMTF tagged location family

`server/model/usmtf2004/sets/location.go:49-94` is a complete, tested USMTF 2004
location parser. Its prefixes are the real moniker vocabulary, and its compact
grammars are formats this plan did not have:

| USMTF type | Example (from `location_test.go`) | Shape | Resolution |
|---|---|---|---|
| `LATD` | `35N079W`, `LATD:40N122W` | `DD[NS]DDD[EW]` | about 111 km |
| `LATM` | `3510N07901W`, `LATM:2130N15730W` | `DDMM[NS]DDDMM[EW]` | about 1.8 km |
| `LATS` | `400948N1221400W`, `VLATS:400948N1221400W` | `DDMMSS[NS]DDDMMSS[EW]` | about 30 m |
| `LATDS` | `331000.0N1183000.0W` | seconds with a fraction | sub-meter |
| `DMPID` | `DMPID:641230.0N1683045.0W` | `LATDS` shape, desired mean point of impact | sub-meter |
| `VLATM` | `3510N9-07901W7` | `LATM` plus a per-axis confidence digit | about 1.8 km |
| `UTMO` / `UTMT` | `32WDL123123`, `32WDL12341234` | MGRS at 100 m and 10 m, despite the name | 100 m / 10 m |
| `GEOREF` | `AABB1535` | `[A-Z]{4}\d{4}` | area |
| `ICAO` | `KJFK`, `ICAO:KJFK` | 3 to 4 alphanumerics | an airfield |
| `BAR` | `BAR:180T-050NM` | bearing and range from a reference point | relative |
| `NAME` | `NAME:PT ALFA`, `Report Point Alpha` | free text | none |

Full prefix list, `location.go:72-94`, longest first because several share a
head: `LATDS:`, `DMPID:`, `DEPLATM:`, `ARRLATM:`, `VLATS:`, `VLATM:`, `LATM:`,
`LATD:`, `LATS:`, `ICAO:`, `DEPLOC:`, `ARRLOC:`, `LOC:`, `BAR:`, `DEPUTMO:`,
`ARRUTMO:`, `UTMO:`, `UTMT:`, `DEPNAME:`, `ARRNAME:`, `NAME:`.

**Correction this forces on the plan.** An earlier draft invented `COORD:`,
`LOC:` and `LAT/LON:` as monikers. In USMTF, `LOC:` means an **ICAO airfield
code**, not a lat/lon pair. Claiming it for coordinates would put this plugin in
direct conflict with the sibling plugin and with the standard. `LOC:` is
removed, and the moniker vocabulary is taken from the list above instead of
invented.

**A happy convergence.** The canonical form this plan independently chose for
DMS, `340322N1181500W`, is exactly USMTF `LATS`. The canonical form is therefore
also a real input format, and `v` equals the author's token whenever they wrote
it that way.

### What aocanywhere does not do, and this plugin would

- **It never converts.** `location_test.go:109-118` asserts
  `loc.Latitude == nil` for both MGRS cases: MGRS and GEOREF are detected and
  displayed, never resolved to a point. There is **no geodesy dependency in
  either tree** (`go.mod` and `webapp/package.json` have none) and no projection
  code. Converting MGRS to latitude and longitude is genuinely new capability
  here, and it is the reason Phase 2 exists.
- **Its grid validation is shape-only.** `location.go:50-53`:
  `^\d{1,2}[A-Z]{3}\d{8}$` for MGRS and `^[A-Z]{4}\d{4}$` for GEOREF. Neither
  checks band letters, 100 km square letter sets, or GEOREF's constrained
  alphabet. That is safe there because those patterns run against a single
  parsed USMTF field, never against free text. It is **not** safe here, which is
  why section 4 validates structurally. Do not copy those regexes.
- **Bare ICAO is `^[A-Z0-9]{3,4}$`** (`location.go:54`), again only ever applied
  to a known field. In free text that matches most acronyms in the language.

### The authoritative superset, and how much of it is unparsed

`public/ato/sets/route.html:400-475` documents the **17 notations** a USMTF route
point may use. `location.go` parses about nine of them. The rest fall through to
`LocationTypeName`, which is a silent catch-all, so a real coordinate in an
unsupported notation is currently indistinguishable from the string
`Report Point Alpha`.

The unparsed ones are formats an operator can legitimately type, so they belong
on this plan's roadmap rather than in its blind spot:

| Notation | Example | Note |
|---|---|---|
| `GEOK` (degrees, thousandths of a minute) | `GEOK:3510.234N07901.123W` | **This is USMTF's name for degrees decimal minutes.** It is the same shape this plan calls `ddm`, so `GEOK:` becomes a `ddm` moniker at no cost |
| Verified degrees | `35N8-079W6` | Verified variants exist at **every** resolution, not just `VLATM` |
| Verified seconds | `351025N6-0790125W4` | |
| Verified deciseconds (`VGEOT`) | `VGEOT:351025.3N9-0790125.7W1` | |
| UTM 1 meter | `32WDL1234512345` | Phase 2, alongside `UTMT` and `UTMO` |
| Georef centiminute | `AABB15233527` | Phase 3 |
| Abbreviated Georef | `AB1535`, `AB15233527` | Phase 3, and almost certainly prefix-only: two letters and four digits is not a token |
| `BEARING` | `BEARING:330T-PT ALFA-50NM` | Bearing and distance from a **named** reference point. Same class as `BAR`, same Phase 5 |

Two further cautions from that corner of the repo:

- **`BAR` has two disagreeing definitions.** `location_test.go:151` uses
  `BAR:180T-050NM`; the shipped grammar in `amsnloc.html` (Table 498) says
  `BAR:330-PT ALFA-50`, that is bearing, named point, range. Trust the document,
  not the test, if this is ever implemented.
- **Areas are a first-class USMTF concept and are entirely unparsed.**
  `AMSNLOC` field 6 and `PTRCPLOT` field 3 carry descriptors that qualify a point
  with a shape: `CIR:50NM` (radius), `RAD:1NM`, `WDTH:100YD` (corridor),
  `ELL:105M-200M-240.0` and `ELP:2000YD-1000YD-135.5` (ellipse: semi-major,
  semi-minor, orientation true), `SECTO:180240T27NM` and
  `SECTIO:058190T45-60NM` (sectors). A real line reads
  `PTRCPLOT/LATS:300105N0803428W/NAME:BLUE RIVER BRIDGE/RAD:1NM/150FT`.
  This is the same "a location is an area, not a point" problem GARS raises, and
  when this plan grows an area representation these are the grammars it should
  be able to express. Note there is **no linear-unit lookup table anywhere in
  that repo** (`NM`, `M`, `KM`, `YD`, `HF`, `FT` appear only in prose), so one
  would have to be written.

### Bugs in the prior art, recorded so they are not inherited

1. **No range validation at all.** `9999N99999W` parses cleanly to latitude
   99.9833. Degrees, minutes and seconds go through `Atoi` and `ParseFloat` with
   the errors discarded (`location.go:224-229`), and nothing checks lat <= 90,
   lon <= 180, or minutes and seconds < 60. This plan validates all of it, and
   the range check is what makes several declined rows work.
2. **`LatLonToLATM` truncates minutes rather than rounding**
   (`udl/peek.go:302-308`), a systematic bias of up to one arc minute, about
   1.8 km, always south and west. This plan's formatter **rounds**, and a test
   pins the boundary.
3. **A truthiness check drops the equator and the prime meridian.**
   `click_handler.ts:183` reads
   `if (!location?.latitude?.decimal || !location?.longitude?.decimal) return;`,
   so a coordinate with an exactly zero component silently does nothing. Add
   `0.0000, 32.5000` and `12.3456, 0.0000` as explicit positive test cases. Note
   this is distinct from the Null Island rule, which declines only the pair
   where **both** are zero.
4. **Axis order is inconsistent across the repo.** `weather/gml.go:29-32` parses
   EPSG:4326 as **lon first** (with a comment confirming it against the live
   API) while everything else is lat first. This plan states lat-then-lon once,
   at every boundary, and never carries a bare pair without naming which is
   which. The query string sidesteps it entirely by carrying `v` rather than a
   coordinate pair.
5. **Verified accuracy digits are matched and thrown away.**
   `location.go:59` captures groups 1 to 6 of `reVerifiedCAP` and never stores
   the accuracy digits, and `Coordinate` has no field for them. This plan keeps
   them, which is the whole point of the "V".

### Things worth reusing

- **`GPSURLTemplate`** (`plugin.json:42`, `server/configuration.go:16`,
  `webapp/src/store/config_store.ts:98`) is a `longtext` admin setting with
  `{lat}` and `{lon}` placeholders and worked examples for Google, Apple,
  OpenStreetMap and Bing. This answers the map-link question this plan deferred,
  and Tactical Fusion should adopt the same setting shape so an operator
  configures both plugins the same way. **With one divergence: the default must
  be empty, not Google Maps.** aocanywhere defaults to
  `https://www.google.com/maps?q={lat},{lon}`; for this audience an outbound
  request carrying a grid reference is an operational event, and it must be an
  admin's explicit choice rather than a default.
- **The USMTF geodetic datum vocabulary**,
  `server/model/usmtf2004/ffirn/geography.go:295-471`, roughly 200 codes
  (`WE` is WGS 84, `NAS-C` is NAD 27 CONUS, `EUR-M` is European 1950 mean). This
  is why "declined, never reinterpreted" is the right rule for a datum-bearing
  token: NAD 27 and WGS 84 disagree by up to about 200 m in CONUS, which is
  several MGRS cells.
- **Display conventions**: `formatters.tsx:163` renders
  `40°09'48.0"N 122°14'06.0"W`, and `location.go:105` rounds decimal degrees to
  four places. Matching both keeps the two plugins reading alike.
- **The raw fallback**: `coordinate_link.tsx:94-97` renders `location.raw` when
  it cannot parse. That is the same instinct as this plan's `AS WRITTEN` row.
  The enhanced-text URL goes further and carries an `orig` parameter
  (`enhanced_text/tagger.ts:135`, `?type=&value=&orig=`) holding the author's own
  text, which is the same idea as this plan's `r`. The difference in cost is
  worth being exact about: aocanywhere builds that link **client side** and
  never stores it, so `orig` is never attacker-supplied and never reaches a
  public server-rendered page. This plugin writes its links into the message
  permanently and serves them from an unauthenticated route, so the same
  parameter has to carry the four-way whitelist in section 2a rather than being
  trusted the way `orig` can be.
- **The link color**: `enhanced_text_styles.tsx:37` paints locations teal,
  `#388ea6` on `rgba(56,142,166,0.12)`. This plugin's DTG decorator uses
  `#3d85c6`, so the location decorator should take the teal and the two plugins
  will agree about what a coordinate looks like.
- **A cross-hemisphere test corpus**: `server/demo/daily_ato_content.go` carries
  real `LATM` values in all four quadrants (`2115N15745W`, `2621N12746E`,
  `1335N14455E`, `6412N16830W`, `3530N12500E`), and
  `customer_ato_content.go:166` has `LATS:300105N0803428W`. Cheaper and more
  realistic than inventing vectors.

### Not present, and therefore not a requirement to inherit

No Cursor on Target handling exists in that repo. Bullseye calls appear only as
demo chatter prose (`server/demo/mission_chatter.go:479`, "bullseye 340/45"),
never parsed.

---

## Phase Strategy

| Phase | Focus | Value |
|-------|-------|-------|
| **Phase 1** (this plan) | DD signed, DD directional, DMS, DDM, **and the USMTF tagged family** (`LATD`, `LATM`, `LATS`, `LATDS`, `DMPID`, `VLATM`, `VLATS`). Framework changes, panel, page, switches, docs, examples, `check` subcommand. **No projection code.** | A complete location decorator with zero ellipsoid maths, and interoperability with the ATO traffic the sibling plugin already parses |
| **Phase 2** (built) | Hand-written geodesy, MGRS (including USMTF `UTMO` / `UTMT`), UTM, the conversion endpoint, MGRS as the panel's lead row | The tactical notations, and the first conversion aocanywhere never had |
| **Phase 3** (built) | GARS, GEOREF, Plus Codes, and USNG as a moniker rather than a format | The remaining area-reference systems. **USNG did not become a grammar**: it is MGRS on WGS 84 character for character, so a separate id would be a second grammar accepting the first one's tokens and the format on the page would become a guess. GEOREF and GARS are label-only, on the same argument that keeps `LATD` behind a label; Plus Codes are bare in upper case only, on a measurement, since a lower-case run of that shape is what a version string with build metadata looks like and about one in 50,000 of those is a valid code |
| **Phase 4** | Enrichment providers behind an interface: map link (`LocationMapURLTemplate`), timezone, country, terrain, distance and bearing | Context beyond conversion |
| **Phase 5** | CoT events; `BAR` bearing and range against a channel or mission reference point | Structured events and relative positions, both of which need workspace state and therefore cannot use the public `/decorate` route |

**ICAO airfield codes are out of scope for this decorator entirely**, at every
phase. `KJFK` is a place identifier, not a coordinate: it names a facility whose
position must be looked up rather than computed, which makes it a different kind
of thing from every grammar here. Resolving one needs a bundled dataset that
must be kept current, and a token whose meaning can change between releases is
not something to write permanently into a message. It would also be the first
thing in this decorator that cannot be validated by round-tripping its own text.
If airfield identifiers are wanted later they belong in a decorator of their own,
with its own dataset, its own staleness story and its own switch. The USMTF
prefixes that introduce them (`ICAO:`, `LOC:`, `DEPLOC:`, `ARRLOC:`) are
therefore permanently excluded from this decorator's moniker list rather than
reserved.

Phase 1 stops exactly where the ellipsoid starts. DD, DMS, DDM and every USMTF
tagged form are multiply-by-sixty arithmetic; only MGRS and UTM touch a
projection. Splitting there means the panel, the page, the tagger integration,
the admin switches, the docs and the examples all land and are proven before the
single highest-risk item in the whole feature (hand-written transverse Mercator,
Norway and Svalbard zone exceptions, polar UPS gaps) is written.

Adding the USMTF family to Phase 1 costs almost nothing, because
`LATD` / `LATM` / `LATS` / `LATDS` are the same degrees, minutes and seconds
this phase already parses, written without separators. It buys the thing that
makes this decorator useful on day one: an operator pasting a line out of an ATO
gets the same panel as an operator typing decimal degrees.

**The cost, named rather than absorbed:** the Phase 1 panel leads with latitude
and longitude, not MGRS, and the lead row changes in Phase 2. MGRS is a
Critical-priority format for this audience and it is not in the first release.

---

## Design Principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Format proliferation | **One decorator, many grammars.** All formats normalize to one `Location`. | A decorator per notation | DTG claims both military and RFC 3339 for the same reason |
| What the URL carries | **`(f, v)` and nothing else.** The format id and the canonical token are the location's whole identity. | Carrying derived values that can disagree with the token | `dtg.validateParams`, `server/decorators/dtg/dtg.go:269` |
| Where grammar lives | Go only. The webapp reads a fixed-width canonical string, never a grammar. | Regexes in TypeScript | `webapp/src/decorators/types.ts` |
| Where geodesy lives | Go only, Phase 2 onward. | A second projection in TypeScript | Coding conventions |
| False positives | **Decline by default.** A grammar ships only when a false positive is implausible. | A permissive regex plus range checks | Epoch seconds, 12-hour clocks and basic-ISO are declined for exactly this reason |
| Panel robustness | Every row the panel can compute from `v` alone, it computes locally and forever. | Rows that go blank when a fetch fails | "Nothing may fail the panel" |
| Precision honesty | Never render more resolution than the token carried, and never store more than it carried either. | Six decimals in the URL for a 10 km square | New |

## Reference Patterns

- `server/decorators/dtg/dtg.go:199-206` and `:293-321` are the model this plan
  copies exactly: an **anchored** canonical regex that bounds what the parameter
  can contain, then a re-parse that bounds what it can mean, then a field-by-field
  reproduction check.
- `server/decorators/dtg/dtg.go:86-97` on compiling patterns once and on the
  submatch-1 trap.
- `server/decorators/dtg/dtg.go:150-166` (`monikerFor`) for composing a label
  over only the grammars that are themselves on.
- `webapp/src/decorators/dtg/index.ts:81-155` for what `fromParams` can and
  cannot check.
- `webapp/src/decorators/dtg/DtgPanel.tsx:159-172` for the reset-on-payload-change
  trap.

---

## Technical Approach

### 1. The identity of a location is `(f, v)`

```go
// server/decorators/location/location.go

// Axis is one half of a coordinate, held as the components the author wrote
// rather than as a number.
//
// This is deliberately textual. An earlier draft held Lat and Lon as float64
// and reconstructed the canonical form from them, which cannot round-trip a
// sexagesimal token: 340322N parses to 34.05611111..., and recovering whole
// seconds from that float can land on 21.99999999, so canonical(parse(v))
// yields 340321N and the link this plugin just wrote into a message renders a
// 400. Integers in, integers out.
type Axis struct {
    Deg  int    // 0-90 for latitude, 0-180 for longitude
    Min  int    // 0-59, unused by dd/ddh/latd
    Sec  int    // 0-59, unused by dd/ddh/latd/latm/ddm
    Frac string // the fractional digits exactly as written, "" if none
    Hemi byte   // 'N' 'S' 'E' 'W', or 0 for signed dd
    Conf int8   // USMTF verified confidence digit, -1 when absent
}

type Location struct {
    Lat, Lon Axis
    Format   Format // dd, ddh, dms, ddm, latd, latm, vlatm  (mgrs, utm Phase 2)
}

// Decimal is derived, never stored. It is what the panel and the page render
// and what Phase 2 feeds to a projection. Nothing in the round trip may read
// it.
func (a Axis) Decimal() float64
```

`Frac` is a **string**, not a count, because it is what `canonical()` writes back
out. `Digits` (section 3) is `len(Frac)`, derived rather than stored, so the two
cannot disagree.

`Conf` is where the USMTF verified confidence digits live. They describe how
well the position is known rather than where it is, so they sit on the axis and
never touch `Decimal()`.

The query string carries **three parameters and no more**:

| Param | Meaning | Example |
|---|---|---|
| `f` | source format id, a closed enum | `dms` |
| `v` | the canonical token, ASCII, fixed shape per format | `340322N1181500W` |
| `r` | the author's own text for the same token, **optional** | `34°03'22"N 118°15'00"W` |

**`(f, v)` is the identity. `r` is display only and derives nothing.** Latitude,
longitude, resolution and every alternative notation come from `(f, v)` alone,
and no code path may read `r` for anything but rendering. That separation is the
single decision the rest of the plan hangs on, and it is what earlier drafts got
wrong. Carrying a derived `lat` and `lon` alongside `v` would mean:

- a crafted link could pair a real token with an unrelated point, and the panel,
  which cannot run a projection, would have no way to notice;
- a 10 km square would be stored in permanent post text as a six-decimal
  coordinate, a resolution uplift the author never wrote, which then travels
  into exports, the search index and every access log on the path;
- `34.0561,-118.2500`, `34.05610000,-118.25000000` and `+34.0561,-118.2500`
  would all reproduce the same derived pair, so three different strings would
  validate against one link.

None of that applies to `r`, because `r` is not derived from anything and
nothing is derived from it. What does apply is that it is text from a message,
echoed onto a **public** HTML page whose query string an attacker controls
freely, which is the surface the round-trip validation exists to close. It is
therefore constrained harder than anything else in the URL: see section 2. The
short version is that `r` must be a string one of this plugin's own scanning
patterns would have matched, **and it must normalize to `v`**. Escaping is then
defense in depth rather than the only defense.

`r` is **omitted entirely when it equals `v`**, which is the common case for the
USMTF compact family, where the author's text already is the canonical form. An
absent `r` reads as "the author wrote `v`".

`r` is the text of the **replaced span**, not of the whole pattern match. A
moniker is not consumed (section 4), so `LATM:2130N15730W` yields
`r=2130N15730W` and the label stays in the message.

Typical URLs:

```text
/plugins/<id>/decorate/location?f=latm&v=2130N15730W
/plugins/<id>/decorate/location?f=dms&r=34%C2%B003%2722%22N+118%C2%B015%2700%22W&v=340322N1181500W
```

Roughly 70 characters when `r` is absent and about 130 for the worst realistic
case, a DMS token carrying degree symbols, where each `°` costs six bytes
encoded. That is the same band as an existing DTG link, and the post-size guard
in `hooks.go` already covers the message that crosses the limit.

Datum is WGS84 and is not a parameter. Nothing accepts another datum, and a
token carrying one (`NAD27`) is declined rather than reinterpreted.

### 2. Canonical forms, and validating them

Every format declares an **anchored** canonical regex and a `canonical()` that
is held to an exact round trip, the way `dtg.canonical()` is:

| `f` | Canonical form | Anchored pattern |
|---|---|---|
| `dd` | `34.0561,-118.2500` | `^-?\d{1,2}(\.\d+)?,-?\d{1,3}(\.\d+)?$` plus range checks |
| `ddh` | `34.0561N,118.2500W` | `^\d{1,2}(\.\d+)?[NS],\d{1,3}(\.\d+)?[EW]$` |
| `dms` | `340322N1181500W` | `^\d{6}(\.\d+)?[NS]\d{7}(\.\d+)?[EW]$` |
| `ddm` | `3403.366N11815.000W` | `^\d{4}(\.\d+)?[NS]\d{5}(\.\d+)?[EW]$` |
| `latd` | `35N079W` | `^\d{2}[NS]\d{3}[EW]$` |
| `latm` | `3510N07901W` | `^\d{4}[NS]\d{5}[EW]$` |
| `vlatm` | `3510N9-07901W7` | `^\d{4}[NS]\d-\d{5}[EW]\d$` |

`dms` doubles as USMTF `LATS` and `ddm` is `LATM` with a fractional minute, so
those two formats need no new canonical form. `LATDS` (`331000.0N1183000.0W`)
and `DMPID` are `dms` with a fractional second, which the pattern above already
admits. `LATD`, `LATM` and `VLATM` are distinct shapes and get their own ids, so
the `AS WRITTEN` row can name the notation the author actually used, which is
the same reason `dd` and `ddh` are separate.

Canonical forms are **ASCII only**. Degree symbols, primes, smart quotes and the
`′ ″ ' " ´` family are an input-normalization table, never part of the output
alphabet. A test asserts `v` matches `^[\x20-\x7E]+$` for every accepted token,
which keeps the anchored patterns narrow. `r` is where the author's symbols
survive, under the separate and stricter rules below.

Canonical forms **preserve the author's decimal count**, because that count is
the resolution and there is nowhere else to carry it. `34.0561,-118.2500` and
`34.056100,-118.250000` are different tokens with different resolutions and
different links, which is correct.

`validateParams` mirrors `dtg.validateParams` layer for layer:

1. `f` must be a member of the closed enum. Never "try each grammar until one
   parses", or `f` is decorative and the format label on the page is
   independently spoofable.
2. `v` must match that format's anchored regex, **before** anything else touches
   it.
3. `parse(v)` must succeed, and `canonical(parsed)` must equal `v` byte for
   byte. This is what rejects `34.05610000,-118.25000000` as an alias.
4. Every derived value is then computed from `parsed`, not read from the URL.
5. `r`, if present, must survive the four checks below. If it does not, the
   **whole link is rejected**, not just the row. A link carrying an `r` this
   plugin would never have written is not one this plugin wrote, and rendering
   the rest of it as though it were would be the same "render them side by side
   as though they agreed" failure `dtg.validateParams` exists to prevent.

### 2a. Validating `r`

`r` is the one parameter that is text an author typed, echoed onto a public page
whose query string is attacker-supplied. Adding it was called out during review
as the obvious way to reintroduce exactly the class of hole the round trip
closes, and that warning is right about a naive `raw` parameter. It is answered
by never treating `r` as free text.

The load-bearing observation is that **`r` is not arbitrary**: by construction it
is a string one of this plugin's own **token sub-expressions** matched. So it can
be whitelisted as tightly as `v` is:

1. **Length.** At most 64 bytes. The longest legal token with symbols and spaces
   is well inside that, and a cap keeps the parameter from becoming a place to
   put things.
2. **Alphabet.** An explicit allowed rune set, and nothing else: ASCII digits,
   `NSEWnsew`, `.`, `,`, `-`, `+`, `'`, `"`, space, and the four typographic
   variants the normalizer accepts (`°`, `′`, `″`, `´`). No `<`, no `&`, no
   control characters, no other non-ASCII. This is a whitelist, never a
   blacklist.
3. **Grammar.** `r` must **anchored-match the token sub-expression for `f`**.
   This is what makes content spoofing structurally impossible rather than
   merely escaped: `PRIORITY TARGET 34.0561 N, 118.2500 W CONFIRMED` is not an
   anchored match for any grammar, so it can never reach the page.

   **The sub-expression, not the scanning pattern.** An earlier draft said
   "the scanning pattern for `f`", which `r` can never match: a scanning pattern
   for a moniker is `PREFIX:(TOKEN)` while `r` is the replaced span, so
   `LATM:2130N15730W` yields `r=2130N15730W` and gate 3 would reject every
   legitimate link. Both the bare and the moniker patterns are built from the
   same token sub-expressions (section 4), which is exactly the property that
   makes anchoring on the sub-expression well defined. Anchor `r` there.
4. **Round trip.** `canonical(parse(r))` must equal `v`. `r` may therefore differ
   from `v` only in the ways the normalizer is allowed to erase: case,
   separators, symbols, and spacing. It can never name a different place.

Escaping on output remains, as the fourth layer rather than the first. The page
already escapes everything it interpolates, and the panel renders `r` as a React
text node, never through `dangerouslySetInnerHTML`.

Two consequences worth stating plainly:

- Check 4 means `r`'s digit counts match `v`'s by construction, since
  `canonical()` preserves decimal count. So `r` cannot display a resolution that
  disagrees with the `RESOLUTION` row.
- The webapp performs checks 1 and 2 only. It cannot do 3 or 4 without a grammar
  in TypeScript, which the Design Principles forbid. That is the same honest
  limit `fromParams` already has, and the acceptance criteria already name the
  page as the round-trip-validated surface. A length-capped, alphabet-restricted
  string rendered as a text node is safe to display without it.

### 3. Resolution

`Digits` is a **digit count**, never a meter count. Meters are rendered from it
at display time.

- `dd` / `ddh`: fractional digits on the latitude. When the two halves differ,
  the **smaller** count wins, because the pair is only as good as its coarser
  half, and a test pins that tie rule.

  **Correction, found in review.** An earlier version of this section said
  canonicalization writes *both* halves at the winning count. It was
  implemented that way and it is wrong: it silently moves the finer half.
  `34.5N,118.2500W` canonicalized to `34.5N,118.2W`, putting the stored
  longitude 5.5 km from the one the author wrote, and a pair such as
  `0.0000,0.00001` collapsed onto Null Island, which the parser then declined,
  producing a permanently dead link. **Each half keeps its own digits in the
  canonical form**; the coarser count governs rendering only.

Resolution is a function of the format as well as the digits, so every format
has an entry and none is left to inference:

| `f` | Base resolution | `Digits` is |
|---|---|---|
| `dd`, `ddh` | 1 degree | `len(Frac)` on the coarser half |
| `latd` | 1 degree | always 0; the grammar has no fraction |
| `latm`, `vlatm` | 1 minute, about 1.8 km | always 0 |
| `ddm` (incl. `GEOK`) | 1 minute | `len(Frac)` of the minutes |
| `dms` (incl. `LATS`) | 1 second, about 31 m | always 0 |
| `dms` fractional (`LATDS`, `DMPID`, `VGEOT`) | 1 second | `len(Frac)` of the seconds |

A meter count cannot work: six-decimal DD is about 0.1 m, which has no integer
meter representation, and `p=0` would be indistinguishable from an absent
parameter. A digit count is also a pure function of the token's text, so it
round-trips exactly and the webapp can derive it by counting characters, which
is not grammar and not geodesy.

**Accepted fractional digits are capped at 8.** Beyond that a `float64` stops
being able to reproduce the decimal string it came from, and `Decimal()` is what
every rendered row is built on. Eight decimal degrees is under a millimeter, so
the cap costs nothing real and removes a whole class of round-trip failure. A
token with more digits is declined rather than truncated, since truncating would
silently change what the author wrote.

**Confidence is not resolution.** `Axis.Conf` carries the USMTF verified digit
and is displayed beside the resolution, never folded into it: `3510N9-07901W7`
resolves to a minute like any other `latm`, and separately claims a stated
confidence. Two different facts, two different phrases on the row.

The panel labels this row **RESOLUTION**, not "precision", and phrases it as
"about 11 m". Precision invites reading as accuracy: a phone GPS with
5 m real accuracy emitting six decimals would otherwise render "0.1 m".

### 4. Grammars, and what each one refuses

**Decimal degrees, signed** (`f=dd`), switch `EnableLocationDDSigned`:

```text
34.0561, -118.2500        -33.8688,151.2093
```

The most dangerous grammar in the plan, and the rules are correspondingly
strict:

- comma separator required; a space between two decimals is not evidence of
  anything;
- **both** values must carry at least **four fractional digits**. Four decimals
  is about 11 m, which is what real coordinate sources emit; version pairs,
  ratios and tolerances essentially never do;
- both in range, and `0.0000, 0.0000` declined outright as Null Island;
- **not adjacent to another number.** `0.1234, 0.5678, 0.9012` is a numeric list,
  and without this rule the leftmost-first engine pairs values that were never a
  coordinate. See the boundary guards below.

The cost is real and is documented rather than hidden: `34.05, -118.25` is
declined.

**Decimal degrees, directional** (`f=ddh`), switch `EnableLocationLatLon`:

```text
34.0561 N, 118.2500 W     34.0561°N 118.2500°W     N34.0561 W118.2500
```

Hemisphere letters make the intent explicit, so no four-digit rule, but **a
decimal point is still required on both values**. `N`, `S`, `E` and `W` are also
the symbols for newton, siemens, east and watt, so `Load 12 N, 5 W` and
`Torque 34 N, 118 W` would otherwise be permanently rewritten, which is exactly
the collision class used to decline GEOREF. Degrees are never signed here; a
sign plus a letter is contradictory and is declined rather than reconciled.

**Degrees minutes seconds** (`f=dms`), same switch:

```text
34°03'22"N 118°15'00"W    34° 03' 22" N, 118° 15' 00" W    34 03 22 N 118 15 00 W
```

Hemisphere letters are **required in every variant**, including the ones
carrying symbols. Without them the symbol-less form is six bare integers.
Degrees 0-90 and 0-180, minutes 0-59, seconds 0-59.999. Seconds may carry a
fraction; minutes may not, since that is DDM.

**Degrees decimal minutes** (`f=ddm`), same switch:

```text
34°03.366'N 118°15.000'W      3403.366N 11815.000W
```

Same hemisphere requirement. Minutes 0-59.999, exactly one decimal group, no
seconds. The compact form is fixed width (`DDMM.mmm` / `DDDMM.mmm`) and the
widths are part of the pattern, because that is the only thing separating it
from DD directional. Relying on "the DD reading happens to be out of range" is
an accident of the examples, not a rule.

**USMTF compact family** (`f=latd|latm|vlatm`, with `dms` and `ddm` covering the
rest), switch `EnableLocationUSMTF`:

```text
35N079W          3510N07901W          400948N1221400W
331000.0N1183000.0W                   3510N9-07901W7
```

These are the shapes `mattermost-plugin-aocanywhere` parses out of ATO and USMTF
2004 traffic, and an operator working from those messages will paste them into a
channel unchanged. Hemisphere letters are structural here rather than optional,
and the field widths are fixed, which is what makes the longer forms safe to
detect bare.

**Bare detection is allowed only from `LATM` upward.** `35N079W` (`LATD`) is
seven characters, two of them letters, resolving to a 111 km square. It is
available **only** behind a USMTF prefix. This is the one place the plan follows
the sibling plugin's tag-gated posture exactly rather than diverging from it.

The **verified** forms (`3510N9-07901W7` and its degree, second and decisecond
siblings) carry a confidence digit after each hemisphere letter. Those digits
are parsed and kept **out of the position**: they describe how well the position
is known, not where it is, so the panel surfaces them on the `RESOLUTION` row
rather than silently discarding them. `location.go:59` matches them and drops
them on the floor; that is the one thing the "V" is for, and this is the only
Phase 1 token that carries a stated accuracy.

**Moniker** (`EnableLocationMoniker`): the USMTF prefixes, taken from
`location.go:72-94` rather than invented, in front of a token whose own grammar
is enabled:

```text
LATD:   LATM:   LATS:   LATDS:   GEOK:   DMPID:   DEPLATM:   ARRLATM:
VLATD:  VLATM:  VLATS:  VGEOT:
(Phase 2)  UTMO:   UTMT:   DEPUTMO:   ARRUTMO:   MGRS:
```

`GEOK:` is USMTF's name for degrees and thousandths of a minute
(`GEOK:3510.234N07901.123W`), which is the same shape this plan calls `ddm`, so
it costs nothing to accept. The `V` forms are the verified variants, which exist
at every resolution and not only at `VLATM` as an earlier reading of
`location.go` suggested.

It composes as `EnableDTGMoniker` does: a moniker with nothing left to label is
dropped, and a moniker vouches for nothing.

Three labels are deliberately **not** monikers:

- **`LOC:`, `DEPLOC:`, `ARRLOC:`, `ICAO:`** mean an **ICAO airfield code** in
  USMTF, not a lat/lon pair. An earlier draft of this plan invented `LOC:` for
  coordinates, which would have put this plugin in direct conflict with both the
  standard and the sibling plugin. Airfield codes are out of scope for this
  decorator at every phase, so these are permanently excluded rather than
  reserved. Reusing one for a coordinate later would be a mistake at any point
  in the future, not just today.
- **`NAME:`, `DEPNAME:`, `ARRNAME:`** introduce free text (`NAME:PT ALFA`).
  There is nothing to resolve.
- **`GRID:`** is generic. A label that could introduce MGRS, UTM, USNG or GARS
  would vouch for whichever grammar happened to be on, which is the opposite of
  composition.

Unlike `DTG:`, a location moniker is **not consumed**. `LATM:` and `DMPID:` are
part of a structured line the author may be quoting verbatim, and deleting one
changes the record; `DTG:` is merely redundant with the token it introduces.
This is possible because of the framework change in section 6, and it means a
decorated USMTF line still round-trips as USMTF.

**Deliberately absent from Phase 1**, and this list is the design:

| Not matched | Reason |
|---|---|
| `34.05, -118.25` | Fewer than four decimals, and no hemisphere letter |
| `34.0561 -118.2500` | No comma; a space is not a delimiter |
| `12, 34` | Integers |
| `12 N, 5 W` | No decimal point; newtons and watts |
| `0.1234, 0.5678, 0.9012` | Part of a numeric list |
| `34,0561, -118,2500` | Comma as a decimal separator is ambiguous with the pair separator. Declined, never guessed |
| `1723385400.1234, 1723385400.5678` | Out of latitude range. Timestamps, kept as an explicit test row |
| `11S LT 12345 67890` (MGRS) | Phase 2 |
| `11S 385000 3769000` (UTM) | Phase 2 |
| `11S 385000 3769000` with band `N` or `S` | Phase 2, and then still declined. `11S` means "band S" to a military tool and "zone 11, southern hemisphere" to a civilian one, and the two readings differ by thousands of kilometers. Every other band letter is unambiguous and accepted |
| `35N079W` bare (USMTF `LATD`) | Seven characters, a 111 km square. Prefix only: `LATD:35N079W` |
| `32WDL123123` (USMTF `UTMO`, MGRS 100 m) | Phase 2, with the letter-set validation aocanywhere's `^\d{1,2}[A-Z]{3}\d{6}$` does not do |
| `006AG39` (GARS) | Phase 3, moniker-gated. Seven alphanumerics with no checkable structure |
| `AABB1535` / `GJPJ3718` (GEOREF) | Phase 3, moniker-gated. `location.go:53` accepts any `[A-Z]{4}\d{4}`, which is safe against a single USMTF field and not against free text. Real GEOREF has a constrained alphabet and Phase 3 validates it |
| `849VCWC8+R9` (Plus Code) | Phase 3 |
| `CWC8+R9` (short Plus Code) | Needs a reference location this plugin does not have. Stays declined |
| `KJFK`, `ICAO:KJFK`, `LOC:EGLL` | **Out of scope at every phase.** An airfield code names a facility whose position must be looked up rather than computed, so it is not a coordinate and cannot be validated by round-tripping its own text. Bare, it is also indefensible: aocanywhere's `^[A-Z0-9]{3,4}$` matches most acronyms in the language and is safe there only because it is applied to a known field |
| `BAR:330-PT ALFA-50`, `BEARING:330T-PT ALFA-50NM`, `bullseye 350/40` | A bearing and range from a reference point that is not in the message. Resolving it needs per-channel or per-mission state, which the public `/decorate` route may not read. Phase 5, and it needs its own route. Note the two USMTF definitions of `BAR` disagree; trust `amsnloc.html`, not `location_test.go:151` |
| `CIR:50NM`, `RAD:1NM`, `ELL:105M-200M-240.0`, `SECTO:180240T27NM` | Area descriptors that qualify a nearby point rather than being one. They need an area representation this plan does not have, and a linear-unit table that exists nowhere in either repo. Phase 3, with GARS, which raises the same question |
| `AB1535` (abbreviated Georef) | Phase 3 at the earliest, and prefix-only. Two letters and four digits is not a token |
| `NAME:PT ALFA`, `Report Point Alpha` | A place name, not a position |
| Polar MGRS (`ZAH...`, no numeric zone) | UPS, not UTM. Not implemented; explicit row so a polar user learns why |
| Any datum-bearing token (`... NAD27`, USMTF `NAS-C`) | NAD 27 and WGS 84 disagree by up to about 200 m in CONUS, which is several MGRS cells. The USMTF datum vocabulary is real (`geography.go:295-471`, roughly 200 codes), so this is declined rather than reinterpreted as WGS 84 |

### 5. Boundary guards

`\b` is the wrong guard here and getting it wrong is message corruption.

Every DTG token starts and ends with `[0-9A-Za-z]`, which is why `\b` works at
`dtg.go:100`. Location tokens do not: they begin with `-`, `+`, `°` or a letter
and end with `"`, `'`, `°` or a digit. `\b` before `-118` demands a word
character immediately before it, asserting the opposite of what is wanted, and
`\b` after `2500` is satisfied by a following `.`, `-` or `,`, so
`-118.2500..-118.2600` would have a link written into the middle of a range.

**Guards must not be part of the regex.** An earlier draft had each grammar
consume a guard character on each side and link only the token inside it. That
does not work, and the reason is worth writing down because it is not obvious:
`findCandidates` iterates with `FindAllStringSubmatchIndex`, which returns
successive **non-overlapping** matches. Given

```text
34.0561,-118.2500 34.0562,-118.2501
```

the first match consumes the space as its trailing guard, the scan resumes at
`34.0562`, and the second pattern finds no leading guard left to match. **The
second coordinate is silently not decorated.** Two grids on one line is the most
ordinary input this feature has.

So the guard moves out of the pattern and into the framework, as
`Pattern.Boundary` (section 6b). The regex matches the bare token; the framework
looks at the runes on either side and drops the candidate if they are wrong.
Nothing is consumed, so the failure above cannot arise, and the match range and
the token range are the same thing, which leaves overlap resolution untouched.

The boundary classes, stated rather than left to guess:

| Side | Rejected when the adjacent rune is | Accepted otherwise, including |
|---|---|---|
| Before | a digit, `.`, `,`, `-`, `+`, `/`, `:`, or a letter | start of message, any whitespace, `(`, `[`, `"`, `'` |
| After | a digit, `.`, `,`, `-`, `+`, `/`, `:`, or a letter | end of message, any whitespace, `)`, `]`, `"`, `'`, `;`, `!`, `?` |

`,` and `.` are rejected on the trailing side even though a coordinate ending a
sentence is common, because `-118.2500.` and `-118.2500,` are indistinguishable
from the middle of a range or a list at the point the guard runs. The cost is a
missed decoration on a coordinate that ends a sentence with no space; the
alternative is a link written into the middle of `-118.2500..-118.2600`. Missing
a decoration is a feature gap and rewriting a range is corruption, so the trade
goes this way. Whitespace is tested with `unicode.IsSpace`, not `== ' '`, so a
non-breaking space or a tab behaves like a space.

A regression test per grammar covers: a token embedded in a longer numeric run
on each side; a token adjacent to a comma-separated list; **two coordinates
separated by a single space**, which is the case the consumed-guard design got
wrong; and a coordinate at the start and at the end of a message.

One property inherited from the existing framework and worth stating: a
candidate rejected by `Boundary` or by `Parse` does not claim its range, but the
regex scan has already advanced past it, so the **same** pattern will not find a
shorter match inside that span. A different pattern still can. This is
pre-existing behavior (`tagger.go:418`), not something this change introduces.

### 6. Framework changes, named rather than absorbed

Five, and none of them is optional:

**a. `Pattern.ReplaceGroup int`** (`server/decorators/decorator.go`,
`server/decorators/tagger.go`). Today `applyReplacements` substitutes
`message[c.start:c.end]`, the whole match, which is what consumes the `DTG:`
moniker. Location monikers must **not** be consumed (section 4), so add an
optional submatch index whose span is what gets replaced. `0`, the zero value,
keeps today's behavior exactly, so DTG is untouched.

`candidate` carries two ranges. The **match** range is what
`overlapsAny(protected)` tests, so a moniker inside a code span still protects
the token behind it. The **replace** range is what `resolveOverlaps` claims and
what `applyReplacements` substitutes. Roughly ten lines, plus tests.

This does **not** solve boundary guards, and an earlier draft claimed it did.
`ReplaceGroup` fixes what gets substituted; it does nothing about what gets
found, and consuming a guard breaks discovery for the next token (section 5).
The two are separate problems and get separate fixes.

**b. `Pattern.Boundary func(before, after rune) bool`**
(`server/decorators/decorator.go`, `server/decorators/tagger.go`). Nil, the zero
value, means no constraint, so DTG is again untouched. `findCandidates` decodes
the rune immediately before the match start and immediately after the match end,
passing `0` at the message edges, and skips the candidate when `Boundary`
returns false. The check runs **before** `Parse`, since it is cheaper and since a
candidate failing it is not a token at all.

This is what makes guards work without consuming anything, and it is why
`\b` can be left alone for DTG while location gets an exact contract.

**c. `Page.StyleCSS string` and `Page.Script bool`**
(`server/decorators/page.go`). The shared stylesheet declares `.countdown`,
`.dtg` and a keyframes pulse; a location page reusing a class named `.dtg` for a
coordinate is not something to leave in the tree. And the shell grants
`script-src 'unsafe-inline'` to every page because DTG carries a countdown
script. The location page carries none, so a single escaping miss there is
script execution where it could be inert markup. `Script: false` emits
`script-src 'none'`.

**`Script` is false at its zero value, which is a trap.** Adding the field
without touching `dtg/page.go` silently demotes the DTG page to
`script-src 'none'` and kills its countdown, which no existing test would catch
because the countdown is client-side. `dtg.RenderPage` must set `Script: true`
**in the same commit**, and a test must assert the DTG page still emits
`'unsafe-inline'` while the location page emits `'none'`.

**d. The "decoration is off" footer in `command_examples.go:126`** is keyed on
`p.dtgFormats()`. With DTG off and location on it would claim decoration is off
entirely. It becomes a registry-wide question: no decorator contributes any
pattern.

**e. `webapp/src/components/rhs/RhsView.tsx:27`** names date-time groups in the
empty state, which is the only way back out of a panel and the surface a
reporter is asked to quote.

The honest version of the framework's claim, after this, is narrower and still
worth having: **nothing in `server/decorators/*.go` or
`webapp/src/decorators/*.ts` changes for a decorator that needs no page styling
of its own and no boundary guards.** Location needs both.

### 7. The panel

Phase 1 renders **entirely from `v`**, with no network call at all:

```text
LOCATION

LAT / LON            34.0561° N, 118.2500° W
DMS                  34°03'22"N 118°15'00"W
DDM                  34°03.366'N 118°15.000'W

RESOLUTION           about 11 m
DATUM                WGS84
ORIGINAL TEXT        34°03'22"N 118°15'00"W
NORMALIZED           340322N1181500W

[Copy lat/lon]  [Copy original text]            Documentation
```

- Deriving latitude and longitude from a canonical DMS string is slicing fixed
  fields and dividing by sixty. That is reading a canonical form, which
  `dtg/index.ts:140` already does with `canonical[6]`, not duplicating a
  grammar. No regex, no projection.
- Every row is rendered to `Digits`. A token written to two decimals renders
  `34.06° N`, never six decimals of confidence it never had. For DMS and DDM
  that means dropping the field the token did not carry, not padding it with
  zeroes: a two-decimal DD renders `34°03'N`, because `34°03'22"N` would be a
  claim.
- **`ORIGINAL TEXT` shows `r`**, the author's own text, and names the format. This
  is what `r` exists for. An earlier draft showed only the canonical form and
  called the row `SOURCE`, which was honest but weak: a reader comparing a grid
  against a printed order needs to see the characters that were typed, and on
  the standalone page, which is what mobile clients get, the link label is not
  on screen at all, so without `r` the author's spelling is simply gone.
- **`NORMALIZED` shows `v`, and only when it differs from `r`.** For the whole
  USMTF compact family the author's text already is the canonical form, so the
  row collapses and the panel is one line shorter. When the two do differ, both
  are shown rather than one being silently substituted for the other: they carry
  different information, and which one to paste into another system depends on
  the system.
- **`ORIGINAL TEXT` is the row a reader copies most**, so it gets the second copy
  button. `navigator.clipboard` is undefined on non-secure origins, which for
  on-prem and air-gapped Mattermost over plain HTTP is the deployment norm
  rather than an edge case, so the buttons are hidden when it is unavailable and
  every value stays selectable either way. "Copied" is announced through
  `role="status"`, following `Customize.tsx:319`, and its timer is cleared on
  unmount and on a payload change.
- `r` is rendered as a React text node. Nothing on this panel uses
  `dangerouslySetInnerHTML`, and a test asserts it, because `r` is the first
  value in this plugin that originated as message text.

**No `Hover` component in Phase 1.** The framework makes it optional, and the
one thing a location hover could show that the link label does not is the MGRS,
which does not exist until Phase 2. A hover restating the token the reader is
already pointing at fails the bar `CLAUDE.md` sets.

### 8. The standalone page

Same rows, same rendering, computed inline in `RenderPage`, which stays a pure
function of its query string.

The page is Go and the panel is TypeScript, so they cannot literally share a
render function and an earlier draft was wrong to say they do. What keeps them
from drifting is a **shared fixture table**: a checked-in list of `(f, v, r)`
inputs and the exact strings every row should render, asserted by a Go test and
a TypeScript test against the same file. That is a weaker guarantee than shared
code and it is the honest one; state it rather than implying compilation-level
safety that does not exist.

The page is where `r` earns its place. A mobile client following a decorator
link gets this page and **not** the message it came from, so before `r` existed
the author's own spelling was unreachable there. It is also the surface where
`r` is fully validated, since all four checks in section 2a run in Go, and it is
the one that escapes on output.

The page and the panel agree completely in Phase 1, because neither fetches
anything. That is a property worth noticing: it exists only because the identity
in the URL is sufficient.

### 9. `/tactical-fusion check <text>`

A new subcommand, ephemeral, that runs the real tagger over the supplied text
and reports what was decorated, what was declined, and **why**.

This exists because the single largest usability gap in the feature is invisible
by construction: `34.0561, -118.2500` links and `34.0561, -118.25` does not, and
nothing about the non-event points the author at the rule. Documentation alone
carried DTG because DTG has three memorable shapes; location has four grammars
in Phase 1 and its most-pasted form on earth is declined by a digit-count rule.

The alternative considered and rejected for Phase 1 was an ephemeral hint posted
automatically on a near-miss. It is a better answer for the author who does not
know to ask, but it needs a second hook (`MessageHasBeenPosted`), a
deliberately more permissive near-miss grammar, and a rate-limiting story.
Recorded as a Phase 2 candidate rather than dropped.

### 10. Admin settings

Five switches in Phase 1, six from Phase 2, **all defaulting to true**:

| Key | Covers | Default |
|---|---|---|
| `EnableLocation` | parent | true |
| `EnableLocationDDSigned` | `34.0561, -118.2500` | true |
| `EnableLocationLatLon` | DD directional, DMS, DDM | true |
| `EnableLocationUSMTF` | `LATD`, `LATM`, `LATS`, `LATDS`, `DMPID`, `VLATM`, `VLATS` | true |
| `EnableLocationMoniker` | the USMTF prefixes | true |
| `EnableLocationGrid` (Phase 2) | MGRS, UTM, `UTMO`, `UTMT` | true |

**Shipped as two switches, not one.** `EnableLocationGrid` is MGRS alone,
keeping the name so an install that had it on keeps MGRS on across the split,
and `EnableLocationUTM` is separate and defaults **off**. They were one switch
until the band letter was read as a band rather than refused, at which point UTM
became the only grammar here that can decorate a real coordinate and point at
the wrong place. An install that wants grid references without that has to be
able to have exactly that.

Signed DD gets its own switch because the plan's own risk table calls it the
weakest grammar, so a workspace bitten by it must be able to kill exactly that.
The other three lat/lon grammars share one because they all require hemisphere
letters and no admin will want DMS off and DDM on.

The USMTF family gets its own switch for a different reason: a workspace that
never sees ATO traffic has no use for compact separator-free coordinates and
should be able to remove that whole class of match, while a workspace that lives
in ATO traffic may well want **only** that class. Turning off everything except
`EnableLocationUSMTF` and `EnableLocationMoniker` reproduces the
`mattermost-plugin-aocanywhere` posture exactly: tagged locations only, nothing
bare. That is a supported configuration, and a test pins it.

This is DTG's structure (parent, one flag per grammar family, moniker), not one
flag per shape: `EnableDTGMilitary` already covers three distinct grammars.

Planned with nothing defaulting to off. **Shipped otherwise:** `EnableLocationUTM`
defaults off and is the only switch that does, because it is the one grammar
that can decorate a real coordinate and point at the wrong place. `MGRS` and
`UTM` are also separate switches rather than the single `EnableLocationGrid`
below. The test named here was replaced by `defaultsOff` in
`configuration_settings_test.go`, a named list with a reason per entry, which
checks both directions. An earlier draft proposed a
default-off switch for bare unspaced MGRS and cited a test that does not exist;
that grammar is cut instead, and the moniker (`MGRS: 11SLT1234567890`, Phase 2)
covers the workflow it existed for.

The moniker alternation is built from the enabled sub-expressions and memoised
on the enabled-format set, not enumerated as one precompiled pattern per
combination. `monikerFor` is a four-case switch over three DTG formats; over six
location grammars that shape is 64 patterns, and `Patterns()` runs for every
decorator on every message.

As with DTG, a switch governs **decoration only**. `RenderPage` never consults
configuration, so a link already in a message keeps working after its format is
switched off.

---

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| One decorator or four? | One, `location` | Downstream of parsing they are the same thing, the same argument that put RFC 3339 inside DTG |
| What does the URL carry? | `f`, `v`, and an optional display-only `r` | Nothing **derived** is carried, so nothing can disagree with the token, no resolution uplift is written into stored text, and aliasing on position is impossible |
| Carry the author's raw text? | **Yes, as `r`, whitelisted four ways** | The panel and especially the standalone page need it: mobile clients get the page without the message, so the author's spelling is otherwise unreachable. Review flagged a naive `raw` parameter as reopening the crafted-link hole, and that is correct, which is why `r` is length-capped, alphabet-restricted, required to anchored-match the token sub-expression for `f` (the scanning grammar can include a moniker, and `r` never does), and required to normalize to `v`. A bad `r` rejects the whole link, not just the row |
| Show `v` when it equals `r`? | No, and `r` is omitted from the URL in that case | The whole USMTF compact family types its own canonical form, so the row and the bytes both disappear for the most common input |
| Resolution as meters or digits? | Digit count | 0.1 m has no integer meter form, and a digit count is a pure function of the token text so it round-trips exactly |
| Does resolution govern the canonical form? | **No.** Resolution is the coarser half; the canonical form keeps each half's own digits | Conflating them moves the stored coordinate. Found in review after being specified and built the wrong way round |
| Geodesy in TypeScript? | No | Phase 1 needs none. Phase 2 puts MGRS and UTM behind a server endpoint |
| DD, DMS, DDM rows in TypeScript? | Yes, and **only** there | Slicing a fixed-width canonical form is not grammar. Computing them locally means those rows can never degrade, which is a better failure state than fetching them |
| Conversion endpoint in Phase 1? | No | There is nothing it could compute that the panel cannot. It arrives in Phase 2 with MGRS and UTM, authenticated, taking `f` and `v` and returning the derived point so the round trip happens once, in Go |
| Conversion cache? | No | A pure function of two parameters, costing microseconds, with no store behind it. The preferences cache exists because there is a KV round trip; that precedent does not transfer |
| Lead with MGRS always? | Phase 2 | Stated goal, but it cannot lead a phase that has no projection |
| Corner or center of a grid square? | Center, stated in the UI (Phase 2) | A 10 km square's corner and center are 7 km apart, and the panel's resolution framing reads as center-plus-tolerance |
| Moniker consumed? | No, unlike `DTG:` | `LATM:` and `DMPID:` are part of a structured line an author may be quoting verbatim; deleting one edits the record, and a decorated USMTF line should still read as USMTF |
| Moniker vocabulary? | The USMTF prefixes from `location.go:72-94`, not invented ones | `LOC:` already means an ICAO airfield code. Inventing a conflicting meaning would break interoperability with the sibling plugin and with the standard |
| **Bare signed DD on by default, when aocanywhere gates every location behind a tag?** | **Yes, but recorded as a knowing divergence** | `patterns.ts` has exactly one location pattern and it is `LATM:`-gated; the only bare pattern in that file is a fully qualified DTG. The counter-argument is that the four-decimal rule plus the numeric-list guard is far narrower than a naive coordinate regex, and that the per-grammar switches let a workspace retreat to the aocanywhere posture in one click. **Revisit this if the near-miss corpus in Task 4 turns up anything real** |
| USMTF family in Phase 1? | Yes | It is the same degrees, minutes and seconds this phase already parses, written without separators, so it is nearly free. It is also what makes the decorator useful on day one to the operators who already use the sibling plugin |
| Resolve ICAO airfield codes? | **No, at any phase** | A code names a facility whose position is looked up rather than computed, so it cannot be round-trip validated the way every other token here can, and it needs a dataset that goes stale. A token whose meaning can change between releases should not be written permanently into a message. If it is wanted, it is a separate decorator with its own dataset and switch |
| Map link default? | An empty `LocationMapURLTemplate`, unlike aocanywhere's Google Maps default | Same setting shape so operators configure both plugins alike, but an outbound request carrying a grid reference must be an admin's explicit choice for this audience. Note `RenderPage` may not read configuration, so the button is panel-only and the page asymmetry is documented |
| Display conventions | Match aocanywhere: `40°09'48.0"N 122°14'06.0"W`, decimals rounded to four places | Two plugins in the same channel should not render the same coordinate two ways |
| A version parameter in the URL? | No | The canonical form is frozen and pinned by exact round-trip tests, which is how DTG solved the same problem |
| GARS in Phase 1? | No, Phase 3 | Seven alphanumerics with no structural check |
| New geodesy dependency (Phase 2)? | **Decided in Phase 2: hand-written, no dependency** | The target is air-gapped installs behind an SBOM and a CVE gate, where a dependency is a permanent cost to the operator, against a published series that has not changed since 1987. The condition attached to that choice is that it is proven against figures with an authority outside this repo rather than against round trips of itself: the WGS 84 meridian quadrant, one degree of latitude at the equator, an exact easting on a central meridian, an exact northing on the equator, and the Norway and Svalbard zone exceptions. Measured error is under a millimeter inside a standard zone and 4 cm in the two hand-widened ones, against a finest square of 1 m |
| Bare unspaced MGRS? | **Yes, upper case and at least three digits per axis.** UTM unspaced stays label-only | Arrived at by being wrong twice, both times plausibly. The part-number argument that gated it behind a label was never tested and was wrong: part numbers did not collide, **short git SHAs did**, at one in 3,900 for an any-case pattern. Upper case fixed that, and the claim that it left nothing behind was also wrong: at two digits per axis, one uppercase run in 75,000 collided, and those ones really did look like part numbers (`26HMA1997`). Three digits per axis finds none in 1.2 million. UTM has no letters to validate after its band, so there is nothing there to narrow |
| Bare spaced UTM? | **Kept, with a zone and easting check, knowing it does not solve the problem** | Measured at 9.07% of "number+letter, 6 digits, 7 digits" text before the check and 6.74% after; `part 14J 622606 6968159 ordered` is still rewritten. The check is worth having on its own merits (it also stopped `31P 000000 1100000`, which named no square and reported an empty MGRS row), but the residual rate is a live risk and the one-line retreat is to move `utmSpacedExpr` out of `bareExprs` |
| UTM band `N` and `S`? | **Reversed during implementation: read as latitude bands** | Planned as declined outright, on the grounds that `11S` is band S to a military reader and "zone 11, southern hemisphere" to a civilian one with nothing in the token to choose between them. That declined the ordinary military spelling of a position in order to protect a civilian reading this audience does not write, and `11S 384640E 3769080N` is as plain a paste as this feature gets. Shipped reading the letter as a band, which is the MGRS convention, with `gridPoint`'s band containment and zone proximity checks as the guard: `N` cannot misplace a position at all, and `TestSouthernHemisphereTokensMostlyDecline` measures the residual on `S` at 10.1%. MGRS is unaffected either way: its band letter is followed by two more letters |
| How is a grid token validated? | **Geometrically.** `parseMGRS` asks `mgrsCenter` for a position, and that one call subsumes the letter sets, the row stagger, the band and the zone | A second implementation of the letter-set rule is a second chance to disagree with the one that writes references back out |
| Does the panel now depend on the network? | **Only for what it cannot compute** | Grid rows on a lat/lon link, the position on a grid link. Everything else is sliced out of the canonical token and is on screen before the request lands, so a failure costs rows rather than the panel |
| Does the conversion endpoint see `r`? | **Yes, and that was not optional** | Two of the four gates on the author's text need the token grammar, which is Go-only, and the alphabet gate had to widen to the whole Latin alphabet for grid letters. Without sending `r`, a hand-edited link could put prose in the panel's "Original text" row beside a position from an unrelated token while the page refused the same link. Sending it makes the panel ask the question the page asks |

---

## Files to Modify

| File | Change |
|---|---|
| `server/decorators/decorator.go` | `Pattern.ReplaceGroup` |
| `server/decorators/tagger.go` | Separate match and replace ranges in `candidate`, `findCandidates`, `resolveOverlaps`, `applyReplacements` |
| `server/decorators/page.go` | `Page.StyleCSS`, `Page.Script`, `script-src 'none'` when false |
| `server/decorators/location/location.go` | **New.** `Decorator`, `Type`, `Patterns`, `Parse`, `RenderPage`, `Formats`, `validateParams` |
| `server/decorators/location/parse.go` | **New.** Scanning patterns, input normalization, per-grammar parsing, anchored canonical patterns, `canonical()` |
| `server/decorators/location/format.go` | **New.** DD, DMS, DDM rendering at a digit count |
| `server/decorators/location/page.go` | **New.** Page body, sharing `format.go` with nothing else |
| `server/plugin.go` | One argument to `NewDefaultRegistry`; `locationFormats()` |
| `server/configuration.go` | Five fields |
| `server/command.go` | `check` subcommand |
| `server/command_examples.go` | Location sections; registry-wide "decoration is off" footer |
| `server/errcode/codes.go` | Codes at `17001+` in the existing `server/decorators/` range, and in `16xxx` for `check` |
| `plugin.json` | Five settings, each with `help_text` |
| `webapp/src/decorators/location/index.ts` | **New.** `Decorator<Location>`, `fromParams` |
| `webapp/src/decorators/location/LocationPanel.tsx` | **New.** |
| `webapp/src/decorators/location/format.ts` | **New.** Canonical-form slicing and rendering. No regex, no projection |
| `webapp/src/decorators/location/CopyButton.tsx` | **New.** |
| `webapp/src/decorators/index.ts` | One `register(...)` line |
| `webapp/src/components/rhs/RhsView.tsx` | Empty-state copy |
| `public/help/formats.html` | Location grammars, the declined table with reasons |
| `public/help/panel.html` | The location panel, the page, why SOURCE is normalized |
| `public/help/admin.html` | One section per switch, with `data-setting` |
| `public/help/commands.html` | `check` |
| `public/help/error-codes.html` | New rows |
| `public/help/troubleshooting.html` | "My coordinate was not linked", quoting the exact rules |
| `public/help/help.html` | Nav cards and the "what a decorator is" framing, both DTG-shaped today |
| `README.md` | Second decorator |
| `CLAUDE.md` | A Location section beside Decorators |

---

## Tasks

**Gating, before anything else**

0. [ ] **Verify Mattermost search still matches a token once the message
   contains `[34.0561, -118.2500](/plugins/...)`**, against a running server.
   `CLAUDE.md` already lists this as unverified for DTG. A grid reference is
   searched far more often than a timestamp, and an operator who cannot find a
   coordinate after this ships has lost a capability rather than missed an
   enhancement. **A negative answer invalidates server-side decoration for this
   decorator** and the plan stops here.

   **Pass means**: post `34.0561, -118.2500`, let it decorate, then search the
   channel for `34.0561` and for the full pair and get the post back. In other
   words the indexer must index the **label** of `[label](url)` and must not
   treat the brackets as part of the term. Run it against both the default
   (Bleve) and Elasticsearch if the target deployment uses one, since they
   tokenise differently. Record the answer in `CLAUDE.md` either way, since the
   DTG entry is still open.

**Framework**

1. [ ] `Pattern.Boundary`, checked in `findCandidates` before `Parse`, with the
   rune classes from section 5. Test two adjacent tokens separated by a single
   space, which is the case the consumed-guard design silently dropped.
2. [ ] `Pattern.ReplaceGroup`, split match and replace ranges. DTG unchanged at
   the zero value.
3. [ ] `Page.StyleCSS`, `Page.Script`, CSP test. **Set `Script: true` in
   `dtg/page.go` in this same commit**, and assert the DTG page still emits
   `'unsafe-inline'` while the location page emits `'none'`.

**Parsing**

4. [ ] `Axis`, `Location`, `Format`, `Decimal()`, and `canonical()` per format,
   held to an exact round trip. `canonical()` reassembles from the integer
   components and `Frac`, and **must never route through `Decimal()`**: a test
   asserts that `340322N1181500W` and every fractional-second sibling round-trip
   byte for byte, which is the case a float intermediate fails.
5. [ ] Token sub-expressions, scanning patterns and input normalization for DD
   signed, DD directional, DMS, DDM and the USMTF compact family, each pattern
   supplying its `Boundary`. Build the negative corpus here, and include the
   aocanywhere USMTF test vectors (`sets/location_test.go`) as positive cases so
   the two plugins agree about what a `LATM` is.
6. [ ] Anchored canonical patterns and `validateParams`, layered as
   `dtg.validateParams` is, including all four `r` checks (gate 3 anchoring on
   the token sub-expression, not the scanning pattern) and the rule that a bad
   `r` rejects the whole link.
7. [ ] `r` emission: the replaced-span text, omitted when it equals `v`, capped
   at 64 bytes, and a test that no accepted token produces an `r` outside the
   allowed rune set.
8. [ ] Moniker patterns, memoised on the enabled-format set, not consumed.
9. [ ] `format.go`: every row rendered at its format's resolution, from the
   table in section 3, with `Conf` shown separately from resolution.

**Integration**

10. [ ] Tagger tests: coordinates in prose, in every protected-span kind,
    adjacent to a DTG, several per message, at the post-size boundary.
11. [ ] `page.go`, plus the shared fixture table and the Go half of the test
    that reads it.
12. [ ] `configuration.go`, `plugin.json`, `locationFormats()`, and the two
    manifest/config agreement tests picking up the new keys.
13. [ ] `errcode` constants, `AllCodes`, `error-codes.html`.

**Webapp**

14. [ ] `fromParams`: shape check against the anchored canonical pattern and the
    `f` enum, plus gates 1 and 2 on `r`. It does **not** claim to re-derive
    anything, and the acceptance criteria say so.
15. [ ] `format.ts`, `LocationPanel`, `CopyButton`, and the TypeScript half of
    the shared fixture test.

**Surfaces**

16. [ ] `check` subcommand; location example groups; registry-wide footer.
17. [ ] Help docs, all seven pages, `README.md`, `CLAUDE.md`.
18. [ ] `make check-style && make test && make sbom-audit`.

---

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **A false positive permanently rewrites a message.** Silent, irreversible without a manual edit, and it lands in exports | Decline-by-default rules above; a regression test per row of the declined table; the `/tactical-fusion examples` declined group is generated by the real tagger, so a rule that loosens shows up as a changed example |
| **The sibling plugin concluded bare coordinate detection was not worth the risk**, and this plan disagrees for one grammar | Recorded as a Decision rather than buried. The strict rules, the per-grammar switches, and the fact that `EnableLocationUSMTF` plus `EnableLocationMoniker` alone reproduces the aocanywhere posture are the retreat path. Task 4's negative corpus is the evidence that decides it |
| **Two plugins in one channel disagree about the same coordinate** | Match aocanywhere's display conventions and four-decimal rounding, and reuse its USMTF test vectors as positive cases. Where behavior genuinely differs (this plugin converts MGRS, that one does not) the difference is additive rather than contradictory |
| **The boundary guards are the new corruption surface**, and `\b` cannot express them | `Pattern.Boundary` (section 6b) rather than guards inside the regex, with explicit rune classes and the adjacent-token case tested. This is the highest-risk code in Phase 1, and the consumed-guard design that preceded it silently dropped every second coordinate on a line, which no test in the first draft would have caught |
| **A float intermediate breaks the canonical round trip** for every sexagesimal format, turning an accepted token into a permanent 400 link | `Location` holds integer components and a `Frac` string per axis; `Decimal()` is derived and never feeds `canonical()`. Tested on `340322N1181500W` and its fractional siblings, which are exactly the tokens that fail otherwise |
| **A token accepted at decoration but rejected at render** becomes a permanent 400 link, editable only by hand | Property test over every grammar's accepted corpus: `RenderPage(Parse(tok))` is 200, and `canonical(parse(canonical(parse(tok)))) == canonical(parse(tok))`. Run it over the `examples` output too |
| Non-finite floats reaching stored post text | `Parse` returns `ok=false` on any non-finite intermediate, and `Location` construction asserts finite and in-range. Note `NaN` compares false against both bounds, so a naive range check passes it. Adversarial-input test asserting no link ever carries a non-numeric value |
| Author never learns why their coordinate was declined | `/tactical-fusion check`, the declined group in `examples`, and a troubleshooting section quoting the exact rules. The automatic near-miss hint is a named Phase 2 candidate |
| Search does not match decorated tokens | Task 0 gates the whole plan |
| Pattern count roughly triples on a code path that runs inside `MessageWillBePosted` for every post | `overlapsAny` is a linear scan and candidate count is patterns times matches. Add a benchmark over a worst-case 16,383-rune message (dense coordinates, dense brackets, an unterminated fence) with a hard CI time budget. A slow hook is a workspace-wide outage |
| Coordinates travel in URLs, so into access logs, proxy logs and browser history | Carrying `(f, v, r)` and nothing derived means nothing more precise than the author's own token ever leaves, and `r` is by construction a string that was already in the message. Make it an explicit invariant that no location log line carries `v`, `r` or message text, and check what the parsers put in panic values. A "what ends up where" section in the help docs |
| **`r` is the first parameter in this plugin that originated as message text**, echoed onto a public page | Four independent gates before it renders (length, rune whitelist, anchored grammar match, normalizes to `v`), any failure rejecting the whole link, plus output escaping on the page and text-node rendering in the panel. One test per gate. This was raised in review as the way to reopen the crafted-link hole, and the answer is that `r` is never treated as free text |
| Two decorators competing for a span | No grammar collision exists (DTG's `\d{6}Z` needs a literal `Z`; its long forms need six leading digits). Add the cross-decorator test anyway, and one asserting that a `Parse` returning false does not claim its range, since location is the first decorator whose own grammars can nest |
| Phase 2 geodesy is wrong in a way self-consistent round trips hide | Test against published external vectors, not round trips, including zone boundaries and the Norway and Svalbard exceptions, where naive implementations break |

---

## UX Summary

| Scenario | Behavior |
|---|---|
| Author posts `34.0561, -118.2500` | Linked, labeled with their own text. Panel shows lat/lon, DMS, DDM, resolution |
| Author posts `34°03'22"N 118°15'00"W` | Linked. `AS WRITTEN` reads back their exact text with the symbols they used, `NORMALIZED` reads `340322N1181500W` |
| Author posts `34°03′22″N 118°15′00″W` with smart quotes from a phone | Same, and `AS WRITTEN` keeps the smart quotes. The typographic variants are in the allowed rune set precisely so this does not become a support ticket |
| Author posts `400948N1221400W` | Linked. `AS WRITTEN` reads it back, and `NORMALIZED` is **absent**, because the author already typed the canonical form. `r` is not in the URL either |
| Reader on mobile follows the link | The page shows the same `AS WRITTEN` row. This is the surface that has no message text to fall back on, and the reason `r` exists |
| Author posts `34.05, -118.25` | **Not linked.** `/tactical-fusion check` names the rule |
| Author posts `34.05 N, 118.25 W` | Linked. The hemisphere letters buy the looser digit rule |
| Author pastes an ATO line, `LATM:2130N15730W` | The prefix stays visible and only the coordinate is linked, so the line still reads as USMTF |
| Author posts `3510N9-07901W7` | Linked. `RESOLUTION` shows both the 1.8 km grid resolution and the confidence digits the token carried |
| Author posts `35N079W` bare | **Not linked.** `LATD:35N079W` is |
| Author posts `BAR:180T-050NM` | Not linked. `check` says it needs a reference point the message does not carry |
| Author posts `LOC:KJFK` | Not linked, now or ever. `check` says airfield codes are not coordinates and this decorator does not handle them |
| Author posts a coordinate in a code fence or a link | Not linked |
| Reader clicks a link | Sidebar opens fully rendered. No loading state, no fetch, no failure mode |
| Reader on mobile, page has no copy button | Values stay selectable in a monospace block |
| Reader clicks a second coordinate | Panel re-renders from the new payload. Any transient "Copied" state is cleared on the payload key |
| Clipboard unavailable (plain-HTTP install) | Copy button hidden; values remain selectable |
| Coarse token, e.g. two decimals | Rows render `34.06° N` and `34°03'N`. No zero-padded seconds |
| Admin turns off `EnableLocationDDSigned` | New DD posts are not linked; every existing link keeps working |
| Both decorators off | `examples` says decoration is off, correctly, for the first time |

---

## Testing Plan

**Unit, Go**
- Grammar acceptance, and more importantly **rejection**: one test per row of the
  declined table.
- Boundary guards: a token inside a longer numeric run on each side, inside a
  comma-separated list, and two tokens sharing one guard character.
- `canonical()` exact round trip per format; idempotence of canonicalization.
  Include `340322N1181500W` and its fractional-second siblings explicitly: those
  are the tokens a `float64` intermediate silently corrupts by one second.
- More than eight fractional digits is declined, not truncated.
- **Boundary**: two coordinates separated by a single space are **both**
  decorated. This is the regression test for the consumed-guard design, and it
  is the one a reader should look for first when reviewing this area.
- Boundary rune classes, one case per rejected character on each side, plus a
  token at the very start and the very end of a message, plus a non-breaking
  space as a separator.
- `validateParams` against crafted links: unknown `f`, `f` disagreeing with `v`,
  a non-canonical alias of a valid token, non-ASCII in `v`, missing parameters.
- **`r` specifically**, one case per check: over 64 bytes; a disallowed rune
  (`<`, `&`, a control character, an unexpected non-ASCII letter); text that is
  not an anchored match for `f` (`PRIORITY TARGET 34.0561 N, 118.2500 W
  CONFIRMED`); and a valid token of the right grammar that normalizes to a
  **different** `v`. Each must reject the whole link, and a test asserts the
  page returns the error page rather than rendering the surviving rows.
- Every accepted token's emitted `r` normalizes back to its own `v`, checked
  over the whole positive corpus rather than case by case.
- Every accepted token renders 200.
- Resolution: digit count derivation, the smaller-half tie rule, and that no row
  ever renders more resolution than the token carried.
- Rounding, not truncation, at every minute and second boundary. `LatLonToLATM`
  (`udl/peek.go:302`) truncates and is biased about 1.8 km south and west; a
  test pins the boundary case so this plugin cannot acquire the same bug.
- Zero components: `0.0000, 32.5000` and `12.3456, 0.0000` are accepted, and
  only `0.0000, 0.0000` is declined. `click_handler.ts:183` drops both through a
  truthiness check, which is the bug this test exists to prevent.
- The aocanywhere demo corpus as cross-hemisphere positive vectors, and
  `9999N99999W` as a negative one, since `location.go` accepts it.
- Adversarial float inputs never produce a link carrying a non-numeric value.
- Benchmark with a CI time budget on the worst-case message.

**Unit, TypeScript**
- `fromParams` accepting the server's output and rejecting each mutation of it,
  within the shape-check limits it honestly has.
- `format.ts` rendering, independently tested. **No cross-tree parity vector
  table**: the Go side renders the page and the TypeScript side renders the
  panel, and each is tested against its own expectations.

**Integration**
- Tagger over messages combining coordinates, DTGs, code, links and fences.
- The post-size boundary, which drops decoration for the whole message.

**Component, Playwright CT**
- Panel at several resolutions, including the coarse case.
- Copy button present, copied, and unavailable.

**Docs**
- `TestEverySettingIsDocumented`, `TestEveryCodeIsDocumented`,
  `TestEveryCrossPageAnchorResolves` and the em dash ban pick the new pages up
  automatically.

---

## Acceptance Criteria

- [ ] Every Phase 1 grammar is recognized in prose and declined inside every
      protected-span kind.
- [ ] Every row of the declined table has a test asserting it stays undecorated.
- [ ] No token is ever accepted at decoration and rejected at render.
- [ ] A crafted link whose `v` is not exactly a canonical token this plugin
      would have produced renders the error page. **The webapp performs a shape
      check only**; the page is the round-trip-validated surface, and the docs
      say so.
- [ ] The panel renders completely with no network request.
- [ ] No rendered value, and no stored URL, carries more resolution than its
      source token.
- [ ] `v` is ASCII for every accepted token.
- [ ] The panel and the standalone page both show the author's own text, and
      show it identically.
- [ ] No `r` reaches a rendered page unless it is under 64 bytes, drawn from the
      allowed rune set, an anchored match for its `f`, and normalizing to `v`.
      A link failing any of those renders the error page.
- [ ] No location component uses `dangerouslySetInnerHTML`.
- [ ] Two coordinates separated by a single space are both decorated.
- [ ] No canonical form is ever produced by way of `Decimal()`.
- [ ] The DTG page still emits `script-src 'unsafe-inline'` and its countdown
      still runs; the location page emits `script-src 'none'`.
- [ ] Every USMTF positive vector in `mattermost-plugin-aocanywhere`'s
      `server/model/usmtf2004/sets/location_test.go` that this phase claims is
      parsed to the same latitude and longitude, to four decimal places.
- [ ] The aocanywhere posture is reachable: with only `EnableLocationUSMTF` and
      `EnableLocationMoniker` on, nothing bare is decorated.
- [ ] `make check-style && make test && make sbom-audit` pass; no new
      dependency.
- [ ] Help docs cover every new setting, code and grammar.
- [ ] `/tactical-fusion examples` demonstrates recognized, declined and skipped
      location tokens, generated by the real tagger, and `check` explains a
      declined one.

## Checklist

- [ ] **Slash command**: `/tactical-fusion check <text>` added; `examples`
      extended.
- [ ] **Error codes**: allocated inside the existing `17000-17999`
      `server/decorators/` range, with `16xxx` for `check`. No new range, since
      the documented key is one range per source tree and this adds no tree.
- [ ] **`CLAUDE.md`**: a Location section written to the standard of the
      Decorators section, including every declined format and why.
- [ ] **No em dashes** anywhere in the plan, the code, the comments or the docs.

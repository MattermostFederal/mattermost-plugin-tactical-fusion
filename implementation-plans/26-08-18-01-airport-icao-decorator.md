# Airport codes: an ICAO decorator

## Overview

Add a third decorator, `airport`, that recognizes an ICAO airfield code behind a
USMTF field label (`ICAO:`, `LOC:`, `DEPLOC:`, `ARRLOC:`) and renders the
airfield behind it: name, type, place, elevation, IATA code, and its position
rendered through the coordinate machinery this plugin already has. The airfield
database is ported from `mattermost-plugin-aocanywhere` and embedded in the
binary.

## Problem Statement

`server/decorators/location/location.go:199` names four USMTF labels it will
never claim:

> LOC, DEPLOC, ARRLOC and ICAO are deliberately absent: in USMTF they introduce
> an ICAO airfield code, which is a facility whose position must be looked up
> rather than computed, and which this decorator does not handle at any phase.

That is the gap. A USMTF line reading `DEPLOC:KIND` is as ordinary in this
audience's traffic as a grid reference, and today it decorates nothing, because
resolving it needs a lookup table rather than arithmetic. The sibling plugin
already ships that table and the rendering for it; this plan brings both across
and fits them to this repo's decorator framework.

Doing it here also unlocks something the sibling plugin cannot do: an airfield
resolves to a position, and this repo already renders a position eleven ways.
`KIND` can carry an MGRS row.

## Current State

**This repo** ships the decorator framework plus two decorators (DTG, Location).
Everything an airport decorator needs already exists:

- `decorators.Decorator` (`server/decorators/decorator.go:130`) - four methods, and one argument in `OnActivate`.
- `Pattern.ReplaceGroup` (`decorator.go:75`) - a pattern matching a label but linking only the token, which is the USMTF-moniker shape (`location.go:352`).
- `Pattern.Boundary` (`decorator.go:88`) - the guard that keeps `logs/ICAO:KIND` from being rewritten mid-path.
- `location.Convert(f, canonical, raw) (Conversion, bool)` (`convert.go:47`) - every derived reading, already rendered as strings.
- `/api/v1/convert` and `/api/v1/features` (`api.go:130`, `:165`) - the authenticated JSON API pattern.
- `webapp/src/decorators/location/convert.ts` - the module-state client cache with its `ready` / `rejected` / `failed` split.

**The sibling plugin** (`mattermost-plugin-aocanywhere/main`) has:

| Thing | Where | What it is |
|---|---|---|
| Source data | `assets/airport-codes.csv` | 8.5 MB, 82,808 rows, DataHub `airport-codes` layout |
| Loader | `server/airport/airports.go` | CSV → `map[string]*model.Airport`, read from the bundle path at activation |
| Record | `server/model/airport.go` | `Airport` struct, pointer fields for optional numerics |
| Route | `server/api.go:164` `handleICAOLookup` | `GET /icao/{ident}` → the record, 404 when unknown |
| Client | `webapp/src/api/icao_client.ts` | `fetchAirport(icao)` |
| Panel | `webapp/src/components/rhs/AirportInfo.tsx` | Name, ident, Location, Coordinates, Elevation, Type, IATA rows |
| Link | `webapp/src/components/usmtf_post/common/icao_link.tsx` | Click → set selection, open RHS |
| Grammar | `webapp/src/enhanced_text/patterns.ts:28` | `/(?:ICAO\|DEPLOC\|ARRLOC):([A-Z]{4})\b/g` - label-only, client-side |
| Names | `webapp/src/data/airport_names.ts` | 9,974 generated `ident → short name` pairs, for rendering `Name (IDENT)` |

### Current Gaps

- No airfield grammar here at all; the four labels are reserved and unclaimed.
- No airfield data compiled into the binary.
- Nothing renders a facility whose position is looked up rather than computed.

### What already contradicts this feature

Two existing artifacts assert that ICAO codes are **out of scope**, and one of
them is enforced by a test. Both must move as part of this work:

- `server/command_example_details.go:342-343` - `LOC:3510N07901W` and `ICAO:KLAX` sit in the `decorates: false` group headed *"Declined: needs a label or spacing"*. `TestEveryDetailDoesWhatItsGroupClaims` checks that flag **in both directions**, so it fails the moment `ICAO:KLAX` starts decorating. `KLAX` is certainly in the dataset.
- `public/help/formats.html:649-653` - the same claim, in the declined table.

## Phase Strategy

| Phase | Focus | Value |
|---|---|---|
| **Phase 1** (this plan) | Embedded data, the `airport` decorator, panel, hover, Go-rendered page, API route, one admin switch, docs, tests | **80% of value** |
| Phase 2 | A map on the airport panel and an airport map page (needs a new page capability or `PageMapping`); inline post rendering | Parity with Location |
| Phase 3 | IATA codes behind a label; airfield search in the slash command | Optional |

## Design Principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Detection | **Label-only, upper case only** | A bare `[A-Z]{4}` pattern, or a case-insensitive label | measured below |
| Separator | `[ \t]*:[ \t]*`, never `\s*` | `\s*`, which crosses a line break | `location.go:341` |
| Moniker | **Not consumed** (`ReplaceGroup: 1`) | Consuming it as `DTG:` does | `location.go:352` |
| Link label | **The token exactly as written** | Substituting `Indianapolis Intl (KIND)` into stored text | `hooks.go` rewrites the stored message |
| Validation | **By lookup.** An ident the dataset does not hold declines | A shape check that links `LOC:HOME` | `parseArea` validates by decoding |
| Data | Pre-filtered CSV, `//go:embed`, parsed at init | A generator, a committed 8.5 MB source, and a `make test` prerequisite | see Decisions |
| Rendering | **One renderer, in Go.** The payload carries rendered strings | A `format.ts` paired with a `format_test.go` fixture table | `Conversion` is all strings (`convert.go:5`) |
| Page | Go-rendered, `PageStatic`, no script | Widening the page CSP for a page with no map | `page_policy_test.go:145` |
| Panel data | One authenticated route, module-cached client | A fetch per hovered token | `location/convert.ts` |

## Reference Patterns

- `server/decorators/location/location.go:340-356` - the moniker pattern, `[ \t]*` and the reason for it, `ReplaceGroup`, `monikerBoundaryOK`.
- `server/decorators/location/grammar.go:347-375` - `boundaryOK` / `badNeighbor`.
- `server/decorators/location/area.go` `parseArea` - validate by decoding.
- `server/decorators/location/convert.go:5-22` - a payload of pre-rendered strings, and why.
- `server/decorators/dtg/page.go` - a Go-rendered page.
- `webapp/src/decorators/location/convert.ts` - module cache, one in-flight promise, `ready`/`rejected`/`failed`.
- `webapp/src/features/store.ts` + `TestWebappFeatureShapeMatches` - the payload-shape pin this plan copies.

## Requirements

- [ ] `ICAO:`, `LOC:`, `DEPLOC:`, `ARRLOC:` followed by a four-letter ident the dataset holds decorates; the label survives verbatim.
- [ ] An ident the dataset does not hold declines, and the message is untouched.
- [ ] Nothing decorates without a label, and nothing decorates behind a lower-case label.
- [ ] The link's stored text is the author's own token, never the airfield name.
- [ ] A label at the end of a line cannot claim a word on the next line.
- [ ] Clicking opens the sidebar; hovering names the airfield.
- [ ] The panel shows name, ident, type, place, elevation, IATA, and every coordinate reading Location would show for the same position.
- [ ] The standalone page shows the same values with no JavaScript.
- [ ] `EnableAirport` turns decoration off without breaking links already written.
- [ ] The plugin builds and tests on an air-gapped checkout with no generator run first.

## Out of Scope

- Any map on any airport surface (Phase 2).
- Inline post rendering (Phase 2).
- IATA-code grammar (Phase 3).
- Runway, frequency, or NOTAM data.
- The sibling plugin's name-abbreviation rules; see Decisions.
- Retiring the four labels from `location.go`'s exclusion list - they stay excluded *there*; this decorator claims them.

## Technical Approach

### 1. The data

**Provenance.** The CSV is the DataHub `airport-codes` dataset, derived from
[OurAirports](https://ourairports.com/data/), which is public domain. The sibling
plugin records no provenance; this repo has an SBOM and an air-gap posture and
must. `server/decorators/airport/data/README.md` records the upstream URL, the
retrieval date, the licence, the SHA-256 of the upstream file, and the exact
filter applied.

**What ships is the already-filtered CSV, embedded.**

```go
//go:embed data/airports.csv
var airportsCSV string

var airports = mustParseAirports(airportsCSV)
```

Rows kept: `ident` matching `^[A-Z]{4}$`, less the reserved `ZZZZ`, leaving
**19,012**. The unfiltered match is **19,013 of 82,808**, exactly
the set the grammar can ever name; the other 63,795 are local codes, heliport
identifiers and numeric-prefixed strips no `[A-Z]{4}` token can reach. Seven are
`type: closed` and are kept: a closed field is a real answer, and dropping it
would make an ident decline rather than say so.

Columns kept: `ident`, `name`, `type`, `municipality`, `iso_country`,
`iso_region`, `iata_code`, `elevation_ft`, `lat`, `lon`. Dropped: `continent`
(unused), `gps_code` and `local_code` (equal to `ident` or empty across this
subset), `icao_code` (redundant with `ident` here). Result is roughly 1.2 MB.

**Coordinates are rounded to four decimals in the shipped file**, and the
rounding is not cosmetic: 31 axis values in the source carry **zero** fractional
digits and over a thousand carry **nine or more**, up to eighteen. `FormatDD`
requires at least four, and `Axis.Frac` caps at eight, so the raw values are
outside the grammar at both ends. Negative zero is normalized to zero before
formatting, for the reason `roundTo` records: a `-0.0000` axis would not
round-trip.

**A one-off filter program, not a generator in the build.** `build/airportdata/`
is a stdlib-only Go program that turns the upstream file into the shipped one. It
is run by hand, `make airport-data` exists for convenience, and it is
**deliberately not a prerequisite of `make test`**. `map-data-check` earns that
slot for two reasons that do not transfer: its transform is an opaque
delta-base-36 encoding whose output cannot be eyeballed, and its drift fails
*invisibly* (the webapp's `crypto.subtle` digest check is skipped on plain-HTTP
origins). This transform is "filter, round, drop columns" and its drift means an
ident declines, which is visible and benign. The upstream 8.5 MB file is **not**
committed; its URL and checksum are.

The program is in the shipping module, like `build/mapdata/`, so it stays
standard-library only or `cyclonedx-gomod` enumerates whatever it pulls in.

**A test over the embedded data**, not the generator, is what holds the file
honest: every row's ident is unique and matches `^[A-Z]{4}$`, every row's
coordinate pair survives a `location.Parse` round trip **and** converts, and a
handful of idents (`KIND`, `KLAX`, `EDDT`, `NZSP`, `PHIK`) are pinned by value.
`NZSP` is Amundsen-Scott at latitude exactly `-90.0000`, which is past Mercator,
past both grid bands, and reachable only by the area-reference rows: the one
fixture that exercises every "this row is blank on purpose" path at once.

### 2. The decorator

`server/decorators/airport/`:

- `airport.go` - `Decorator` with `Enabled func() Formats`, `Type() = "airport"`, `Patterns()`, `Parse()`, `RenderPage()`.
- `grammar.go` - the label alternation, the ident sub-expression, the boundary guard.
- `data.go` - the embed, the parser, `Lookup(ident string) (Airport, bool)`.
- `format.go` - every rendered string: place line, elevation, type in Title Case, and the position bridge.
- `page.go` - the standalone page under `PageStatic`.

No `rows.go`. Location has a row catalog because row ids reach the KV store and
`preferences.go:236` validates them against `location.KnownRow`; there is no
airport Customize surface, so nothing would hold a catalog to agreement and it
would be a contract with no second party.

**Grammar. One pattern, upper case only:**

```
(?:ICAO|DEPLOC|ARRLOC|LOC)[ \t]*:[ \t]*([A-Z]{4})
```

with `ReplaceGroup: 1`.

Three things here differ from Location's moniker and each is deliberate:

- **`[ \t]*`, never `\s*`.** RE2's `\s` includes `\n`, so a label at the end of a line reaches across the break and claims the next line's first word. `location.go:341` documents this defect (`"MGRS:\n58cbe40"` decorated a git SHA). Here it would be worse: 343 of the 19,012 idents are English dictionary words, so `Divert to LOC:` followed by a line beginning `FAST` would permanently rewrite `FAST` into an airfield link.
- **The label is upper case only**, where Location's is `(?i:)`. Location's tokens carry digits, so a lower-case label in prose cannot reach one. An airfield ident is four letters, which is what prose is made of: `loc: fast turnaround on the ramp` passes every guard and links `FAST`. USMTF field labels are upper case, so this narrowing costs nothing real, and it is the same measured-narrowing move `mgrsBareCompactExpr` and `olcBareExpr` already make.
- **The ident is upper case only**, which also makes `v` self-canonicalizing: `Parse` has no case to fold, the URL can be spelled exactly one way, and the `loc.Canonical() != v` invariant every other route here holds is satisfied by construction rather than by a check.

**The trailing `//` is matched, not permitted by the guard.** Keeping
`badNeighbor` as it stands would decline `DEPLOC:KIND//`, which is a genuine
USMTF set line and the traffic this feature opens with. Loosening the guard to
allow a trailing `/` would fix that and reopen something worse: `ICAO:KIND/foo`
is path-shaped, and rewriting the middle of a path is the failure this guard
exists for.

`Pattern.boundaryOK` is handed the runes flanking the **whole match**, while
`ReplaceGroup` narrows only what is rewritten (`tagger.go:497`, `:521`). So the
terminator goes in the pattern instead:

```
(?:ICAO|DEPLOC|ARRLOC|LOC)[ \t]*:[ \t]*([A-Z]{4})(?://)?
```

`DEPLOC:KIND//` matches with the `//` inside the match, so the guard sees
whatever follows it and the line decorates, with only the ident rewritten and
the `//` left in the text. `ICAO:KIND/foo` cannot match the optional group, so
the match ends at the ident, the guard sees `/` and refuses. The guard itself
is byte-identical to Location's, with no asymmetry anywhere.

`boundaryOK` and `badNeighbor` are unexported in `location`, so this would
otherwise be a third hand-written copy of the guard with nothing holding them
together. Export `decorators.BadNeighbor(r rune) bool` from the framework - it is
framework-shaped, not location-shaped - and have `location.badNeighbor` delegate
to it, so there is one origin.

**`Parse`** looks the ident up and returns `url.Values{"v": ident}` when it is
found, `ok=false` otherwise. That is what keeps `LOC: HOME` and `ICAO: FAST` from
becoming links. It is pure and is one map read, so it is cheap enough for the
post path. `ok=false` does not claim the range (`tagger.go:471`), and this
decorator has a single pattern, so the "scan has moved past a rejected match"
caveat costs nothing.

**One parameter, `v`.** Nothing derived travels in the URL; the identity of an
airfield *is* its ident. No `r` parameter: the label is not consumed and both
label and ident are upper case, so the author's text and the canonical form are
always identical and there is nothing for `r` to carry.

**`RenderPage` must not require the ident to still exist.** Location's
round-trip refusal exists because a crafted URL could pair a token with an
unrelated instant; here the URL carries one value and derives everything from it,
so a hard refusal buys nothing and costs something real: refresh the dataset,
lose an ident, and every message that ever named it holds a link that answers 400
forever. So an unknown ident renders the code and a line saying it is not in this
build's airfield database, at HTTP 200. A `v` that is not four upper-case letters
is still a 400, and that shape check is what makes echoing `v` on the page safe,
so it runs before any rendering path.

### 3. Position

```go
token := strconv.FormatFloat(a.Lat, 'f', 4, 64) + "," + strconv.FormatFloat(a.Lon, 'f', 4, 64)
conv, ok := location.Convert(location.FormatDD, token, "")
```

**No space after the comma.** `Convert` routes through `validateParams`, which
requires `loc.Canonical() != v` to be false (`location.go:519`), and
`canonicalString` writes the DD separator as a bare comma (`coord.go:319`). A
space makes `ok` false for **every** airfield, silently, and the identical helper
already exists at `mappage_test.go:83`. A test asserts the built token equals its
own canonical form.

Every reading on every airport surface is then a field of one
`location.Conversion`, so there is no second coordinate renderer and the MGRS an
airfield reports is the MGRS Location reports for the same point.

`Convert` and `RenderPage` never consult format configuration, so airport
readings survive `EnableLocationDDSigned` being off. That is the existing rule
and it is the wanted behaviour.

**Import direction:** `airport` imports `location`; never the reverse.

### 4. Rendering, in Go, once

The payload carries **already-rendered strings**, the way `Conversion` does and
for the reason `convert.go` gives: the rendering rules are the interesting part
and they live in Go, and handing the webapp raw fields would put two
implementations of them in one repository. `format.go` renders `place`,
`elevation` and `type`; the panel and the page both print those strings against
labels.

That is what removes the duplication, not the choice of where the page renders.
There is no `format.ts`, no `format.spec.ts`, and no paired fixture table.

### 5. The page

Go-rendered, `decorators.PageStatic`, **no script at all**, so `script-src
'none'`. (Note: the DTG pages are *not* an example of that - `dtg.go:243`
supplies `ScriptJS`, so they are served under `script-src 'sha256-...'`. The
airport page is the first page here with no script.) `PageStatic` is the zero
value, `ScriptSrc` is refused outside `PageMapping` (`page.go:152`), and
`TestPageCapabilityDecidesTheWholePolicy` pins both policies as whole strings, so
this page adds nothing to that test.

The page therefore has **no copy buttons** - those need a delegated listener the
page does not have. The panel has them. That is a real per-surface difference and
is stated in the UX table rather than hidden.

### 6. The API

`GET /api/v1/airport?v=KIND`. A **discriminated shape**, not a flat record:

```json
{"found": true, "ident": "KIND", "airport": {…}, "position": {…Conversion…}}
{"found": false, "ident": "ZZZZ"}
{"found": true, "ident": "XXXX", "airport": {…}}
```

`position` is omitted when there is none. Flattening or embedding `Conversion`
would be two live defects: a `found: false` body would carry `lat: 0, lon: 0`,
and the webapp's `asConversion` accepts `0,0` deliberately because it is a real
position, so an unknown ident would render as Null Island; and
`Conversion.Region` (`json:"region"`, the country) would collide by depth with an
outer `iso_region`, with `encoding/json` silently dropping the shallower loser.
The third shape above is the "found, but no usable position" state, which the
embedded-data test makes unreachable today and which must still be
representable.

Handler rules, each with a precedent:

| Case | Answer | Why |
|---|---|---|
| Non-GET | 405 `APIMethodNotAllowed` | `api.go:131`, `:166` |
| `v` absent, wrong length, or not `^[A-Z]{4}$` | 400 | length checked before the pattern, as `location.go:512` caps `r` first |
| Ident not in this build | 200 `{found:false}` | same answer the page gives; one shared `lookup + validate` function, never two |
| `EnableAirport` off | still answers | a format switch governs decoration only; a link written while it was on must keep resolving |
| Configuration not loaded | no gate | unlike `/features`, this answer is not cached as an admin decision |
| Routing | one `if r.URL.Path == airportPath` before the preferences branch | `serveAPI` is an `==` chain (`api.go:81`) |

`Cache-Control: private, max-age=300`, set only on the 200 path after validation.
`private` because the URL itself reveals which airfield a named reader looked at.
`max-age=300` is **not** "the answer is constant for the life of a build" - that
would argue for far longer. It is there to bound how long a browser keeps an
answer across a plugin upgrade.

### 7. The webapp

`webapp/src/decorators/airport/`: `airport.ts` (client and module cache),
`AirportPanel.tsx`, `AirportHover.tsx`, `index.ts`, plus one line in
`registerBuiltinDecorators()`. No `format.ts`, no `rows.ts`, and no
`AirportReadings.tsx` split - `LocationReadings` exists because three surfaces
share it, and this has one.

The client imports `Conversion` and `asConversion` from `../location/convert`
rather than redeclaring them, so the conversion half inherits
`TestWebappConversionShapeMatches` for free. A new `TestWebappAirportShapeMatches`
in the `main` package, modeled on `TestWebappFeatureShapeMatches`, pins the
airport half by name, type and order.

The cache is shaped like `convert.ts` - `request()` checks the cache itself
rather than leaving that to its one caller, one in-flight promise is shared, a
`failed` read is never cached - with one deliberate difference. `convert.ts`
caches a verdict forever because a conversion is a pure function of its token.
**An airport answer is a function of `(ident, build)`, not of `ident`**, and this
plan's own design says idents can be dropped and records refreshed. So the
airport store uses the existing `CACHE_TTL_MS`, the constant `preferences/store.ts`
and `features/store.ts` already share and `TestWebappCacheLifetimeMatches`
already pins across languages. Do not copy `convert.ts`'s justification comment;
it is false here.

`elevation_ft` is `number | null` and is never tested for truthiness: sea level
is `0` and is a real elevation, and 1,444 rows in the subset have none. This is
the defect this repo already records as deliberately not inherited from the
sibling plugin ("a truthiness check that drops the equator and the prime
meridian").

Five states, specified for both the panel and the hover so the two cannot
disagree the way `LocationHover` and its panel once did: `loading`,
`ready & found`, `ready & !found`, `rejected`, `failed`. The hover renders `null`
for anything but `ready & found`, which the framework's `:empty` rule on
`HOVER_CARD_CLASS` turns into no card rather than an empty bordered box.

The hover is the airfield **name and place, one line** - the one thing worth
knowing without opening the sidebar, which for a four-letter code is exactly what
it means.

### 8. Admin settings

A fourth section, **Airfields**, with one switch:

| Key | Default | What |
|---|---|---|
| `EnableAirport` | `true` | Recognize ICAO airfield codes behind a USMTF label |

**One switch, not two.** The decorator is label-only by construction, so a
`Moniker` switch would turn the whole decorator off under a second name.
`Plugin.airportFormats()` sits beside `dtgFormats` and `locationFormats` and is
read fresh per message.

On by default: its worst case is a false positive where somebody typed an
upper-case `LOC:` in front of four letters that happen to be an airfield, which
is the ordinary cost every other on-by-default switch carries. `EnableLocationUTM`
stays the only off-by-default switch and `defaultsOff` needs no entry.

### 9. The slash command

Both example commands iterate the live registry, so registering a decorator
without touching them fails two tests:

- `TestExamplesCoversEveryRegisteredDecorator` (`command_examples_test.go:77`) requires a `/decorate/<type>?` link per registered decorator in the `examples` post. **`command_examples.go` must gain an airfield row.**
- `TestDetailsCoverEveryRegisteredDecorator` (`command_check_test.go:294`) requires the same of `example-details`.

`example-details` gets a **third `detailSet`**, plus an entry in
`detailSetOrder`, not a group inside the coordinates set:
`TestDetailsPostOnePostPerDecorator` asserts `len(messages) == len(detailSetOrder)`
and CLAUDE.md's rule is one top-level post per decorator.

The two rows at `command_example_details.go:342-343` move out of the declined
group. `LOC:3510N07901W` still declines (the airport pattern needs four
*letters*), but its note becomes misleading and must be rewritten.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Bare four-letter idents? | **No. Label-only, permanently.** | Measured over the 19,013-ident subset: **343 are English dictionary words** (`FACT`, `FAST`, `FALL`, `FOOD`, `FLAT`, `EDGE`, `BITE`), and among military acronyms `USCG` is Chelyabinsk Shagol (Russia), `LIMA` is Torino-Aeritalia, `UNIT` is Tura Mountain, `ETIC` is Grafenwöhr AAF. A bare grammar would rewrite "USCG" in a sentence into a link to a Russian airfield. That is the `EnableLocationUTM` failure mode - confidently wrong, not merely noisy - at a far higher rate. |
| Case-insensitive labels? | **No. Upper case only, label and ident.** | `loc: fast turnaround` passes every guard and links `FAST`. Location can afford `(?i:)` because its tokens carry digits. This also makes `v` self-canonicalizing. |
| `\s*` or `[ \t]*` around the colon? | **`[ \t]*`.** | `\s` includes `\n`; `location.go:341` records the defect. With dictionary-word idents the consequence is rewriting the next line's first word into stored text, permanently. |
| `DEPLOC:KIND//` (a real USMTF set line) | **Match the `//` in the pattern**, outside `ReplaceGroup` | Loosening the guard to allow a trailing `/` would also permit `ICAO:KIND/foo`, which is path-shaped, and rewriting the middle of a path is what the guard exists to stop. `boundaryOK` sees the whole match while `ReplaceGroup` narrows only the rewrite, so the terminator can be consumed without being linked. The guard stays identical to Location's. |
| Substitute the airfield name into the link label? | **No.** | `MessageWillBePosted` rewrites the **stored** message. Putting `Indianapolis Intl (KIND)` there edits what somebody wrote, using data that changes between builds, and survives uninstall. The sibling plugin renders `Name (IDENT)` client-side where nothing is stored; that argument does not transfer. |
| Which labels? | `ICAO`, `LOC`, `DEPLOC`, `ARRLOC` | Exactly the four `location.go:199` reserves, for the reason it reserves them. `NAME`/`DEPNAME`/`ARRNAME` introduce free text and stay unclaimed. |
| Coordinate precision | **Four decimals (~11 m)**, rounded in the shipped file | Not only an honesty argument: 31 source axis values have zero fractional digits and 1,000+ have nine to eighteen, and `FormatDD` requires ≥4 while `Axis.Frac` caps at 8, so the raw values are outside the grammar at both ends. Four is also the coarsest the grammar admits, which is the right direction for a crowd-sourced reference point whose meaning (tower? ARP? terminal?) the source never states. |
| Generator + `make test` check? | **No. Ship the filtered CSV and `//go:embed` it.** | `map-data-check` earns its slot because its encoding is opaque and its drift fails invisibly on HTTP. This transform is filter-round-drop and its drift is a declining ident. Embedding also removes the "1.2 MB of generated Go slows `go build`" question rather than mitigating it. Provenance and the filter program stay; the 8.5 MB upstream file is not committed. |
| Row catalog? | **No.** | Location's exists because ids reach the KV store and `preferences.go:236` validates them. No airport Customize surface means no second party to a contract. |
| A `format.ts` beside `format.go`? | **No.** The payload carries rendered strings. | The `Conversion` precedent, and it removes a permanent cross-language drift seam rather than pinning one with fixtures. |
| Reuse the page bundle for the page? | **No, Go-render under `PageStatic`.** | `Page.ScriptSrc` is refused outside `PageMapping`, which grants `worker-src`, `img-src` and `connect-src` on a route that echoes a request value. A `PageScripted` capability granting only `script-src 'self'` is the clean answer and is deferred to Phase 2, when an airport map would need the bundle anyway. |
| Copy `airport_names.ts`? | **No.** | It exists so the sibling plugin can render `Name (IDENT)` with no server call. Here the name arrives through the API and the page shell, so a second 10,000-entry copy in the bundle is pure weight, and its 30-character abbreviation rules are a display convention for a different layout. |
| `ZZZZ` | **Excluded from the dataset** | Found during implementation. It is a real upstream row (Satsuma Iojima, Japan) and also the ICAO code for an aerodrome that is **not listed**, with the real field named in remarks. This decorator reads `DEPLOC` and `ARRLOC`, which are flight plan fields, so shipping it would make `DEPLOC:ZZZZ` name a specific island airfield where the message means "see remarks". Same class as the UTM band letter: a real code rendered as a confidently wrong place. `AFIL` is not upstream. |
| Unknown ident on the page | **200 with a note**, not 400 | Otherwise a refreshed dataset turns every historical link naming a dropped ident into a permanent 400. |
| Client cache lifetime | **`CACHE_TTL_MS`**, not forever | An airport answer is a function of `(ident, build)`. Caching forever means a reader with an open tab keeps seeing "not in this build" after an upgrade adds the ident, with nothing suggesting a reload. |

## Files to Modify

### New

| File | Change |
|---|---|
| `server/decorators/airport/data/airports.csv` | The filtered, rounded dataset, embedded |
| `server/decorators/airport/data/README.md` | Upstream URL, retrieval date, licence, SHA-256, filter applied |
| `server/decorators/airport/{airport,grammar,data,format,page}.go` | The decorator |
| `server/decorators/airport/*_test.go` | Grammar, data integrity, format, page, conversion bridge |
| `build/airportdata/main.go` | Stdlib-only one-off filter, not in the build path |
| `webapp/src/decorators/airport/{index,airport}.ts` | Decorator and client cache |
| `webapp/src/decorators/airport/Airport{Panel,Hover}.tsx` (+ harnesses, `.pw.tsx`) | Sidebar and hover |

### Modified

| File | Change |
|---|---|
| `server/decorators/decorator.go` | Export `BadNeighbor` |
| `server/decorators/location/grammar.go` | `badNeighbor` delegates to the framework's |
| `server/plugin.go` | One argument in `NewDefaultRegistry`; `airportFormats()` |
| `server/configuration.go` | `EnableAirport` |
| `server/api.go` | `/api/v1/airport` route and handler |
| `server/errcode/codes.go` | New codes in the 13000 and 17000 ranges |
| `server/command_examples.go` | An airfield row (required by `TestExamplesCoversEveryRegisteredDecorator`) |
| `server/command_example_details.go` | A third `detailSet` + `detailSetOrder` entry; move and rewrite the two rows at `:342-343` |
| `webapp/src/decorators/index.ts` | One line in `registerBuiltinDecorators()` |
| `webapp/src/decorators/airport` sync test in `server/` | `TestWebappAirportShapeMatches` |
| `plugin.json` | Airfields section, one switch |
| `Makefile` | `airport-data` (convenience only, **not** a `test` prerequisite) |
| `public/help/{formats,admin,error-codes,panel,commands,help}.html` | Grammar, switch, codes, panel, command, nav. `formats.html:649-653` must move out of the declined table |
| `CLAUDE.md` | An "Airfields" section carrying the rationale in this plan |

## Tasks

1. [ ] Retrieve the upstream CSV; write `build/airportdata/` (filter to `^[A-Z]{4}$`, round to 4 dp, normalize negative zero, drop columns, refuse a duplicate ident or a delimiter character in a field); produce and commit `data/airports.csv` and `data/README.md`.
2. [ ] `data.go`: `//go:embed`, parser, `Lookup`. Data-integrity test: unique idents, shape, every pair round-trips through `location.Parse` **and** converts, plus pinned fixtures including `NZSP`.
3. [ ] Export `decorators.BadNeighbor`; make `location.badNeighbor` delegate; confirm the location tests still pass unchanged.
4. [ ] `grammar.go` + `airport.go`: the upper-case pattern with `[ \t]*`, the guard, the optional `//` terminator, `Parse`.
5. [ ] Tagger tests: labeled decorates; bare declines; lower-case label declines; unknown ident declines; `logs/ICAO:KIND` refused; `DEPLOC:KIND//` decorates; a label at end of line does **not** claim the next line's word; two idents on one line both decorate; inside a fence and inside a link untouched; idempotent over its own output.
6. [ ] Labeled-path regression corpus: all 343 dictionary-word idents behind each of the four labels, upper and lower case, in sentences. Gate it on `testing.Short()` - CLAUDE.md records that `make coverage` needs `-short` and says anything slow enough should use it rather than a bigger timeout.
7. [ ] `format.go`: rendered strings and the position bridge, with the canonical-form assertion.
8. [ ] `page.go` under `PageStatic`; page-policy test.
9. [ ] `/api/v1/airport`: the discriminated shape, the handler rules table above, error codes, tests for each row of it.
10. [ ] Webapp: `airport.ts` (five states, `CACHE_TTL_MS`, `failed` not cached, `Conversion` imported not redeclared), panel, hover, registration.
11. [ ] `TestWebappAirportShapeMatches`.
12. [ ] `plugin.json` section and switch; `EnableAirport`; `Plugin.airportFormats()`. Existing settings tests should pass unmodified.
13. [ ] Slash command: airfield row in `examples`; third `detailSet` in `example-details`; move and rewrite the two declined rows.
14. [ ] Help docs, including moving `formats.html:649-653` out of the declined table.
15. [ ] `CLAUDE.md` "Airfields" section.
16. [ ] `make check-style && make test && make sbom-audit`.

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| A label at end of line claims the next line's word | `[ \t]*`, and a regression test with the label at end of line. This is the single highest-consequence bug available here. |
| A lower-case label in prose links a dictionary word | Upper-case-only labels, plus the labeled-path corpus in task 6. |
| The `Convert` bridge silently returns `ok=false` | No space in the token, plus a test asserting the built token equals its own canonical form. Both reviews found this independently in the first draft. |
| `found:false` renders as Null Island | Discriminated shape; `position` absent rather than zero. |
| Airfield data goes stale | The value carries its own provenance line, as the Region row carries "Natural Earth 110m". Refresh is one program run. |
| A refreshed dataset drops an ident | The page answers 200 with a note, never 400. |
| The exported `BadNeighbor` refactor changes location's behaviour | Pure move; location's existing guard tests are the check and must pass unchanged. |
| `ICAO:KIND/foo` rewritten mid-path | The optional `//` cannot match a single slash, so the guard still sees `/` and refuses. Regression test for both spellings. |
| `airport` → `location` grows into a cycle | One direction only; if it ever reverses, the bridge moves to a shared package. |
| Licence/SBOM friction | OurAirports is public domain; provenance is recorded; no runtime dependency added; the filter program is stdlib only. |

## UX Summary

| Scenario | Behavior |
|---|---|
| `DEPLOC:KIND` in a message | `DEPLOC:` stays, `KIND` becomes a link |
| `DEPLOC:KIND//` (USMTF set line) | Decorates; the trailing `//` is untouched |
| Hover the link | One line: `Indianapolis International Airport - Indianapolis, IN, US` |
| Click the link | Sidebar: name, ident, type, place, elevation, IATA, then every coordinate reading, each with a copy button |
| Same link on the mobile app | Standalone page, same values, **no copy buttons** (no script on that page) |
| `ICAO: ZZZZ` | Nothing decorates |
| `icao:kind` (lower case) | Nothing decorates |
| `LOC:` at end of a line, `FAST` starting the next | Nothing decorates |
| `USCG` in a sentence | Nothing decorates |
| `logs/ICAO:KIND` | Nothing decorates |
| Hand-edited link to a dropped ident | Page renders the code and says it is not in this build's database |
| `EnableAirport` off | New messages stop decorating; existing links keep resolving |

## Testing Plan

**Unit (Go)** - grammar in every direction above; `Lookup`; the conversion bridge for a northern, southern, eastern, western and prime-meridian airfield plus `NZSP` at the pole; `Parse` purity; page rendering; page policy as a whole string; embedded-data integrity.

**Unit (TypeScript)** - `airport.ts` cache behaviour: in-flight sharing, `failed` not cached, TTL honoured, all five states.

**Component (Playwright)** - panel for each of the five states; hover renders nothing but a card for `ready & found`; copy buttons hidden without a clipboard.

**Integration** - `MessageWillBePosted` end to end for every row of the UX table; `/api/v1/airport` for every row of the handler table; the settings tests unmodified; the two example-command coverage tests.

**Regression corpus** - the labeled-path sweep of task 6, `testing.Short()`-gated.

## Acceptance Criteria

- [ ] Every row of the UX table behaves as stated.
- [ ] Every row of the handler table answers as stated.
- [ ] The stored message keeps the author's token and label verbatim.
- [ ] Panel, hover and page agree on every value for the same ident.
- [ ] Airport coordinate readings equal Location's readings for the same point.
- [ ] `make check-style && make test` pass, with no new `make test` prerequisite.
- [ ] `make sbom-audit` adds no dependency.
- [ ] A clean checkout builds and tests with no program run first.
- [ ] Every new error code is in `codes.go`, `AllCodes` and `error-codes.html`.
- [ ] Every new setting is in a named section and documented in `admin.html`.
- [ ] No artifact anywhere still claims ICAO codes are out of scope.

## Checklist

- [ ] **Help docs**: `formats.html` (including the declined-table move), `admin.html`, `error-codes.html`, `panel.html`, `commands.html`, `help.html` nav.
- [ ] **Slash command**: `examples` row and a third `example-details` set.
- [ ] **CLAUDE.md**: an "Airfields" section - rationale lives there, not in comments.
- [ ] **No prose comments** in new or modified code.
- [ ] **No em dashes** anywhere.
- [ ] Conventional commit subject (`feat: ...`), since PRs squash-merge.

## Outcome

Implemented. `make check-style`, `make test` (1,856 Go tests, 376 webapp tests),
`make dist` and `make sbom-audit` all pass, and the audit adds no dependency.

Four things changed from the plan during implementation, each recorded above
and in CLAUDE.md:

0. **The airfield surfaces print no coordinate readings.** The plan had both
   rendering all eleven. The panel now **draws the map** under the airfield
   details, through the location decorator's own `LocationMap`, and **the Place
   value is the link** that swaps the sidebar in place via `setSelection`. The
   page, which is `PageStatic` and runs no script, makes its Place an `<a>` to
   the coordinate page instead. The wire carries
   `coordinate {format, value, region}` instead of a whole `Conversion`, built
   in Go because Go and JavaScript disagree about formatting a float in the last
   digit. The panel map is a fifth `LocationMap` mount site and honours
   `mapPanel` like the other four. "No airfield map" is therefore no longer a
   deferral at all.

1. **`ZZZZ` is excluded from the dataset.** Found when a "declined" example row
   using it decorated: it is a real airfield upstream and also the flight plan
   code for "aerodrome not listed". See Decisions.
2. **The dictionary-word count is 343, not 388.** The higher figure counted
   proper nouns; 343 is what the shipped test measures over the shipped data,
   and every reference now quotes that.
3. **`help.html` carried stale text** claiming GARS, GEOREF and Plus Codes were
   not implemented. Corrected while editing that paragraph.

One pre-existing inaccuracy was left alone: `formats.html`'s declined table
still lists "GARS, GEOREF, Plus Codes ... Not implemented in this version",
which contradicts the coordinates section on the same page. It predates this
work and correcting it needs a decision about what that row was meant to say.

## Open Question

**Coordinate precision.** Four decimals (~11 m) rather than the source's digits.
This is the one call in the plan that is a judgment rather than a constraint: the
grammar forces *some* normalization (raw values sit outside it at both ends), but
four versus five or six decimals is a choice about how precisely to quote a
crowd-sourced airfield reference point. Four is recommended.

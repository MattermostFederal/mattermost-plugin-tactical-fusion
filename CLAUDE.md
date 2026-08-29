# Tactical Fusion Plugin Guidelines

## Overview

Mattermost Tactical Fusion enriches conversations with mission-relevant context:
geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational
information. The server is Go, the webapp TypeScript/React.

Shipped today: the decorator framework and three decorators, DTG, Location and
Airfields, plus the bundled offline map, the Cursor on Target renderer and the
GeoJSON renderer. The rest is not implemented.

A decorator finds a token in a posted message, rewrites it in
`MessageWillBePosted` into a markdown link whose query string carries the
already-parsed data, and renders the detail behind it in a hover card, the
right-hand sidebar, and a standalone server-rendered page.

## Architecture

### `server/`

| Path | What lives there |
|---|---|
| `main.go` | `plugin.ClientMain`, blank-imports `time/tzdata` so zones work in minimal images |
| `plugin.go` | The `Plugin` struct, `OnActivate`, `OnPluginClusterEvent` |
| `configuration.go` | Config struct and `OnConfigurationChange`, plus `dtgFormats`/`locationFormats`/`locationMaps` |
| `hooks.go` | `MessageWillBePosted`, `decoratePost`, `stampStandalonePost`, the post size constants |
| `hooks_stamp.go` | The forged-type strip, the props ladder, the attachment ownership check |
| `http.go` | `ServeHTTP`: `/decorate/<type>`, `/map`, `/api/v1/*`, and the session gate |
| `api.go` | Authenticated JSON API: `/preferences`, `/convert`, `/features`, `/airport` |
| `preferences.go`, `preferences_cache.go` | Per-reader KV store and its cluster-aware cache |
| `command*.go` | The `/tactical-fusion` slash command and its example builders |
| `errcode/` | The `TF-NNNN` catalog |
| `decorators/` | Framework: registry, tagger, boundary guard, shared page shell |
| `decorators/dtg/` | Date-time groups and RFC 3339 timestamps |
| `decorators/location/` | Coordinate grammars, geodesy, MGRS, rendering, conversion; `mapdata/` holds the generated country polygons |
| `decorators/airport/` | ICAO airfields; `data/` holds the embedded CSV and its provenance |
| `cot/` | Cursor on Target: the bounded XML parse, the type tables, the post props |
| `geojson/` | GeoJSON: the bounded JSON walk, the parts/rings shape, the post props |

### `webapp/`

| Path | What lives there |
|---|---|
| `src/index.tsx` | `initialize()`, registration, the disposer list run by `uninitialize()` |
| `src/decorators/` | Framework: registry, click handler, styles, selection store, theme, `Tooltip` |
| `src/decorators/{dtg,location,airport}/` | Panels, hovers, and per-decorator clients |
| `src/cot/` | The Cursor on Target post body, its card and its map |
| `src/geojson/` | The GeoJSON post body, its card, its map, its panel and its reader |
| `src/decorators/location/map/` | `LocationMap`, MapLibre loading, the basemap reader, span arithmetic |
| `src/page/` | Standalone pages' entry point, built by a second webpack config into `public/app/page.js` |
| `src/components/rhs/` | `RhsView` and `RhsTitle` |
| `src/preferences/`, `src/features/` | Module-state caches in front of the two API routes |

### Elsewhere

- `plugin.json` generates `server/manifest.go` and `webapp/src/manifest.ts` at build time (both gitignored).
- `build/mapdata/` (stdlib-only, `make map-data`) generates the country polygons; `build/maptiles/` (containerised, `make map-tiles`) generates the PMTiles basemap and glyph ranges. Both outputs are committed.
- `build/airportdata/` (stdlib-only, `make airport-data`) filters the upstream airfield CSV. Not in the test path.
- `public/help/` is the built-in documentation, served by Mattermost with no route in the server code.
- `docker-compose.dev.yml` and `docker/` back `make deploy`.
- `implementation-plans/` holds the plans features were built from.

## Design notes

Anything a code comment would have recorded lives in `docs/design/`, by area.
**Read the matching file before changing that area**, and write new rationale
there rather than here or in a comment.

| File | Covers |
|---|---|
| [`docs/design/decorators.md`](docs/design/decorators.md) | The framework, the DTG grammars, the tagger and protected spans, `siteURLPath`, page CSP, the slash command, adding a decorator, the post size limit |
| [`docs/design/location.md`](docs/design/location.md) | Every coordinate grammar, boundary guards, rendering and resolution, geodesy, `/api/v1/convert`, copy buttons, prior art |
| [`docs/design/airfields.md`](docs/design/airfields.md) | The label-only ICAO grammar, the embedded database, `/api/v1/airport`, the page and panel |
| [`docs/design/cot.md`](docs/design/cot.md) | Cursor on Target: why it is not a decorator, the exclusivity rule, the props budget, `edit_at` over a digest, the parser's refusals, the CE circle |
| [`docs/design/geojson.md`](docs/design/geojson.md) | GeoJSON: why recognition is narrow, why format order stayed format-major, what the two stampers share and what they must not, the parts/rings shape, why `decimalShape` is not reused, the ringed map prop, extent-only, the cross-shape antimeridian unwrap |
| [`docs/design/mapping.md`](docs/design/mapping.md) | The vector basemap, the OpenStreetMap detail tier and its seam, detail map packages, `PageStatic` vs `PageMapping`, the page bundle, zoom numbers, the country lookup, `Conversion`, the map page, the panel map, turning maps off, the map under a post |
| [`docs/design/preferences.md`](docs/design/preferences.md) | The KV store, both caches, the location hover, the location rows, the zone picker and ordering |
| [`docs/design/admin-settings.md`](docs/design/admin-settings.md) | The twenty-five switches, the two map-package settings, the six sections, why `EnableLocationUTM` and `EnableGeoJSONUnlabeled` ship off |
| [`docs/design/help-and-errors.md`](docs/design/help-and-errors.md) | `public/help/` and the `TF-NNNN` catalog |
| [`docs/design/unverified.md`](docs/design/unverified.md) | Claims that need a running server or a phone and have never been checked |

## Invariants

The short list of things that break something real if you get them wrong. Each
one is argued in the design note beside it.

**Decoration rewrites the stored message.** It is permanent, it lands in
exports, and it survives uninstall. Nothing on that path may ever stop somebody
from posting: a panic is recovered, an over-long result is skipped, and the
recover logs through an API handle captured before the deferred call. There is
no `MessageWillBeUpdated` hook and a test asserts it stays absent.

**`findProtectedRanges` is the entire safety story.** Anything it fails to
recognize is a corruption bug. Widen it only with a regression test per
construct, and never let overlapping spans be discarded rather than merged.

**Boundary guards live in `Pattern.Boundary`, never in the regex.** A pattern
that consumes its own guard breaks the next match on the same line. Use the
shared `decorators.BoundaryOK`/`BadNeighbor`; do not hand-write a third copy.

**Separators are `[ \t]*`, never `\s*`.** RE2's `\s` includes `\n`, so a label
ending a line would claim the start of the next one.

**A link may never disagree with itself.** The URL carries the identity only
(`f`/`v` for location, `v` for airfields, the canonical token for DTG), and
every route re-derives the rest and requires it to round-trip. Nothing derived
travels in the URL.

**Render to the resolution the token carried, and round rather than truncate.**
Padding a field the author never wrote is a claim. Grid cells and area cells are
the exception: a cell is chosen by containment, so those encoders truncate.

**A zero value means "use the default", everywhere** in reader preferences. That
is what makes "Restore defaults" a delete. Preference row ids reach the KV store:
add and retire freely, rename never.

**A PUT replaces the whole preferences blob.** Always go through
`savePreferencesSection`/`resetPreferencesSection`, which re-read first.

**Format switches govern decoration only; map switches are read at render.**
A link already written keeps working after its format is switched off, which is
why decorators stay registered with everything off. Maps are the deliberate
exception and the `Formats` doc comment names it.

**`Page.Capability` decides the whole CSP.** `PageStatic` is what a page should
want; `PageMapping` gives back `script-src 'self'`, `worker-src`, `img-src data:`
and `connect-src 'self'` and makes escaping the only defense on a route that
echoes author text. `ScriptSrc` must be relative.

**A map that cannot become ready must say so.** MapLibre tiles in a worker, so a
worker that never arrives leaves `load` unfired and raises no error: the note sat
on "Loading map…" forever. Readiness is bounded by `READY_DEADLINE_MS`, the same
medicine `loadMapLibre` and `basemap.ts` already apply to their own fetches.

**MapLibre's worker and its shared chunk ship together in a content-keyed
directory.** The worker imports the shared chunk by a fixed relative name, and
`/static/plugins/**` is served with a one year max-age, so a fixed name is a
chunk that outlives the worker it belongs to. `make bundle` fails on a pair that
is missing, split, or not in a `maplibre-<hash>/` directory.

**The basemap's cache buster is the archive's digest, never the plugin version.**
The bundle is re-extracted on every install, so `Last-Modified` moves while the
bytes do not, and a browser revalidating a cached byte range then gets the whole
43 MB archive instead of a 206. `docs/design/mapping.md` has the measurement.

**Setting `Post.Type` costs the post its Elasticsearch/OpenSearch matches**,
its auto-translation, its embeds and its slack-style message attachments. Its
FILE attachments survive: they are drawn outside `PostBodyAdditionalContent`.
This line said "file attachment list" and a card grew a download link on the
strength of it. The inline map,
the Cursor on Target card and the GeoJSON card are the only three things that do
it; `EnableLocationMapInline`, `EnableCot` and `EnableGeoJSON` are how an install
opts out of each.

**A stamped post is never decorated, and the stamp is atomic.** Two formats
stamp: Cursor on Target, then GeoJSON. Each declares its own recover as its
first statement, spanning its own source-finding, filestore call and parse,
because `decoratePost` calls them from outside `decorateMessage`'s recover and
there is no other one on that path.

CoT is tried first and wins, because the card renders the text around an event
as plain text and a decorator link written into that text could not render
there. The same is true of the GeoJSON card. The type and
the props are committed together on a clone, after the whole props map has been
measured against `PostPropsMaxUserRunes`: a half-stamp costs the post its search
matches forever, and props sized off the event alone can have the server refuse
the post outright.

**A style an author wrote passes two gates before it is paint.** Go validates a
color to a hex triple and a width, opacity or marker size to its own range;
`styleOf` validates every one of them again in the webapp, and a shape that
fails carries nothing so the theme decides. The second gate is load-bearing:
`Number('')` is 0 and `Number('1e999')` is `Infinity`. Widths are refused, never
clamped, because a clamp draws what the document did not ask for while claiming
to honor it. The map's style tests read the FEATURE property, never the layer's
paint: the paint is a data-driven expression and reads the same whatever the
author supplied. `marker-symbol` is listed and not drawn, because there is no
sprite to point it at.

**The unlabeled GeoJSON spellings ship off.** A `json` fence, a `.json` file and
a bare object are read only with `EnableGeoJSONUnlabeled`, because stamping is
permanent and ordinary JSON is pasted into chat constantly.

**A map shape carries its rings, and a point is a marker.** `geometry-fill` and
`geometry-outline` render no point, and a single-ring polygon paints a hole as a
solid island. `MapShape` carries its own color, so `paintGeometry` is a
`coalesce` over the feature property rather than a scalar, and `styleOf` is the
gate the scalar used to be.

**Longitudes are unwrapped across the whole overlay, never per shape.** Two
shapes either side of the antimeridian each unwrap to themselves and union to a
359 degree box, which slips under `spansTheWorld`'s 360 test and frames the
planet backwards.

**Recognition is narrow where a false positive is permanent.** GeoJSON reads a
fence labeled `geojson` and a `.geojson` file, never `json` or `.json`: a wrongly
stamped post loses its search matches and its embeds forever, with no way for
the author to undo it, and ordinary JSON is pasted into chat constantly. Two tests assert the ambiguous spellings are ignored, so widening
this has to change a test rather than slip through.

**The strip clears every stamped props key on every post, not just the one
matching the post's type.** The commit copies existing props forward, so a
forged sibling blob would otherwise reach stored props permanently.
`custom_tf_location` is deliberately outside the table.

**`plugin.json` may not contain a backtick.** Both generated manifests embed it
inside a literal a backtick terminates, so one breaks the Go and the webapp build
at once, hundreds of lines from the cause.

**The post size limit cannot be read from the API.** `decoratePost` uses the
floor (`safePostRunes`), and `examples` measures every message against the same
floor before it writes any of them, refusing the whole run rather than posting
some of it.

**Every user-facing failure and every `p.API.Log*` call carries a `TF-NNNN`.**
Adding one is four edits that go together: the constant, the `AllCodes` entry,
the call site, and a row in `public/help/error-codes.html`.

**Anchor ids in `public/help/` are a contract**, and those pages must stay
light-only and self-contained so they render air-gapped. The only script they
may load is their own `copy.js`, which enhances and never enables: every page
must still work with scripting off.

### Cross-language sync points

Go and TypeScript hold duplicate copies of a few things, and Go and the map
generator hold one more, each guarded by a test that fails in Go when either
side moves alone. Change both halves together.

| Duplicate | Guard |
|---|---|
| Canonical token shapes, band/column/row classes, area alphabets | `webapp_sync_test.go`, `TestWebappAreaAlphabetsMatch` |
| The `Conversion` payload: names, **types** and order | `webapp_sync_test.go` |
| The row catalog: ids, labels, order | `TestWebappRowCatalogMatches` |
| The format id list | `TestWebappFormatListMatches` |
| The `/features` payload | `TestWebappFeatureShapeMatches` |
| The `/airport` payload | `TestWebappAirportShapeMatches` |
| The 30 minute cache TTL | `TestWebappCacheLifetimeMatches` |
| The `data-maps` attribute and its tokens | `TestWebappMapSurfaceAttributeMatches` |
| The seam zoom: `seamZoom` and `SEAM_ZOOM` in `map/span.ts` | `TestSeamZoomMatchesTheWebapp`, `TestDetailPackagesStartAtTheSeam` |
| The detail layer set: `DETAIL_SOURCE_LAYERS` in `map/maplibre.ts` | `TestArchiveCarriesEveryLayerTheStyleDraws`, and `style.spec.ts` holds the built style to the same list |
| The package name grammar: `packageNamePattern` and `PACKAGE_NAME` | `TestWebappPackageNameGrammarMatches` |
| The same grammar again, as the `case` in the Makefile's bundle guard | `TestBundleGuardAcceptsExactlyWhatDiscoveryDoes`, which compares behavior rather than text and holds the guard to `LC_ALL=C` |
| The `data-packages` attribute and its separator | `TestWebappPackagesAttributeMatches` |
| The CoT post type, props key and props version | `TestWebappCotPostTypeMatches` |
| The GeoJSON post type, props key, props version and source kinds | `TestWebappGeoJSONPostTypeMatches` |
| The GeoJSON `kind` vocabulary and its order | `TestWebappGeoJSONKindsMatch` |
| The GeoJSON panel's hideable sections: ids, labels, order | `TestWebappGeoJSONSectionCatalogMatches` |
| The GeoJSON props shape, walked rather than scraped | `TestWebappGeoJSONShapeMatches` |
| The GeoJSON reader truncating nothing | `TestWebappGeoJSONTruncatesNothing` |
| The CoT props shape: every key Go writes and the webapp reads | `TestWebappCotShapeMatches`, on a fixture built from `cot.FixtureDetail()` |
| The `<detail>` extension registry: every key it can write | Same test, plus `TestEveryRegisteredKeyIsWrittenFromTheFixture` |
| The CoT semantic classes | `TestWebappCotClassesMatch` |
| The CoT panel's hideable sections: ids, labels, order | `TestWebappCotSectionCatalogMatches` |
| The caps a forged blob is held to: vertices, checklist kinds, events | `TestWebappBlobCapsMatch` |
| The source kinds a stamped post may name | `TestWebappCotPostTypeMatches` |
| The processing path, which is an array rather than a rendered string | `TestWebappReadsTheProcessingPath` |
| A checklist, counted by element name rather than decoded | `TestWebappReadsTheChecklist` |
| The package list's 60 second lifetime | `TestWebappPackageCacheLifetimeMatches` |
| The map schema: `mapSchemaVersion` and `schemaPrefix`, and `MAP_SCHEMA` and `SCHEMA_PREFIX` in `build/maposm/build.sh` | `TestMapSchemaMatchesTheGenerator` |
| Rendering fixtures | `format_test.go` and `format.spec.ts` hold the same table |
| Zone ordering and its tiebreak | Both sides assert the same London/Reykjavik pair |

The token grammar itself is Go-only, so the two sides cannot drift on it.

## Coding conventions

- **Do not comment the code.** No prose comments in new or modified code, and
  delete the ones in code you touch. This overrides "match the surrounding
  style", because much of this repository predates the rule. Keep directives
  (`//go:embed`, `//go:generate`, `//nolint:...`, `//go:build`, `// #nosec`,
  `// eslint-disable-*`, `// @ts-expect-error`) and generated-file and license
  headers.
- **Rationale goes in `docs/design/`**, in the file for that area, and it goes
  in before the comment comes out rather than after. Write it there when a
  comment would have recorded a measurement, a defect that caused the current
  shape, or a contract a later change would silently break.
- **Before deleting a comment**, check whether it is the only record of one of
  those three. Move the content into the design note first, and ask rather than
  dropping it quietly. Do not strip comments from code you are not otherwise
  touching.
- Carry the rest in the code: name things so the intent reads, give a magic
  number a named constant, extract a named function rather than heading a block
  with a comment, and put invariants in test names
  (`TestRoundToNormalizesNegativeZero`, `TestGEOREFIsLongitudeFirst`).
- Never use em dashes anywhere, including commit messages. A test enforces it.
- Keep the plugin minimal. Avoid abstractions the code that exists today does
  not need.
- Server: follow Mattermost plugin API conventions, and log through
  `p.API.LogError`/`LogWarn`/`LogInfo`.
- Webapp: functional React components with hooks.
- Tests that want "every row", "every format" or "every code" must read the
  catalog rather than list it.

## Build and test

- `make dist` builds the bundle. `make check-style` lints both halves.
- `make test` runs the tests and depends on `map-data-check`, which fails on a
  basemap that is not what the committed source produces. That ordering is
  load-bearing: the webapp's digest check is skipped on plain-HTTP origins, so
  drift would otherwise surface only on an HTTPS install as every panel
  reporting that the map could not be loaded.
- `make map-data` / `make map-data-check` regenerate the country polygons and
  fail on stale artifacts. `make map-tiles` and `make airport-data` are one-off
  generators in the test path of nothing.
- `make coverage` merges backend and frontend summaries. It passes
  `-coverpkg=./server/...` (without it the shared page shell reads as 0%) and
  `-short`, which skips the generated corpus sweeps in `location`. Anything else
  slow enough to need that should get `testing.Short()` rather than a bigger
  timeout. The sweeps run in full under `make test`, which is what CI gates on.
- Local stack: `make docker-setup` (Mattermost plus PostgreSQL on `:8065`,
  `admin`/`password`), `make deploy` to install into it, `make deploy-local` for
  your own server, `make docker-logs`/`docker-reset`/`docker-stop`/`docker-down`,
  and `make nuke` to tear everything down.

## Commits and releases

Releases are automated with **release-please** driven by
[Conventional Commits](https://www.conventionalcommits.org/). Details in
[`docs/RELEASING.md`](docs/RELEASING.md).

- Write conventional subjects, on commits and on PR titles, since PRs
  squash-merge. `feat:` bumps minor, `fix:`/`perf:`/`deps:` patch, `feat!:` or a
  `BREAKING CHANGE:` footer bumps minor while pre-1.0
  (`bump-minor-pre-major`). `chore:`/`docs:`/`test:`/`refactor:`/`style:`/
  `build:`/`ci:` do neither and stay out of the changelog. A `Release-As:`
  footer sets the version directly.
- **Do not hand-edit `plugin.json`'s `version` or `CHANGELOG.md`.**
  release-please owns them through its Release PR.
- A release ships when the maintainer merges the open "chore(main): release
  X.Y.Z" PR.

## CI and security

Workflows: `pr.yml` (style/test/build), `security.yml` (SBOM plus Grype plus
CodeQL into Code Scanning), `release-please.yml`, `release.yml`. Everything is
reproducible locally, so verify with `make check-style && make test`,
`make sbom-audit`, `make codeql-analyze && make security-gate`, and `make
release` for the full tagged pipeline.

Suppress a false-positive CVE in `.grype.yaml` with a documented reason, never
blanket-ignore, and note that suppression is not available for anything that
ships and runs in the reader's browser (MapLibre): the process there is upgrade
or pin. See [`docs/SECURITY.md`](docs/SECURITY.md).

GitHub Actions are pinned to full commit SHAs with a `# vX.Y.Z` comment. Resolve
the tag to its SHA when adding or bumping one, and keep the comment accurate.
Never use floating tags.


## Dependency Licensing

The plugin bundle ships to customers, so **no copyleft dependencies in
anything that ends up in the bundle** (Go modules in `go.mod`, npm packages in
`webapp/package.json`, or vendored code under `public/`).

- Forbidden: GPL (v2/v3), AGPL (v3), SSPL, CC BY-SA, and other strong or
  network copyleft licenses.
- Weak copyleft (LGPL, MPL 2.0, EPL) is off limits by default. Ask before
  adding one.
- Preferred: MIT, BSD-2/3-Clause, Apache 2.0, ISC, Unlicense, CC0.
- Check the license before adding any dependency, including transitive ones.
  Unclear or unstated license means treat it as forbidden and ask.
- `make license-check` enforces this from the SBOMs, and `make sbom-audit`
  runs it alongside the CVE scan. Both run in CI on every PR, so a copyleft
  dependency fails the build rather than being caught by review.
- An unavoidable exception goes in `.licenses.json`, keyed to one component and
  one license, with a reason. `docs/SECURITY.md` explains what is already there
  and why.
- `make bundle` writes `THIRD-PARTY-NOTICES.txt` into every bundle from the
  dependencies' own license files, and fails on one that ships no license text.
  Adding a dependency that publishes none means recording what it does declare
  under `noticeFallbacks`.
- If a copyleft library seems unavoidable, present alternatives and tradeoffs
  instead of adding it.

Copyleft is fine in tooling that never ships: scripts under `scripts/`, dev
containers, CI workflows, and test-only tooling that stays out of the bundle.

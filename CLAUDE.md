# Tactical Fusion Plugin Guidelines

## Overview

Mattermost Tactical Fusion enriches conversations with mission-relevant context:
geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational
information. The server is Go, the webapp TypeScript/React.

Shipped today: the decorator framework and three decorators, DTG, Location and
Airfields, plus the bundled offline map. The rest is not implemented.

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
| `http.go` | `ServeHTTP`: `/decorate/<type>`, `/map`, `/api/v1/*`, and the session gate |
| `api.go` | Authenticated JSON API: `/preferences`, `/convert`, `/features`, `/airport` |
| `preferences.go`, `preferences_cache.go` | Per-reader KV store and its cluster-aware cache |
| `command*.go` | The `/tactical-fusion` slash command and its example builders |
| `errcode/` | The `TF-NNNN` catalog |
| `decorators/` | Framework: registry, tagger, boundary guard, shared page shell |
| `decorators/dtg/` | Date-time groups and RFC 3339 timestamps |
| `decorators/location/` | Coordinate grammars, geodesy, MGRS, rendering, conversion; `mapdata/` holds the generated country polygons |
| `decorators/airport/` | ICAO airfields; `data/` holds the embedded CSV and its provenance |

### `webapp/`

| Path | What lives there |
|---|---|
| `src/index.tsx` | `initialize()`, registration, the disposer list run by `uninitialize()` |
| `src/decorators/` | Framework: registry, click handler, styles, selection store, theme, `Tooltip` |
| `src/decorators/{dtg,location,airport}/` | Panels, hovers, and per-decorator clients |
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
| [`docs/design/mapping.md`](docs/design/mapping.md) | The vector basemap, the OpenStreetMap detail tier and its seam, detail map packages, `PageStatic` vs `PageMapping`, the page bundle, zoom numbers, the country lookup, `Conversion`, the map page, the panel map, turning maps off, the map under a post |
| [`docs/design/preferences.md`](docs/design/preferences.md) | The KV store, both caches, the location hover, the location rows, the zone picker and ordering |
| [`docs/design/admin-settings.md`](docs/design/admin-settings.md) | The twenty switches, the two map-package settings, the four sections, why `EnableLocationUTM` ships off |
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
recognise is a corruption bug. Widen it only with a regression test per
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
and `connect-src 'self'` and makes escaping the only defence on a route that
echoes author text. `ScriptSrc` must be relative.

**Setting `Post.Type` costs the post its Elasticsearch/OpenSearch matches**,
its auto-translation and its embeds. Only the inline map does it, and
`EnableLocationMapInline` is how an install opts out.

**The post size limit cannot be read from the API.** `decoratePost` uses the
floor (`safePostRunes`), the slash commands use the default and
`example-details` retries smaller on the first post.

**Every user-facing failure and every `p.API.Log*` call carries a `TF-NNNN`.**
Adding one is four edits that go together: the constant, the `AllCodes` entry,
the call site, and a row in `public/help/error-codes.html`.

**Anchor ids in `public/help/` are a contract**, and those pages must stay
static, light-only and self-contained so they render air-gapped.

### Cross-language sync points

Go and TypeScript hold duplicate copies of a few things, each guarded by a test
that fails in Go when either side moves alone. Change both halves together.

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
| The `data-packages` attribute and its separator | `TestWebappPackagesAttributeMatches` |
| The package list's 60 second lifetime | `TestWebappPackageCacheLifetimeMatches` |
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

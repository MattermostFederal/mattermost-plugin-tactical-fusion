# Cyber context: an indicator decorator with air-gapped enrichment

## Context

Incident and threat traffic in this audience's channels is full of indicators:
an IP address, a file hash, a CVE, a CWE, an ATT&CK technique id. Today none of
them decorates. A reader who wants to know whether `CVE-2024-3094` is in CISA's
Known Exploited Vulnerabilities list, which AS owns `203.0.113.7`, or what
`T1059.001` is, leaves the conversation to look it up, and on an air-gapped
network has nowhere to go.

This plan adds a fourth decorator, `cyber`, on the same framework the DTG,
Location and Airfield decorators use: a token in a message becomes a link, the
link opens a hover, a sidebar panel and a standalone page, and the enrichment
behind it comes from datasets that ship with the plugin or sit in a directory
the operator syncs, never from the internet. The datasets are built the way the
map packages are built: a generator under `build/` turns upstream sources into
the on-disk shape the plugin reads, small ones ship in the bundle, large ones
ship as release assets and are dropped into a configured directory.

Decisions already taken with the user:

- **Domains and URLs are deferred** to a later phase. A bare `https://` URL is
  already a protected span in the tagger, and a bare `example.com` is a
  permanent false-positive risk in ordinary chat.
- **IP enrichment reads both** the generator's TSV range files and vendor
  `.mmdb` files (GeoLite2, DB-IP), the latter through one ISC-licensed pure-Go
  dependency.
- **No external providers.** This is for air-gapped installs; everything comes
  from datasets built into the plugin or synced by the operator.
- **Datasets reach the server through an admin-synced directory plus a fetch
  tool**, exactly the `LocationMapPackagesDir` shape.

## Phase strategy

| Phase | Focus |
|---|---|
| **Phase 1 (this plan)** | The five indicator kinds, embedded ATT&CK and CWE catalogs, file-backed CVE, KEV, IP and watchlist datasets, the generator, panel, hover, page, API, prior-mentions pivot, admin switches, bridge, slash command, help, design note |
| Phase 2 | Domains and URLs (defanged and labeled forms first), CIDR and ASN kinds, a System Console dataset uploader like `PackageUploader`, hideable panel sections in preferences, CVE to ATT&CK mappings |
| Phase 3 | ATT&CK software and group ids, YARA/Sigma rule ids, a `/tactical-fusion ioc` subcommand |

## Design principles

| Concern | Approach | Reference |
|---|---|---|
| Recognition | Narrow. A kind with an explicit prefix (`CVE-`, `CWE-`, `T1059`) is recognized by shape; a bare shape (IP, hash) is recognized only where the shape is unambiguous; catalog-backed kinds validate by lookup | `airport.go:131`, `docs/design/airfields.md` |
| Boundary | `decorators.BoundaryOK` on the whole match; trailing punctuation and `:port` consumed by the pattern outside `ReplaceGroup` | `airport.go:93-122` (the `//` trick), `boundary.go` |
| Separators | `[ \t]*` never `\s*` (no labels in Phase 1, so this only matters for the CVE/CWE hyphen, which is literal) | `location.go:341` |
| URL | `k` (kind) and `v` (canonical value) only. Every route re-derives and requires `Parse(v)` to yield `k` again | CLAUDE.md "A link may never disagree with itself" |
| Decoration is a function of the build | `Parse` reads only embedded catalogs and `net/netip`. It never touches the dataset directory, so the same message decorates the same way on every node and every day | `hooks.go`, "Decoration rewrites the stored message" |
| Enrichment is read at render | Datasets are admin state read fresh per request, the way `Decorator.Maps` is; a link keeps resolving after a dataset is removed | `docs/design/admin-settings.md` "map switches are read at render" |
| Rendering | Once, in Go. The wire carries rendered strings; no `format.ts` | `airport/format.go`, `Conversion` |
| Page | `PageStatic`, no script, local enrichment only, never per-reader data | `decorator.go:150-167` |
| Per-reader data | Prior mentions live on `/api/v1/cyber/mentions`, never on the page and never on the bridge | `docs/design/bridge.md` |
| Datasets on disk | Sorted TSV, header stamped with a schema, looked up by binary search over the file with `ReadAt`; no in-memory index, no goroutine, a 5 second rescan like map packages | `server/packages.go` |
| Post path cost | Nothing on the message hook opens a file | CLAUDE.md invariants |

## Reference patterns

- `server/decorators/airport/airport.go` the four-method decorator, `ReplaceGroup`, `Extract`, `RenderPage` answering 200 with a note for an unknown value.
- `server/decorators/airport/data.go` `//go:embed`, eager parse, panic at init on bad data, `TestBadEmbeddedDataPanicsAtInit`.
- `server/decorators/airport/format.go` `Describe` shared by page and API.
- `server/api.go:310-360` `serveAirport`: GET only, shape check first, `Cache-Control: private, max-age=300`, discriminated response.
- `server/packages.go` directory discovery: bundle dir plus configured dir, later overrides earlier by name, name whitelist, header validation, `warnOnce`, 5 second cache keyed on the dir list.
- `server/plugin.go:100-110` `airportFormats()` ANDing the parent switch in Go.
- `server/bridge.go:199-253` the three switch arms a new type needs.
- `server/command_examples.go:37-82` `exampleSetOrder` and `exampleSets`.
- `webapp/src/decorators/airport/airport.ts` the module cache: `CACHE_TTL_MS`, one in-flight promise, `failed` never cached, five states.
- `webapp/src/decorators/airport/AirportPanel.tsx:316` swapping the sidebar in place through `setSelection`.
- `webapp/src/decorators/airport/AirportHarness.tsx` and `Airport.pw.tsx` the component test shape.
- `server/airport_sync_test.go` scraping `types.ts` interfaces by name, type and order.
- `server/decorators/airport/data/README.md` the provenance template.
- `build/airportdata/main.go` a stdlib-only generator inside the shipping module.

## Requirements

- [ ] `CVE-2024-3094`, `cwe-79`, `T1059.001`, `TA0002`, `203.0.113.7`, `203.0.113.7:443`, `2001:db8::1` and a 32/40/64 hex digit run decorate in ordinary prose.
- [ ] The stored link label is the author's token verbatim; `v` is the canonical form (upper-case ids, `netip` string form, lower-case hex).
- [ ] `T9999`, `CWE-99999`, `1.2.3.4.5`, `1.2.3.4/24`, `01.2.3.4`, `0.0.0.0`, `127.0.0.1`, `::`, `::1`, `12:30:45`, a MAC address, a 33 hex digit run, and any token inside a fence, code span, link, `@mention`, `~channel` or `#hashtag` do not decorate.
- [ ] Hover shows one line; the panel shows the enrichment, the watchlist verdicts, the related-entity pivots and prior mentions in this team; the page shows everything but prior mentions with no JavaScript.
- [ ] Every enrichment answer names its dataset and the dataset's generation date, and says in words when no dataset is installed.
- [ ] A `.tsv` dropped into `CyberDatasetsDir` is read within a few seconds without a restart; a file with a wrong schema stamp is skipped and logged once.
- [ ] `EnableCyber` and the five per-kind switches govern decoration only; links already written keep resolving.
- [ ] `make cyber-data` reproduces every committed and bundled dataset from pinned upstream sources on a connected host; a clean air-gapped checkout builds and tests with nothing run first.
- [ ] No copyleft dependency; every dataset's licence and provenance recorded; GeoLite2 never redistributed.

## Out of scope (Phase 1)

- Domains, URLs, email addresses, CIDR ranges, AS numbers as tokens.
- Any network call from the server.
- A System Console uploader for datasets (drop-in directory only).
- Hideable panel sections in reader preferences.
- Reverse DNS, WHOIS, passive DNS.
- Inline post rendering or a custom post type.

## Technical approach

### 1. The grammar (`server/decorators/cyber/grammar.go`)

Five kinds, each its own `Pattern`, each with `ReplaceGroup: 1`, `Extract` group 1, and `Boundary: decorators.BoundaryOK`. Trailing sentence punctuation and a port are matched inside the pattern, after group 1, so the guard looks past them and the text keeps them:

| Kind | Pattern (group 1 is the token) | Validation in `Parse` | Canonical `v` |
|---|---|---|---|
| `cve` | `((?i:CVE)-\d{4}-\d{4,7})[.,;:!?]?` | year 1999 or later | upper case |
| `cwe` | `((?i:CWE)-\d{1,5})[.,;:!?]?` | **lookup** in the embedded CWE catalog | upper case, no leading zeros |
| `attack` | `(T\d{4}(?:\.\d{3})?\|TA\d{4})[.,;:!?]?` | **lookup** in the embedded ATT&CK catalog | as written (already upper) |
| `ip` v4 | `(\d{1,3}(?:\.\d{1,3}){3})(?::\d{1,5})?[.,;:!?]?` | `netip.ParseAddr`, `Is4`, not unspecified, not loopback (netip refuses leading zeros) | `Addr.String()` |
| `ip` v6 | `([0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7})(?:%[\w.\-]+)?[.,;:!?]?` | `netip.ParseAddr`, `Is6`, not unspecified, not loopback | `Addr.String()` |
| `hash` | `(?:(?i:sha256\|sha1\|md5)[ \t]*[:=][ \t]*)?([0-9A-Fa-f]{64}\|[0-9A-Fa-f]{40}\|[0-9A-Fa-f]{32})[.,;:!?]?` | shape, and a label that names the wrong length declines | lower case |

Why the consumed tail works: `findCandidates` hands `Boundary` the runes outside the **whole match** (`tagger.go:561-566`) while `ReplaceGroup` narrows the claim and the rewrite (`tagger.go:580-591`, `:632-648`), and `FindAllStringSubmatchIndex` resumes after a match whether or not it was later rejected. So `1.2.3.4.5` matches `1.2.3.4.` with `5` after it, the guard refuses, nothing is claimed and the lone `5` cannot match. `1.2.3.4:443` matches through the port, the guard sees the space, only `1.2.3.4` is rewritten. `1.2.3.4: connection refused` consumes the colon and decorates, which is the log line this audience pastes most and the case DTG had to write its own guard for. `1.2.3.4/24` stops at the address, the guard sees `/` and refuses: CIDR is declined in Phase 1 rather than half-linked. A 33 digit hex run matches its first 32 and the guard sees a hex digit after. The port and the punctuation are always one rune the guard already required to be non-word, so the next token on the line is unaffected; a two-tokens-per-line test per kind pins it.

The hash label is matched and kept, not consumed: `sha256:e3b0...` would otherwise decline on the `:` before the digest, and Docker digests are the commonest hash spelling here. It sits inside the match and outside group 1, so the guard looks past it and the text keeps it. A label on the wrong length (`md5:` before 64 digits) declines in `Parse`.

IPv6 candidates are permissive and `Parse` is the gate: `12:30:45`, a MAC address and `1:2:3` fail `netip.ParseAddr`; `std::vector`'s `d::` and a timestamp's `T12:30:45` fail the guard's letter test. `::` and `0.0.0.0` are unspecified and `::1` and `127.0.0.1` are loopback; all four are refused, because nothing can enrich them and a link over an empty panel is a permanent rewrite for nothing. A zone (`fe80::1%eth0`) is consumed outside group 1 so the guard looks past it and `v` stays unzoned. Two residual shapes survive and are accepted: an eight-group EUI-64 written with colons, and prose such as `cafe::babe`. IPv4-mapped and bracketed `[2001:db8::1]:443` forms are declined (the bracketed span is protected anyway). All of this goes in the declined table on `cyber.html` and in the design note.

No existing DTG or location pattern can claim any part of these tokens: every location bare pattern is `BoundaryOK`-guarded and every DTG pattern is `\b`-anchored, so neither can take a sub-span of a contiguous run, and no separator shape lines up (a DD pair needs a comma, RFC 3339 needs `T`, MGRS compact is upper-case and guarded on both sides). Register `cyber` last in `NewDefaultRegistry`; nothing depends on it winning a tie.

Two kinds validate by lookup, the airport rule: `T1059` is in the build's catalog and `T9999` is not, so the shape alone never writes a link to nothing. CVE cannot, because CVEs are minted daily and the catalog is not embedded; a CVE is recognized by shape and the page says when the installed dataset does not hold it. Hashes are shape only: a Git commit id is also a SHA-1 and is labelled as such rather than declined.

`Formats` has five bools (`IP`, `Hash`, `CVE`, `CWE`, `Attack`); `Patterns()` returns only the enabled kinds. `Type() = "cyber"`. `ParamKind = "k"`, `ParamValue = "v"`. `Parse` returns `{k: kind, v: canonical}`. A shared `Recognize(value string) (Kind, canonical string, ok bool)` is what `Parse`, the API and the page all call, so the `k`/`v` agreement check is one function.

### 2. The tagger: mentions, channel links and hashtags become protected spans

`findProtectedRanges` protects bare URLs because rewriting inside one destroys the reader's link. `@user`, `~channel` and `#hashtag` are the same class and are unprotected today (`tagger.go:93-120` has no expression for them). `#CVE-2024-1234` and `~ip-203-0-113-7` would otherwise be rewritten. Add three expressions to `inlineProtectedRes`:

```
(?:^|[^\w@])@[\w.\-]+
(?:^|[^\w~])~[\w.\-]+
(?:^|[^\w#])#[A-Za-z][\w.\-]+
```

The leading context keeps `~~strike~~` and `##` headings out of it, and an over-wide span costs only a decoration. One regression case per construct in `TestProtectedSpansAreNeverRewritten`, and a paragraph in `docs/design/decorators.md` beside the bare-URL one. This is a framework change and it reaches every decorator: a DTG written as `#091630Z` stops decorating, which is the safe direction. A grep of every decorate-path fixture finds no `@`, `~` or `#` directly before a token, so nothing existing flips.

Two edges to record: the consumed context rune joins the protected range (`findProtectedRanges` records the whole `m[0]..m[1]`), so `1.2.3.4,@bob` declines where `1.2.3.4 @bob` decorates; and `user@1.2.3.4` still decorates the address, because `r` is a word character and `@` is not a bad neighbor. Both are acceptable and both get a test so they are deliberate.

### 3. Embedded catalogs (`server/decorators/cyber/data/`)

Two files, `//go:embed`, parsed eagerly at init, panic on bad data, the airport shape:

- `attack.tsv`: `id, name, kind (technique/subtechnique/tactic), tactics (TA ids, comma separated), parent, platforms, summary, status (active/deprecated/revoked), url`. Built from `enterprise-attack.json` (STIX 2.1, MITRE ATT&CK terms of use, attribution required). Around 800 rows.
- `cwe.tsv`: `id, name, abstraction, status, summary, parents (ChildOf in view 1000)`. Built from the CWE research view CSV. Around 1,000 rows.

`summary` is the first sentence, capped at 300 runes by the generator, because the panel and page print it and the page echoes nothing it has not escaped. A `validText` whitelist like `airport/data.go:70-87` refuses autolink triggers and control characters in every field at parse.

Both are the same catalog on every node, which is what lets `Parse` validate by lookup.

### 4. File-backed datasets (`server/decorators/cyber/intel/`)

A pure package with no plugin API: given a list of directories it discovers, validates and reads datasets. `server/cyberdata.go` wires it to the plugin the way `packages.go` wires map packages.

**On-disk format.** One file per dataset, `<name>.tsv`, first line a stamp `#tactical-fusion-cyber/1<TAB><name><TAB><generated RFC 3339><TAB><source>`, then rows sorted **bytewise on the first field**, tab separated, no quoting (the generator refuses a tab or newline in any field). The stamp is what a wrong or stale file fails on (`CyberDataSchemaMismatch`), distinct from an unreadable one (`CyberDataUnreadable`).

| File | Key | Fields | Ships |
|---|---|---|---|
| `cve.tsv` | `CVE-YYYY-NNNN` | published, modified, cvss score, severity, vector, cwes, summary | release asset, around 70 MB |
| `epss.tsv` | `CVE-YYYY-NNNN` | score, percentile, model date | release asset, around 10 MB, kept apart from `cve.tsv` so its licence is judged on its own |
| `kev.tsv` | `CVE-YYYY-NNNN` | date added, due date, ransomware use, vendor/product, required action | bundled, around 1 MB |
| `ip.tsv` | start address as 32 hex digits (`Addr.As16()`) | end (same form), asn, as name, country, region, city | release asset, around 45 MB |
| `watchlist.tsv` | canonical indicator | kind, verdict, source, note, updated | operator only, never bundled |

Bytewise order on the key is what makes the reader's binary search and the generator's sort agree without either knowing the key's semantics, which is why IPv4 addresses are stored mapped into the 16 byte form and hex encoded: one file, one comparator, both families.

**Lookup.** Open the file once, binary search over byte offsets: seek to the midpoint, read forward to the next line start, compare the key, narrow. About 20 reads of a 4 KB block per lookup on `cve.tsv`; the page cache does the rest. For `ip.tsv` the search finds the greatest start not above the address and checks the end. No index is built, so a 70 MB dataset costs no heap and no warm-up. `watchlist.tsv` is small and may hold several rows per indicator, so it is read whole into a map on open.

**MMDB.** Any `*.mmdb` in the configured directory is opened with `github.com/oschwald/maxminddb-golang/v2` (ISC) and classified by its metadata `DatabaseType` (`GeoLite2-ASN`, `GeoLite2-City`, `GeoLite2-Country`, `DBIP-*`). Field precedence for an IP answer: MMDB city, then MMDB ASN, then `ip.tsv`, first source with a non-empty value wins per field. MMDB files are never bundled: the GeoLite2 EULA forbids redistribution.

**Discovery and caching (`server/cyberdata.go`).** `datasetDirs()` = `GetBundlePath()/assets/cyber` then `CyberDatasetsDir`, later overrides earlier by file name, the `packageDirs` shape (`packages.go:158-181`) including its `p.API == nil` branch and the `warnOnce` on a missing bundle path. Rescan on a 5 second TTL keyed on the dir list; unlike `packagesIn`, which reopens per call, the `*os.File` handles here live across calls and a file is reopened only when its size or mtime moves or the dir list changes. Bad files skip with `warnOnce`. `assets/` is copied into the bundle by the `bundle` target (`Makefile:312-318`) and Mattermost serves only the bundle's `public/` directory (`docs/SECURITY.md:253-257`), which is why bundled datasets live there rather than under `public/` and why the watchlist never may. The design note records that the "not served" claim is Mattermost's behavior, not something this repo can test.

### 5. Enrichment (`server/decorators/cyber/format.go`)

`Describe(kind, value, intel)` returns `Details`, the one struct both the page and the API print. Everything is a rendered string:

| Kind | Rows |
|---|---|
| `cve` | summary, published, last modified, CVSS `9.8 Critical` plus vector, EPSS `0.97 (99th percentile)`, KEV `Listed 2024-03-29, due 2024-04-19, ransomware use: unknown`, weaknesses (CWE ids, each a related link) |
| `cwe` | name, abstraction, status, summary, parents (related links) |
| `attack` | name, kind, tactics (related links to the `TA` entries), parent or sub-techniques (related links), platforms, status, summary, ATT&CK URL as text |
| `ip` | version, scope (`private`, `loopback`, `link-local`, `multicast`, `documentation`, `global` from `netip` predicates and the RFC 5737/3849 ranges), ASN and AS name, country, region, city, source dataset |
| `hash` | algorithm (`SHA-1 (also the shape of a Git object id)`), byte length |

Plus, for every kind: `Watchlist []Entry` (verdict, source, note, updated), `Related []Link{Kind, Value, Label}`, `Status string` (`Not in the CVE dataset generated 2026-09-01`, `No CVE dataset is installed`, `Watchlist: 2 entries`), and `Datasets []DatasetStatus{Name, Present, Generated}`.

The three status sentences are built here once so hover, panel and page cannot disagree.

### 6. The page (`server/decorators/cyber/page.go`)

`PageStatic`, no script, `Cache-Control: private, max-age=60`: shorter than the airport page's 300 because this page is a function of install state (the drop-in directory) as well as of the build, and a dataset an operator just replaced should show within about a minute. A malformed `k` or `v`, or a pair that does not round-trip, is 400 `CyberPageInvalid`; a well-formed value the datasets do not hold is 200 with the status sentence. Related links are `<a>` to sibling `/decorate/cyber?k=&v=` pages (`coordinateHref` precedent). The page never shows prior mentions.

`RenderPage` needs the datasets, so `Decorator` carries `Intel func() intel.Reader` beside `Enabled func() Formats`; nil means no datasets, which is what tests use. This is the `Maps` precedent: admin state read at render, not per-reader state.

### 7. The API (`server/api.go`)

`GET /api/v1/cyber?k=&v=`: 405 on non-GET, 400 `APICyberInvalid` on a malformed or disagreeing pair, otherwise 200 with `cyberResponse` and `private, max-age=60`. Answers with every switch off, because a link written while it was on must keep resolving, and never consults `EnableCyber*`. Wire shape, pinned by a sync test per interface:

```
cyberResponse {kind, value, label, status, cve?, cwe?, attack?, ip?, hash?, watchlist[], related[], datasets[]}
```

`GET /api/v1/cyber/mentions?k=&v=&team=<id>`: session required, `team` required and validated as a Mattermost id (`APICyberTeamInvalid`; an empty team never means "all teams"), `p.API.SearchPostsInTeamForUser(team, userID, {Terms: "\"<v>\"", IsOrSearch: false, Page: 0, PerPage: 20})`, `Cache-Control: no-store`. Answers `{value, mentions: [{post_id, channel_id, channel, create_at, snippet, permalink}], truncated}`; `snippet` is the message with markdown links collapsed to their labels and cut at 200 runes; `permalink` is `/<team name>/pl/<post id>` from `p.API.GetTeam`. `SearchPostsInTeamForUser` applies the reader's own channel permissions, which is why the older `SearchPostsInTeam` is never used. Whether every search backend tokenizes a dotted quad or a hyphenated id into a findable phrase is recorded in `docs/design/unverified.md`.

### 8. The webapp (`webapp/src/decorators/cyber/`)

- `index.ts`: `type: 'cyber'`, `fromParams` reads `k` and `v`, checks `k` against the kind list and `v` against that kind's shape regex (the literals are scraped by a sync test against `cyber.ShapeExpr(kind)` exported for that purpose), `summary` is the kind label, `style` a teal chip.
- `cyber.ts`: the airport cache verbatim in shape, keyed on `k:v`, `CACHE_TTL_MS`, five states.
- `mentions.ts`: a second client for the mentions route, keyed on `team:k:v`, same TTL, fetched by the panel only, never by the hover.
- `types.ts`: `CyberResponse`, `CveDetails`, `CweDetails`, `AttackDetails`, `IpDetails`, `HashDetails`, `WatchEntry`, `RelatedLink`, `DatasetStatus`, `MentionsResponse`, `Mention`.
- `CyberHover.tsx`: one line per kind (`9.8 Critical, in KEV`, `AS15169 Google LLC, US`, `SHA-256, watchlist: malicious`, `Command and Scripting Interpreter: PowerShell`, `Improper Neutralization of Input During Web Page Generation`), `null` for every state but `ready`.
- `CyberPanel.tsx`: header rows, an enrichment table, a Watchlist section, a Related section whose entries call `setSelection({type: 'cyber', payload})` in place, a Mentions section listing posts with the team's permalinks, and the datasets footnote. The current team id comes from a `currentTeamId()` accessor added to `selection.ts` beside `openRhs`, reading `store.getState().entities.teams.currentTeamId` through a narrow local type (no `mattermost-redux`; `@mattermost/types` is a type-only devDependency). An empty id skips the mentions fetch. Permalinks navigate through `window.WebappUtils.browserHistory` when present and fall back to a plain anchor, recorded as unverified.
- `CyberHarness.tsx` (the airport harness with a second `url.includes('/api/v1/cyber/mentions')` branch and its own reply prop and request counter), `Cyber.pw.tsx`, `index.spec.ts`, `cyber.spec.ts`, `mentions.spec.ts`, and a `selection.spec.ts` case for `currentTeamId()` with a fake store.
- One line in `registerBuiltinDecorators()`.

### 9. Admin settings

A seventh section, **Cyber context**, six switches and one path:

| Key | Type | Default |
|---|---|---|
| `EnableCyber` | bool | true |
| `EnableCyberIP` | bool | true |
| `EnableCyberHash` | bool | true |
| `EnableCyberCVE` | bool | true |
| `EnableCyberCWE` | bool | true |
| `EnableCyberAttack` | bool | true |
| `CyberDatasetsDir` | text | `""` |

`Plugin.cyberFormats()` ANDs each kind with `EnableCyber`. All on by default: every one trades a false positive that is merely noisy (a Git SHA labelled SHA-1, a four-part version number labelled an address) against a missed decoration, none is the confidently-wrong class `EnableLocationUTM` is. The help text says so in words for both.

Thirty-one switches across seven sections: extend `spellNumber` in `help_docs_test.go` (`:581-596`) with 31 to 40, and update the four tested phrases (`admin.html:46`, `help.html:187`, `docs/design/admin-settings.md:7`, `CLAUDE.md:83`) plus the untested "on by default" counts in `admin.html` and `README.md:111`. No backtick anywhere in `plugin.json`.

### 10. Bridge, command, errors, docs

- `bridgeclient/types.go`: `TypeCyber = "cyber"`. `server/bridge.go`: the `formatEnabled`, `parsesWithEveryFormat` and `bridgeInfo` arms. `bridge_test.go:545`: the count becomes 4.
- `server/command_examples.go`: a cyber set (`CVE-2021-44228`, `T1059.001`, `CWE-79`, `203.0.113.7`, the EICAR MD5 `44d88612fea8a8f36de82e1278abb02f`); `command_check.go` `whyNothingMatched` gains the cyber decline rules.
- `server/errcode/codes.go`: `APICyberInvalid`, `APICyberTeamInvalid`, `APICyberSearchFailed` (13000s), `CyberPageInvalid` (17000s), and a new `20000-20999 server/cyberdata.go` range for `CyberDataNoBundlePath`, `CyberDataUnreadable`, `CyberDataSchemaMismatch`, `CyberDataBadName`, `CyberDataMMDBUnreadable`; widen `TestCodesAreInAKnownRange` to `< 21000`, add the range to the package doc and a table to `error-codes.html`.
- `public/help/cyber.html` (new, listed in `helpPages`, added to every page's nav), plus `formats.html`, `admin.html`, `error-codes.html`, `troubleshooting.html`, `commands.html`, `help.html`, `panel.html`.
- `docs/design/cyber.md` carries every measurement and decision in this plan; `CLAUDE.md` gets the directory rows, the sync-test rows and one invariant paragraph.

### 11. The generator (`build/cyberdata/`)

Stdlib-only Go in the shipping module, like `build/airportdata/`, plus a `fetch-sources.sh` and `sources.lock` like `build/maptiles/`. `make cyber-data` runs it against `build/cyberdata/source/` (gitignored) and writes:

| Output | From | Licence |
|---|---|---|
| `server/decorators/cyber/data/attack.tsv` (committed) | `mitre-attack/attack-stix-data` `enterprise-attack.json` | MITRE ATT&CK terms of use |
| `server/decorators/cyber/data/cwe.tsv` (committed) | MITRE CWE research view CSV (`1000.csv.zip`) | MITRE CWE terms of use |
| `assets/cyber/kev.tsv` (committed, bundled) | CISA `known_exploited_vulnerabilities.json` | US Government work |
| `build/cyberdata/out/cve.tsv` (release asset) | NVD 2.0 yearly feeds `nvdcve-2.0-YYYY.json.gz` | NVD public domain |
| `build/cyberdata/out/epss.tsv` (release asset, or drop-in only) | FIRST `epss_scores-current.csv.gz` | **Verify at fetch time.** If FIRST's terms are not permissive, the generator still writes it but `cyber-release` does not attach it and the help page tells operators to fetch it themselves |
| `build/cyberdata/out/ip.tsv` (release asset) | iptoasn.com `ip2asn-combined.tsv.gz`, optionally DB-IP city lite CSV | PDDL; CC BY 4.0 with attribution |

`make cyber-release TAG=` attaches the release assets, the `map-release` pattern. `make license-check` reads SBOMs and never sees data files, so the data licence judgments live in the two READMEs and `docs/design/cyber.md`. Not a prerequisite of `make test`: the transform is filter and sort, and its drift is a missing row, visible and benign. `server/decorators/cyber/data/README.md` and `assets/cyber/README.md` record URL, retrieval date, licence, SHA-256 and the transform, the airport template. A test over the embedded catalogs, not the generator, holds them honest: every id matches the grammar, every field passes `validText`, pinned rows (`T1059.001`, `TA0002`, `CWE-79`), and every `parents`/`tactics` reference resolves inside the catalog.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Bare domains and URLs | Deferred | User decision; a live URL is already protected, a bare domain is a permanent false positive |
| Validate CVE by lookup? | No, by shape | The CVE list is not in the build and grows daily; a lookup would make decoration depend on which node and which day |
| Validate ATT&CK and CWE by lookup? | Yes | The catalogs are embedded, so the check is a function of the build, and it keeps `T9999` from becoming a link to nothing |
| `1.2.3.4` as a version number | Accepted, on by default | Same class as every other on-by-default switch: noisy, not confidently wrong. Leading zeros and octets above 255 decline through `netip` |
| Git SHA vs SHA-1 | Decorate, label honestly | They are the same hash; the panel says so and the mentions pivot is useful for either |
| CIDR, `ip:port`, ranges | Port consumed and kept; CIDR and `a-b` ranges declined | A link over half a CIDR misrepresents; a CIDR kind is Phase 2 |
| Where enrichment runs | Page and API, never `Parse` | Nothing on the post path may open a file |
| In-memory index vs on-file binary search | On-file | 70 MB of CVEs would cost tens of MB of heap and a warm-up; sorted bytewise keys make the search trivial and the generator owns the order |
| One `ip.tsv` for both families | Yes, 16 byte hex keys | One comparator; `As16()` maps IPv4 into the same order |
| Bundle `cve.tsv` and `ip.tsv`? | No, release assets | Bundle upload limits and the map-package precedent; `kev.tsv` is small enough to bundle |
| MMDB dependency | `maxminddb-golang/v2`, ISC | User asked for both formats; pure Go, one module, allowed licence |
| Watchlist verdict vocabulary | Free text, with `malicious`, `suspicious`, `benign` recognized for styling | A MISP export should load without editing |
| Prior mentions on the page | No | Per-reader data; the page is a pure function of its query |
| Loopback and unspecified addresses | Declined | Nothing can enrich them; a link over an empty panel is a permanent rewrite for nothing, and both families are treated alike so `127.0.0.1` and `::1` cannot disagree |
| Hash labels (`sha256:`) | Matched and kept, outside the replace group | The `:` would otherwise decline the digest; consuming the label would rewrite what the author wrote |
| EPSS in `cve.tsv` or apart | Apart, `epss.tsv` | Its licence is judged on its own; it also refreshes daily where CVE records do not |
| `SearchPostsInTeam` vs `SearchPostsInTeamForUser` | `ForUser` | The former ignores channel membership and would leak private channels |
| Protect mentions/hashtags in the tagger vs a cyber-only guard | Tagger | They are Mattermost link constructs, the same class as bare URLs; every decorator benefits |
| Preferences hideable sections | Phase 2 | Airport shipped without; adds two catalogs and two sync tests to an already large change |
| Console uploader | Phase 2 | The directory is the mechanism; the uploader is convenience |

## Files

### New

| File | Change |
|---|---|
| `server/decorators/cyber/{cyber,grammar,catalog,format,page}.go` | The decorator |
| `server/decorators/cyber/data/{attack.tsv,cwe.tsv,README.md}` | Embedded catalogs and provenance |
| `server/decorators/cyber/intel/{intel,tsv,search,ip,mmdb,watchlist}.go` | Dataset discovery, stamp validation, on-file binary search, IP range and MMDB readers, watchlist |
| `server/decorators/cyber/*_test.go`, `intel/*_test.go` with `testdata/` fixtures | Grammar, catalog integrity, describe, page, search, ranges, MMDB (a tiny fixture built with the library's writer or a committed test mmdb), watchlist |
| `server/cyberdata.go`, `server/cyberdata_test.go` | Plugin wiring, dirs, cache, logging |
| `server/cyber_sync_test.go` | Shape, type, kind list and shape-regex sync tests |
| `server/api_cyber_test.go` | Both routes |
| `assets/cyber/{kev.tsv,README.md}` | Bundled dataset |
| `build/cyberdata/{main.go,fetch-sources.sh,sources.lock,README.md}` | The generator |
| `webapp/src/decorators/cyber/{index,cyber,mentions,types}.ts`, `Cyber{Hover,Panel}.tsx`, `CyberHarness.tsx`, `Cyber.pw.tsx`, `*.spec.ts` | Sidebar, hover, clients |
| `public/help/cyber.html` | The feature page |
| `docs/design/cyber.md` | The design note |

### Modified

| File | Change |
|---|---|
| `server/decorators/tagger.go`, `tagger_test.go` | Mention, channel and hashtag protection |
| `server/plugin.go` | `NewDefaultRegistry` argument, `cyberFormats()`, `cyberIntel()` |
| `server/configuration.go`, `plugin.json` | Six switches, one path, one section |
| `server/api.go` | `/cyber` and `/cyber/mentions` |
| `server/errcode/codes.go`, `codes_test.go` | Codes and the 20000 range |
| `server/bridge.go`, `bridge_test.go`, `bridgeclient/types.go` | The fourth type |
| `server/command_examples.go`, `command_check.go` | Examples and decline rules |
| `server/help_docs_test.go` | `helpPages`, `spellNumber` |
| `webapp/src/decorators/index.ts`, `selection.ts` | Registration, `currentTeamId()` |
| `go.mod`, `go.sum` | `maxminddb-golang/v2` |
| `Makefile` | `cyber-data`, `cyber-release`, `cyber-fetch-sources` |
| `public/help/*.html` | Nav on every page; formats, admin, errors, troubleshooting, commands, panel, help |
| `docs/design/{decorators,admin-settings,help-and-errors,unverified}.md`, `CLAUDE.md` | Rationale, counts, tables |

## Tasks

1. [ ] Tagger: add the three protected-span expressions with a regression case each; run the whole `decorators` suite and the location, dtg and airport corpora unchanged.
2. [ ] `build/cyberdata/`: fetch script and lock; parsers for STIX, CWE CSV, KEV JSON, NVD 2.0 feeds, EPSS CSV, iptoasn TSV, DB-IP CSV; bytewise sort; stamp; field whitelist; write the two committed catalogs, `kev.tsv`, and the release assets (`cve.tsv`, `epss.tsv`, `ip.tsv`). Record each source's licence in the READMEs, EPSS's after reading FIRST's current terms. Commit `attack.tsv`, `cwe.tsv`, `kev.tsv` and the READMEs.
3. [ ] `catalog.go`: embed and parse both catalogs; integrity tests including the init panic.
4. [ ] `grammar.go`, `cyber.go`: the five patterns, `Recognize`, `Parse`, `Formats`; tagger tests for every row of the UX table below.
5. [ ] `intel/`: stamp reader, on-file binary search with fixtures (including a key at the first and last line, a missing key, an empty body), IP range lookup for both families, MMDB reader, watchlist map, dir precedence.
6. [ ] `server/cyberdata.go`: dirs, 5 second cache, size/mtime reopen, `warnOnce`, codes.
7. [ ] `format.go`: `Describe`, the status sentences, related links, `netip` scope classification.
8. [ ] `page.go` under `PageStatic`; page policy test; escaping test with hostile dataset rows.
9. [ ] `/api/v1/cyber` and `/api/v1/cyber/mentions`; tests for every handler rule, including the permission-scoped search fake.
10. [ ] Webapp clients, hover, panel, harness, component tests; `currentTeamId()`.
11. [ ] Sync tests: response shapes, `type: 'cyber'`, the kind list, each kind's shape regex.
12. [ ] `plugin.json`, `configuration.go`, `cyberFormats()`, `spellNumber`, the four counts.
13. [ ] Bridge arms and constant; count 4.
14. [ ] Command examples and check rules.
15. [ ] Help pages including nav on every page; error-code rows; troubleshooting rows.
16. [ ] `docs/design/cyber.md`, the CLAUDE.md rows, `unverified.md` entries (search tokenization, permalink navigation from the RHS, first-lookup latency on `cve.tsv`).
17. [ ] `make check-style && make test && make sbom-audit && make dist`.

## Risks

| Risk | Mitigation |
|---|---|
| A four-part version number decorates as an address | On-by-default with the cost stated in help text; `EnableCyberIP` turns it off; leading zeros and large octets decline |
| IPv6 candidates in ordinary text | `netip.ParseAddr` plus the guard; a table-driven test over times, MACs, ratios, `::` and C++ scope operators |
| Trailing punctuation consumed by the pattern breaks the next match on the line | Only one rune is consumed and the guard already required a non-word rune there; a two-tokens-per-line test per kind |
| Search backends tokenize indicators differently | Quoted phrase terms; recorded as unverified; the panel says "no mentions found" rather than implying none exist |
| A hostile dataset row reaches the page | `validText` at parse for catalogs; every dataset field HTML-escaped at render; a test with `<script>` and `](` in a fixture row |
| `cve.tsv` binary search on a file being replaced | Reopen on size/mtime change; a lookup on a half-written file fails closed with the unreadable code |
| Bundle size | Only `kev.tsv` and the two catalogs ship inside; the two large datasets are release assets |
| Licence friction | ISC dependency only; data licences recorded per file; GeoLite2 never redistributed |
| The tagger change flips an existing fixture | Run every corpus before and after; any flip is reviewed as a deliberate safe-direction change and recorded |

## UX summary

| Scenario | Behavior |
|---|---|
| `Patched CVE-2024-3094 today.` | `CVE-2024-3094` links; the `.` stays |
| `cve-2024-3094` | Links; label as written, `v=CVE-2024-3094` |
| `T1059.001` / `TA0002` / `T9999` | Links / links / nothing |
| `CWE-79` / `CWE-99999` | Links / nothing |
| `203.0.113.7:443` | The address links; `:443` stays |
| `1.2.3.4: connection refused` | The address links; `:` stays |
| `1.2.3.4.5`, `1.2.3.4/24`, `01.2.3.4`, `v1.2.3.4`, `0.0.0.0`, `127.0.0.1` | Nothing |
| `2001:db8::1` / `fe80::1%eth0` / `12:30:45` / `::` / `::1` | Links / links (zone stays) / nothing / nothing / nothing |
| 32, 40, 64 hex digits / `sha256:` then 64 / `md5:` then 64 / 33 hex digits | Links / links, label stays / nothing / nothing |
| `#CVE-2024-3094`, `~cve-2024-3094`, in a fence, in a link | Nothing |
| Hover a CVE | `9.8 Critical, in KEV` or the status sentence |
| Click | Panel: enrichment, watchlist, related pivots, prior mentions in this team |
| Same link on mobile | Page with everything but mentions, no script |
| No datasets installed | Panel and page say so per dataset, links still resolve |
| Drop a newer `cve.tsv` into the directory | Read within 5 seconds |
| `EnableCyber` off | New messages stop decorating; existing links keep resolving |

## Verification

- `make check-style && make test`: Go and webapp suites including the new sync tests, the tagger regression cases and the Playwright component tests.
- `make sbom-audit`: the ISC dependency passes `license-check`; `THIRD-PARTY-NOTICES.txt` carries its licence.
- `make dist`: bundle carries `assets/cyber/kev.tsv`; `make cyber-data` on a connected host reproduces the committed files byte for byte.
- `make docker-setup && make deploy`: post each UX row; hover and click each link; open the page with `_page=1`; drop `cve.tsv` and `ip.tsv` into a directory, set `CyberDatasetsDir`, confirm enrichment appears without restart; drop a `watchlist.tsv` and confirm verdicts; confirm the mentions section lists the posts just made and nothing from a private channel the reader is not in.

## Open questions

- Whether the mentions search should also run across every team the reader belongs to (one call per team) rather than the current team only. Current team is recommended for Phase 1.
- Whether `assets/cyber/kev.tsv` should be refreshed by release-please cadence or by hand; recommended: by hand with `make cyber-data` before a release, recorded in `docs/RELEASING.md`.

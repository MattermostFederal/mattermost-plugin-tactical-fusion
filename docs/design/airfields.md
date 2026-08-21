# Airfields

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Airfields

The airport decorator recognizes an **ICAO airfield code behind a USMTF field
label** and renders the field behind it. It is the one decorator here whose
position is **looked up rather than computed**, which is exactly why
`location.go` has always refused the four labels it claims: `ICAO`, `LOC`,
`DEPLOC` and `ARRLOC`. They stay refused there; this claims them.

**It draws the airfield's position and links to the readings. It prints no
readings of its own.** An airfield resolves to a position, and the location
decorator already renders a position eleven ways with copy buttons and whichever
rows the reader chose to keep. A second table here would say the same thing
worse, would not honour those choices, and would drift. So the panel draws the
**map** under the airfield details, and **the Place value is the link**: it
hands the coordinate to the location panel in place through `setSelection`, and
the page's Place is an `<a>` to the coordinate page.

The link is the value rather than a line beneath the table because the place is
where the airfield is, so it is the thing to follow to see where that is; a
separate "Location" line under it was a second way to say the same thing. The
place keeps its copy button either way, since copying a value and following it
are different acts.

**That makes Place load-bearing rather than decorative.** An airfield with no
place would be one whose position a reader cannot reach on either surface. None
of the 19,012 has an empty one, because the country is always present, and
`TestEveryAirfieldHasAPlaceToHangTheLinkOn` is what stops a refreshed database
introducing one silently.

The map is `LocationMap`, the location decorator's own component, not a second
one: two implementations of a projection and a palette are two things that can
disagree, and this is where a reader looks first. The payload it draws from is
built through `location.fromParams` on the `(f, v)` pair the server sent, so the
sidebar draws exactly what a click on a coordinate link would have drawn.

**No conversion is needed to draw it.** `viewFor` reads latitude and longitude
out of the parsed `dd` token, so the map has everything it needs the moment the
airfield lookup lands and `pending` is a constant false. Only the region comes
from the server, and it is on the wire for one reason: it is the map's
accessible label, and since the Region row was retired that label is the only
place the country reaches a reader at all.

**This is a fifth `LocationMap` mount site, so it honours `mapPanel`** like the
other four. Off means nothing is fetched rather than nothing drawn, and gating
the mount site is what achieves that: the archive, the glyph ranges and the
library are reached from that component and nowhere else. Reusing the existing
switch rather than adding `EnableAirportMap` is deliberate, and the cost is
named: a map in the sidebar is a map in the sidebar, but `locationMaps()` ANDs
with `EnableLocation`, so an install that turns coordinate decoration off loses
the airfield map too.

**The standalone page draws no map and links instead**, because it is
`PageStatic` with no script. That is the one place the two airfield surfaces
deliberately differ, and it follows from the CSP choice rather than from an
oversight: a map there would need the page bundle, which needs `ScriptSrc`,
which is refused outside `PageMapping`.

**The identity of an airfield is its ident**, and the link carries one parameter,
`v`. Nothing derived travels in the URL, so a link cannot disagree with itself,
and there is no `r`: the ident is upper case only, so the author's text and the
canonical form are always identical.

### The grammar is label-only, and that is measured

Nothing is detected without a label, permanently. Of the 19,012 idents shipped,
**343 are English dictionary words**, and the collisions are not theoretical:
`FACT` is Cape Town International, `FAST` is Somerset East, `USCG` is
Chelyabinsk Shagol in **Russia**, `LIMA` is Torino-Aeritalia, `UNIT` is Tura
Mountain, `ETIC` is Grafenwohr Army Air Field. A bare grammar would rewrite
"USCG" in an ordinary sentence into a link pointing at a Russian airfield.

That is the `EnableLocationUTM` failure mode, a real code rendered as a
confidently wrong place rather than a false positive on text that was never a
code, at a far higher rate. So there is no bare pattern and there is no switch
that produces one.

**The label is upper case only**, where every location moniker is `(?i:...)`.
Location can afford any case because its tokens carry digits, so a lower-case
label in prose cannot reach one. An airfield ident is four letters, and
`loc: fast turnaround on the ramp` passes every guard and would link `FAST`.
USMTF field labels are upper case, so the narrowing costs nothing real. The
**ident** is upper case too, which additionally makes `v` self-canonicalizing:
there is no case to fold, so one airfield is one URL and the
`loc.Canonical() != v` invariant every other route holds is satisfied by
construction rather than by a check.

**The separator is `[ \t]*:` with nothing after the colon**, and both halves of
that are defects that were shipped.

`\s` is never used, for the reason `location.go` records: RE2's `\s` includes
`\n`, so a label ending a line would claim whatever started the next one, and
with 343 word-shaped idents "Divert to LOC:" above a line beginning "FAST" would
rewrite ordinary prose permanently into stored post text.

**Nothing may follow the colon either, and that was measured after the fact.**
The separator was `[ \t]*:[ \t]*`, and the argument for upper-case-only cites
`loc: fast turnaround on the ramp` as the collision it closes. It closes the
lower-case half and leaves the upper-case one, which is the half this audience
writes: military traffic is routinely all caps, and

    LOC: FAST TURNAROUND ON THE RAMP  ->  [FAST](...) TURNAROUND ON THE RAMP
    LOC: SITE ALPHA IS SECURE         ->  [SITE](...) ALPHA IS SECURE
    ICAO: LIMA TEAM MOVING            ->  [LIMA](...) TEAM MOVING

all rewrote stored text to point at a real but wrong airfield (`FAST` is
Somerset East, `SITE` a helipad in Sao Paulo, `LIMA` is Torino-Aeritalia). That
is the `EnableLocationUTM` failure mode this decorator's whole design exists to
avoid, one layer in, behind the label.

Dropping the trailing `[ \t]*` costs `ICAO: KIND` a decoration and costs a
genuine `DEPLOC:KIND` nothing, which is the trade this repository makes
everywhere: a missed decoration is a feature gap and a rewrite is corruption.
The leading `[ \t]*` stays, since a space *before* the colon cannot separate a
following word from the label.

**The residual is the no-space form.** `LOC:SITE` is still a word-shaped ident
behind a label, and it is not measured. What bounds it is that it reads as a
field rather than as a sentence. Whether `LOC` belongs in `monikerPrefixes`
beside the three unambiguous labels is open: `DEPLOC` and `ARRLOC` are flight
plan fields that do not occur in prose, and `ICAO` nearly as good, while `LOC:`
reads naturally as "location:".

`TestNoIdentIsReachedByCapitalizedProse` sweeps all 19,012 idents behind all
four labels in a sentence and requires every one to decline. It reports 76,048
rewrites against the old separator and zero against this one.

### The corpus tests used to skip in CI

All three sweeps in `corpus_test.go` run on every pull request now. Two of them
did not, and the way they failed is worth keeping: they read
`/usr/share/dict/words`, which exists on macOS and not on the GitHub Actions
runner image, and `pr.yml` installs no apt packages. So `t.Skip` fired on every
PR, silently. **The one measurement that could have failed never ran where
failing would have mattered**, which is how the separator defect above shipped.

Two separate fixes, because there were two separate problems.

**The word list is committed.** `data/words4.txt` is every four-letter English
word, 4,360 of them, from the BSD `web2` list whose 1934 copyright has lapsed.
It is embedded with `//go:embed` beside `airports.csv` and for the same reason:
a clean checkout must run `go test` with no network and no prior generator run.
`TestManyIdentsAreOrdinaryWords` still reports 343 of 19,012, which is the whole
argument for this decorator having no bare pattern, and it now reports it on
every PR.

**The behavioural sweep never needed a word list at all.** It asserted three
things (a bare ident does not decorate, a lower-case or mixed-case label does
not, an upper-case one does) and every one of them holds for *all* 19,012
idents, not only the word-shaped ones. Restricting it to dictionary words made
it 55 times narrower **and** gave it a dependency it did not need, which is what
made it skip. It sweeps every ident now, so it is both stronger and unskippable;
only the `-short` gate remains, and `make test` does not pass `-short`.

**Validation is by lookup, not by shape.** `Parse` returns `ok=false` for an
ident the build does not hold, which is what keeps `LOC: HOME` and `ICAO: FAST`
from becoming links. Four letters vouches for nothing, so the database is the
grammar.

### The `//` is matched, not permitted

A USMTF set line ends `//`, and `DEPLOC:KIND//` is the traffic this feature
opens with. `BadNeighbor` rejects `/` on both sides, so the line would decline;
loosening the guard to allow a trailing `/` would fix that and reopen something
worse, because `ICAO:KIND/foo` is path-shaped and rewriting the middle of a path
is the failure the guard exists for.

`Pattern.boundaryOK` is handed the runes flanking the **whole match** while
`ReplaceGroup` narrows only what is rewritten, so the terminator goes in the
pattern instead: `...([A-Z]{4})(?://)?`. `DEPLOC:KIND//` matches with the `//`
inside the match, the guard looks past it, and only the ident is linked.
`ICAO:KIND/foo` cannot match the optional group, so the match ends at the ident
and the guard still sees `/`. **The guard itself is byte-identical to
Location's**, with no asymmetry anywhere.

`BoundaryOK` and `BadNeighbor` therefore moved to `server/decorators`, and
`location.boundaryOK` delegates to them. A third hand-written copy of that guard
would be a third thing to get wrong, and the two defects it records (the
consumed-guard break, the missing `_`) travel with it.

### The link is never relabelled

The link's stored text is the author's own token, with the field label consumed
in front of it, exactly as a coordinate moniker and `DTG:` are. The sibling plugin renders
`Name (IDENT)`, and that argument does not transfer: it decorates client-side
where nothing is stored, while `MessageWillBePosted` here rewrites the **stored
message**. Putting `Indianapolis Intl (KIND)` in stored text edits what somebody
wrote, using data that changes between builds, and survives uninstall. The name
belongs in the hover, the panel and the page.

### The data

`server/decorators/airport/data/airports.csv`, embedded with `//go:embed` and
parsed at init. 19,012 rows, about 1.6 MB. Its provenance, the exact filter and
the four-decimal argument are in `data/README.md` beside it, which is also where
the upstream URL and SHA-256 live.

**`//go:embed` rather than a generated Go file**, which is where this diverges
from `mapdata`. That encoding exists because Go literal slices of that size are
a compile-time problem; embedded bytes never reach the Go parser at all, so the
question does not arise. `build/airportdata` is a one-off filter, `make
airport-data` runs it, and it is **deliberately not a prerequisite of `make
test`**: `map-data-check` earns that slot because its encoding is opaque and its
drift fails *invisibly* on a plain-HTTP origin, where this transform is filter,
round and drop and its drift means an ident declines. The upstream 8.5 MB file
is not committed.

**Coordinates are rounded to four decimals in the shipped file**, and that is
not presentation. The upstream carries zero to eighteen fractional digits: 31
axis values have none and over a thousand have nine or more, while `FormatDD`
requires at least four and `Axis.Frac` caps at eight, so the raw values are
outside the grammar at **both** ends and would convert for nobody. Four is also
the coarsest the grammar admits, which is the right direction for a
crowd-sourced reference point whose meaning (tower, ARP, terminal) the source
never states. Negative zero is normalized away, for the reason `roundTo`
records.

**`ZZZZ` is excluded, and it is a real upstream row.** It is Satsuma Iojima in
Japan, and it is also the ICAO code for an aerodrome that is **not listed**,
with the real field named in remarks. This decorator reads `DEPLOC` and
`ARRLOC`, which are flight plan fields, so shipping the row would make
`DEPLOC:ZZZZ` resolve to a specific island airfield where the message means "see
remarks". Same argument as the UTM band letter and the same answer. `AFIL`, the
other reserved code, is not upstream at all.

### The coordinate is carried as a pair, and built in Go

```go
token := strconv.FormatFloat(a.Lat, 'f', 4, 64) + "," + strconv.FormatFloat(a.Lon, 'f', 4, 64)
if _, ok := location.Convert(location.FormatDD, token, ""); !ok { ... }
```text

`Details` carries `Format` and `Token`, the location decorator's own `(f, v)`
pair, and **not** a `Conversion`. That is what every surface links with.

**No space after the comma.** `Convert` routes through `validateParams`, which
requires the token to reproduce its own canonical form, and `canonicalString`
writes the DD separator as a bare comma. A space makes `ok` false for **every**
airfield, silently, and every surface would then offer no link at all. Two
independent reviews caught this in the plan before it was written, which is the
only reason it is a paragraph here rather than a defect.

The conversion itself is **discarded**, and running it anyway is the point: it
is the same gate the coordinate page applies, so a token that survives here
cannot be refused by the page it points at. `TestEveryAirfieldLinksToAPageThatRenders`
and `TestEveryAirfieldTokenIsAcceptedByLocation` drive the whole file rather
than a sample, because one bad row is one permanently dead link.

**The pair is built in Go rather than rebuilt in the webapp** because the two
languages do not agree about formatting a float: Go rounds ties away from zero
and JavaScript's `toFixed` is not exact, so a token assembled in TypeScript
could differ in the last digit and be refused by the very view it opens.

`airport` imports `location`, never the reverse.

### The page and the panel

The page is Go-rendered under **`PageStatic` with no script at all**, so
`script-src 'none'`. It is the first page here with one: the DTG pages supply
`ScriptJS` and are served under a digest. The cost, stated rather than hidden:
**the page has no copy buttons**, because those need a delegated listener it has
no script to hang. The panel has them.

Its coordinate link is **relative and a sibling**, `location?f=..&v=..`, because
both pages are served from `/decorate/` and a page renderer is a pure function
of a query string that cannot see `SiteURL`. That is the same constraint
`ScriptSrc` is refused for being absolute under.

**The payload carries already-rendered strings**, the way `Conversion` does and
for the reason `convert.go` gives. That, rather than where the page renders, is
what keeps one renderer: there is no `format.ts` and no paired fixture table.

There is **no row catalog**. Location has one because row ids reach the KV store
and `preferences.go` validates them against `location.KnownRow`; there is no
airfield Customize surface, so a catalog would be a contract with no second
party.

### `/api/v1/airport`

A **discriminated shape**. An ident the build does not hold carries no airfield
and no coordinate at all, rather than zero values a reader would take for real:
`0,0` is a position like any other and this plugin deliberately does not inherit
the truthiness check that drops it, so a flattened record would have drawn an
unknown airfield at Null Island. "Found but unconvertible" also needs somewhere
to be, and gets it.

It carries `coordinate` as `{format, value, region}` and **no readings**. It
carried the whole `Conversion` at first, and that field went the moment the
surfaces stopped printing readings: a field nothing reads is how a payload
starts drifting from its consumer. `region` survived that cut because it is
read, by the map's accessible label, and it is the one field here that may
legitimately be **empty**: a position over open ocean is in no country, so it is
checked for its type and never for its content.

A `source` field went the same way when the citation line was removed from both
surfaces. **No airfield surface prints a credit for the data**, which is the
same judgment the Natural Earth basemap credit already got: OurAirports is
public domain, so a notice is a courtesy rather than a requirement, and the
provenance is recorded where a reader would look for it (`data/README.md` and
`public/help/formats.html`). The **region's** citation is a different thing and
stays, for the reason it always did.

`found: false` at **200, never 404 or 400**, and the page does the same. A
refreshed database drops idents, and a refusal would turn every message that
ever named one into a permanent failure with hand-editing the only way back. A
malformed `v` is still 400, and that shape check runs before anything echoes the
value.

The route does **not** consult `EnableAirport`: a format switch governs
decoration only, so a link written while it was on keeps resolving. No
`configurationLoaded` gate either, unlike `/features`, because this answer is
discarded rather than cached as an admin decision.

### The webapp cache has a TTL, and Location's does not

`convert.ts` caches a verdict forever because a conversion is a pure function of
its token: arithmetic plus compiled-in polygons. **An airfield answer is a
function of `(ident, build)`**, not of `ident`: the database ships with the
plugin, codes are retired and positions corrected. Caching forever would leave a
reader with an open tab seeing "not in this build's database" after an upgrade
added the code, with nothing suggesting a reload.

So `airport.ts` uses `CACHE_TTL_MS`, the constant `preferences/store.ts` and
`features/store.ts` already share and `TestWebappCacheLifetimeMatches` pins
across languages. Everything else is `convert.ts`'s shape, including the parts
learned the hard way: `request()` checks the cache itself, one in-flight promise
is shared, and a `failed` read is **never** cached.

`elevation` is a string and empty means "not stated", which is a different thing
from sea level (`0 ft`). Nothing may test it for truthiness; that is the defect
this plugin does not inherit from the sibling plugin, alongside the one that
drops the equator and the prime meridian.

**Five states, specified for both surfaces** so a panel and a hover cannot
disagree about the same link: `loading`, `ready & found`, `ready & !found`,
`rejected`, `failed`. The hover renders `null` for everything but the second,
which the framework's `:empty` rule turns into no card rather than an empty box;
a card that said "looking up" would flicker on every code a cursor crossed. The
hover is the **name and place, one line**, which is the same bar the DTG hover
meets with the countdown.

### One switch

`EnableAirport`, in an **Airfields** section, on by default. One rather than
two: the grammar is label-only by construction, so a moniker switch would turn
the whole decorator off under a second name. `EnableLocationUTM` stays the only
off-by-default switch and `defaultsOff` needs no entry.

### Not done, deliberately

No inline post rendering, and no IATA grammar (three letters, and the code is
shown in the panel for a field that has one).

**No map on the standalone page**, which would need the page bundle and
therefore `PageMapping`, widening the policy on a route that echoes a request
value. It links to the coordinate page instead, which draws one.


## Testing the two surfaces

`AirportHarness.tsx` stubs the one route both surfaces read and switches between
them. Three things in it are decisions rather than mechanics, and each was
arrived at by getting it wrong first.

**A mixed-frame counter is the wrong instrument here**, which is what separates
this harness from `LocationPanelHarness`. Every field the airfield panel renders
comes from the *answer* rather than from the payload, so the two airfields can
never appear on screen together and a counter looking for both at once reads
zero however the reset is written. The first version of
`never shows the previous airfield once the code has changed` passed with the
mid-render reset in `useAirport` reverted into an effect, which is exactly the
test that proves nothing.

What does separate the two implementations is whether the **old** airfield is
committed again after the code has changed, so `useStaleFrameCounter` counts
that instead. Its dedupe key carries the **code as well as the text**, and that
is the whole instrument: the stale frame renders text identical to the frame
before it, so deduping on text alone skips precisely the commit being watched
for. Checked against a reverted fix in both directions.

**"One request for two surfaces" needs the setup ordinal.** The harness resets
the module cache when it sets itself up, so a remount would reset the cache and
the request count together and the second surface's own request would read as
the first's. `data-testid='setup'` carries how many harnesses have set
themselves up on the page, so the test can tell a re-render from a remount and
the count means what it says.

**The request counter is notified rather than read.** The stub is built in a
`useState` initializer, before the state that displays the count exists, so it
reaches it through a module-level `onRequest`. Assigning that in an effect does
not work: effects run child first, so the surface's own request goes out before
a parent effect could have subscribed, and the first request is the one that
matters. Counting in a module variable and reading it during render does not
work either, since the count would only reach the DOM on some later unrelated
commit and a stale low reading passes the assertion it exists to fail.

`draws the map and none of the furniture` in `LocationMap.pw.tsx` is
**flaky under coverage instrumentation**: it waits on a WebGL map being ready
within 5 s, and the V8-instrumented parallel run misses that occasionally. It
passes in isolation. Unrelated to this decorator, recorded here because it is
what a `make coverage-frontend` failure most often turns out to be.

## Two failures the webapp client is bounded against

**`fetchAirport` aborts at ten seconds.** A stalled fetch never rejects, so
without the bound `inflight` is never cleared: the hover starts the request, the
click that follows joins the same pending promise, and the panel sits on
"Looking up this airfield..." for the life of the tab with a reload the only way
back. This is the third time that defect has been written in this repository;
`features/store.ts` and `basemap.ts` carry the same ten seconds for the same
reason. It bites harder here than in `convert.ts`, which still lacks it, because
an airfield panel has nothing computed locally to fall back to.

**A coordinate the webapp cannot parse is reported as that, not as missing
data.** `positionPayload` returns null for two unrelated causes: the server sent
no coordinate, and `location.fromParams` refusing the pair the server just
issued. Both used to render "This airfield has no position in the database",
which is a false statement about the data in the second case and blames the
database for a bundle mismatch. It is also the drift this repository has already
had once, when the UTM band class widened in Go and not in `format.ts`, and it
would have hit **every** airfield at once while logging nothing. The panel now
branches on the cause and warns to the console, the way `page/payload.ts` does,
and the Go page has never had the bug because it renders from `HasPosition`,
which is the database fact rather than a parse result.

## The panel mounts its map from outside the status branch

The sidebar keeps the panel mounted across a change of selection, so a second
code drives `useAirport` back to `loading`. With `LocationMap` mounted inside
the "found" branch of `renderBody`, that unmounted it: the WebGL context was
released and the whole basemap re-tessellated when the next answer landed,
against a browser cap of roughly sixteen live contexts shared with the hover
card and every inline map on screen. The reader saw the frame flash back to
"Loading map..." and the panel collapse and re-expand by `MAP_HEIGHT`.

It is mounted from `AirportPanel` instead, with `pending` carrying the wait and
a null position while the lookup is in flight, which is the same pair
`LocationReadings` passes for a grid token whose conversion has not arrived.
`keeps one map across a change of selection` pins it on the canvas node, since
MapLibre builds one per map and a torn-down component leaves the old one
detached. Checked against a reverted fix.

**A row carrying a classification gets no copy button.** `Type` reads
"Large Airport", and copying that gets you a category rather than something to
paste, which is the rule the coordinate table already follows for Resolution,
Confidence and Datum. Code, Place, Elevation and IATA keep theirs.

**The panel renders the ident from the link, not from the answer.** The hover
always did, so a reply echoing a different code put one airfield's details under
another's heading on one surface and not the other. `asAirport` now also
requires the code to be four upper-case letters and `fetchAirport` requires it
to be the code that was asked for, which is the check every other entry point
already made: the Go scan pattern, `Parse`, the page, the API and `fromParams`.
A mismatch is `failed` rather than `rejected`, so it is not cached and the next
open asks again.

## The airfield under a code-only message

When a posted message is nothing but an airfield code, the message itself is
rewritten to carry the field's details as a markdown table under the author's
own line:

```text
| Airfield | [Indianapolis International Airport](/plugins/<id>/decorate/airport?v=KIND) |
|:--|:--|
| Code | KIND |
| Place | Indianapolis, IN, US |
| Type | Large Airport |
| Elevation | 797 ft |
| IATA | IND |
```

**A markdown table in the stored message, not a custom post type.** The
coordinate decorator does the opposite: it stamps `Post.Type` and the webapp
draws a map. The two mechanisms now coexist, and the difference is where the
result lives.

What writing it into the message buys:

- The post stays **ordinary**, so it keeps its Elasticsearch and OpenSearch
  matches, its Recent Mentions, its link previews and its auto-translation. A
  custom type loses all of those, since both search backends allowlist
  `type: default` rather than skipping a `system_` denylist. That is the single
  largest cost of the inline map, and this feature does not pay it.
- It renders on **every client**, including the mobile app and anything else
  that never runs the webapp bundle, where a post-type component draws nothing.
- It survives uninstall as plain text rather than as a post nothing can render.

What it costs, and both are permanent:

- **The rewrite is much larger than a link.** An author who types `ICAO:KIND`
  has eight lines of markdown in their stored message. They see all of it when
  they edit the post, and it is in every export. They can also change or delete
  it, which a post-type body does not allow.
- **The values are frozen at post time.** The database ships with the plugin, so
  a table written today keeps today's elevation after an upgrade corrects it.
  The panel, which looks up on every open, is where a reader gets the current
  answer.

**The link is the airfield's name**, and the table is the whole message rather
than a block under the author's line. That relabels the link, which this
decorator refused to do for as long as the stored message was only a link: the
sibling plugin writes `Name (IDENT)` and the argument against it was that a name
changes between builds and this hook rewrites stored text. The table concedes
that already, since the name is in the message either way, and one destination
on the name beats the same destination twice.

**What is not conceded is the author's own token, which is the Code row.**
Without it the string the author typed appears nowhere in the message and
searching for the code stops finding the post, which is the property this whole
approach was chosen for. The `//` a USMTF set line ends with travels with it,
in that same row, rather than being dropped: the author wrote it.

### `MessageExpander`, and why it is a second interface

`decorators.MessageExpander` is optional and type asserted, the same shape
`PostRenderer` takes, so adding it reached no other decorator. `decoratePost`
asks for the expansion **after** decorating and measures the result against
`safePostRunes`; an expansion that would not fit falls back to the plain
decorated message rather than costing the author the decoration as well.

**Nothing parses the decorated message back apart**, and an earlier version did.
`Result` carries `Trail`, the span the pattern matched past the one it rewrote,
because `soleTokenResult` has both spans in hand and was discarding them; the
destination comes from `Tagger.URLFor`, which is exported for exactly this kind
of caller. That deleted a `splitLink` helper along with its coupling to
`labelEscaper` and to how `buildURL` encodes a destination, and it fixed a bug
for free: the helper recovered the trail by slicing after the link, so it also
picked up the message's trailing whitespace and rendered `KIND ` in the Code
row. `match.end` is already the trimmed end, so `Trail` cannot carry any.

Recording the claim that helper made, because it was wrong and is the kind of
thing that gets repeated: it said `buildURL` percent-encodes parens so the last
`)` must be the link's own. `buildURL` encodes parens in the **query**;
`EscapedPath` leaves them in a **subpath**, so a paren really can appear in a
destination. The helper still worked, for a different reason: nothing follows
the link but the optional `//`.

### The message is expanded or stamped, never both

`decoratePost` calls `stampStandalonePost` only when the message came through
unexpanded. The two optional interfaces are siblings and a decorator
implementing both is the obvious next step, so this cannot be left to the
convention that none does: a stamp says the webapp owns the body and renders it
from the message's sole decorator link, and an expanded message is not one, so
`DecoratorPostBody` would fall through and print the raw table as literal text
on a post its custom type had also dropped from the search index. Silent,
permanent, and one `if` away.

### The whitelist is the first layer, escaping is the last

`parseAirfields` refuses any field carrying a rune outside an explicit
whitelist, and any `www.` or `://` sequence. Measured against all 19,012 rows
rather than guessed, so a refreshed database carrying something new fails at
init and in every test.

It is in `parseAirfields` rather than in `build/airportdata` deliberately: the
generator is not a prerequisite of `make test`, so a check there would not run
where it matters and a hand-edited CSV would bypass it entirely.

**Escaping alone cannot be the defence here, which is the whole argument for
the whitelist.** `@here` in an airfield name is the case: backslash-escaping it
does suppress the rendered mention, but Mattermost's mention scan reads the RAW
message text and treats a backslash as a separator, so the notification fires
anyway, from a message whose author typed four letters. Emoji and hashtags have
the same shape. This is the rule the location decorator already states for its
`r` parameter: an explicit whitelist, never a blacklist, with escaping as a
later layer rather than the first.

*Unverified, and worth confirming on a running server before it is relied on:
the mention behaviour above is read from Mattermost's source, not observed.*

### The escaping is new, and it is not hypothetical

Airfield names come from a crowd-sourced third party and reach **markdown** for
the first time here. Every other surface renders them as escaped HTML or as a
React text node, neither of which markdown is. A pipe would end the cell and
shift every value after it into the wrong column; the emphasis, code and link
characters would be read as formatting.

The shipped file already carries **19 names using a backtick where an
apostrophe belongs**, and two with brackets. It carries no pipe today, and a
refreshed database is exactly the event this guards against. `mdCell` escapes
the backslash first, then the pipe, backtick, asterisk, underscore, brackets,
angle brackets and tilde, and flattens any stray carriage return or newline to a
space.

`TestEveryAirfieldTableIsWellFormed` renders all 19,012 and requires every row
to carry exactly three unescaped pipes, which is the property a broken cell
actually violates.

**`TestEveryAirfieldTableIsWellFormed` is a shape check and not escaping
coverage**, and saying so is the point: the shipped file carries none of the
characters `mdCell` escapes, so reducing `mdCell` to the identity function left
that test green. It was described as the strong guard and was not one.
`TestTheShippedDatabaseCarriesNothingTheTableCannotEscape` now asserts the
invariant that silence rests on, and the escaper is pinned by a unit test that
compares the whole output rather than a substring.

**The escaping is tested on `mdCell`, not on the rendered table**, and that
separation was arrived at by getting it wrong twice. The table legitimately
writes unescaped pipes as its cell separators and unescaped brackets as its one
link, so asking whether the rendered table contains a raw `[` answers the wrong
question and fails on correct output. Asking whether it contains the substring
`| Heliport` answers a different wrong question, since `\| Heliport` contains it
too. What the table is held to is the row shape, plus the name appearing
escaped inside the link label, where an unescaped bracket would end the label
early and break the link.

### `EnableAirportTable`

One more switch, ANDed with `EnableAirport`. It governs decoration, like every
format switch, so turning it off stops new messages being expanded and cannot
un-expand the ones already posted. That is the same rule every decoration
follows: the text is what the author's message now says.

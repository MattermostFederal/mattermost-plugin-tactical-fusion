# Reader preferences

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Reader preferences

### Leaving is not editing

Every editor seals its controls until a read has succeeded, because a failed
FIRST read degrades to the defaults and saving an edit made on top of those
replaces the reader's real section with settings they never chose. `loaded`
stays false for good after that failure, which is deliberate.

Back must not be sealed by it. The DTG editor gated Back on the same flag, so
the one path where a reader most wants out was the one path where the control
was dead for the life of the panel: they were shut in the editor, with closing
the whole sidebar the only way back to the timestamp they opened it from.
`HideableEditor` had already got this right (`disabled={busy}`), which is the
kind of divergence extracting a shared component is supposed to end, and did
not here because the DTG editor was left out of the extraction.

The same pair of fixes travels together: a failed save refocuses the Save
button, because disabling a button that holds focus blurs it to the body and
leaves the reader nowhere to retry from. `HideableEditor` carried that and the
DTG editor did not.


"Customize your view" is a link below three panels. On the DTG it chooses the
timezone rows and how close a DTG has to be before the countdown flashes; on the
location panel it chooses **which rows to show**; on the Cursor on Target panel
it chooses **which groups to show**, argued in
[`cot.md`](cot.md#customize-your-view).

The two that hide things, location and Cursor on Target, are **one component**:
`preferences/HideableEditor.tsx`. What differs between them is a catalog and
four strings; what is the same is every property that took a defect to get
right, and those had been copied twice and fixed at different times in each
copy. The DTG editor stays its own, because a zone picker and a number field
are not a checkbox list.

**Nothing is editable until a read has succeeded.** A failed FIRST read degrades
to the defaults, which renders as every box ticked, and the form used to be
enabled over it: a reader who edited that and saved wrote a selection derived
from settings they never had, and a save replaces the whole section. `error`
alone cannot tell that from a later failure, which keeps the last good blob and
is safe to edit, so the store carries a `loaded` flag and the editors seal on
it. All three had the defect; all three now seal.

**Focus is placed on both halves of the swap.** The editor replaces the whole
panel, so the link that opened it unmounts and focus fell to the body: a
keyboard reader was dropped at the top of the document with nothing announced,
and again on the way back. The editor focuses its Back link on mount, and each
panel focuses its Customize link when it returns. `LinkButton` forwards a ref
for exactly this. A failed save does the same smaller thing: disabling the
button while it held focus blurred it, so the reader had nowhere to retry from.

**A hint describes its checkbox; it does not name it.** The hint span used to
sit inside the `<label>`, which made it part of the accessible name, so every
box announced a dozen words of prose before its own state. It is a sibling now,
referenced by `aria-describedby`. Tests match on labels rather than hints, which
is the more honest assertion anyway.

All three editors write through `savePreferencesSection` / `resetPreferencesSection`,
never a whole blob of their own, and that is load-bearing twice over. A `PUT`
replaces the entire blob, so an editor building one from its own state deletes
whatever the reader chose in the other one; and `loadPreferences` fetches **once
per page load and never again**, so the cached blob is as stale as the tab is
old and spreading it carries a snapshot from minutes ago back over a newer one.
The store therefore **re-reads immediately before writing**. That narrows the
window from the lifetime of the tab to the length of one request rather than
closing it: two saves inside that window still resolve last-write-wins, and
closing it properly needs a revision the server checks.

"Restore defaults" is **section-scoped** for the same reason. It writes the
section's zero value and deletes the blob only when every section is back to
zero. That keeps the promise the delete was there for: a zero value *is* "no
choice made", so an empty row list is not today's rows and the reader keeps
tracking whatever the rows become. Before it was scoped, pressing it under a
legend reading "Rows to show" deleted the whole blob and took the timezone table
with it. It is stored per user in the plugin KV store under
`prefs-<userID>` and served from `/api/v1/preferences` (GET, PUT, DELETE).

The editor **takes the panel over** rather than expanding below the table: its
timezone picker is several hundred rows, which underneath the table would bury
the DTG the reader opened the sidebar to see. `DtgPanel` owns the switch, so
`Customize` is mounted only while somebody is actually editing, which is what
keeps its few hundred offset measurements off the path of every panel that is
only ever read.

Saving and restoring defaults both close it, since the changed table behind is
the receipt. A **failed** save does the opposite and stays put with the reason
on screen, because closing would throw away both the message and the reader's
edits. There is a Back link as well: without one, a reader who opened the editor
by accident would have no way out that did not write something.

Both links (the one that opens the editor, and Back) go through
`components/LinkButton.tsx`: Mattermost's link color, underlined only on hover
or keyboard focus. Everything in this plugin styles itself inline, and inline
styles have nowhere to declare a `:hover` rule, so that underline is driven from
React state. Focus is included deliberately, or the cue would be invisible to
anybody moving through the panel by keyboard.

The editor carries no heading of its own: the sidebar **header follows the
panel** into the editor and back. Mattermost
renders the header and the body as two separate components, so the panel cannot
pass this down and `summary` cannot see it: which view is up lives in
`decorators/dtg/editing.ts`, a module store both read. `DtgPanel` also resets it
whenever the payload changes, because React keeps the panel mounted across a
change of selection, so clicking a second DTG while editing would otherwise land
on the editor rather than on the DTG that was clicked.

**`/api/v1` and `/decorate` both require a session but answer a missing one
differently**, and they are deliberately siblings rather than one nested inside
the other. The API refuses with 401 JSON, because its callers are `fetch` and
want a status to branch on; the pages redirect to the login, because their caller
is a person who can sign in and carry on. `sessionUserID` is the one function
that reads `Mattermost-User-Id`, so the two can differ in what they do about the
answer without differing in how they get it. Keeping the routes apart means
neither can inherit the other's rules by accident.

**A zero value means "use the default", everywhere.** An empty zone list, a zero
threshold, an absent blob and a blob that failed to parse all render the
built-in table and the built-in 30 minutes. That is what makes "Restore
defaults" a **delete** rather than a write of today's defaults: a reader who has
not chosen keeps tracking whatever the defaults become. For the same reason the
editor collapses a selection that is exactly the defaults back to empty on save,
so opening the panel and pressing Save does not silently freeze somebody's table
at today's list forever.

Consequences worth knowing:

- **The standalone page always shows the defaults.** It requires a login now,
  but it does not ask who is reading it: the renderer takes a query string and
  no user. So the RHS and the page still disagree about the same DTG for a
  reader who has customized either setting. That was inherent while the route
  was public and is a **choice** now, and the choice is deliberate: honoring
  preferences means a KV read on a route served with a cache lifetime, and the
  renderers stop being pure functions of their query strings. Worth revisiting,
  but as its own piece of work.
- **`DEFAULT_URGENT_WITHIN_MS`, `urgentWithin` in `page.go` and the page's
  countdown script must still agree.** Those two have no reader; they are the
  default.
- **Nothing may fail the panel.** A read that fails, a blob that will not parse
  and a zone this browser cannot format each degrade to a default rather than
  taking the panel with them. A *save* does the opposite and reports its error,
  since a save that quietly did nothing is worse than one that says so.

There are **two caches, both 30 minutes, and their TTLs mean different things.**

The server's is the pattern from `mattermost-plugin-aocanywhere`: a read-through
`expirable.LRU` in `preferences_cache.go`, with writes invalidating rather than
repopulating and publishing a best-effort cluster event so other nodes drop
their copy. Its TTL is the backstop for a lost event, not the mechanism: every
write already corrects it, so the timer only bounds how long a node can be wrong
about a write it never heard. The cost of the longer setting is exactly that one
case, and only on the nodes that missed the event; a reader on the node that
saved sees their change immediately regardless.

The webapp's is module state in `preferences/store.ts`, so a channel full of
links makes one request rather than one per hover. Its TTL is a
staleness bound and nothing else: there is no invalidation for it to hear, so
the timer is the only thing that ever refreshes it. It used to have none at all,
and a blob read on the first hover was kept for the life of the tab, so a reader
who changed their settings in another tab or on their phone saw the old ones
here until they reloaded. That is also what made a save dangerous, and is why
`savePreferencesSection` re-reads: the TTL bounds how wrong the cached copy gets
between reads, and the re-read stops a save acting on it at all.

The two being equal is a **decision rather than a coincidence**, and
`TestWebappCacheLifetimeMatches` reads the TypeScript constant and fails if
either moves alone. A number agreed between two files in two languages is one
that drifts, and changing what a stale blob is worth should be a decision in
both places at once.

`usePreferences` calls `loadPreferences` on mount only, which is what turns the
TTL into a refresh: panels and hovers mount constantly, so the first one to open
after it lapses does the read and the rest of the half hour is served from
memory. A **failed** read deliberately does not stamp the clock, or a reader
whose settings were briefly unreachable would be stuck on the defaults for
thirty minutes.

Two details in that cache are load-bearing and easy to undo. A **generation
counter** guarded by the same lock as the fill, so a read that started before an
invalidation can tell that it did and decline to cache what it found: removing a
key is not enough on its own, because a key still being read is not yet in the
cache to remove, so the write invalidates nothing and the slower read then
installs the value the write had just replaced. And every value handed out is
**cloned**, since the cache returns the same value to every caller and a caller
that appended to `Zones` would be editing what the next reader gets.

### The location hover

The card is `LocationMap` in **`preview` mode**, not a second map: two
implementations of a projection and a palette are two things that can disagree,
and this one would disagree in the place a reader looks first. Preview turns off
everything that makes the panel's map operable, because a card is dismissed by
moving the pointer: no controls (too small to hit before the card vanishes), no
gestures (a wheel handler inside a hover would swallow a scroll over the
channel), no Reset view and no zoom readout. What is left is the picture.

**The card's map carries its own width and height**, 320x180, and the framework's
tooltip caps at that plus its padding. The frame carries only a height everywhere
else, because a block element fills the sidebar; inside a tooltip that sizes
itself to its content there is nothing to fill, and the map came out a narrow
strip. Every behavioral test passed while it did: a pin lands, labels draw and
the wheel is ignored at any width at all, so `is a map rather than a strip`
measures the box instead. The 360px cap on the card is a max rather than a width,
so the DTG countdown still shrinks to its own line.

**It carried no hover for a long time, and the blocker was real.** A hover fires
on pointer movement rather than on a click, so wiring one to `/api/v1/convert`
would have put a request behind every coordinate a cursor crossed in a busy
channel. What unblocked it is the module cache in `convert.ts`.

**The request is bounded at ten seconds, and the bound covers the BODY.** That
second half is the part that was wrong three times over: `fetch` resolves when
the response HEADERS arrive, so a `clearTimeout` placed straight after it leaves
`await response.json()` unguarded, and a server that sends headers and then
stalls the body reopens the exact defect the bound exists to close. The
`finally` therefore wraps the body read as well, which also means an abort part
way through the body correctly rejects rather than being ignored.

`features/store.ts` had it this way first, `convert.ts` was written from it and
`airport.ts` from that, so one misplaced line propagated to three files before a
review caught it. `basemap.ts` is the one that always had it right: its
`finally` wraps `await response.arrayBuffer()`.

**The request is bounded at ten seconds.** A stalled fetch never rejects, so
without the bound `inflight` is never cleared and every later caller for the
same token joins one pending promise: the hover starts the request, the click
that follows joins it, and the grid rows read `converting…` for the life of the
tab with a reload the only way back. It is the same ten seconds
`features/store.ts`, `basemap.ts` and `loadMapLibre` carry, and the same defect
each of them records. This one was written without it and stayed that way long
enough for `airport.ts` to be copied from it and inherit the gap; both are
bounded now. Aborting turns the stall into a rejection, which `load` already
degrades to `failed` and `remembered` already refuses to cache, so nothing
downstream needed changing. What softens it here, and does not soften it on the
airfield panel, is that every row the token yields locally is already on screen.

**That cache needs no TTL, and that is a property of the data rather than a
shortcut.** A conversion is a pure function of `(format, canonical, raw)`: the
projection is arithmetic and the region comes from polygons compiled into the
binary, so the same token converts to the same readings forever. Reader
preferences can change in another tab; a grid reference cannot. What it does
*not* remember is a **failure**, because caching an outage would mean one bad
minute costs every coordinate in the channel for the life of the tab with a
reload the only way back. `ready` and `rejected` are verdicts about a fixed
token and are kept; `failed` is weather and is not. That is the same split
`basemap.ts` makes about the archive.

The in-flight map matters as much as the answers one: the click that follows a
hover arrives while the hover's own fetch is still outstanding, and joins it
rather than issuing a second. `request()` therefore checks the cache **itself**
rather than leaving that to its one caller, which it did not at first: a cache
only the hook consulted would have sent the next caller to the network with
every appearance of being cached.

Two costs, stated because a hover is not a click. Pointing at a coordinate now
pulls the MapLibre chunk, about 950 KB, where it used to take a click; it is one
cacheable response per session and the panel would have pulled it on the first
open anyway. And a hover builds a WebGL context and tears it down again, which
is why the panel's "one map, created once and moved" rule does not extend here:
a card is mounted and unmounted by the pointer, and `LocationMap` already
releases its context on unmount.

### The location rows

The table opens with **As written**, the author's own text, so every reading
under it is visibly derived from that one line rather than from nothing. Then
the three angular readings, then MGRS, then USMTF and UTM: the notations that
read the same way sit together. MGRS led the table for a while, on the grounds
that it is the reading this audience reaches for most, and the order is a
judgment either way rather than a derivation.

**Normalized stays last**, at the far end from the row it is defined against.
It is the rarest row, absent whenever the author's text already is the canonical
form, so leading with it would put an empty slot second on most coordinates. The
row id is still `raw`, because ids reach the KV store and renaming one silently
unhides a row for everybody who hid it; only the label moved, from "Original
text".

**Order is part of the contract, not a per-surface choice.** `Rows` drives the
panel, the page and the reader's hidden-row list from one list, and
`webapp_sync_test.go` holds the two languages to the same ids in the same
sequence, so reordering for one surface alone is not something the code can
express.

`Rows` in `server/decorators/location/rows.go` is the catalog, and it is now a
catalog and nothing else: an id, a label and whether the row is worth copying.
It carried a `Value` closure per row while the page was rendered in Go, and with
that gone so are `ResolutionText`, `ConfidenceText`, `humanMeters` and
`trimZeroes`, whose only callers were those closures. Resolution and Confidence
are rendered by `format.ts` on every surface, and `format.spec.ts` is the whole
guard on them rather than half of a Go/TypeScript pair. A row present in two of those and not the third fails differently each
time and none of them is loud, so `TestWebappRowCatalogMatches` holds the
TypeScript half to the same ids, labels and order, the same way the band class
is held.

Tests that want "every row" must read the catalog rather than list it. The
component test for hiding all of them listed eleven ids by hand, and when the
three area-reference rows arrived it still hid eleven, still passed, and had
quietly stopped meaning what its name said.

**The stored value is the rows a reader HID, not the ones they kept**, and the
direction is the whole design. Empty then means "all of them", so a reader who
never chose is stored as nothing, which is what lets "Restore defaults" stay a
delete. It also decides what happens when a row is **added**: stored this way a
new row appears for everybody, including readers who customized, which is the
same promise the DTG defaults make. Stored the other way round it would be
invisible to exactly the readers who cared enough to choose. The editor still
presents it as what to *show*, because "hide this" is the honest mirror of the
storage and the wrong thing to ask a person.

Reading is **more forgiving than writing**. `validHiddenRows` refuses an unknown
id on the way in, for the same reason a bad timezone is refused: it can only
come from a hand-written request or a bug, and storing something that will never
do anything reports success for a setting that does not exist. But both
`asRowIDs` in the webapp and the panel's own filter simply ignore an id nothing
renders, so **retiring a row cannot lock a reader out of their own settings**.
Row ids are therefore a contract in one direction only: add and retire freely,
rename never, since renaming one silently unhides a row for everybody who hid
it.

Hiding every row is allowed and leaves the note and the links, which is what
makes it recoverable: the way back is the Customize link itself.

All three editors **spread the existing preferences** before saving, because a
PUT replaces the whole blob and building one fresh would wipe whatever the reader
chose in another editor. The type checker catches this today, since
`Preferences` requires all three keys.

**Three sections mean three places that have to know there are three.**
`hasNoChoices` in `preferences/store.ts` is spelled out per section rather than
deep-compared, so a fourth is a visible omission there rather than a blob that
outlives every choice in it; `clone` in Go copies each section's slice, or the
cache hands one caller a value the next caller edited; and `EMPTY_PREFERENCES`,
`fromWire` and `toWire` each carry all three. None of those fails loudly when a
section is missed: the blob simply never gets deleted, or a setting silently
never reaches the wire.

Stored blobs are stamped with `preferencesVersion`. Nothing reads it yet; it is
there so a later change of shape can tell an old blob from a new one, which is
far cheaper to add now than to retrofit onto data already in the KV store.

Zone identifiers are validated server side against the embedded tzdata.
`"Local"` is rejected: it resolves to whatever zone the server process runs in,
which is not a place and can differ between nodes.

### Military bases

`MILITARY_BASES` in `webapp/src/decorators/dtg/zones.ts` is the named catalog
the picker offers, the nine defaults included. It is **webapp-only**: the
server-rendered page shows the defaults and never a reader's selection, so it
has no need of the list.

**Several bases may share a zone**, and both can be chosen. That is why a
selection entry is `{iana, name}` rather than a bare identifier: somebody at
Stuttgart wants to see "Stuttgart", not the Ramstein row that keeps the same
clock. The two rows read identically to the minute, which is the accepted cost
of naming both. Identity is the pair (`zoneKey`), so the picker keys options on
it, removal matches on it, and both sides deduplicate on it. Nothing may key off
the zone alone.

**Names are never inferred from a zone.** A name reaches a row only by being
stored with it, so a bare `Europe/Berlin` picked out of "All timezones" reads
"Berlin", not "Ramstein", or it would sit next to a real Ramstein row
looking identical. Abbreviations are the exception: those are keyed off the
zone, because only the nine curated ones are hand-written and the rest are
measured, which moves with the season.

The **server stores the name but never resolves it**. It validates the
identifier exactly as before and treats the label as bounded free text, which is
what keeps the catalog from having to be maintained in Go as well. The cost is
that a base renamed later keeps its old label until the reader picks it again.

Both sides accept a **bare identifier** where an entry is expected, because
that is what blobs written before names existed hold. They read as unnamed
zones, which is exactly what they were.

The picker is a **combobox**, `ZonePicker.tsx`, not a native select. A select
was unusable here: every label starts with an offset, and a native select's
typeahead matches from the start of the option text, so its one way of finding
anything among several hundred zones was gone.

Type to filter. Every term has to match, in any order, so "berlin spang" works;
the identifier is searched both as written and with its separators opened out,
so "america/los" and "los angeles" both find Los Angeles; and the offset is
searchable, so "+05:45" works. Filtering is memoised separately from the list it
filters, so a keystroke does not re-measure a few hundred offsets.

It follows the ARIA combobox pattern: arrow keys move a single active option
through the groups as though they were one list, Enter picks it, Escape closes,
and `aria-activedescendant` tracks it so a screen reader follows along. The
active option is scrolled into view, or the arrow keys look broken once the list
is taller than its box. `onMouseDown` is prevented on the list, or the input
would blur and unmount it before a click ever landed.

The list **closes on a pick but the query survives it**, so it cannot sit over
the buttons below while the input still has focus, and one arrow key brings the
rest of the same search back with whatever was just added gone from it.

The picker is grouped: the named catalog first, then every zone the browser
knows, each ordered by offset independently. A base's zone appears in the second
group too, unnamed, since pruning it would make "all timezones" a lie. Several
bases are backward links (`Asia/Bahrain` links to `Asia/Qatar`, `Asia/Kuwait` to
`Asia/Riyadh`) that a browser's canonical list leaves out, which is why the full
list is unioned with the catalog rather than taken as-is.

Adding or retiring a base is one line. Tests enforce that every identifier
resolves, that no two entries share an identity, and that the catalog covers
every default row.

### Zone ordering

Rows are ordered **west to east by UTC offset**, in the panel, the editor and
the server-rendered page alike. The picker names each zone `(UTC+05:30)
Asia/Kolkata` and runs in the same order.

Offsets are **measured at the DTG's instant**, never looked up from a table:
half these zones observe daylight saving, so a stored offset would be an hour
wrong for part of the year. `OrderedZones` in Go and `orderedZones` in
TypeScript must agree down to the tiebreak (offset, then name, then identifier)
or the sidebar and the page would list the same zones two different ways. Both
sides assert the same London/Reykjavik pair, which flips between seasons, and
the same nine-row default order.

Because the order is always computed, **the order a selection is stored in
carries no meaning**. Nothing may read it as though it did: removal keys off the
identifier rather than the row position, and `normalizeZoneSelection` compares
selections as sets.

A zone whose offset cannot be measured sorts last rather than as UTC, since
treating unknown as zero would file it under Zulu, which is a claim rather than
an admission.


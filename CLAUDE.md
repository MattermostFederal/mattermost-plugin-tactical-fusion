# Tactical Fusion Plugin Guidelines

## Overview

Mattermost Tactical Fusion enriches conversations with mission-relevant context, including geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational information. The server is written in Go and the webapp in TypeScript/React.

The repo ships the decorator framework and one decorator (DTG). The remaining enrichment features are not implemented yet.

## Architecture

- `server/` - Go plugin code. Entry point is `main.go`, which calls `plugin.ClientMain(&Plugin{})` and blank-imports `time/tzdata` so the zone database travels in the binary: minimal container images often ship no copy, and without this UTC keeps working while every other zone fails at runtime.
  - `plugin.go` - the `Plugin` struct, which embeds `plugin.MattermostPlugin`, plus `OnActivate` and `OnPluginClusterEvent`. `OnConfigurationChange` lives in `configuration.go` beside the config struct.
  - `decorators/` - the decorator framework: registry, tagger, and the shared HTML page shell.
  - `decorators/dtg/` - the Date-Time Group decorator: token grammar, parser, zones, and the standalone page.
  - `hooks.go` - `MessageWillBePosted` and `decoratePost`.
  - `http.go` - `ServeHTTP`, routing `/decorate/<type>` to a decorator page and `/api/v1/*` to `api.go`.
  - `api.go` - the authenticated JSON API, whose one resource is `/api/v1/preferences`.
  - `preferences.go` / `preferences_cache.go` - the per-reader KV store and its cluster-aware cache.
  - `command.go` / `command_examples.go` - the `/tactical-fusion` slash command.
  - `errcode/` - the `TF-NNNN` catalogue.
- `webapp/` - TypeScript/React webapp. Entry point is `src/index.tsx`. The `Plugin` class's `initialize()` method receives a `PluginRegistry` and Redux `Store` and is where components and hooks are registered.
  - `src/decorators/` - the webapp half of the framework: registry, click handler, styles, selection store, theme sniffing, and the shared `Tooltip`.
  - `src/decorators/dtg/` - the DTG panel, hover, title, countdown, zone catalogue and preference editor.
  - `src/components/rhs/` - `RhsView` and `RhsTitle`, which look the panel up by decorator type.
  - `src/preferences/` - the wire types and the module-state cache in front of `/api/v1/preferences`.
  - `src/HeaderIcon.tsx` - the channel header button, registered in `index.tsx`. It clears the selection and toggles the sidebar, so it always lands on the empty state, which is also the only way back from a decorator panel. The mark is `assets/icon.svg` without its plate, and `server/icon_test.go` asserts the two keep the same pin colour.
- `plugin.json` - plugin manifest. Generates `server/manifest.go` and `webapp/src/manifest.ts` at build time (both gitignored).
- `build/` - build tooling from mattermost-plugin-starter-template (`setup.mk`, `custom.mk`, `manifest/`, `pluginctl/`).
- `assets/` - plugin icon and other static assets bundled at the top level.
- `public/help/` - the built-in documentation, bundled and served by Mattermost.
- `docker-compose.dev.yml` and `docker/` - the local Mattermost and PostgreSQL stack `make deploy` targets. `docker/` is generated and is not committed content.
- `implementation-plans/` - the plan the decorator framework and DTG were built from.

## Decorators

A decorator finds a token in a posted message, rewrites it into a markdown link
whose query string carries the **already-parsed** data, and renders the detail
behind it.

The DTG decorator claims two grammars: military date-time groups and **RFC 3339
timestamps** in the extended form (`2026-08-09T16:30:00Z`,
`2026-08-09T20:30:00+04:00`, seconds optional, fractional seconds parsed and
discarded). Downstream of parsing the two are the same thing, an instant plus
the offset it was written in, which is why they share one decorator rather than
being two.

The military grammar is three shapes, longest first, and the order matters
because Go's regexp is leftmost-first and the bare form would otherwise match
the head of a longer token and stop there:

| Shape | Example | Notes |
|---|---|---|
| `DDHHMM<Z>MMMYYYY` | `091630ZAUG2026` | canonicalises to the two-digit year |
| `DDHHMM<Z>MMMYY` | `091630ZAUG26` | two-digit years always mean 20NN |
| `DDHHMMZ` | `091630Z` | literal `Z` only; month and year inferred |

Zone letters are every military letter except **I** and **J**. I is skipped in
the alphabet because it reads as a 1. J is the observer's own local time, which
cannot resolve to one instant for every reader, so a J date-time group is
declined rather than guessed at.

The short form is Zulu-only for the same reason the basic ISO form is absent: a
bare six-digit run followed by any letter collides with part numbers, serials
and truncated hashes. Its **month and year are taken from the reference time**,
which is the post's `CreateAt` when it has one (an imported or scheduled post)
and "now" otherwise. That inference travels in the `a` parameter and the UI says
so rather than presenting an inferred date as fact.

Years are clamped to **2000-2099**, because the canonical form carries two year
digits and accepting 2150 would canonicalise to "50" and read back as a
different century from the text the author typed. Instants are clamped to
**1970-2200** on both sides. Decoration and rendering have to agree about what
is representable: a token accepted at decoration but rejected at render would be
rewritten permanently into a link whose own page answers 400, and editing the
post by hand would be the only way back. The military grammar cannot reach that
bound, but RFC 3339 can, and `1918-11-11T11:00:00Z` is an ordinary thing to
write.

Only formats that **resolve to a single instant** are eligible, since the URL
bakes one in and the panel counts down to it. That rules out bare dates and
zoneless times: they would need a zone invented for them. It also rules out
anything a false positive would corrupt, since decoration rewrites the stored
message: epoch seconds (any ten-digit number), 12-hour clocks, and named zone
abbreviations (`CST` is three different zones, `EST` collides with "est.") are
all deliberately absent. The **basic** ISO form (`20260809T163000Z`) is absent
for the same reason: a hyphen is a word boundary, so it would match inside
`snapshot-20260809T163000Z.zip`.

A leading **`DTG:` moniker** is recognised as well, since some military formats
use it to mark where a time starts: `DTG: 091630ZAUG26` becomes
`[091630ZAUG26](...)`, with the label matched so it can be **consumed** and only
the token captured. That works because `Pattern.Value` is both what `Parse` is
given and what the link is labelled with, so a pattern can match more than it
links. Spacing either side of the colon is optional and the moniker is
case-insensitive; the token keeps its own casing rules. It **vouches for
nothing**: a token it marks still has to be one, so `DTG: 091630R` stays
declined exactly as `091630R` does.

Both the bare patterns and the labelled one are built from the same token
sub-expressions, so a change to what a token looks like cannot reach one and
miss the other.

The URL carries four parameters: `t` (the resolved instant in milliseconds),
`dtg` (the canonical token), `a` (which components were inferred: `""` or `my`),
and exactly one of `z` or `o`. **That last pair is the only thing telling the
two forms apart**: a date-time group says `z` (a military letter), a timestamp
says `o` (an offset in minutes, because RFC 3339 offsets can be half or quarter
hours and a letter cannot name those). A link carrying both or neither is
rejected. Links already written into messages carry `z` and are untouched by
this.

Both the page and the panel **re-derive the whole payload from `dtg` and require
it to reproduce every other parameter**. Validating each in isolation is not
enough on a public route where the URL is user-supplied: a crafted link could
pair an arbitrary token with an unrelated instant and a third zone, and the page
would render all three side by side as though they agreed. Round-tripping the
canonical form removes that whole class rather than the individual
combinations, which is also why `canonical()` on both types is held to an exact
round trip.

Decoration happens **on the server**, in `MessageWillBePosted`. That is the only
way the link reaches clients that never run the webapp bundle, notably the
mobile app. It also means:

- **Stored post text is permanently rewritten.** Editing a post shows the author
  raw markdown, exports contain the links, and uninstalling the plugin leaves
  links that 404. This is a deliberate trade for cross-client support.
- **Only new posts are decorated.** History from before install is untouched.
- **Edits are stored verbatim.** There is no `MessageWillBeUpdated` hook, on
  purpose: re-decorating would mean transforming text a user deliberately
  authored. Deleting the link syntax while editing is the supported way to opt
  one post out. A test asserts the hook does not exist so this cannot be undone
  by accident.
- **System posts are skipped**, matched on `model.PostSystemMessagePrefix`. The
  deny list is deliberately that narrow: skipping every non-empty `Type` would
  also skip custom post types from integrations and other plugins, which may
  carry real mission content.
- Decoration is otherwise skipped, and the post left alone, when `EnableDTG` is
  off, when the format in question is off, or when the result would exceed the
  maximum post size (`PostMessageMaxRunesV1`, the conservative choice, since an
  admin can lower the effective limit below V2). A 12-character DTG becomes
  roughly 120 once linked, so a message that visibly fits can cross the limit
  here; dropping the decoration beats showing the author an opaque "too long"
  error for text they can see fits. A panic in a decorator is recovered and the
  post passes through unmodified. Nothing here may ever stop somebody from
  posting, which is also why the recover and the size warning both log through
  an API handle captured *before* the deferred call: logging through `p.API`
  inside the recover would panic again from within the deferred function and
  escape the hook entirely.

Stored URLs are **root-relative** and never carry a scheme or host, so they
follow whichever server the reader is on and a domain migration does not break
every historical post. `SiteURL` is consulted for one thing only, its path
component, which is what makes a subpath install such as
`https://host/mattermost` work. An unset or malformed `SiteURL` therefore just
means "no subpath" and decoration continues normally; a path that is not rooted
is ignored rather than emitted, since that would produce a URL relative to
whatever page the reader happens to be on.

**The tagger protects spans, not whole messages.** A token in ordinary prose is
decorated even when the message also contains a link or a code block; a token
*inside* one of those is left exactly as written. So
`[the plan](https://example.com) says 091630ZAUG26` links the DTG and leaves the
link alone.

Protected spans are fenced code (including unterminated and `~~~` fences),
indented code, inline code of any backtick width, links, images, reference
definitions, any bracketed span, angle autolinks, inline HTML tags, and bare
`scheme://` and `www.` URLs. Overlapping spans are **merged, never discarded**:
dropping one because it overlapped an earlier one let a construct lose its
protection entirely and have a link written into its interior, which is the
opposite of what a protected range is for.

`findProtectedRanges` is the entire safety story here, so anything it fails to
recognise is a corruption bug in code that permanently rewrites what a user
wrote. An early version corrupted messages five separate ways, including
injecting a nested link inside a real one and rewriting the middle of a pasted
URL. Block constructs are scanned line by line rather than matched with regexes,
because Go's RE2 has no backreferences and so cannot express "a closing fence
matching the opener" or bound an unterminated one. Widen this only with a
regression test per construct.

Links and bracketed spans allow **balanced delimiters**, because CommonMark
does: `[link [foo [bar]]](/uri)` is a link verbatim from the spec, and so are
`[x](/a(b)c)` and `` [x](/a "(note)") ``. RE2 has no recursion, so "balanced to
any depth" cannot be written as a pattern and the nesting is spelled out to
`nestDepth = 4`. **That is a bound on how much protection there is, not a tuning
knob**: past it the failure mode is a rewrite rather than a missed decoration,
so raise it before narrowing it. The simpler non-nesting expressions are kept
alongside the balanced ones rather than replaced by them, since protection is
the union of every expression and a balanced expression matches *nothing* when
the delimiters do not balance. Removing the limit outright needs a hand-written
scanner counting delimiters, the way the fences are scanned, which is the honest
fix if this ever bites.

Two more things the framework owns rather than the decorator: link labels are
escaped (`labelEscaper`) so a token cannot be re-parsed as markdown inside the
brackets, and `Decorate` is **idempotent**, because a decorator link it already
wrote is itself a protected span.

`/decorate/*` is a **public** route. The clients it serves have no Mattermost
session. That is safe only because a decorator page is a pure function of its
query string: no workspace lookup, and it never reads `Mattermost-User-Id`. A
decorator needing workspace data must not use this route.

Hovering a decorator link shows a card, and clicking it opens the right-hand
sidebar; the browser never navigates. A decorator gets a hover by declaring an
optional `Hover` component, so the tooltip is registered once for the whole
plugin and nothing in the bootstrap changes when a decorator adds one. Keep a
hover to the one thing worth knowing without opening the sidebar: the DTG hover
is the countdown and nothing else, and leaves the reading and the timezone table
to the panel. It still honours the reader's flash threshold, or pointing at a
link and opening it could disagree about whether the same DTG is imminent.

The sidebar also opens from a **channel header button**, which clears the
selection first so it always lands on the empty state. That state is the only
way back out of a decorator panel, and it carries the plugin version, which is
the fastest thing to ask a reporter for.

`index.tsx` keeps a disposer list and runs it in `uninitialize()`. Without that,
a re-registration leaves the old capture listener attached and every click is
dispatched twice. `registerBuiltinDecorators()` is idempotent for the same
reason: the registry lives in module state that survives a re-registration while
`initialize()` runs again, and throwing on the second pass would leave the
sidebar dead until a page reload.

Query parameters starting with `_` are reserved for the framework, so decorators
can name their own params freely. There are two:

- **`_page=1`** makes the webapp stand aside so the browser follows the link to
  the server-rendered page instead. Purely for testing, since the page is
  otherwise only reachable from a client that does not run the webapp bundle.
  Every such link is pointed at one shared window (`PAGE_TARGET`), so following
  a second one replaces the first rather than collecting a tab per link. That
  needs `noopener` and `noreferrer` stripped from the link's `rel`: either token
  makes a browser ignore the target name and open a fresh context every time,
  and Mattermost adds both to rendered links. Safe only because the destination
  is this plugin's own page on the same origin, so there is no cross-origin
  opener to withhold. All of this lives in the framework's click handler, so it
  applies to every decorator type without any of them knowing about it.
- **`_theme=light|dark`** tells that page which way to paint itself. It is a
  separate document and cannot read the webapp's CSS variables, so without a
  hint it can only follow the *operating system* preference, which is a
  different setting: a light Mattermost on a dark laptop would open a dark page
  next to a light sidebar. The webapp adds this on the way out, reading
  `--center-channel-bg` from the live DOM so it matches whatever the sidebar is
  actually painted with. Clients that cannot know the theme omit it and get the
  operating system fallback. Only the two keywords are accepted, since the value
  reaches a stylesheet.

`/tactical-fusion examples` posts a live demonstration, in four groups: examples
built from the moment the command runs (so the panel opens on a countdown that
is actually moving, including a negative offset, which counts up and is the half
of the behaviour easiest to forget exists), fixed examples of each grammar, the
near-misses that are deliberately **declined**, and tokens **skipped** because
they sit inside a protected span. A ready-made `_page=1` link follows. The
command runs the tagger itself rather than relying on the message hook, because
its own output is full of fences and links and would therefore be skipped. That
also keeps the "declined" and "skipped" rows honest, since they are genuinely
the tagger's output rather than hand-written, and the live rows go through the
decorator's own `FormatZulu`, so an example cannot drift into something the
decorator declines.

`/tactical-fusion` with no subcommand lists the subcommands; an unknown one is
an error carrying a `TF-NNNN` code. Both replies are ephemeral.

### Adding a decorator

1. **Server**: create `server/decorators/<type>/` implementing
   `decorators.Decorator`, then add one argument to the
   `decorators.NewDefaultRegistry(...)` call in `OnActivate`.
2. **Webapp**: create `webapp/src/decorators/<type>/` exporting a
   `Decorator<T>`, then add one `register(...)` line to
   `registerBuiltinDecorators()`. Add a `Hover` component if a glance at the
   link should show something; omit it for no hover. Add a `Title` component
   only if the panel has more than one view and the header has to follow it;
   otherwise the header is `summary`, which stays required either way.

Nothing else in either `decorators/` tree changes. The token grammar lives in Go
only; the webapp reads the query params the server produced, so the two sides
cannot drift.

## Admin settings

`plugin.json` declares four switches, all defaulting to on: `EnableDTG` (the
date-time group decorator) and one per format below it, `EnableDTGMilitary`,
`EnableDTGMoniker` and `EnableDTGTimestamp`. Mattermost's settings schema has no
nesting, so the grouping is by ordering and naming; the parent is enforced in
code, in `Plugin.dtgFormats`.

There is no separate "decorate at all" switch. With one decorator it only
duplicated `EnableDTG`; a second decorator should bring its own switch rather
than reviving a global one. **Disabling the plugin is not a substitute**: that
also stops `/decorate` serving, so every link already written into a message
would 404. `EnableDTG` stops new decoration and leaves the history working.

`decoratePost` has no configuration check of its own. A decorator with
everything switched off contributes no patterns, so nothing matches and the post
is returned unchanged, which is the same path as a message with no token in it.

**A format switch governs decoration only.** `RenderPage` never consults the
configuration, so a link already written into a message keeps working after its
format is switched off. Turning one off stops new messages being decorated with
it, and that is all. For the same reason the decorator stays registered even
with everything off, rather than being left out of the registry: an unregistered
type would 404 every link in the history.

The moniker composes: `EnableDTGMoniker` only labels formats that are themselves
on, so turning military formats off also stops `DTG: 091630ZAUG26`. A moniker
with nothing left to label is dropped entirely.

Every field is false at zero, deliberately: an unloaded configuration should
decorate nothing rather than guess. That makes the manifest's `default: true`
load-bearing, which is why a test asserts every switch declares one, and two
more assert the setting keys and the `configuration` fields match in both
directions. A key that binds to no field is a switch an admin can toggle
forever with nothing happening, and nothing else would report it.

`Decorator.Enabled` is read fresh for every message, so a change in the admin
console takes effect without a restart.

## Reader preferences

"Customize your view", a link below the DTG's timezone table, lets a reader
choose their own timezone rows and how close a DTG has to be before the
countdown flashes. It is stored per user in the plugin KV store under
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
`components/LinkButton.tsx`: Mattermost's link colour, underlined only on hover
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

**`/api/v1` is authenticated and `/decorate` is public**, deliberately as
siblings rather than one nested inside the other. The API is the only place in
the plugin that reads `Mattermost-User-Id`, and it refuses a request without
one. Keeping the two apart means neither can inherit the other's rules by
accident.

**A zero value means "use the default", everywhere.** An empty zone list, a zero
threshold, an absent blob and a blob that failed to parse all render the
built-in table and the built-in 30 minutes. That is what makes "Restore
defaults" a **delete** rather than a write of today's defaults: a reader who has
not chosen keeps tracking whatever the defaults become. For the same reason the
editor collapses a selection that is exactly the defaults back to empty on save,
so opening the panel and pressing Save does not silently freeze somebody's table
at today's list forever.

Consequences worth knowing:

- **The standalone page always shows the defaults.** It is served without a
  session, so it has no reader to ask. The RHS and the page can therefore
  disagree about the same DTG for a reader who has customised either setting.
  This is inherent to a public route, not an oversight.
- **`DEFAULT_URGENT_WITHIN_MS`, `urgentWithin` in `page.go` and the page's
  countdown script must still agree.** Those two have no reader; they are the
  default.
- **Nothing may fail the panel.** A read that fails, a blob that will not parse
  and a zone this browser cannot format each degrade to a default rather than
  taking the panel with them. A *save* does the opposite and reports its error,
  since a save that quietly did nothing is worse than one that says so.

Caching follows the pattern in `mattermost-plugin-aocanywhere`: a read-through
`expirable.LRU` in `preferences_cache.go`, with writes invalidating rather than
repopulating and publishing a best-effort cluster event so other nodes drop
their copy. The TTL is the backstop for a lost event, not the mechanism. The
webapp caches too, in module state in `preferences/store.ts`, so a channel full
of DTG links makes one request rather than one per hover.

Two details in that cache are load-bearing and easy to undo. A **generation
counter** guarded by the same lock as the fill, so a read that started before an
invalidation can tell that it did and decline to cache what it found: removing a
key is not enough on its own, because a key still being read is not yet in the
cache to remove, so the write invalidates nothing and the slower read then
installs the value the write had just replaced. And every value handed out is
**cloned**, since the cache returns the same value to every caller and a caller
that appended to `Zones` would be editing what the next reader gets.

Stored blobs are stamped with `preferencesVersion`. Nothing reads it yet; it is
there so a later change of shape can tell an old blob from a new one, which is
far cheaper to add now than to retrofit onto data already in the KV store.

Zone identifiers are validated server side against the embedded tzdata.
`"Local"` is rejected: it resolves to whatever zone the server process runs in,
which is not a place and can differ between nodes.

### Military bases

`MILITARY_BASES` in `webapp/src/decorators/dtg/zones.ts` is the named catalogue
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
what keeps the catalogue from having to be maintained in Go as well. The cost is
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

The picker is grouped: the named catalogue first, then every zone the browser
knows, each ordered by offset independently. A base's zone appears in the second
group too, unnamed, since pruning it would make "all timezones" a lie. Several
bases are backward links (`Asia/Bahrain` links to `Asia/Qatar`, `Asia/Kuwait` to
`Asia/Riyadh`) that a browser's canonical list leaves out, which is why the full
list is unioned with the catalogue rather than taken as-is.

Adding or retiring a base is one line. Tests enforce that every identifier
resolves, that no two entries share an identity, and that the catalogue covers
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

### Unverified before deployment

Two prerequisites from the implementation plan need a running server and have
**not** been checked:

- Whether Mattermost's search still matches a DTG once the message contains
  `[091630ZAUG26](/plugins/...)`. If the indexer splits on brackets, decorated
  posts become unfindable and the server-side approach needs rethinking.
- Whether the mobile app resolves root-relative markdown links against the
  server URL.

Also unverified: which post sources actually reach `MessageWillBePosted`
(`p.API.CreatePost`, incoming webhooks, bot posts, `in_channel` command
responses). Record the real behaviour here once tested.

## Built-in documentation

`public/help/` holds seven static HTML pages and one stylesheet. Mattermost
serves the bundle's `public/` directory at `/plugins/<id>/public/**`, so
**there is no route for this in the server code** and nothing to add to
`ServeHTTP`. The build already copies it: `build/setup.mk` sets `HAS_PUBLIC`
from the directory's existence and the `bundle` target acts on it.

| Page | Covers | Kept in sync with |
|---|---|---|
| `help.html` | Landing page, what a decorator is, the consequences of server-side decoration, nav cards | The overall surface |
| `formats.html` | Every recognised grammar, the declined list with reasons, protected spans | `server/decorators/dtg/dtg.go`, `parse.go`, `tagger.go` |
| `panel.html` | The sidebar, the hover, the standalone page, Customize your view, the picker, zone ordering | `webapp/src/decorators/dtg/` |
| `admin.html` | One section per switch, and what a switch does not do | `plugin.json` `settings_schema.settings` |
| `commands.html` | `/tactical-fusion examples`, bare and unknown subcommands | `server/command.go` |
| `troubleshooting.html` | Symptom, cause, fix, quoting the exact user-facing strings | Every message in the server |
| `error-codes.html` | The `TF-NNNN` registry, grouped by source file | `server/errcode/codes.go` |

Three things discover it, and `server/help_docs_test.go` guards the first:

- **`plugin.json` `settings_schema.header`**, a markdown link. This is the one
  place the plugin id is written out, which is correct: `plugin.json` defines
  it.
- **The sidebar panel**, a Documentation link beside "Customize your view",
  built from `docsUrl()` in `webapp/src/plugin_url.ts`.
- **`README.md`**.

There is deliberately **no slash command surface**. Were one added, note the
trap the sibling plugin documents: a Go helper building the URL from
`manifest.Id` must be a **function, not a package-level `var`**, because var
initialisers run before the generated `init()` populates the manifest and a var
would read nil and panic at activation.

The pages are **static, light-only, and self-contained**: no JavaScript, no
remote fonts or assets, no dark mode. They must render on an air-gapped host.
A test enforces this, along with the repo-wide em dash ban, which
`check-style` does not cover because it does not lint HTML.

**Anchor ids are a contract.** Pages deep-link into each other, and a renamed id
fails silently: the browser lands at the top and the reader never learns they
missed the section. `TestEveryCrossPageAnchorResolves` walks every
`href="page.html#id"` in the bundle and is the reason renaming one is safe.

Admin setting headings carry `data-setting="EnableDTG"` alongside a readable
`id`. The attribute exists only so `TestEverySettingIsDocumented` can pair a
section with a manifest key exactly, rather than encoding a
PascalCase-to-kebab convention that would break the first time a key was named
something the rule did not expect.

## Error codes

Every user-facing failure and every `p.API.Log*` call carries a stable
`TF-NNNN` identifier, so the code a reader quotes from the sidebar and the code
an operator greps out of the log are the same one. Wording can be improved; a
code cannot change.

`server/errcode` holds the catalogue. Codes are allocated in thousand-wide
ranges, one per source file, listed in the package documentation. Within a range
they go in source order the first time a file is instrumented and drift
afterwards, which is fine. What is not fine is renumbering a committed code or
reusing a retired one: both are quoted in support tickets. Retire a number by
leaving a comment in its place.

- `WithCode(code, msg)` suffixes a string. `Errorf(code, format, ...)` builds an
  error already suffixed, for `preferences.go`, whose `err.Error()` reaches the
  reader verbatim.
- A log call takes `"error_code", errcode.X` as its **first** key/value pair.
- Where a failure is both logged and returned, the two share one code. They are
  one failure.

Adding a code means four edits that go together: the constant, the `AllCodes`
entry, the call site, and a row in `public/help/error-codes.html`.
`TestAllCodesComplete` parses `codes.go` with `go/ast` to enforce the first two
(nothing at runtime can see a constant that is never mentioned), and
`TestEveryCodeIsDocumented` enforces the fourth.

## Coding conventions

- Match the style of surrounding code.
- Keep the plugin minimal: avoid abstractions that are not needed by the code that exists today.
- Server: follow Mattermost plugin API conventions. Use `p.API.LogError`/`LogWarn`/`LogInfo` for logging.
- Webapp: prefer functional React components with hooks.

## Build and test

- `make dist` - build the plugin bundle
- `make check-style` - lint both Go and webapp code
- `make test` - run tests
- `make coverage` - backend and frontend coverage summaries. The backend one
  passes `-coverpkg=./server/...`, which is load-bearing: without it each
  package is measured only by its own tests, so the shared page shell in
  `server/decorators` reads as 0% while being fully exercised from `server`,
  which under-reports the total and points a reader at the wrong files. The
  frontend one merges Playwright unit and component runs.

The repo ships a Docker Compose stack, which is what `make deploy` targets:

- `make docker-setup` - start Mattermost and PostgreSQL, wait for readiness, and
  create the `admin` / `password` system admin and a default team. Serves on
  `http://localhost:8065`; override with `MM_PORT`.
- `make deploy` - build the bundle, install it into that stack, and enable it
  (an alias for `docker-deploy`).
- `make deploy-local` - deploy to your own running server instead, via the
  bundled `pluginctl`. Authenticates through local mode, `MM_ADMIN_TOKEN`, or
  `MM_ADMIN_USERNAME` + `MM_ADMIN_PASSWORD`.
- `make docker-logs`, `docker-reset`, `docker-stop`, `docker-down` - operate the
  stack. `make nuke` tears down everything, including `docker/`, `node_modules`
  and every build artifact.

## Commits and releases

This repo automates releases with **release-please** driven by
[Conventional Commits](https://www.conventionalcommits.org/). Full details live
in [`docs/RELEASING.md`](docs/RELEASING.md); the essentials:

- **Write conventional commit subjects** (and PR titles, since PRs squash-merge).
  The prefix drives the version bump: `feat:` → minor, `fix:`/`perf:`/`deps:` →
  patch, `feat!:` or a `BREAKING CHANGE:` footer → major. `chore:`/`docs:`/
  `test:`/`refactor:`/`style:`/`build:`/`ci:` don't bump or appear in the
  changelog.
- **Do not** hand-edit `plugin.json`'s `version` or `CHANGELOG.md` for a normal
  release. release-please owns them via its Release PR. The version is seeded at
  `0.1.0`.
- A release ships when the maintainer merges the open "chore(main): release
  X.Y.Z" PR, which tags `vX.Y.Z` and fires `release.yml`.

## CI and security

Workflows live in `.github/workflows/`: `pr.yml` (style/test/build), `security.yml`
(SBOM + Grype + CodeQL → Code Scanning), `release-please.yml`, and `release.yml`.
Everything is reproducible locally through `make`, and CI runs the same targets, so
verify changes with these before pushing:

- `make check-style && make test` - what `pr.yml` gates on
- `make sbom-audit` - dependency CVE scan (fails on HIGH/CRITICAL)
- `make codeql-analyze && make security-gate` - static analysis + finding gate
- `make release` - the full security-gated pipeline `release.yml` runs on a tag

When touching dependencies or adding code, expect the security workflow to gate
the PR. Suppress false-positive CVEs in `.grype.yaml` with a documented reason
(never blanket-ignore). See [`docs/SECURITY.md`](docs/SECURITY.md) for the full
process, the Code Scanning requirement, and release signing.

GitHub Actions are pinned to full commit SHAs with a `# vX.Y.Z` comment. When
adding or bumping an action, resolve the tag to its commit SHA and keep the
comment accurate. Don't use floating tags like `@v4`.

# Tactical Fusion Plugin Guidelines

## Overview

Mattermost Tactical Fusion enriches conversations with mission-relevant context, including geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational information. The server is written in Go and the webapp in TypeScript/React.

The repo ships the decorator framework and two decorators, DTG and Location. The remaining enrichment features are not implemented yet.

## Architecture

- `server/` - Go plugin code. Entry point is `main.go`, which calls `plugin.ClientMain(&Plugin{})` and blank-imports `time/tzdata` so the zone database travels in the binary: minimal container images often ship no copy, and without this UTC keeps working while every other zone fails at runtime.
  - `plugin.go` - the `Plugin` struct, which embeds `plugin.MattermostPlugin`, plus `OnActivate` and `OnPluginClusterEvent`. `OnConfigurationChange` lives in `configuration.go` beside the config struct.
  - `decorators/` - the decorator framework: registry, tagger, and the shared HTML page shell.
  - `decorators/dtg/` - the Date-Time Group decorator: token grammar, parser, zones, and the standalone page.
  - `decorators/location/` - the Location decorator: `coord.go` (the textual `Axis`/`Grid`/`Location`, `Point()` and `canonical()`), `grammar.go` (token sub-expressions, the bare/labeled split and the boundary guard), `parse.go` (per-grammar parsing and range checks), `geodesy.go` (the WGS 84 transverse Mercator series, zones and bands), `mgrs.go` (the 100 km letter scheme and grid encoding), `format.go` (rendering at resolution), `convert.go` (the derived readings the API serves), `location.go` (the `Decorator`, patterns, monikers, `validateParams` and the `r` gates), and `page.go`.
  - `hooks.go` - `MessageWillBePosted` and `decoratePost`.
  - `http.go` - `ServeHTTP`, routing `/decorate/<type>` to a decorator page and `/api/v1/*` to `api.go`.
  - `api.go` - the authenticated JSON API: `/api/v1/preferences` and `/api/v1/convert`.
  - `preferences.go` / `preferences_cache.go` - the per-reader KV store and its cluster-aware cache.
  - `command.go` / `command_examples.go` / `command_example_details.go` / `command_check.go` - the `/tactical-fusion` slash command.
  - `errcode/` - the `TF-NNNN` catalog.
- `webapp/` - TypeScript/React webapp. Entry point is `src/index.tsx`. The `Plugin` class's `initialize()` method receives a `PluginRegistry` and Redux `Store` and is where components and hooks are registered.
  - `src/decorators/` - the webapp half of the framework: registry, click handler, styles, selection store, theme sniffing, and the shared `Tooltip`.
  - `src/decorators/dtg/` - the DTG panel, hover, title, countdown, zone catalog and preference editor.
  - `src/decorators/location/` - `LocationReadings.tsx` (the map and the table, with no opinion about where its data came from, rendered by both the sidebar and the pages), `LocationPanel.tsx` (the sidebar's environment around it), the copy buttons, `convert.ts` (the conversion client and its degrade-versus-refuse split), and `format.ts`, which slices a canonical token and renders it. No grammar and no projection live there.
  - `src/page/` - the standalone pages' entry point, built by a second webpack configuration into `public/app/page.js`. It renders the same components the sidebar does; see Mapping, "The page bundle".
  - `src/components/rhs/` - `RhsView` and `RhsTitle`, which look the panel up by decorator type.
  - `src/preferences/` - the wire types and the module-state cache in front of `/api/v1/preferences`.
  - `src/HeaderIcon.tsx` - the channel header button, registered in `index.tsx`. It clears the selection and toggles the sidebar, so it always lands on the empty state, which is also the only way back from a decorator panel. The mark is `assets/icon.svg` without its plate, and `server/icon_test.go` asserts the two keep the same pin color.
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
| `DDHHMM<Z>MMMYYYY` | `091630ZAUG2026` | canonicalizes to the two-digit year |
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
digits and accepting 2150 would canonicalize to "50" and read back as a
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

A leading **`DTG:` moniker** is recognized as well, since some military formats
use it to mark where a time starts: `DTG: 091630ZAUG26` becomes
`[091630ZAUG26](...)`, with the label matched so it can be **consumed** and only
the token captured. That works because `Pattern.Value` is both what `Parse` is
given and what the link is labeled with, so a pattern can match more than it
links. Spacing either side of the colon is optional and the moniker is
case-insensitive; the token keeps its own casing rules. It **vouches for
nothing**: a token it marks still has to be one, so `DTG: 091630R` stays
declined exactly as `091630R` does.

Both the bare patterns and the labeled one are built from the same token
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

**A clock is rendered to the resolution the token carried**, which is the same
rule Location renders every row under. The reading and the timezone table are
`HH:MM`, since a date-time group has no seconds field, and become `HH:MM:SS`
when the instant carries seconds, which only an RFC 3339 timestamp can do:
`2026-08-09T16:30:45Z` shown as `16:30` would drop 45 seconds the author wrote,
beside a canonical line that still shows them. A timestamp written to the whole
minute, or with an explicit `:00`, has nothing to lose and keeps the narrow
form, so no field is ever padded on. Every zone offset is a whole number of
minutes, so the seconds field is the same in every row and the decision is made
once from the instant: `clockLayout` in `page.go` and `hasSeconds` in
`describe.ts`, which `DtgPanel` also reads for its `Intl` options. All four
renderings must agree. Sub-second digits are truncated at parse (`parse.go`),
which is the one deliberate exception: the canonical form has nowhere to put a
fraction, and rounding up could carry into the second the token visibly names.

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
  off, when the format in question is off, or when the result would exceed
  `safePostRunes`. A 12-character DTG becomes roughly 120 once linked, so a
  message that visibly fits can cross the limit here; dropping the decoration
  beats showing the author an opaque "too long" error for text they can see
  fits. A panic in a decorator is recovered and the
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
recognize is a corruption bug in code that permanently rewrites what a user
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

A decorator page that needs JavaScript supplies it as `Page.ScriptJS`, and the
shell serves it under `script-src 'sha256-...'` pinned to that script's digest,
never under `'unsafe-inline'`. The distinction is load-bearing rather than
fastidious: these pages echo author text from a message, on a route whose query
string anybody can write, so what has to survive an escaping mistake is that
injected markup cannot execute. A hash keeps that; `'unsafe-inline'` would not.
The field is documented as a source constant for the same reason `StyleCSS` is,
and a script containing `</script` is dropped whole rather than escaped. An
empty `ScriptJS`, the zero value, still means `script-src 'none'`.

`/decorate/*` and `/map` **require a session**, and a request without one is
redirected to the login carrying its own URL back, so an expired session costs a
sign-in rather than the link. The gate is one check in `ServeHTTP` and lives
nowhere else.

They were public until that gate was added, on the argument that the clients
they exist for are the ones without a session: the mobile app opening a link in
an in-app browser. **Whether that is true was never verified in either
direction** (see "Unverified before deployment"), which is what makes redirecting
rather than refusing load-bearing: if the in-app browser has no session, every
decorated link on mobile becomes a sign-in and then the page, not a dead end.

A page is still a **pure function of its query string**: no workspace lookup, and
the renderer is handed no reader. That is no longer a security argument, it is
what keeps a page renderable from a `url.Values` in a test and what stops a route
served with a cache lifetime growing a per-reader answer. A decorator needing
workspace data still needs its own route under `/api/v1`.

Hovering a decorator link shows a card, and clicking it opens the right-hand
sidebar; the browser never navigates. A decorator gets a hover by declaring an
optional `Hover` component, so the tooltip is registered once for the whole
plugin and nothing in the bootstrap changes when a decorator adds one. Keep a
hover to the one thing worth knowing without opening the sidebar: the DTG hover
is the countdown and nothing else, and leaves the reading and the timezone table
to the panel. **The location hover is the map and nothing else**, for the same
reason: a glance at a coordinate is asking where, and every reading is one click
away. It still honors the reader's flash threshold, or pointing at a
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

- **`_page`** makes the webapp stand aside so the browser follows the link to
  the server-rendered page instead. Purely for testing, since the page is
  otherwise only reachable from a client that does not run the webapp bundle.
  Its **presence** is what counts, not its value: the click handler tests
  `params.has`, and a test pins `_page=` with an empty value as still honored.
  Everything that emits one writes `_page=1`, so that is what you will see, but
  nothing checks for the `1`.
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

  **The click handler is not the only thing that appends it.** It covers every
  `/decorate` link, and `/map` is deliberately outside that prefix, so
  `parseDecoratorHref` returns null and the handler stands aside: a map page
  opened from a light sidebar on a dark laptop came up dark, map palette and all,
  because `mapColors` reads the same variable. So the links pointing at `/map`
  write the parameter themselves, and so does the map page's own way back, which
  runs in a document with no click handler around it at all. `withTheme` lives in
  `decorators/theme.ts` beside `detectTheme` rather than in the click handler for
  that reason. Those links read the theme at **render** rather than at click,
  which is one step less live in exchange for surviving a middle-click.

### The slash command

Three subcommands. `examples` and `example-details` both **post to the channel**;
only `check` is ephemeral. The split between the first two is scope, not
audience: `examples` is one message showing the ordinary shape of each grammar,
`example-details` is the exhaustive set. Both run the tagger themselves rather
than relying on the message hook, because their own output is full of fences and
links and would therefore be skipped. That is also what keeps the declined rows
honest, since they are genuinely the tagger's output rather than hand-written,
and the live rows go through the decorator's own `FormatZulu`, so an example
cannot drift into something the decorator declines.

`/tactical-fusion examples` is a `CommandResponseTypeInChannel` reply, one row
per format, in the shape `Label: <typed> → <link> - note`. It exists for
introducing the plugin to a team, so one person runs it and everybody clicks
through. Everything exotic stays in `example-details`.

Being public shapes its failure modes. A row that does not decorate is
**dropped**, because a bare token beside rows that did become links is a
permanent post advertising that the plugin does nothing (with UTM shipping off,
this is the ordinary case rather than an edge one); with nothing left, the
command **refuses ephemerally** rather than posting an empty message; and it
measures itself against the post limit and refuses rather than letting the
server reject it with an error nobody can act on, which a long enough install
subpath does reach. All three refusals carry a `TF-NNNN` code.

Its two live rows are five minutes ahead and four hours behind. That pair is the
point of the command being live at all: five minutes opens the countdown already
inside the flash threshold, and four hours behind counts up. A fixed date shows
neither.

`/tactical-fusion example-details` is the exhaustive one and posts through
`p.API.CreatePost`, **one top-level post per decorator and none of them a
reply**. Each is a reference somebody comes back to; a reply is filed under the
post above it and read as a remark about it, which the coordinate examples are
not. `detailPost` builds every one of them, in one place, so neither caller can
set a `RootId` by accident, and `runDetails` checks it on every post.

**One post per decorator**, which is the unit a reader thinks in: one for
date-time groups, one for coordinates. Both fit at the ordinary post limit, so
that is the shape rather than an aspiration. A set that does not fit continues
into another message rather than being packed in beside its neighbour; sharing a
post between two decorators saves a message and costs the thing that made the
output readable.

The set heading is applied **after** its own packing, against a reserved
`headingBudget`, and is numbered `(n of m)` only when that set needed more than
one message. Unnumbered is the ordinary case, since the heading alone already
says which decorator it is.

Its rows are organized as `detailGroup`s: a heading, a `decorates` flag, and
rows. The heading is where the **rule** gets stated, which for a declined row is
the entire content of the row, and the flag is what
`TestEveryDetailDoesWhatItsGroupClaims` holds every row to **in both
directions**. Both directions matter: a "recognized" row the tagger silently
leaves alone is how a whole format once sat in that list decorating nothing, and
a "declined" row it rewrites would advertise a near miss as safe. That test
reads the row out of the posted output rather than calling the tagger, and
scopes each check to the row, because the output deliberately carries an example
beside its own labeled variant and `LATD:35N079W` contains `35N079W`.

UTM has a group of its own, headed with the fact that it ships off, because on a
default install every row in it renders with no link and under a generic grid
heading that reads as a bug.

The first group is generated from the moment the command runs, at offsets either
side of the flash threshold plus a negative one, which counts up and is the half
of the behavior easiest to forget exists. A live group has no fixed text, so the
tests that look for rows by their text skip it and `TestDetailsIncludeLiveTimes`
covers it instead, accepting either side of a minute boundary.

Within a set the split is driven by the **post size limit** and nothing else.
The atomic unit is a line and at least one always goes in, so packing cannot
stall however long the links get, and a **group heading is repeated with
"(continued)"** when its rows are split, because a continuation opening on a
bare list of rows would have lost the one thing saying what they are.

`packDetails` takes its budget as a **parameter** rather than reading a
constant, both so `postDetails` can retry smaller and so the splitting can
be exercised: at the ordinary limit each set fits in one message, which would
otherwise leave every branch that deals with a message running out of room as
dead code held only by the hope that nobody deletes it.
`TestDetailsSplitWhenAMessageRunsOutOfRoom` drives it at four budgets and checks
that every message still belongs to exactly one decorator however finely it
split. The reply loop is tested through `postRemainingDetails` directly for the
same reason.

An earlier version made the reader choose (`examples dtg`, `examples location`),
which put the size limit in front of them as a decision they had no way to make.

Whether a post these commands create reaches `MessageWillBePosted` is
**unverified** (see below), and both are correct either way: the text is already
decorated, and a decorator link is a protected span, so `Decorate` is
idempotent. `TestExamplesSurvivesTheMessageHook` and
`TestDetailsSurviveTheMessageHook` pin that, since the alternative failure is a
nested link written inside a real one.

`/tactical-fusion check <text>` decorates supplied text and explains the rules
that most often decline a coordinate, without posting anything.

`/tactical-fusion` with no subcommand lists the subcommands; an unknown one is
an error carrying a `TF-NNNN` code. Both replies are ephemeral, and
`TestEverySubcommandIsAdvertised` parses the dispatch switch out of `command.go`
so a subcommand cannot be added without reaching the autocomplete data and
`subcommandList` too.

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

## Location

The location decorator normalizes every coordinate grammar it recognizes to one
WGS84 latitude and longitude. **The identity of a location is the pair `(f, v)`**,
the format id and the canonical token, and everything else is derived from those
two. Nothing derived is ever carried in the URL, which is what makes a link
impossible to disagree with itself.

A third parameter, `r`, carries the author's own text. It is **display only and
derives nothing**, and it exists because the standalone page is what mobile
clients open: that page shows only the link, not the message it came from, so
without `r` the author's spelling is unreachable there. `r` is omitted when it
would only repeat `v`, which is the common case for the USMTF shapes.

`r` is the one parameter that is text from a message echoed onto a public page,
so it is never treated as free text. Four gates, in `validateRaw`, and a failure
rejects the **whole link** rather than dropping the row:

1. at most 64 bytes;
2. an explicit rune whitelist, never a blacklist;
3. it must **anchored-match the token sub-expression for `f`**, which is what
   makes content spoofing structurally impossible rather than merely escaped;
4. `canonical(parse(r))` must equal `v`, so it can never name a different place.

Escaping on output is the fifth layer, not the first.

### Why Location holds text, not floats

`Axis` holds `Deg`, `Min`, `Sec`, a `Frac` **string** and a `FracUnit`, and
`Decimal()` is derived and **must never be reached from `canonical()`**. Holding
a float and rebuilding the canonical form from it cannot round-trip a
sexagesimal token: `340322N` parses to 34.05611111..., and recovering whole
seconds from that float can land on 21.99999999, so the canonical form comes back
as `340321N`. Since a link is rejected when it does not reproduce its own token,
that would turn a token accepted at decoration into a permanent 400 on the page
behind it, editable only by hand.

`Frac` is a string because it is what `canonical()` writes back out. `Digits()`
is `len(Frac)`, derived, so the two cannot disagree. Fractional digits are capped
at 8, past which a float64 stops reproducing the decimal string it came from.

`Conf` carries the USMTF verified confidence digits. They say how well a position
is known rather than where it is, so they never reach `Decimal()` and are
rendered beside the resolution rather than folded into it. The reference
implementation in `mattermost-plugin-aocanywhere` matches them and drops them.

### Grammars

`dd` (signed decimal degrees), `ddh` (hemisphere letters), `dms` (which doubles
as USMTF LATS, and with a fractional second as LATDS and DMPID), `ddm` (USMTF
GEOK), the fixed-width `latd`, `latm` and `vlatm`, and the two projected
grammars `mgrs` and `utm`.

Every scanning pattern and every anchored validator is built from the same
sub-expressions in `grammar.go`, so a change to what a token looks like cannot
reach one and miss the other. That shared origin is also what makes gate 3 above
well defined.

**Signed decimal degrees is the weakest grammar** and has its own switch for that
reason. It requires a comma and at least four fractional digits on both values.
`34.05, -118.25` is declined, deliberately, and that is documented rather than
hidden.

**`latd` has no bare pattern.** `35N079W` is seven characters resolving to a
111 km square, and is reachable behind a USMTF label only. Turning everything off
except `EnableLocationUSMTF` and `EnableLocationMoniker` reproduces the posture of
`mattermost-plugin-aocanywhere`, whose `enhanced_text/patterns.ts` decorates a
coordinate only when the author labeled it. That is a supported configuration
and a test pins it.

**MGRS and UTM are separate switches** (`Formats.MGRS` and `Formats.UTM`), and
**UTM is the one grammar in this package that ships off**. They were one switch
until the band letter was read as a band rather than refused: at that point UTM
became the only grammar that can decorate a *real coordinate* and point at the
wrong place, which is a different cost from every other grammar here, where the
worst case is a false positive on text that was never a coordinate. An install
that wants grid references without that has to be able to have exactly that, so
the switch is split and the default is the safe reading. See the Admin settings
section for the whole argument and `TestMGRSAndUTMSwitchIndependently` for the
pin. Turning UTM off costs no UTM **row**: that is derived from the position, not
matched in the text, and `TestUTMSwitchedOffStillRendersTheUTMRow` says so.

**The grid grammars have a bare pattern narrower than their labeled one**, and
that distinction is per-*expression* rather than per-format, which is why it
lives in `bareExprs` in `grammar.go` rather than in `bareFormats`.

A run-together `18SUJ2347806483` **is** detected unlabeled, under two
restrictions that were arrived at by measuring rather than by arguing. The
argument they replaced was wrong in an instructive way: the stated worry was
part numbers, and the real collision is **hexadecimal**. Over 200,000 generated
short git SHAs, in sentences, through the real tagger, an any-case run-together
pattern decorated 51 of them, about one in 3,900. `58cbe40` is zone 58, band C,
square BE, a 10 km square off the coast of Antarctica.

So `mgrsBareCompactExpr` is **upper case only** and requires **three digits per
axis**. Both numbers were arrived at by being wrong first, which is worth
recording: uppercase-only was claimed to leave nothing behind, and at two digits
per axis it did not, at about one uppercase run in 75,000 (`26HMA1997`,
`3UTA7623`, `6CZA9867` are all valid grid references and all look like part
numbers). Three digits finds none across 1.2 million.
`TestBareGridReferencesDoNotMatchOrdinaryRuns` pins it with a *generated* corpus
sized against the rate each restriction removes, because the version before it
used 30,000 samples against a 1-in-75,000 rate and passed only because its seed
was lucky. Neither restriction puts anything out of reach, since spacing or a
label reaches both.

**UTM run together stays label-only**, and that is not an oversight. After the
zone and band it is thirteen bare digits with no letters to check, so the only
validation available is that the northing lands in the band, which a great many
thirteen-digit runs do. There is nothing there to make narrower.

The letter classes are written once in upper case (`bandBody`, `colBody`,
`rowBody`) and lowered mechanically by `anyCase`, because the bare grammar
depends on the upper-case class being exactly the upper half of the any-case
one.

**The letter after a UTM zone is a latitude band, never a hemisphere.** `11S` is
band S (34 north), not "zone 11, southern hemisphere" (56 south). Nothing in the
token disambiguates the two, so this is a **convention rather than a
deduction**, chosen because this is a plugin for a military audience: MGRS is
the military implementation of UTM (USGS) and the third character of an MGRS
grid zone designator is a latitude band (NGA). UTM therefore takes the whole
band alphabet and there is no separate `utmBandLetter` class.

An earlier version refused `N` and `S` outright as ambiguous. That declined the
ordinary military spelling of a position in order to protect a civilian reading
this audience does not write, and the coordinate that exposed it,
`11S 384640E 3769080N`, is as plain a paste as this feature gets.

**The cost is not symmetric between the two letters**, which is what to know
before touching this again:

- **`N` is nearly free.** Hemisphere-north and band N both use the northing as
  written, so the two readings put the point in the *same place*; the band
  reading merely also requires it to be in the first 8 degrees north. Reading N
  as a band can decline a civilian token but can never misplace one, and
  `TestBandNCannotMisplaceAPosition` says so.
- **`S` is where the risk is.** Hemisphere-south measures from the false origin
  and band S does not, so the readings differ by up to 90 degrees of latitude.

What limits that is `gridPoint`, which already validated **band containment and
zone proximity** before this change, so a token survives only when the military
reading is geometrically consistent with itself.
`TestSouthernHemisphereTokensMostlyDecline` measures the residual over 20,000
generated southern-hemisphere pairs: **10.1% survive**, so about nine in ten are
declined by the band check rather than silently relocated. The assertion is a
loose bound (20%) because the number is a property of the notation, not of this
code; it exists to fail if the band check is ever weakened.

**The axis letters are optional**: `11S 384640E 3769080N` and
`11S 384640mE 3769080mN` parse to the same canonical form as the bare pair, so
they are display only and the `r` parameter carries the author's spelling. They
must be **adjacent to their digits**. An optional letter separated by a space
would reach into the following word: in `11S 384640 3769080 East` the token
would swallow the `E`, the boundary guard would then see a letter, and a token
that decorates today would silently stop.
`TestUTMAxisLettersDoNotReachIntoTheNextWord` pins it.

MGRS is unaffected by all of this: its band letter is followed by two more
letters and could never be read as a hemisphere.

**Monikers are the USMTF field labels**, taken from the standard rather than
invented, and unlike `DTG:` they are **not consumed**: `LATM:` is part of a
structured line an author may be quoting verbatim, so a decorated USMTF line
still reads as USMTF. `LOC:`, `DEPLOC:`, `ARRLOC:` and `ICAO:` are permanently
excluded, because in USMTF they introduce an **ICAO airfield code**, which is a
facility whose position must be looked up rather than computed. An earlier draft
of the plan invented `LOC:` for coordinates, which would have collided with both
the standard and the sibling plugin.

### Boundary guards

`\b` is the wrong guard for a token that does not start and end with a word
character, and the guard **must not be part of the regex**. A pattern that
consumed its own guard characters breaks the *next* match, because
`FindAllStringSubmatchIndex` returns non-overlapping matches: the first token eats
the space the second one needs as its leading guard, and the second goes silently
undecorated. Two grids on one line is the most ordinary input this feature has,
and there is a regression test for exactly it.

**The labeled patterns use the same guard as the bare ones.** They did not: the
moniker guard refused only a letter or a digit on the leading side, so
`logs/MGRS:18SUJ2347806483` was rewritten in place while the bare token in the
identical position was correctly declined, because `badNeighbor` rejects `/` and
the moniker guard did not. Rewriting the middle of a path is the failure this
file is arranged around. The cost, named rather than hidden: a USMTF line quoted
with slash delimiters no longer decorates, which turns out to cost almost
nothing because a genuine one ends `//` and was already declined by the trailing
side of the same guard.

So the guard lives in `Pattern.Boundary`, which the framework calls with the
runes flanking a match. `.` and `,` are rejected on the trailing side, which
costs a decoration when a coordinate ends a sentence with no space. That is a
deliberate trade: at the point the guard runs, `-118.2500.` and the middle of
`-118.2500..-118.2600` are the same thing, and a missed decoration is a feature
gap while rewriting a range is corruption.

### Rendering

**Neither surface has a lead line.** The panel and the page both open straight
onto the table. There used to be a large line above it repeating the grid
reference, three lines above the labeled row that already carried it with a copy
button beside it, so it said the same thing twice and the copy of it that a
reader could actually use was the lower one. The page's
`described` class belongs to the DTG page now.
`TestRenderPageHasNoLeadLineAboveTheTable` and the panel's "shows each reading
once" hold the two halves together, since this layout is implemented twice in
two languages.

Every row renders at the resolution the token carried and no finer, and **rounds
rather than truncating**. A coordinate written to two decimals renders
`34.06° N`; a degrees-only one renders `35°N`, not `35°00'00"N`, because padding
a field the author never wrote is a claim. `LatLonToLATM` in the sibling repo
truncates minutes, which biases every result up to 1.8 km south and west.

"No finer" is a **ceiling and not a floor**, which is why a **value** renders
per axis while the **resolution** renders for the pair. The two halves need not
have been written to the same precision, and `ddh` admits that from ordinary
text: `34.0561N,118.2W` is a thing people paste. Rendering its latitude at the
longitude's one decimal gave `34.1° N`, **4.9 km north of what the author
wrote**. That is the identical defect `canonicalString` is held away from, with
the identical magnitude, so it is fixed the identical way: `Location.Digits()`
(the coarser half) sizes `ResolutionText` and every derived grid row, and
`axisResolutionDegrees` sizes the decimal, DMS and DDM rows from each half's own
`Axis.Digits()`. A pair reading `34.0561° N, 118.2° W` beside "about 11.1 km" is
telling the truth twice rather than contradicting itself, and `Digits()` must
reach nothing that writes a value out.

Both implementations are still live and still have to agree, though not for the
reason they used to. Every surface renders through `format.ts` now, but Go
renders the same values into the `Conversion` payload, which is where a grid
token's readings come from on every surface. So `renderFixtures` in
`server/decorators/location/format_test.go` and the matching table in
`webapp/src/decorators/location/format.spec.ts` are the same inputs and the same
expected strings. **Change one and change the other.** Two of those rows are
mixed-precision pairs and exist only to pin the split above.

The webapp also keeps its own copy of the **canonical shapes**, in `CANONICAL`
in `format.ts`, because `fromParams` has to validate the token a link carries
and the grammar is Go-only. That is a smaller duplication than the grammar and
still a duplication, and it has cost once: the band class was widened here to
read N and S as latitude bands, the webapp kept the older narrower one, and a
UTM link the server had just issued failed the webapp's check.

**That failure is silent by construction.** The click handler reads a null
payload as "not one of ours", stands aside, and the browser follows the link to
the standalone page. The page renders correctly, so it looks like a routing
choice rather than a rejected payload, and nothing is logged on either side. The
only symptom is that clicking a coordinate opens a page instead of the sidebar.

Two things guard it now. The webapp writes the band class **once**, as `BAND`,
and builds both grid patterns from it, for the same reason `bandBody` is written
once here. And `webapp_sync_test.go` reads `format.ts` and compares the band,
column and row classes against this package's, so a change on either side that
does not reach the other fails in Go with the reason spelled out.

The row is labeled `RESOLUTION`, not "precision": a phone with 5 m of real
accuracy still emits six decimals, and "precision" invites reading that as a
claim about the fix.

The **USMTF row** is derived the same way and is the one place a family rather
than a format has to be collapsed to a single answer. The shape follows the
token's resolution on the same principle `gridDigitsFor` uses to size a square,
the coarsest field set no coarser than what the author wrote: LATD for whole
degrees, LATM for whole minutes, LATS for whole seconds, LATDS below that.
Padding LATM onto a degrees-only token would be a claim about two fields nobody
wrote.

It is sized from the **pair**, unlike the DMS and DDM rows and like the grid
rows, because a USMTF token is one fixed-width shape covering both halves: there
is no spelling of it in which latitude carries seconds and longitude only
minutes. `34.0561N,118.2W` therefore renders `3403N11812W`, losing the fine
half's digits in that column alone, and a fixture pins exactly that.

It never carries **confidence**. A verified token states how well its position is
known, which is a property of the measurement rather than of the arithmetic, so
a derived reading cannot inherit it; the token keeps its own Confidence row.

It is also the only derived row whose output is an input grammar, so it is held
to something the others cannot be:
`TestUSMTFRowIsATokenThisPackageAccepts` re-parses every fixture's rendering and
requires it to land within its own resolution. A row emitting a shape nothing
here recognizes would be a value a reader could paste into an ATO and have
refused by the next tool along.

The resolution rule applies to the **derived** grid rows too, which is where it
is easiest to break: every conversion tool in existence hands back a ten-figure
grid reference whatever it was given. `gridDigitsFor` picks the largest square
that is no bigger than the token's resolution, so `35N079W` renders `17S PU`, a
100 km square with no digits at all, rather than a one meter one.

**`roundTo` normalizes negative zero, and that is not tidiness.** On arm64 the
compiler may contract a multiply and a subtract into a single FMA, which the Go
spec permits, so the seconds residue in `degMinSec` came out at about -4e-14
rather than zero. `math.Round` keeps the sign and `fmt` renders `-0`, filling the
field width exactly, so nothing looked wrong: the DMS row showed `0°01'-0"N` and
the USMTF row `0001-0N1000100E`, which no USMTF grammar accepts. It reached 0.13%
of two-decimal hemisphere coordinates, and the same source on amd64 was correct,
so the Go page and the TypeScript panel disagreed about the same link.

That defect is also a lesson about the test that was supposed to catch it.
`TestUSMTFRowIsATokenThisPackageAccepts` asserts a **universal** property, and
ran it over ten hand-picked fixtures, so it was checking a claim about all
inputs against inputs chosen to satisfy it. It is now driven by a generated
corpus, with the fixtures kept only for the shapes a generator reaches rarely.
`TestNoRenderedFieldCarriesASign` is its sibling and states the defect directly,
because the DMS row rendered a sign without failing to re-parse and a round-trip
test alone would have left it broken.

`mgrsAt` **truncates** where every other rendering path rounds, and that is the
same rule rather than an exception to it: a square is chosen by containment and
a distance by nearness. `utmAt` therefore rounds.

### Geodesy

`geodesy.go` is the WGS 84 transverse Mercator series (Snyder, USGS Professional
Paper 1395), hand-written rather than taken from a dependency. The target is
air-gapped installs behind an SBOM and a CVE gate, where a dependency is a
permanent cost to the operator, against maths that has not changed since 1987.

The price of hand-writing it is the obligation to prove it, so `geodesy_test.go`
checks against **figures with an authority outside this repository**: the WGS 84
meridian quadrant (10,001,965.729 m), one degree of latitude at the equator, an
easting of exactly 500000 on a central meridian, and a northing of exactly zero
on the equator. A round trip proves the inverse undoes the forward and says
nothing about whether either is right, so the round trips come after the anchors
and measure error rather than asserting correctness. Inside a standard zone that
error is under a millimeter; in the two hand-widened zones (south-west Norway,
Svalbard) it reaches 4 cm, still twenty times finer than the finest square.

Two things are load-bearing and easy to undo. The **zone exceptions** are real
and getting either wrong puts a point in the neighboring zone, hundreds of
kilometers of easting away. And `unprojectUTM` **guards its footpoint latitude**:
decoding a 100 km row letter means trying candidate northings 2,000,000 m apart,
and a candidate past the pole does not fail loudly, it returns a number. One came
back as latitude 55.4 with a longitude of 3883 degrees, plausible enough on the
latitude alone to be mistaken for the answer by a caller checking only that.

**A grid token is validated geometrically, not textually.** `parseMGRS` builds
the `Grid` and then asks `mgrsCenter` for a position; that one call subsumes the
column letter sets, the row stagger, the band and the zone. Nothing re-checks
those separately, because a second implementation of the letter-set rule is a
second chance to disagree with the one that writes references out.

**Grid-to-grid conversion never goes through latitude and longitude.** `gridPoint`
stays in grid coordinates, because MGRS and UTM are the same easting and northing
relabeled. Routing it through the projection twice landed a meter out:
`33U 291000 5628000` came back as `33U TS 90999 28000`, because an easting
sitting exactly on a cell boundary lost a fraction of a millimeter and then had
it truncated away.

Also note the re-encode round trip is **not** exact at a zone boundary, and that
is a property of MGRS rather than a defect here: squares are defined inside a
zone and clipped where one ends, so a square against a boundary can have its
center in the next zone's grid. `TestGridSquaresDecodeIntoTheSquareTheyName`
asserts a distance everywhere; the exact equality is asserted only away from
boundaries, where it holds.

### The conversion endpoint

`GET /api/v1/convert?f=&v=&r=` returns every derived reading, already rendered.
Strings rather than numbers, because the rendering rules are the interesting part
and they live in Go; handing the webapp numbers would put two implementations of
"never claim more than the token carried" in one repository.

It calls `validateParams`, the same function the public page uses, rather than a
check written for it. A conversion accepting a link the page rejects would let
the sidebar render a coordinate the page refuses.

Passing `r` closes a real gap. Two of the four gates on the author's text need
the token grammar, which is Go-only, so the webapp can check length and alphabet
and nothing more, and the alphabet had to widen to the whole Latin alphabet when
the grid grammars arrived. Before the conversion carried `r`, a hand-edited link
could put prose in the panel's "As written" row beside a position derived from an
unrelated token, with a copy button next to it, while the page refused the
identical link. The panel now asks the same question the page asks and renders
**Not a coordinate** when the answer is no.

That verdict is kept distinct from a request that simply did not arrive.
`rejected` (HTTP 400) refuses to render; `failed` degrades, because nothing may
fail the panel: every row the token yields locally is already on screen and stays
there. Rows waiting on the server read `converting…` and then `unavailable`,
never a zero, since `0.0000° N, 0.0000° E` from a failed conversion is a
position, and a wrong one. Copy buttons are absent over a placeholder.

### Copying

Every row carrying a value has a copy icon at its right edge, on the panel and
on the page alike; the prose rows (resolution, datum, confidence) do not, since
copying "about 11 m" gets you a sentence rather than a position. That is seven
controls, which is why they are icons: a column of "Copy MGRS", "Copy lat/lon",
"Copy DMS" would be wider than the coordinates it sat beside.

The page's script is the reason that page now has one at all, and it is written
so that **no coordinate is ever interpolated into it**: a delegated listener
reads the value out of the row's own cell. That keeps the script a constant the
policy can pin by digest, and keeps the number of escaping contexts at one.

On both surfaces the controls are **hidden until something proves the clipboard
exists**: the page's stylesheet hides them and its script reveals them, and the
panel's `clipboardAvailable()` returns null. A plain-HTTP origin has no
`navigator.clipboard`, which for on-prem and air-gapped installs is the norm, so
a control that cannot work is never drawn. The values stay selectable regardless.

### Prior art

`mattermost-plugin-aocanywhere` parses the same USMTF shapes in
`server/model/usmtf2004/sets/location.go`, and its test vectors are reused here
as positive cases so the two plugins agree about what a LATM is. Four bugs in it
are deliberately not inherited, each with a test here: no range validation at all
(`9999N99999W` parses to latitude 99.98), truncation instead of rounding, a
truthiness check that drops the equator and the prime meridian, and lon-first
axis order in one corner of the repo and lat-first everywhere else.


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

**The label is HTML-escaped specifically.** It lands in the container's
`aria-label`, and `TestMapEscapesAHostileLabel` holds it. Its content is a
country name from generated data, so nothing from a request reaches it today,
but the escaping is what makes that a defence rather than an accident.

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
only a bad one.

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

## The post size limit

**It cannot be read.** `Post.IsValid` takes the limit as an argument, the server
computes it, `AppError.params` carries the real figure but is unexported, and
neither `plugin.API` nor the `model.Config` it hands back exposes it. Everything
below follows from that: the plugin does not know the number and has to be
correct anyway.

Two constants, in `hooks.go`, used where each one's failure mode is survivable:

- **`safePostRunes`** = `PostMessageMaxRunesV1` (4,000). The floor; no store
  reports less.
- **`defaultPostRunes`** = `PostMessageMaxRunesV2` (16,383), which is
  `PostMessageMaxBytesV2 / 4`, the worst-case rune count for the TEXT column the
  message is stored in. What Postgres and MySQL report by default.

The two directions are **not symmetric**, which is the whole reason for the
split. Too high in `decoratePost` means a decorated message the server then
refuses, so the **author cannot post at all**; too low there only means
occasionally skipping decoration. So the hook uses the floor. In the slash
commands the same mistake is a post that is refused, which is reported and
recoverable, so they use the default.

**`example-details` does not trust either.** It packs at `defaultPostRunes`,
and if the first post is refused it repacks at `safePostRunes` and starts again.
The **first post is the canary**: every message in a packing shares one budget,
so if it fits the rest do, and until it lands nothing has been written, so
starting again costs a reader nothing. Once anything has landed there is no
going back, which is why the retry is there and not around the ones that
follow.

That retry is not defensive programming for its own sake. This was hard-coded at
1 MiB for exactly one iteration and the real limit turned out to be 16,383, so a
17,442-rune post came back as *"Post Message property is longer than the maximum
permitted length (TF-16006)"*. A guess that cannot be checked will be wrong
again; the command now survives it rather than reporting it.

`examples` has no fallback, because it is one post by definition, so it measures
against the **floor**: for it the only choices are "fits everywhere" and "might
be refused".

The fake API's `postSizeLimit` refuses an over-long message with the same
`AppError` the server produces, which is the only way to test code whose job is
to discover a limit it is not told.
`TestDetailsPostWhateverTheServerAccepts` runs the whole command at 16,383,
5,000 and 4,000 and requires every message to fit and nothing to be dropped.

## Admin settings

`plugin.json` declares eleven switches: `EnableDTG` (the date-time group
decorator) with `EnableDTGMilitary`, `EnableDTGMoniker` and `EnableDTGTimestamp`
below it, and `EnableLocation` with `EnableLocationDDSigned`,
`EnableLocationLatLon`, `EnableLocationUSMTF`, `EnableLocationGrid`,
`EnableLocationUTM` and `EnableLocationMoniker`. Mattermost's settings schema has
no nesting, so the grouping is by ordering and naming; the parent is enforced in
code, in `Plugin.dtgFormats` and `Plugin.locationFormats`.

**Ten default on. `EnableLocationUTM` defaults off, and is the only one.** The
reason is a difference in kind rather than in degree: every other switch trades
a false positive against a missed decoration, so its worst case is that
something which was not a coordinate gets linked, or something which was does
not. UTM can decorate a **real coordinate and point at the wrong place**,
because its band letter is ambiguous (see Location above) and the band
containment check declines only about nine in ten of the civilian reading. A
format whose failure mode is a confidently wrong position should not be
something a workspace gets without asking.

That default is pinned by `defaultsOff` in
`configuration_settings_test.go`, a **named list with a reason per entry**
rather than a rule, since there is no rule: each is a judgment, and the only way
to be sure one is deliberate is to have written it down. The test checks both
directions and errors on a name in the list that no longer matches a setting,
which is how a default comes back on unnoticed. A second test requires an
off-by-default switch to say so in **both its display name and its help text**,
because an off-by-default format is otherwise indistinguishable from a broken
one: an admin pastes a UTM position, nothing happens, and the setting they would
check is the one already sitting at its default.

`EnableLocationGrid` is MGRS alone despite the generic name, which is kept so an
install that had it on keeps MGRS on across the split.

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

"Customize your view" is a link below both panels. On the DTG it chooses the
timezone rows and how close a DTG has to be before the countdown flashes; on the
location panel it chooses **which rows to show**.

Both editors write through `savePreferencesSection` / `resetPreferencesSection`,
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
  reader who has customised either setting. That was inherent while the route
  was public and is a **choice** now, and the choice is deliberate: honouring
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
strip. Every behavioural test passed while it did: a pin lands, labels draw and
the wheel is ignored at any width at all, so `is a map rather than a strip`
measures the box instead. The 360px cap on the card is a max rather than a width,
so the DTG countdown still shrinks to its own line.

**It carried no hover for a long time, and the blocker was real.** A hover fires
on pointer movement rather than on a click, so wiring one to `/api/v1/convert`
would have put a request behind every coordinate a cursor crossed in a busy
channel. What unblocked it is the module cache in `convert.ts`.

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

**The stored value is the rows a reader HID, not the ones they kept**, and the
direction is the whole design. Empty then means "all of them", so a reader who
never chose is stored as nothing, which is what lets "Restore defaults" stay a
delete. It also decides what happens when a row is **added**: stored this way a
new row appears for everybody, including readers who customised, which is the
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

Both editors **spread the existing preferences** before saving, because a PUT
replaces the whole blob and building one fresh would wipe whatever the reader
chose in the other decorator's editor. The type checker catches this today,
since `Preferences` requires both keys.

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

### Unverified before deployment

Two prerequisites from the implementation plan need a running server and have
**not** been checked:

- Whether Mattermost's search still matches a DTG once the message contains
  `[091630ZAUG26](/plugins/...)`. If the indexer splits on brackets, decorated
  posts become unfindable and the server-side approach needs rethinking.
- Whether the mobile app resolves root-relative markdown links against the
  server URL, **and whether its in-app browser carries the session**. This one
  got more expensive when `/decorate` and `/map` were gated: the whole argument
  for the route being public was that this client has no session, and nobody has
  ever checked. If it does not, every decorated link on mobile costs a sign-in
  before the page. Answer it with a phone and record both halves here.

Also unverified: which post sources actually reach `MessageWillBePosted`
(`p.API.CreatePost`, incoming webhooks, bot posts, `in_channel` command
responses). Record the real behavior here once tested.

## Built-in documentation

`public/help/` holds seven static HTML pages and one stylesheet. Mattermost
serves the bundle's `public/` directory at `/plugins/<id>/public/**`, so
**there is no route for this in the server code** and nothing to add to
`ServeHTTP`. The build already copies it: `build/setup.mk` sets `HAS_PUBLIC`
from the directory's existence and the `bundle` target acts on it.

| Page | Covers | Kept in sync with |
|---|---|---|
| `help.html` | Landing page, what a decorator is, the consequences of server-side decoration, nav cards | The overall surface |
| `formats.html` | Every recognized grammar, the declined list with reasons, protected spans | `server/decorators/dtg/dtg.go`, `parse.go`, `tagger.go` |
| `panel.html` | The sidebar, the hover, the standalone page, Customize your view, the picker, zone ordering | `webapp/src/decorators/dtg/` |
| `admin.html` | One section per switch, and what a switch does not do | `plugin.json` `settings_schema.settings` |
| `commands.html` | `examples`, `example-details`, `check`, bare and unknown subcommands | `server/command*.go` |
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

Every user-facing failure and every `p.API.Log*` call carries a `TF-NNNN`
identifier, so the code a reader quotes from the sidebar and the code an
operator greps out of the log are the same one. That is the whole job: it
pairs a message with its branch within one build.

`server/errcode` holds the catalog. Codes are allocated in thousand-wide
ranges, one per source file, listed in the package documentation. Within a range
they go in source order the first time a file is instrumented and drift
afterwards, which is fine.

**Codes are not stable across releases.** A number may be renumbered, and a
retired one goes back in the pool rather than leaving a gap, so a code carries
meaning only together with the version that emitted it. That is a deliberate
trade for a pre-1.0 plugin, and the thing it costs is old support tickets: a
`TF-16003` quoted last release may name a different branch this one. The
version is in the sidebar's empty state, which is the fastest thing to ask a
reporter for.

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

- Match the style of surrounding code, **except its commenting**. This codebase
  was written with heavy explanatory comments and is no longer written that way:
  do not add prose comments, and remove the ones you encounter in code you touch.
  Compiler and tool directives (`//go:embed`, `//nolint`, `//go:build`,
  `// Code generated ... DO NOT EDIT.`, `// #nosec`, `// eslint-disable-*`) are
  syntax rather than prose and stay. See `~/CLAUDE.md` for the full rule.
- Carry meaning in names and tests instead. An invariant belongs in a test name
  (`TestRoundToNormalizesNegativeZero`); durable design rationale belongs in this
  file. Much of what is written here started as a comment.
- Deleting a comment can destroy the only record of a measurement, a defect that
  caused the current shape, or a contract a future change would silently break.
  Say so and ask before removing one of those, rather than deleting it silently.
- Keep the plugin minimal: avoid abstractions that are not needed by the code that exists today.
- Server: follow Mattermost plugin API conventions. Use `p.API.LogError`/`LogWarn`/`LogInfo` for logging.
- Webapp: prefer functional React components with hooks.

## Build and test

- `make dist` - build the plugin bundle
- `make check-style` - lint both Go and webapp code
- `make test` - run tests. Depends on `map-data-check`, so a basemap that is not
  what the committed source produces fails here rather than in production. That
  ordering is load-bearing: `crypto.subtle` is undefined on a plain-HTTP origin,
  so the webapp's digest check is skipped on dev boxes and on air-gapped HTTP
  installs, and drift would otherwise surface only on an HTTPS install, as every
  panel reporting that the map could not be loaded
- `make map-data` / `make map-data-check` - regenerate the basemap, and fail when
  the committed artifacts are stale
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
  patch, `feat!:` or a `BREAKING CHANGE:` footer → minor while the version is
  pre-1.0, because `bump-minor-pre-major` is set. No ordinary prefix reaches
  `1.0.0` on its own. The one exception is a `Release-As:` footer, which sets
  the version directly and bypasses the calculation entirely. `chore:`/`docs:`/
  `test:`/`refactor:`/`style:`/`build:`/`ci:` don't bump or appear in the
  changelog.
- **Do not** hand-edit `plugin.json`'s `version` or `CHANGELOG.md` for a normal
  release. release-please owns them via its Release PR. The version is seeded at
  `0.0.0` with `initial-version` set to `0.1.0`, which is what makes the first
  release `0.1.0`. `bump-minor-pre-major` does not govern the first release,
  only the ones after it.
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

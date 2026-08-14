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
  - `src/decorators/location/` - the location panel, the copy buttons, `convert.ts` (the conversion client and its degrade-versus-refuse split), and `format.ts`, which slices a canonical token and renders it. No grammar and no projection live there.
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
to the panel. It still honors the reader's flash threshold, or pointing at a
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
onto the table, whose first row is MGRS. There used to be a large line above it
repeating the grid reference, three lines above the labeled row that already
carried it with a copy button beside it, so it said the same thing twice and the
copy of it that a reader could actually use was the lower one. The page's
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

The page is Go and the panel is TypeScript, so they cannot share a render
function. `renderFixtures` in `server/decorators/location/format_test.go` and the
matching table in `webapp/src/decorators/location/format.spec.ts` are the same
inputs and the same expected strings. **Change one and change the other.** Two
of those rows are mixed-precision pairs and exist only to pin the split above.

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
could put prose in the panel's "Original text" row beside a position derived from an
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

### The location rows

`Rows` in `server/decorators/location/rows.go` is the catalog, and it is the
single source for **three** things: the standalone page renders from it, the
panel renders from it again in TypeScript, and a reader's hidden-row list names
its ids. A row present in two of those and not the third fails differently each
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
  server URL.

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

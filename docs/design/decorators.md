# Decorators

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

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

`siteURLPath` returns the **escaped** path, **cleaned**, and both halves are
load-bearing. This is the one value here that can leave the origin, and three
things reach off-origin through a decoded path: `//elsewhere`, which a browser
reads as scheme-relative; `/\elsewhere`, which WHATWG folds to `//` for a
special scheme where Go does not; and `/%09/elsewhere`, since browsers strip
TAB, LF and CR before parsing. `EscapedPath` re-encodes the last two so they
cannot be folded or stripped, closing the class rather than enumerating it, and
it also keeps a space as `%20`: the decoded form produced a markdown destination
containing a literal space, which CommonMark refuses, so the link rendered as
text forever in whatever post it was written into.

`path.Clean` is what makes `//elsewhere` collapse rather than be refused, and
that distinction is the difference between hardening and a regression.
Mattermost derives its own subpath with `path.Clean`, so such an install **is**
served at `/elsewhere` and is merely typo'd; refusing it outright swapped an
off-origin redirect for root-relative links that 404 there, permanently, in
stored post text that correcting `SiteURL` afterwards cannot repair.

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

**A Hover may also decline at render**, by returning `null`. The framework hides
the card's chrome when it renders empty, through a `:empty` rule on
`HOVER_CARD_CLASS` in `buildDecoratorStyles()`, so that is no card rather than an
empty bordered box. The location hover uses it when an admin turns the panel map
off, since a card that is the map has nothing left to be.

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
beside its own labeled variant and `LATD:21N157W` contains `21N157W`.

UTM has a group of its own, headed with the fact that it ships off, because on a
default install every row in it renders with no link and under a generic grid
heading that reads as a bug.

**The rows sit in Hawaii and on Guam**, because those are the two detail map
packages this plugin bundles. Every example is something a reader clicks, and a
coordinate outside the bundled coverage opens on the global tier's coastline
outline: the reader's first encounter with the feature then shows the map at its
worst, on a default install, with nothing wrong. Hickam carries the Hawaii
rows and Andersen and Apra carry the Guam ones, which also gives the eastern
hemisphere and a second UTM zone for free.

**Hickam rather than Pearl Harbor**, and the difference is not cosmetic. Pearl
Harbor is the name that comes to mind first and the middle of it is open water,
so a marker placed there sits in East Loch on every surface that draws one. The
value used is the airfield reference point `PHIK` carries in this plugin's own
airfield database, which also means the coordinate rows and the `ICAO:PHIK` row
name the same place and cannot drift apart.

Five rows deliberately stay outside that coverage, because their subject is
arithmetic no in-theater position can demonstrate: the pole and the antimeridian,
Null Island, a south-and-east pair, the south polar GARS cell, and the UTM band
trio. Hawaii is band Q and Guam is band P, so neither can show that `N` and `S`
in a UTM token are latitude bands rather than hemispheres, which is the whole
content of those three rows.

**"Drawn as a map in the channel" is the one group whose claim is per row rather
than per group**, carried by `detailExample.inline`. Every row in it decorates,
so `decorates` says nothing about what the group is for, and the group
deliberately holds both answers because the distinction *is* the content: what a
reader needs is which messages qualify, and a group of only-qualifying rows would
not teach that. The rows cannot demonstrate it, only describe it, since an
inline map needs the whole message and every row here lives inside a post full of
other rows. A reader copies one and posts it alone.

`TestEveryInlineDetailDrawsWhatItClaims` holds those rows to the flag in both
directions **through `decoratePost`**, not through the tagger, so a row promising
a map fails if anything on the stamping path stops working rather than only if
the tagger's verdict moves. It is scoped to that one group, because most single
coordinates elsewhere in the list would also stamp and marking each of them would
turn an incidental property into thirty claims nobody reads; a second loop
refuses the flag anywhere outside the group, which is what keeps that scoping
honest rather than merely narrow.

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

### `SoleFencedBlock`, and its relationship to protected spans

`SoleFencedBlock` answers "is this message exactly one closed fenced block, and
what is around it". It lives here, beside `blockRanges`, and is built from the
same `fenceWidthOf` / `closesFence` / `isIndentedCode` primitives, because a
second fence scanner elsewhere in the tree is exactly the drift this package
writes tests against.

It returns the info string as well as the body. An info string is fence syntax,
not the caller's syntax, so it belongs to whatever parses fences.

`TestSoleFencedBlockIsAlwaysProtected` holds it to `findProtectedRanges`: any
block this function accepts lies inside a range that one reports. That is what
guarantees no tagger candidate is ever found inside a fence, so the decoration
step and the Cursor on Target step can never disagree about the same characters.

It is **not** what keeps those two steps apart. Prose outside the fence is
decoratable, and the exclusivity rule in [`cot.md`](cot.md) is what settles that.

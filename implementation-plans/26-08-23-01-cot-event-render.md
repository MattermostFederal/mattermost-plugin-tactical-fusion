# Cursor on Target event rendering

## Overview

Render a Cursor on Target (CoT) event, posted either as a fenced ```xml / ```cot
block or as the sole file attached to a post, as a custom post type: a card
carrying the event's identity, position, timing and accuracy, with a map drawn
under it showing the position and its stated circular error.

## Problem statement

CoT is named in `CLAUDE.md` as one of the things this plugin exists to enrich,
and nothing implements it. An operator who pastes a CoT event today sees a wall
of XML: the position is a pair of attribute values nobody can place by eye, the
`type` is an opaque hierarchy string, `ce` is a number with no picture attached,
and `stale` is an ISO timestamp that says nothing about whether the track is
still good.

The plugin already owns almost every part needed to answer those questions. What
is missing is the recognition step, one post body, and one map layer.

## Current state

`MessageWillBePosted` -> `decoratePost` runs the tagger over the message,
rewrites recognized tokens into decorator links, and, when the whole message was
one token, stamps a custom post type plus props so the webapp owns the body.
`webapp/src/decorators/PostBody.tsx` renders that body; `LocationMap` draws the
map inside it.

### Current gaps

- **A fenced code block is a protected range** (`blockRanges` in
  `decorators/tagger.go`), so the tagger deliberately sees nothing inside one.
  CoT recognition has to be a separate step, not a new decorator pattern.
- **Three early returns in `decoratePost` block the file case**, not one:
  `hooks.go:95` returns on `post.Message == ""`, which is the file case by
  definition; `hooks.go:98` returns on `p.decorators == nil`, which CoT does not
  need; and the tail returns `nil` whenever the tagger changed nothing.
- **No code anywhere reads a file attachment.** `plugin.API.GetFile` and
  `GetFileInfo` are never called.
- **The `Decorator` interface does not fit CoT.** It is token -> link -> page.
  CoT produces no link, so it has no query string and `RenderPage` would have
  nothing to render.
- **`LocationMap` draws pins and rectangles, not circles**, so there is no way
  today to draw a stated accuracy.
- **There is no XML parsing anywhere in the plugin**, in either language.

## Phase strategy

| Phase | Focus | Value |
|---|---|---|
| **Phase 1a** | Recognition, parse, stamp, card with no map. The risky mechanics, landing first | Correctness |
| **Phase 1b** | The map, the CE circle, the position link | **The picture** |
| **Phase 2** | CoT sidebar panel, DTG links for the timestamps, live countdown in the panel only | Depth |
| **Phase 3** | `<link>` relations, multi-event blocks, a bare `<event>` with no fence | Shipped |
| **Phase 4** | TAK data packages (`.zip`), live CoT feeds | Deferred |

Phases 1a and 1b are this plan and ship together. They are separated so the post
hook and the fallback paths are green before any map work starts.

## Design principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Recognition | A separate step in the hook, gated and recovered on its own | A decorator pattern reaching into a protected range | `decorators/tagger.go` `blockRanges` |
| Parsing | Go only. The webapp reads what Go produced | A second CoT parser in TypeScript | `decorators/types.ts`, "The webapp side never parses the token" |
| What the webapp reads | Props, and **never `post.message`** | Any byte offset, digest or fence rule duplicated across the two languages | The thirteen sync rows in `CLAUDE.md`, each a liability |
| Freshness | `post.edit_at === 0`, plus `file_ids` still naming the stamped file | A hash the two languages must agree on | Mapping, "Props and message must agree" |
| Position | Go emits the location decorator's `(f, v)` at **the digits the XML carried** | Padding a float to six places and calling it the author's resolution | `CLAUDE.md`, "Render to the resolution the token carried" |
| Unknown position | Show the value, name it as the CoT sentinel, draw no pin | Asserting "unknown" about a value we hold, or a pin at 0,0 | `LocationMap`, "no pin is ever drawn at a guessed position" |
| Accuracy | Drawn, not just stated | A dot that looks identical at CE 3 m and CE 9 km | This plan's own worst failure mode |
| Time | Absolute Zulu, plus a delta between two server-side values | A verdict computed from the reader's clock | Below, "Time without a trusted clock" |
| Unknown type code | Show the raw string and say it is unrecognized | Guessing a label from a prefix | `airportResponse`'s `found` shape |
| The post path | Cheap gates first, size caps, its own recover, an atomic commit | Anything that can stop somebody posting | `hooks.go:17-49`, `safePostRunes` |

## Reference patterns

- `server/hooks.go:decoratePost` / `stampStandalonePost` - the stamp, the
  `post.Type == ""` guard, `AddProp`, the captured-API recover.
- `docs/design/mapping.md:1252` "The map under a post" - the measured cost of
  setting `Post.Type`, including that **attachments are dropped** when a plugin
  owns the body.
- `webapp/src/decorators/PostBody.tsx` - the link outside `ErrorBoundary`, the
  map inside it, the `compactDisplay` opt-out.
- `webapp/src/decorators/location/LocationInline.tsx` - `useNearViewport`, the
  reserved height, the feature gate read in the outer component.
- `server/api.go:airportResponse` - a discriminated JSON shape and Go-rendered
  display strings.
- `webapp/src/decorators/location/map/LocationMap.tsx:626` - a zero cell already
  means "draw no cell", so the CoT map needs no change there.

## Requirements

- [ ] A message containing exactly one ```xml or ```cot fence whose body is a
      valid CoT event is stamped `custom_tf_cot`. Text around the fence is
      allowed and is rendered by the card as plain text.
- [ ] A post with exactly one attachment named `*.cot` or `*.xml` whose content
      is a valid CoT event is stamped the same way. A covering note is allowed
      and is rendered the same way.
- [ ] The card names the event: callsign or uid, the decoded type, affiliation,
      and how the position was obtained.
- [ ] The card renders the substance of a position report, because a card that
      shows less than the XML it replaced is worse than the XML: position,
      altitude, circular and linear error, speed and course, team and role.
- [ ] The map draws the position **and its stated circular error**, so the
      picture cannot claim more precision than the event does.
- [ ] The position links to the location decorator, so the sidebar, the eleven
      readings and "Open larger" all work with no new plumbing.
- [ ] `time`, `start` and `stale` render as absolute Zulu DTG strings, with a
      staleness reading that does not depend on the reader's clock.
- [ ] The XML stays reachable: a `<details>` disclosure for the fence case, a
      download link for the file case.
- [ ] Nothing here can refuse a post, and no failure produces a card that
      disagrees with the source it came from or a post body that renders blank.
- [ ] An author who wrote an explicit ```cot fence that was refused is told why.
- [ ] Every log call and user-facing failure carries a `TF-NNNN`.

## Out of scope

- A standalone server-rendered CoT page. There is no link to open one from, and
  a query string cannot carry an event.
- A CoT right-hand-sidebar panel (Phase 2). See "What a Phase 2 panel will cost"
  below, which is recorded now because the choice is cheap now and not later.
- Rendering the timestamps as DTG decorator links (Phase 2).
- A live ticking countdown anywhere in the channel. See "Time without a trusted
  clock".
- More than one `<event>` in a block, a bare `<event>` with no fence, `<link>`
  relations, TAK data packages, live feeds.

## Technical approach

### Recognition, in the hook

`decoratePost` grows a second step, and the two are **mutually exclusive**. The
restructure is larger than "move the return", because two of the existing early
returns belong to the decoration step alone:

```
decoratePost(post, ref):
    if post == nil: return nil                    # hoisted, shared
    if stamped := cotStamp(post, ref); stamped != nil:
        return stamped                            # CoT owns this post body
    return decorateMessage(post, ref)             # owns the Message=="" and
                                                  # decorators==nil returns
```

**A CoT post is never decorated, and that is the whole reason CoT is tried
first.** Once text around the fence is allowed, one message can carry a
coordinate in its lead and a CoT event in its fence. Decorating that lead would
rewrite it into a markdown link, and the card renders `lead` and `trail` as
plain text nodes, so the reader would see `[34.0561N 118.2500W](/plugins/…)`
spelled out above the card. The cause is the same one that forces `lead` and
`trail` to be plain text in the first place: a plugin post body has no access to
Mattermost's markdown renderer, so a link written into that text cannot render
there either.

Capturing `lead` and `trail` from the *undecorated* message and decorating
anyway was the obvious alternative and is declined: it leaves a decorator link
in the STORED message that the webapp then suppresses, so mobile, exports and
Postgres search results would show a link the webapp does not, and the plugin
would have written a permanent rewrite into a message whose rendering it hides.

Decoration and CoT stamping are two rendering strategies for one post body and
only one may own it. The cost, recorded now rather than discovered later: a
coordinate or a date-time group written beside a CoT event is not decorated. The
position inside the event is rendered by the card with its own location link
regardless.

`cotStamp` runs under **its own** recover, with the API captured before the
deferred call exactly as `decoratePost` does. Because it now runs first, the
recover's job is to leave the post exactly as it found it so that decoration
still gets its turn: a panic in CoT parsing must cost the author neither their
post nor their decoration, and must never reach the hook.

**The commit is atomic.** The candidate props map is built and measured in full
first; only then are `post.Type` and the props written, with nothing between
them that can panic. A half-stamp is worse than no stamp: `Post.Type` survives every edit
and there is no `MessageWillBeUpdated` hook, so a post that got the type and not
the props would lose search, embeds and translation forever while rendering as
plain text. `cotStamp` works on a clone it owns and returns it, so a panic
mid-way discards the clone rather than leaving the caller's post mutated.

Gates, cheapest first, all of which must pass:

1. `p.configurationLoaded()` and `EnableCot`.
2. `post.Type == ""`. Another integration's custom type is real mission content.
   (No separate `isSystemPost` call: that is a prefix test on the same field and
   is subsumed.)
3. A source, in this precedence:
   - **The fence case**: `decorators.SoleFencedBlock(post.Message)` returns
     exactly one closed fence whose info string is `xml` or `cot`.
   - **The file case**, tried only if there is no qualifying fence:
     `EnableCotFile`, `len(post.FileIds) == 1`, `GetFileInfo` reports a name
     ending `.cot` or `.xml` and `Size <= maxCotBytes`, then `GetFile`.
4. `cot.Parse` accepts the bytes.
5. The props fit. See "The props budget", which is a real refusal path.

### `SoleFencedBlock`, in `decorators`

`decorators.SoleFencedBlock(message) (info, body, lead, trail string, ok bool)`
is added beside `blockRanges` and built from the same `fenceWidthOf` /
`closesFence` / `isIndentedCode` primitives. A second fence scanner in
`server/cot/` is exactly the drift this repository writes tests against.

It requires **exactly one** closed fence in the message, and returns the text
before and after it. `info` is returned rather than checked at the call site
because an info string is fence syntax, not CoT syntax, and belongs to whatever
parses fences.

`TestSoleFencedBlockIsAlwaysProtected` asserts that any block this accepts lies
inside a range `findProtectedRanges` reports. That closes the narrower question,
that no tagger candidate is ever found INSIDE a fence, which is what keeps the
two steps from disagreeing about the same characters. It is not what keeps them
apart: prose outside the fence is decoratable, and the exclusivity rule above is
what settles that.

### `server/cot/`

New package, not under `decorators/`, because CoT is not a token, has no link
and has no page.

| File | What lives there |
|---|---|
| `cot.go` | `PostType`, the caps, the `Event` struct, `Props`, display rendering |
| `parse.go` | `Parse([]byte) (Event, error)` |
| `types.go` | The CoT type hierarchy and `how` decoding |

Three files, not five: the digest is gone (below) and the display strings are
one struct's rendering, which belongs beside it.

**Refusals, before decoding**: input over `maxCotBytes` (64 KiB); input that is
not valid UTF-8.

**Refusals, during decoding**, all token-aware rather than by substring scan:

- Any `xml.Directive` token. `<!DOCTYPE …>` arrives as a `Directive`, verified
  against the pinned SDK, so this cannot be smuggled past with a comment or
  leading whitespace the way a `strings.Contains` check can.
- Any `xml.ProcInst` other than a leading `xml` declaration.
- More than one root element. Verified: `encoding/xml` decodes
  `<event/><event/>` without complaint, so "exactly one" is an explicit check
  and a test, not an assumption.
- A root that is not `event`.
- Element depth over `maxCotDepth`, and a total element budget.

`Strict: true` with no entity map and no `CharsetReader` closes the rest: an
undefined entity is a syntax error (verified: `&x;` fails), which kills entity
expansion, and a non-UTF-8 `encoding=` declaration errors because no
`CharsetReader` is set, which closes the UTF-16 smuggling route. The byte cap is
the real bound on CPU; the depth and element budgets are defence in depth.

**What must be present to stamp at all**: a root `event` carrying a non-empty
`uid`, a non-empty `type`, and a parseable `time`. Without those the card would
be a table of "unknown" that is strictly worse than the code block it hid, so
the post is left alone.

It reads `event/@{version,uid,type,how,time,start,stale}`,
`point/@{lat,lon,hae,ce,le}`, and from `detail`: `contact/@callsign`,
`__group/@{name,role}`, `track/@{speed,course}`, `remarks`. Everything else in
`detail` is ignored rather than refused, because CoT detail is open-ended by
design and an unknown child is ordinary traffic.

**Every author-controlled string is capped and sanitised.** `remarks` at
`maxRemarksRunes` (1024) with a truncation marker; `uid`, `callsign`, the raw
`type`, `group` and `role` at `maxFieldRunes` (128). All of them have C0 control
characters (except tab and newline) and the bidirectional overrides
(U+202A-202E, U+2066-2069) stripped. A 400-character or right-to-left-override
callsign must not own the card header directly above a coordinate.

**The unknown sentinels.** CoT writes `9999999.0` for an unknown `hae`, `ce` or
`le`, and real emitters write it as `9999999`, `9999999.0`, `9999999.00` and
sometimes `-1`. The test is therefore on the parsed float: `>= 9999999`, or
negative for `ce`/`le`, means unknown. The row says "not stated"; it never shows
`9999999.0 m` as a figure an operator might act on.

**Position honesty.** A pin is drawn only when `lat` and `lon` parse and are in
range. `0,0` parses and is in range, and is also overwhelmingly what an unset
CoT point contains, so the card shows the value **and** says it is the CoT
sentinel for an unset position, and draws no pin. Saying "position unknown"
about a value we are holding would be the card asserting an ignorance it does
not have; drawing the pin would put a marker in the Gulf of Guinea. Neither is
acceptable, and stating both facts is.

**Position precision.** The `(f, v)` pair is built from the **literal `lat` and
`lon` attribute strings**, validated against the `dd` grammar and normalised
only where that grammar requires it. It is never reformatted to a fixed width.
`lat="30.0"` yields `v=30.0,-86.0`, not `v=30.000000,-86.000000`, because
`CLAUDE.md` requires rendering at the resolution the source carried and a padded
figure is a claim the event never made. A position whose literal strings do not
satisfy the `dd` grammar carries no `(f, v)` at all, and the card renders the
position as text with no link.

**Type decoding is narrow on purpose.** For an atom (`a-*`), the affiliation
letter and the battle-dimension letter come from two small tables, plus a short
table of well-known whole types (GeoChat `b-t-f`, SPI `b-m-p-s-p-i`, emergency
`b-a-o-tbl`, CASEVAC `b-r-f-h-c`, route `b-m-r`, drawing `u-d-f`). Anything else
renders the raw string and says it is unrecognized. The tables and their
provenance go in `docs/design/cot.md`.

### Time without a trusted clock

The staleness verdict must not come from the reader's workstation. Clock drift
of tens of minutes is routine on field kit and air-gapped VMs, and two readers
looking at one post must not disagree about whether to act on it.

- **`time`, `start` and `stale` render as absolute Zulu DTG strings**, produced
  in Go. Those are facts about the event and need no clock at all.
- **The validity window is `stale − time`**, rendered as "valid for 2m". Both
  values come from the event, so the figure is the same on every machine.
- **The age reading is `stale_at − post.create_at`**, rendered as "stale 2m
  after posting". Both are server-side values, so this too is clock-independent.
  It is computed **in the webapp**, not in Go, because `post.CreateAt` is 0
  inside `MessageWillBePosted`: `referenceTime`'s own comment in `hooks.go`
  records that the server fills it in later. `valid_for` needs no such care and
  stays in Go, since `stale − time` is entirely internal to the event.
- **There is no live countdown in the channel.** `dtg/Countdown.tsx` runs a 1 s
  interval and a 600 ms pulse and deliberately ignores `prefers-reduced-motion`,
  which was argued for a single RHS panel. Thirty PLI cards would be sixty
  timers and a wall of pulsing red, and, worse, a ticking clock on a one-time
  paste reads as a live feed. The card says in words that it is a snapshot of
  what was posted. A live countdown belongs in the Phase 2 panel, where the
  existing argument for it holds.

### Props

Under a **distinct** key, `tactical_fusion_cot`, with its own version. Sharing
`decorators.PostPropsKey` was the first draft and is declined: `post_props.ts`
accepts any blob under that key with `version === 1` and a string `type`, then
flattens it into `URLSearchParams`, so one version number would describe two
incompatible shapes across two languages and neither side could bump it alone.

```json
{
  "version": 1,
  "source": "fence",
  "lead": "latest PLI from ALPHA",
  "trail": "",
  "src": "<event …>…</event>",
  "file_id": "", "file_name": "",
  "event": {
    "uid": "J-01334", "callsign": "DELTA1",
    "cot_type": "a-f-G-U-C", "type_label": "Combat unit",
    "affiliation": "friend", "how": "m-g", "how_label": "Machine, GPS",
    "time": "051143Z APR 05", "start": "…", "stale": "…",
    "stale_at": "1112694338070", "valid_for": "2m",
    "format": "dd", "value": "30.009027,-85.957874",
    "lat": "30.009027", "lon": "-85.957874",
    "position_note": "", "region": "United States",
    "hae": "-42.6 m", "ce": "45.3 m", "ce_meters": "45.3", "le": "99.5 m",
    "speed": "0.0 m/s", "course": "180.0°",
    "group": "Cyan", "role": "Team Member",
    "remarks": "…"
  }
}
```

Every value is a string, for the reason `Conversion` and `airportDetails` are
strings: the rendering rules live in Go and a second implementation in
TypeScript is a second thing to get wrong. `stale_at` and `ce_meters` are the
only two the webapp does arithmetic on, for the age reading and the circle.

**`lead`, `src` and `trail` are why the webapp never reads `post.message`.**
They are the text before the CoT source, the source itself, and the text after
it, and the card renders them in that order with the card in the middle. Storing
them costs roughly one extra copy of the message in props; what it buys is that
no byte offset, no fence rule and no digest has to mean the same thing in Go and
in JavaScript, which is where every reviewer independently expected this design
to break. For the file case `lead` is the covering note, `trail` is empty, and
`src` is the file's text, which is also what makes "Show XML" work for files
without a second fetch.

**`src` is capped for storage at `maxInlineSrcRunes` (8192), separately from the
64 KiB parse ceiling.** `lead` and `trail` are bounded by the message length and
need no cap of their own, but a file-case `src` is bounded only by what `Parse`
accepts and is stored JSON-escaped on every such post. A CoT event is typically
one to three kilobytes, so the parse ceiling is a ceiling and not an
expectation. Over the cap `src` is omitted and the disclosure says the source is
too large to show inline; the event is still parsed, still stamped, and in the
file case the download link is already there.

**Absent rather than empty**, following `airportResponse`: an event with no
usable position carries no `format`, `value`, `lat` or `lon` keys at all, so the
webapp cannot build `/decorate/location?f=&v=` and hand a reader a teal chip
that opens an empty panel. `position_note` carries the reason when there is one.

**The freeze is accepted and written down.** Every displayed string is frozen at
post time, so a corrected type table or an updated country polygon never reaches
posts already stamped, and a card can come to disagree with the location panel
its own position link opens. The mitigations are that the raw values
(`cot_type`, `lat`, `lon`, `ce_meters`) are stored beside the rendered ones so a
later version can re-derive, and that `version` is the migration lever: a card
built from a version this bundle does not know falls back rather than
misreading. `docs/design/cot.md` records this as a decision, not an oversight.

### The props budget, which is a refusal path

`Post.IsValid` rejects a post whose **whole props map** exceeds
`model.PostPropsMaxRunes` (800000), and `AddProp` deliberately shares that map
with any other integration's props. A cap derived from `maxCotBytes` rather than
from that limit would let a CoT post carrying a webhook's props be refused
outright, which is the "author cannot post at all" failure the entire comment
block at `hooks.go:17-49` exists to prevent, reproduced in the props channel.

So: build the candidate map from a copy of the post's existing props, then
measure it with **the same call `Post.IsValid` uses**,
`utf8.RuneCountInString(model.StringInterfaceToJSON(props))` at
`model/post.go:566`, and refuse if it exceeds `model.PostPropsMaxUserRunes`
(760000, the limit less the 40000 runes the SDK reserves for "system / pre-save
modifications"). Mirroring that exact call matters: the limit is rune-oriented
over the marshaled JSON, so an estimate from byte counts or from `maxCotBytes`
can be wrong in either direction. Over budget the clone is discarded and the
post is left alone, and the log line carries the measured size so an admin can
tell a large event from a large foreign blob. A test posts a maximum-size event
beside a large foreign props blob and asserts the post still lands.

### Telling the author when a ```cot fence is refused

An ```xml fence is ambiguous and silence is right for it. An explicit ```cot
fence is an unambiguous statement of intent, and an author who gets nothing back
cannot tell malformed XML from a disabled switch from a covering note, and will
retry by pasting the event again into the channel.

So for the `cot` info string only, and only for refusals the author can act on
(a parse failure, an oversized event, a props overflow), `cotStamp` sends an
ephemeral post naming the reason and its `TF-NNNN`. Not for `EnableCot` off,
which is not the author's to fix. It runs after the stamp decision, inside the
recover, and its result is ignored (`SendEphemeralPost` returns a post, not an
error): this may never be the thing that delays a post.

**The text must be self-contained.** An ephemeral sent from this hook carries no
`PostId` and cannot point at anything, so it names the CoT event the author has
just tried to post, in words, rather than saying "this post". Whether it arrives
before or after the post itself is an entry for `docs/design/unverified.md`.

### The map, and the CE circle

`LocationMap` gains one optional prop, `accuracyMeters?: number`, and with it a
GeoJSON source and a fill-plus-line layer drawing a 64-gon approximation of the
circle. Every existing caller passes nothing and is unaffected; a test asserts
the location surfaces still draw exactly what they drew.

**The vertices are geodesic, not equal-degree.** A metre is a different number
of longitude degrees at every latitude, so the offsets are
`dLat = m / DEGREE_METERS` and `dLon = m / (DEGREE_METERS * cos lat)`, reusing
the `DEGREE_METERS` already exported from `map/span.ts` and already pinned to Go
by `webapp_sync_test.go`. An equal-degree polygon is correct at the equator and
increasingly wrong toward the poles, which for a layer whose entire purpose is
to stop the map overstating a fix would be the wrong way to be wrong. Tested at
two widely separated latitudes.

This is the deliberate cost of not shipping an unqualified dot. A 3 m fix and a
9 km fix must not render identically, and CE is the channel through which this
plan's own worst failure mode arrives. The accessible label gains the accuracy
alongside the region, so the figure is not colour-and-geometry only.

An event whose `ce` is unknown draws the pin with **no** circle and the card
says the accuracy is not stated. Drawing a default radius would be inventing
one.

`webapp/src/cot/CotMap.tsx` is the thin wrapper: `LocationMap` with
`{lat, lon, cellDegLat: 0, cellDegLon: 0, region, accuracyMeters}`, inside
`useNearViewport` with `MAP_HEIGHT` reserved, exactly as `LocationInline` does,
because the WebGL context budget is the same budget. It reads `features.mapInline`
and the reader's `INLINE_ID` row preference in the outer component, so a reader
who hid the map under coordinate posts has hidden it here too.

### Admin settings

Two new switches in a fifth section, `cot`, titled "Cursor on Target".

| Key | Default | What it does |
|---|---|---|
| `EnableCot` | true | The whole feature. Off: nothing new is stamped; posts already stamped keep rendering. |
| `EnableCotFile` | true | Whether an attached `.cot`/`.xml` file is read at post time. The only setting in the plugin that puts a filestore read on the post path. |

**There is no `EnableCotMap`.** `EnableLocationMapInline` already exists and
already means "the map under a post"; the CoT map reads the same
`features.mapInline`, which is ANDed with `EnableLocation` and
`EnableLocationMap` **in Go** beside `locationFormats`, so the parent switches
cannot be re-implemented differently here. The work is widening that setting's
wording in `plugin.json`, `README.md` and `public/help/admin.html`, not adding a
twenty-first switch and a `/features` field. `featuresResponse` does not change.

`EnableCot`'s help text carries the same warning `EnableLocationMapInline`
carries, in the same words: a stamped post is indexed by Elasticsearch and
OpenSearch and never matches, so it is absent from search and Recent Mentions,
and link previews, embeds, attachments and auto-translation are dropped for it.

`EnableCot` governs stamping only, matching the format-switch invariant; the map
is read at render, matching the map invariant.

### The webapp

New directory `webapp/src/cot/`, a sibling of `decorators/`.

| File | What |
|---|---|
| `types.ts` | `CotPayload`, `fromProps(props)`, mirroring the Go shape |
| `CotPostBody.tsx` | The registered post body and its fallbacks |
| `CotCard.tsx` | Header, rows, XML disclosure, download link |
| `CotMap.tsx` | `LocationMap` with the circle, the viewport gate, the reserved height |

`index.tsx` registers it beside the decorator bodies, unconditionally, the same
posture decorators take with every switch off: a post already stamped must keep
rendering after an admin turns the feature off.

`CotPostBody` decides in this order:

1. `fromProps(post.props)` returns null, or its `version` is unknown -> fallback.
2. `post.edit_at !== 0` -> fallback. An edit is the one event that can make props
   and message disagree, and `edit_at` answers it exactly, on the post, with no
   code on either side. **The digest this plan first proposed is gone**: FNV-1a
   64 hand-written twice would have been a fourteenth cross-language duplicate,
   it is underspecified across UTF-8 bytes and UTF-16 strings and JavaScript's
   missing 64-bit integer, and it would have been computed inside a multi-plugin
   hook chain where a later plugin can still rewrite the message, so one
   co-installed formatting plugin would have turned every CoT post on that
   install into raw XML forever with nothing logged.
3. `source === 'file'` and `post.file_ids` no longer contains `file_id` ->
   fallback. `PostPatch` carries `file_ids`, so an attachment can be swapped out.
4. Otherwise the card.

**The fallback can never be blank.** It renders `post.message` as pre-wrap text,
and, when the message is empty, a short line with a download link per entry in
`post.file_ids`. This matters because the file case has an empty message by
construction and, per `docs/design/mapping.md:1317` measured against Mattermost
master, **attachments are already dropped when a plugin owns the body**. That
question is answered, not unverified: without this, a props loss on a file post
leaves a permanently blank post with no exit.

For the same reason the card **always** renders the source file as a download
link in the file case. It is not contingent on anything.

The card's nesting mirrors `DecoratorPostBody` exactly and for the same reason:
Mattermost wraps a registered body in `PluggableErrorBoundary` and replaces the
whole body on a throw. So `lead`, the header, the rows, the XML disclosure, the
download link and `trail` render **outside** `components/ErrorBoundary`, and
only the map renders inside it.

**Accessibility**, stated here because this repository states it everywhere else:

- The rows are a `<dl>` inside a group with an `aria-label`, following
  `LocationReadings.tsx`'s `aria-label='Readings for this coordinate'`.
- "Show XML" is a native `<details>` / `<summary>`, so it exists for a keyboard
  user without a single line of key handling.
- The stale state carries **text**, not only a warning colour.
- Nothing on the card updates on a timer, so there is no live region and no
  `prefers-reduced-motion` question.

`compactDisplay` renders `lead`, the header, the rows and `trail`, and no map.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Decorator or new package? | New `server/cot/` | No token, no link, no page; `RenderPage` would have nothing to render |
| Parse in Go or TypeScript? | Go, and the webapp reads props | The standing rule; a second CoT parser is permanent drift |
| Parse at post time or render time? | Post time, into props | The file case would otherwise need a per-reader authorized file route; props inherit the post's own visibility |
| How do props and message stay honest? | `edit_at === 0` plus `file_ids` | Free, exact for the case that matters, and adds no sync point |
| Text around the fence, or a covering note? | Allowed, rendered as plain text | The common real flow; the cost is that markdown beside a CoT event is flattened, which the help pages state |
| Which source wins when both are present? | The fence | It is the visible content; the file is a fallback source |
| A CoT post that also carries a decoratable token? | Not decorated. CoT is tried first and wins | The card renders surrounding text as plain text, so a decorator link written into it would show as literal markdown |
| CE on the map? | Drawn as a circle, Phase 1 | An unqualified dot is this feature's worst failure mode |
| `0,0`? | Show the value, name the sentinel, no pin | Never assert ignorance about a value we hold, never draw a pin we cannot justify |
| Position digits? | The source's literal digits | `CLAUDE.md`: render to the resolution the source carried |
| A live countdown? | No, not in a channel body | A ticking clock on a one-time paste reads as a live feed, and thirty cards is sixty timers |
| A separate props key? | Yes, `tactical_fusion_cot` | One `version` cannot describe two shapes across two languages |
| A third admin switch for the map? | No, reuse `EnableLocationMapInline` | It already means "the map under a post", and its parent ANDs already live in Go |
| Where does the fence scanner live? | `decorators`, beside `blockRanges` | One fence implementation, held to the protected ranges by a test |
| Error code range? | `11000-11999` | Every new call site is in `server/hooks.go`; `server/cot` returns errors and logs nothing |
| `/tactical-fusion example cot`? | Cut | `examplePostLines` builds every row by running the tagger, which by construction never matches a CoT fence |

## What a Phase 2 panel will cost

Recorded now, while the choice is still cheap. The RHS dispatches only through
the decorator registry: `RhsView.tsx` is `get(selection.type)`, `RhsTitle` does
the same, and `selection.ts` is only ever set from `parseDecoratorHref` plus
`decorator.fromParams`. `server/cot/` and `webapp/src/cot/` sit outside both
registries by design. A CoT panel therefore costs either a second RHS dispatch
path, or a sham decorator registration that also emits a stylesheet rule and a
click route for a `/decorate/cot` page that `http.go` will 404. This goes in
`docs/design/cot.md` in Phase 1.

## Files to modify

| File | Change |
|---|---|
| `server/cot/cot.go` | New: `PostType`, caps, `Event`, `Props`, display rendering |
| `server/cot/parse.go` | New: bounded, strict, token-aware XML parse |
| `server/cot/types.go` | New: type and `how` decoding |
| `server/cot/*_test.go` | New: fixtures, refusals, budgets, sentinels, precision |
| `server/decorators/tagger.go` | Add `SoleFencedBlock` |
| `server/decorators/tagger_test.go` | The protection-agreement test |
| `server/hooks.go` | Hoist `post == nil`; move `Message == ""` and `decorators == nil` into the decoration step; add `cotStamp` with its own recover and atomic commit |
| `server/hooks_test.go` | Every gate, the atomic commit, the props budget, disjointness |
| `server/configuration.go` | `EnableCot`, `EnableCotFile`, and a `cot()` accessor beside `locationMaps` |
| `server/errcode/codes.go` | New codes in the `11000` range plus `AllCodes` entries |
| `plugin.json` | The `cot` section, two settings, and widened `EnableLocationMapInline` wording |
| `README.md` | The widened `EnableLocationMapInline` wording |
| `webapp/src/cot/*` | New, as tabled above |
| `webapp/src/cot/*.spec.ts`, `*.pw.tsx` | New tests and harnesses |
| `webapp/src/decorators/location/map/LocationMap.tsx` | Optional `accuracyMeters` plus the circle layer and the label |
| `webapp/src/decorators/location/map/maplibre.ts` | The circle source and layers |
| `webapp/src/decorators/location/map/style.spec.ts` | The new layers in the built style |
| `webapp/src/decorators/location/map/LocationMap.pw.tsx` | The circle drawn, absent, and not drawn for existing callers |
| `webapp/src/index.tsx` | Register the post type component with its disposer |
| `docs/design/cot.md` | New: the whole rationale |
| `docs/design/decorators.md` | `SoleFencedBlock` and its relationship to protected spans |
| `docs/design/mapping.md` | The CoT surface in "The map under a post"; the CE circle |
| `docs/design/admin-settings.md` | The new section, the two switches, the widened one |
| `docs/design/unverified.md` | The two open questions below |
| `public/help/formats.html` | What is recognized, and both gates, in words |
| `public/help/panel.html` | The post-body surface, at its existing anchors |
| `public/help/admin.html` | The two switches and the widened one |
| `public/help/error-codes.html` | One row per new code |
| `CLAUDE.md` | The design-note table, the invariants, the sync table |

## Tasks

**Phase 1a - the mechanics**

1. [ ] `decorators.SoleFencedBlock` plus `TestSoleFencedBlockIsAlwaysProtected`.
2. [ ] `cot.Parse`: struct, bounded token-aware decoder, every refusal, fixtures
       from real ATAK traffic (PLI, GeoChat, SPI, emergency, CASEVAC) and
       hostile input (DOCTYPE behind a comment, deep nesting, oversized, invalid
       UTF-8, UTF-16 declaration, two roots, wrong root, `<event/>`).
3. [ ] Type and `how` decoding, including the unrecognized path.
4. [ ] Rendering: display strings, the `>= 9999999` and negative sentinels, the
       literal-digit `(f, v)` pair round-tripping through the location page at
       one and seven decimals, the region lookup, the caps and the control and
       bidi stripping.
5. [ ] Config: two settings, the `plugin.json` section, help text, accessors.
6. [ ] `hooks.go` restructure and `cotStamp`: hoisted nil check, the two
       demoted returns, gates, own recover, atomic commit on an owned clone,
       the props budget against `PostPropsMaxUserRunes`, error codes.
7. [ ] The file path: `GetFileInfo` gate then `GetFile`, with a mocked API
       failing at each step.
8. [ ] The ephemeral refusal notice for an explicit ```cot fence.
9. [ ] `webapp/src/cot/types.ts` plus `TestWebappCotShapeMatches` and
       `TestWebappCotPostTypeMatches`.
10. [ ] `CotPostBody`: all four branches, the never-blank fallback, the
        ErrorBoundary nesting.
11. [ ] `CotCard`: `lead`, header, rows, `<details>` XML, download link,
        `trail`, the accessibility markup.
12. [ ] Register the post type in `index.tsx` with its disposer.

**Phase 1b - the picture**

13. [ ] `LocationMap` `accuracyMeters`, the circle source and layers, the
        accessible label, and the regression test that existing callers are
        unchanged.
14. [ ] `CotMap`: the wrapper, `useNearViewport`, reserved height,
        `features.mapInline`, the `INLINE_ID` preference, the position link and
        "Open larger".

**Both**

15. [ ] Help pages and error-code rows.
16. [ ] `docs/design/cot.md`, the four other design notes, the `CLAUDE.md` tables.
17. [ ] `make check-style && make test`, then `make deploy` and walk the
        verification list.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| A parse bug stops somebody posting | Own recover with the API captured first; atomic commit on an owned clone; byte, depth and element budgets; a test that a panicking parse leaves both the post and the decoration intact |
| The props push the post over the server's limit | Measure the **whole** props map against `PostPropsMaxUserRunes` and back out; a test with a large foreign props blob |
| A filestore read on the post path is slow | `GetFileInfo` size gate before `GetFile`, 64 KiB cap, one object read; `EnableCotFile` removes it entirely |
| XXE or entity expansion | Token-aware `Directive` refusal, `Strict: true`, no entity map, no `CharsetReader`, byte cap, depth cap, element budget |
| XSS or spoofing from author text | React text nodes only; no `dangerouslySetInnerHTML`; every field capped; control and bidi characters stripped |
| A blank post body | The fallback renders file links when the message is empty; the download link is unconditional |
| The card disagrees with its source | `edit_at`, plus the `file_ids` check |
| A later plugin rewrites the message | Not detectable, and no longer relevant: the card renders `lead`/`src`/`trail` from props and never reads `post.message` |
| Stamping costs search and embeds | The same words `EnableLocationMapInline` uses; `EnableCot` is the way out; a UX row says so |
| The map overstates a fix | The CE circle, and no circle at all when CE is unknown |
| WebGL exhaustion in a busy channel | `useNearViewport`, the same budget the inline map respects |
| Type decoding overclaims | Two small tables plus a short whole-type list; anything else says "unrecognized" |
| Frozen props go stale against corrected tables | Accepted and recorded; raw values stored beside rendered ones; `version` is the lever |

## To verify on a running server

Only two remain. The attachment-list question is **already answered** by
`docs/design/mapping.md:1317` and is handled unconditionally rather than checked.

- Does `plugin.API.GetFile` succeed inside `MessageWillBePosted`, before the
  file is attached to the post? Expected yes: the upload already wrote the
  `FileInfo` and the filestore object, and only `PostId` is unset. If not, the
  file case moves to `MessageHasBeenPosted` plus `UpdatePost`, which is a
  visible edit and a different design.
- Does an ephemeral post sent from inside `MessageWillBePosted` arrive after the
  post it is about?

Two carried over from the existing note, unchanged by this plan: whether
`registerLinkTooltipComponent` fires for an anchor inside a plugin-owned body,
and whether `ShowMore` clamps a plugin-owned body at 600px.

## UX summary

| Scenario | Behavior |
|---|---|
| A ```cot fence alone | Card, map with the CE circle, collapsed XML |
| A ```cot fence with a covering note | The note as plain text, then the card. Markdown in the note is flattened |
| A coordinate or DTG beside a CoT event | Not decorated, and left verbatim. The card owns the body, so no decorator link could render there |
| An ```xml fence that is not CoT | Nothing changes; ordinary code block |
| A ```cot fence that is not CoT | Nothing changes, and an ephemeral tells the author why |
| One `.cot` file, with or without a note | Card, with the file as a download link |
| Two attachments | No card; ordinary post |
| Position `0,0` | The value shown, named as the CoT unset sentinel, no map |
| `ce` not stated | Pin, no circle, and the row says so |
| `stale` before `time` | The stale row reads as expired, in text as well as colour |
| Edit the post at all | Card disappears; message renders as plain text, permanently |
| Searching for a callsign in the card | **Not found** on Elasticsearch or OpenSearch installs; found on Postgres |
| Recent Mentions | A mention inside a CoT post does not appear, on ES/OS installs |
| Notification preview, push and desktop | The raw message text, fence and all |
| Pinned, saved, and search result panes | The card, since they render post bodies |
| Mobile app | The raw message; no bundle, so no card. Consistent with decorated links today |
| Message export | The raw message plus the props blob |
| No WebGL, or the basemap fails | `LocationMap`'s own notice; every row still on screen |
| `EnableCot` off | New posts are ordinary; existing cards keep rendering |
| `EnableLocationMapInline` off | Card without a map; nothing else changes |
| Reader hid the inline map row | Card without a map |
| Compact display | Text, header and rows, no map |
| Post formatting off | Mattermost never reaches the component; raw message |

## Testing plan

**Go unit.** Parse fixtures and every refusal; the budgets; type and `how`
decoding including unrecognized; the sentinels including `9999999`, `-1` and
`9999999.00`; the literal-digit `(f, v)` at both extremes; the caps and the
control and bidi stripping; `SoleFencedBlock` across CRLF, tildes, four-backtick
fences, unterminated fences, indented fences, two fences, and a fence with prose
on both sides.

**Go hook.** Each gate refused in isolation; the stamp; a non-empty `post.Type`
left alone; a panicking parse leaving post and decoration intact; **no
half-stamp**, asserted by injecting a panic between the two writes; the props
budget with a large foreign blob; the file path with a mocked API failing at
`GetFileInfo`, at `GetFile`, on size, and on extension; the fence-wins
precedence; and **exclusivity**, that a message carrying both a CoT fence and a
decoratable coordinate outside it is stamped and left UNdecorated, with the
coordinate surviving verbatim in `lead`.

**Cross-language.** `TestWebappCotShapeMatches` and
`TestWebappCotPostTypeMatches`, each with a row in `CLAUDE.md`'s sync table.
No digest test, because there is no digest.

**Webapp unit.** `fromProps` refusing every malformed shape and every unknown
version.

**Webapp component.** All four `CotPostBody` branches including the never-blank
fallback on an empty message; the card with a known position, an unknown one and
a `0,0` one; stale and fresh; compact; a throwing map contained by the
ErrorBoundary while the rows survive; the `<details>` reachable by keyboard;
`LocationMap` drawing the circle, omitting it, and being unchanged for every
existing caller.

## Acceptance criteria

- [ ] A ```cot and an ```xml fence carrying the same event render the same card.
- [ ] A covering note renders above the card, in reading order.
- [ ] A coordinate beside a CoT event survives verbatim and is not rewritten.
- [ ] The position link opens the location sidebar panel and is styled, with no
      change to `click_handler.ts` or `styles.ts`.
- [ ] A CE of 45 m and a CE of 9000 m draw visibly different circles, and an
      unknown CE draws none.
- [ ] The position link's `v` carries exactly the digits the XML did.
- [ ] Editing the post removes the card and leaves readable text.
- [ ] A file-case post with its props stripped still renders a download link.
- [ ] A DOCTYPE behind a comment, a 1 MiB file, a deeply nested document, a
      UTF-16 declaration and two roots each leave the post untouched.
- [ ] A maximum-size event beside a large foreign props blob still posts.
- [ ] `make check-style && make test` pass; coverage does not fall.

## Checklist

- [ ] **Design note before code**: `docs/design/cot.md` exists first, because
      none of this rationale may go in a comment.
- [ ] **Rationale lands in the right file**: `SoleFencedBlock` in
      `decorators.md`, the CoT surface and the circle in `mapping.md`, the
      switches in `admin-settings.md`.
- [ ] **No prose comments** in any new or modified file; directives kept.
- [ ] **No em dashes**, anywhere, including commit messages. A test enforces it.
- [ ] **Sync table rows** for every new cross-language duplicate (two, not three).
- [ ] **Invariants**: the atomic stamp and the props budget belong in
      `CLAUDE.md`'s invariant list.
- [ ] **Help anchors** in `public/help/` are a contract; add, do not rename.
- [ ] **Conventional commits**: `feat(cot): …`; do not hand-edit `plugin.json`'s
      version or `CHANGELOG.md`.

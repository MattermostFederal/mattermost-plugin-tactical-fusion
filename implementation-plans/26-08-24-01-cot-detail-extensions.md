# Cursor on Target: a known-extension registry for `<detail>`

## Overview

Replace the five hand-wired `<detail>` children the parser reads today with a
declared registry of known TAK extensions, fix the any-depth element matching
that registry exposes, and let a small closed set of semantic classes vary one
summary line on the card while the panel carries every extension the event held.

## Problem Statement

`readChild` (`server/cot/parse.go:224`) reads exactly five things out of
`<detail>`: `contact`, `__group`, `track`, `link` and `remarks`. Everything else
a real TAK emitter writes is dropped: device and battery state, precision
provenance, sensor field of view, video association, GeoChat, MEDEVAC requests,
platform attitude, processing history, geometry.

Three consequences:

1. **The card under-reports.** An event carrying a MEDEVAC request renders as a
   table of uid, type and time, which is less than the code block it replaced.
2. **A GeoChat event renders as a position report**, because the card has one
   layout and a chat message has nowhere to go.
3. **Matching is depth-blind.** `parse.go:229` is `case start.Name.Local ==
   "contact":` with no parent guard, and `parse.go:236` is the same for `link`.
   A `<contact>` nested inside another extension becomes the event's callsign,
   and a `<link>` inside `<__video>` becomes a relation on the card.

`<detail>` has no schema to validate against: it is an open container into which
ATAK, FreeTAKServer, TAK Server and third-party plugins each write their own
conventions. A registry of what we know, with everything else falling through to
the source pane, is the only shape that fits it.

## Phase Strategy

| Phase | Focus | Value |
|-------|-------|-------|
| **Phase 1** (this plan) | The registry, the parent fix, ~15 flat extensions, four classes | **80% of value** |
| **Phase 2** | Geometry: `shape`, `polyline`, `ellipse`, `route`, drawn on `CotMap` | Map fidelity |
| **Phase 3** | The long tail: `__chatReceipt`, `__serverdestination`, `_radio`/`__network`, `attachment_list`, `__geofence`, `checklist` | Completeness |

Phase 1 is the **Core** and **High** rows of the requested inventory, less
geometry. Every one of them is the same shape of problem: an element under
`<detail>` whose meaning is entirely in its attributes. One registry row, one
fixture, one panel section each, no new rendering primitives.

Phase 2 is a different problem: vertex lists, a second traversal shape, new
MapLibre layers, and a new answer to what the map frames when an event's extent
is a polygon. Phase 3 is deferred on value, not difficulty, and the registry is
what makes those rows data entry.

### Control events are already handled, and are not in Phase 1

This was reviewed and cut. The goal ("do not show `0,0` on a map just because
every CoT event contains `<point>`") **already ships**, on three existing lines:

- `cot.go:209` returns before writing `format`/`value` for `0,0`.
- `types.ts:242` `isLinkable` is `format !== '' && value !== ''`, so it is false.
- `CotCard.tsx:272` mounts `CotMap` only when `events.some(isLinkable)`.

`types.go:84` already carries `"t-x-takp-v": "TAK Client Version"`, so the card
already names the exchange. A `TakControl` registry entry, its three nested
children and a `control` class would deliver one extra version string in exchange
for four registry rows, three nested bindings, a class, a card branch and a
Playwright case. Cut. The one cheap piece is kept: two rows added to
`wholeTypes` for the TAK request and response types.

### Deferred explicitly

- `shape`, `polyline`, `ellipse`, `route` - Phase 2
- `__chatReceipt`, `__serverdestination`, `_radio` / `__network`,
  `attachment_list`, `__geofence`, `checklist`, `checklistColumn`,
  `TakControl` and its children - Phase 3
- Protobuf CoT and TAK transports - out of scope. This plugin reads XML a human
  pasted or attached; it is not a TAK transport.
- Resolving a `link` to another post - still refused, per `docs/design/cot.md`.

## Current State

| Piece | Where | Today |
|---|---|---|
| Parse | `server/cot/parse.go:224` | Five element names, matched at any depth |
| Model | `server/cot/parse.go:94` | `Detail{Callsign, Group, Role, Speed, Course, Remarks, Links}` |
| Props | `server/cot/cot.go:92` | A flat map of rendered strings per event |
| Card | `webapp/src/cot/CotCard.tsx` | One layout: naming, map, `EventDetail` rows |
| Panel | `webapp/src/cot/CotPanel.tsx` | Event rows, countdown, Remarks, Source file, As posted |
| Guard | `server/cot_sync_test.go:58` | Go's written keys against the webapp's `text(event, '...')` reads |

### Current Gaps

- Thirty-odd `<detail>` children parsed as nothing.
- `contact`, `link`, `__group`, `track` matched at any depth (above).
- Matching is namespace-blind: `attrValue` and `readChild` switch on
  `Name.Local`, so `<x:contact callsign="ADMIN"/>` is read as the callsign.
- Repeated elements have no stated winner. `attrValue` is first-wins
  (`parse.go:277`); a repeated `<contact>` is last-wins, because each arm
  assigns. Two opposite rules, neither written down.
- `budget.enter()` counts elements and depth only. **Attributes are free.**
- **`props["src"]` is capped at `maxInlineSrcRunes` = 8192 runes
  (`cot.go:82`) while `Parse` accepts `MaxSourceBytes` = 64 KiB.** The
  verification pane covers the first eighth of what the parser reads.

## Design Principles

| Concern | Our approach | Avoid | Reference |
|---|---|---|---|
| Unknown `<detail>` children | Fall through to the source pane, **and the pane is widened to cover everything parsed** | Storing unknown XML a second time in props | See Decisions |
| Extension shape | One Go registry of closed, declared names, read by the parser, the props builder and the sync test | Twenty hand-written `case` arms | `atomPaths` / `data/types.csv` |
| Props key space | Every event key is registry-derived and closed. No author string is ever a props key | An author-chosen key beside `format`, `value` or `affiliation` | Threat review |
| Author strings | **No author string is ever used as a URL, an `src`, a `style` value or a `className`** | A swatch coloured from an unvalidated attribute | `Dot` reads a fixed table, `CotCard.tsx:124` |
| Classification | Type code decides; element presence may only promote an otherwise unclassified event; layout degrades when the block is absent | A class chosen by one empty element | Threat review |
| Card weight | One summary line, single-event cards only, nothing moves | Twenty new rows in the channel | "The source lives in the panel, not on the card" |
| Over-capacity | Registry names are closed, so nothing drops. The one open list (`flow`) drops its **oldest** entries | A cap that silently drops MEDEVAC precedence | See Decisions |
| Times | `dtg.FormatZulu`, always | A second time rendering for flow tags | "one DTG rendering in the repository" |

## Reference Patterns

- `server/cot/types.go:44` `atomPaths` - a large table as data with a README
  recording provenance, degrading to the deepest known ancestor.
- `server/cot/cot.go:129` `addLinks` - a capped author-controlled list folded
  into props.
- `server/cot/cot.go:345` `putIfSet` - a key is absent when there is nothing to
  say, and the webapp treats absent and empty alike.
- `webapp/src/cot/types.ts:194` `AFFILIATION_COLORS` - colour only ever from a
  fixed table, never from the event.
- `webapp/src/components/ErrorBoundary` as used at `CotCard.tsx:273`.

## Requirements

- [ ] A declared registry of known `<detail>` extensions, read by the parser,
      the props builder and the sync test.
- [ ] Extensions read only as children of `<detail>`, in no namespace, with a
      stated winner for repeats and duplicate attributes.
- [ ] Every Phase 1 extension parsed, sanitised, rendered and shown in the panel.
- [ ] The event props key space stays closed and registry-derived, asserted by a
      test that feeds it hostile attribute names.
- [ ] Four classes vary one card summary line; nothing else moves.
- [ ] The source pane covers everything the parser read.
- [ ] A post that stamps today still stamps after this change.
- [ ] The cross-language guard covers every registry entry automatically.

## Out of Scope

- Any `EnableCotDetail` switch. `EnableCot` governs stamping.
- Rendering `color/@argb` as the marker or affiliation colour.
- Making any author-supplied URL or hash clickable.
- Resolving `attachment_list` (Phase 3, and it must reuse `cotFileOwnedBy`).

## Phase 1 Extension Inventory

**Block** is the props key prefix. Keys are `<prefix>_<attr key>`, all
registry-derived. A `Unit` of `""` renders the stated string, sanitised.

### Identity and provenance

| Element | Attributes | Block | Notes |
|---|---|---|---|
| `contact` | `callsign` (existing), `endpoint` | `contact` | `endpoint` is panel-only |
| `__group` | `name`, `role` (existing keys `group`, `role`) | - | Unchanged |
| `uid` | `Droid` | `uid_extra` | Not the event's own `uid` attribute |
| `takv` | `device`, `platform`, `os`, `version` | `takv` | |
| `precisionlocation` | `geopointsrc`, `altsrc`, `pdop`, `hdop`, `vdop` | `precision` | dop values flagged for provenance |
| `archive` | presence | `archive` | See Decisions on presence-only blocks |
| `usericon` | `iconsetpath` | `usericon` | Text only. Never an `img src` |
| `color` | `argb` | `color` | Decoded in Go to `#RRGGBB`, see Decisions |

### Telemetry

| Element | Attributes | Block | Unit |
|---|---|---|---|
| `track` | `course`, `speed` (existing), `slope` | - | `slope` in `°`, signed |
| `status` | `battery`, `readiness` | `status` | `battery` in `%` |
| `Attitude` | `roll`, `pitch`, `yaw` | `attitude` | all `°`, all signed |

### Semantic payloads

| Element | Attributes | Block | Class |
|---|---|---|---|
| `sensor` | `azimuth`, `elevation`, `range`, `fov`, `vfov`, `roll`, `model` | `sensor` | Sensor |
| `__video` | `uid`, `url`; child `ConnectionEntry`: `address`, `port`, `protocol`, `path` | `video`, `video_conn` | Video |
| `__chat` | `senderCallsign`, `chatroom`, `id`, `parent`, `groupOwner`; child `chatgrp`: `uid0`, `uid1`, `id` | `chat`, `chatgrp` | Chat |
| `_medevac_` | see below | `medevac` | MEDEVAC |
| `_flow-tags_` | arbitrary `system="timestamp"` | `flow` (an ordered array) | - |

**Nested children get their own prefix**, never their parent's. `__chat/@id` and
`chatgrp/@id` are different values (thread id versus group id) and flattening
them into one block silently overwrote one with the other.

`_medevac_` attributes: `urgent`, `priority`, `routine` (patient **counts** by
precedence), `litter`, `ambulatory` (counts), `casevac`, `freq`, `security`,
`hlz_marking`, `terrain_none`, `equipment_detail`, `equipment_none`,
`zone_prot_selection`, `nationality`, `nbc`, `medline_remarks`, `title`.

Attribute names on this element are inconsistently cased in the wild
(`Security` and `security` both occur). Both spellings are **listed as separate
registry rows mapping to one key**, which is data rather than a case-folding
flag, and first-wins resolves the collision with no new branch.

## Classification

Four classes, written into props **only when they change layout**, per the
`putIfSet` precedent: `chat`, `medevac`, `sensor`, `video`. Absent means the
default, which is today's layout unchanged.

### Two passes, so one empty element cannot re-shape an event

```
1. Match the TYPE CODE against an ordered, case-sensitive table.
     b-t-f*      -> chat
     b-r-f-h-c*  -> medevac
     b-l-p-c*    -> sensor
   A match here is final.

2. Only if the type matched nothing, and only then, promote on element presence:
     _medevac_ present -> medevac
     __chat    present -> chat
     sensor    present -> sensor
     __video   present -> video
   Ordered, first match wins.

3. Otherwise: no class.
```

Pass 1 being final is the whole point. Under a single ordered table, a hostile
contact carrying an empty `<__chat/>` classifies as chat and its remarks are
promoted into a message-shaped block. Ten bytes, chosen by the author. An `a-*`
atom is never re-classed by a `<detail>` child.

**Match kind is a field on the row** (exact or prefix), not something a reader
infers from a glob. **Matching is case-sensitive**, because `cot.md` fixes that
case is part of a CoT code and `decodeType` already matches that way; a
`classify` that folded case would disagree with the label beside it.

**The layout degrades to the default when its block is absent.** A `b-t-f` with
no `__chat` gets no sender and no room, so the chat heading would be empty
chrome. Classification is type-driven; layout selection needs the data.

**Classes apply to single-event cards only.** `CotCard.tsx:278` already renders
`EventDetail` for one event and `EventList` for many, and `cot.md:252` fixes
that "the card lists them; the panel carries them". A multi-event post keeps
today's list, unclassified, and the panel carries each event's own class.

## Technical Approach

### Server

**1. Parent-aware, namespace-aware descent.** An extension is read only when it
is a child of the `<detail>` element at depth 2, its `Name.Space` is empty, and
its attributes' `Name.Space` are empty. Nested children (`chatgrp`,
`ConnectionEntry`) only under the block that owns them. The existing five
elements move onto the same rule.

The parent stack lives **beside `counts`, updated in the same places
`budget.enter`/`leave` are**, including inside `readText`, which swallows tokens
and calls `counts.enter()` itself (`parse.go:306`). A stack maintained only in
`Parse`'s main loop desyncs the first time a `<remarks>` contains markup, and
the symptom would be a later sibling attributed to the wrong parent, which is
the exact bug this change exists to fix.

**2. First-wins, everywhere, stated once.** A repeated element and a duplicate
attribute both resolve to the first occurrence, matching `attrValue`'s existing
behaviour. `encoding/xml` performs no duplicate-attribute check, verified. This
also means repeats add no keys, which matters for the sync fixture below.

**3. Attributes join the budget.** `budget.enter()` adds `len(start.Attr)` to
the element count, so an element with a thousand attributes is no longer free.

**4. `server/cot/extensions.go`** holds the registry:

```go
type Attr struct {
    Name string
    Key  string
    Unit string
    Zero bool
}

type Extension struct {
    Element string
    Parent  string
    Prefix  string
    Attrs   []Attr
}
```

Four fields, not seven. `MaxAttrs` is gone: every entry declares a closed list,
so `len(Attrs)` is the cap and a `MaxAttrs` below it would drop declared
attributes in author-controlled document order, which for `_medevac_` means
losing casualty precedence. `FoldAttrCase` is gone (both spellings are rows).
`OpenAttrs` is gone (`_flow-tags_` is not a registry entry, see 6).

`Unit` replaces a seven-value `Kind` vocabulary. `""` renders the sanitised
stated string; `"m"`, `"°"`, `"%"`, `"m/s"` parse through the existing
`knownNumber`/`trimFloat` pair and append the unit, inheriting the `9999999.0`
sentinel handling for free.

**`courseText` may not be reused for `°`.** `cot.go:232` rejects anything below
zero, and `Attitude/@pitch`, `@roll`, `track/@slope` and `sensor/@elevation` are
all legitimately negative. A signed degrees renderer is the one new formatter in
this plan.

**`Zero` is for counts.** `putIfSet` drops `"0"`, and for MEDEVAC precedence a
stated zero and an unstated field are different facts: without this a card reads
"1 priority" for an event that stated "0 urgent, 1 priority, 0 routine".

**5. `server/cot/classify.go`** holds the two-pass table and `classify(event)`.

**6. `_flow-tags_` is handled outside the registry**, as one function, because
it is the only element whose attribute *names* are the data:

- Stored as an **ordered array** under `flow`, never a map: `json.Marshal` sorts
  map keys and the ordering **is** the processing path.
- `xmlns` declarations arrive as attributes and `Name.Local` on
  `xmlns:x="urn:evil"` is `"x"`, verified against the pinned SDK. Any attribute
  whose `Name.Space` or `Name.Local` is `xmlns` is skipped.
- `version` is excluded by name, or it becomes a system called "version" whose
  timestamp is "1".
- A name is **dropped rather than truncated**. A truncated key is our word, not
  the event's, and two long names would collapse into two rows a reader cannot
  tell apart.
- `maxCotFlowTags` drops from the **front**. Flow tags are appended, so document
  order is oldest first, and dropping the tail discards the most recent hops,
  which are the ones a reader is looking for.
- A timestamp that is not RFC 3339 is shown as the stated string with the hop
  kept, not omitted. Omitting it would show a shorter route than the event
  described.

**7. `eventProps` gains** the registry-derived keys, `flow`, `class` (only when
non-default), and `detail_unknown`: a count of `<detail>` children this build did
not recognise. That last one costs one integer and closes a trust gap: once the
panel enumerates blocks, an event with none reads as "carried nothing" rather
than "we did not recognise what it carried".

**8. The block map is assigned in one statement**, and no registry code path
takes the event props map as an argument. `format`, `value` and `affiliation`
are load-bearing security values in that map (`isLinkable` builds a URL from the
first two; `affiliation` keys the marker colour), and nothing author-derived may
land beside them.

**9. `color/@argb` is decoded in Go.** ATAK writes a signed 32-bit decimal
(`argb="-1"`), not a hex string. Go parses it, drops alpha, and writes a
validated `#RRGGBB` or nothing. The webapp re-validates against
`/^#[0-9a-f]{6}$/i` before it reaches a style property. React sets style values
through `setProperty` without sanitising, so an unvalidated value is the first
author-controlled CSS in this codebase and would make
`background: url(https://attacker/px)` a read receipt on every reader who opens
the panel.

**10. Graded degradation before refusal.** `cotStamp` currently discards the
whole clone when props exceed budget (`hooks_cot.go:79`). With extension keys
added, a post that stamps today could stop stamping, which is a regression a
reader meets as raw XML. The ladder becomes: measure the full map; if over,
rebuild without the extension keys and measure again; only then refuse. The
second rung is logged with its own `TF-NNNN`.

**11. `maxInlineSrcRunes` rises to `MaxSourceBytes`.** The pane is what a reader
opens to check the card, and today it covers 8192 of 65536 runes, so a block
parsed from byte 20000 has nothing to verify it against. The budget affords it:
`PostPropsMaxUserRunes` is 760,000 runes (`PostPropsMaxRunes` 800,000 less the
SDK's 40,000 reserve, confirmed in the pinned SDK), against ~65 K of `src` plus
`lead` and `trail` at `maxNoteRunes` = 65,536 **each**.

### Webapp

**12. Flat, registry-derived keys**, so `TestWebappCotShapeMatches` keeps working
with the regex it already has (`cot_sync_test.go:62`) and no mirrored catalog and
no second and third sync tests are needed. Labels live in the TSX beside the
rows they name, as they do today.

**13. `CotEvent` gains the new fields**, `class`, `flow` and `detailUnknown`.
`fromProps` reads them; a post stamped before this change gets none of them and
renders exactly today's layout.

**14. Panel structure.** Order is pinned: heading, map, Event rows, Goes stale,
Remarks, then the extension groups, then Source file, As posted. The countdown
is argued in `CotPanel.tsx:70` as living in the panel because the reader chose
one event, and burying it under eight new sections defeats that. Blocks are
grouped under a fixed small set of headings (Device, Position quality,
Telemetry, Payload, Processing) so the panel never exceeds about seven top-level
headings whatever the block count. Processing is a native `<details>`, collapsed.

**15. Real headings.** `CotPanel.tsx:17` styles sections as uppercase `<p>`
elements. At five sections that is a smell; at N events times eight blocks it
removes the only way a screen reader user skims. `<h2>` per event, `<h3>` per
group, `<h4>` per block, same visual styling. Both `<dl>` `aria-label`s
(`CotCard.tsx:168`, `CotPanel.tsx:120`) carry the callsign or uid, so a
three-event panel does not announce one string three times.

**16. `ErrorBoundary` around the card body and each `EventSection`.** Only
`CotMap` is wrapped today (`CotCard.tsx:273`) and `CotPanel` has none; this plan
adds many data-driven render paths and a throw would escape into Mattermost's
post list or RHS.

**17. The chat layout is a reading of an event, not a message.**
`__chat/@senderCallsign` is author-chosen text with no relationship to the
Mattermost identity that posted. Anything shaped like a quoted message from a
named sender borrows Mattermost's own attribution, so an author sets
`senderCallsign` to a colleague's name and the card is a message from that
person. The heading stays inside the existing `Naming`/header treatment and is
labelled as what the event states; no blockquote, no avatar, no username
styling. The message text is rendered **once** (the existing Remarks row is
suppressed for this class, or the promotion is dropped and Remarks is where it
stays; pick at implementation, and a test asserts it appears once).

### Provenance

`docs/design/cot.md` gains a table recording, per extension, where its shape came
from: ATAK-CIV, the MITRE `_flow-tags_` XSD, or the FreeTAKServer model.
Anything not tied to one of those is listed in `docs/design/unverified.md`. It
needs no second rendering mode: with `Unit == ""` meaning "stated string,
sanitised", an unverified entry already renders as stated.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Bump `PropsVersion` to 3? | **No.** | Every new key is additive and optional. An older bundle reads the blob, ignores keys it does not know, and renders today's card. Bumping would make it refuse and fall back to raw text, which is strictly worse. `version` is the lever for a change that would be **misread**; this is not one. |
| Store unknown `<detail>` XML in props? | **No, and widen the pane instead.** | A second copy doubles stored size. But the claim "it is already in `src`" was **false** past 8192 runes, which is exactly the extension-rich population this plan targets. Raising `maxInlineSrcRunes` to `MaxSourceBytes` makes the claim true rather than restating it. |
| Nested props: flatten into the parent block? | **No, own prefix.** | `__chat/@id` and `chatgrp/@id` are different values and flattening silently overwrote one with the other, with nothing on screen to say so. |
| `Attitude/@yaw` as "Heading"? | **No. "Yaw", under "Orientation".** | Yaw is orientation about the vertical axis; `track/@course` is the event's own word for direction of travel. An event carrying both would show two contradictory "Heading" rows. Confirmed correct in domain review. |
| `_flow-tags_` times as `20:10:00Z`? | **No. Full Zulu DTG.** | Truncating to a time of day is lossy across a day boundary, and `cot.md` fixes one DTG rendering in the repository. The section is still labelled "Processing path"; the element name is not shown. |
| Decode `_medevac_/@security` to the 9-line vocabulary? | **No, in Phase 1.** | The numeric vocabulary is plausible and uncited. Shipping a decode we cannot cite is the derived claim `TestNoCountryIsDerivedForAnEvent` exists to prevent. Show the stated value; revisit with a citation. |
| MEDEVAC counts through `putIfSet`? | **No. `Zero: true`.** | A stated `0` and an unstated field are different facts, and these are the highest-consequence numbers in the inventory. |
| `color/@argb` as the marker or dot colour? | **No.** | Those are the plugin's own channel for what a track is; an author-controlled colour could paint a hostile track friendly blue. Shown as a stated value, decoded in Go, validated twice, with the hex **as text** beside an `aria-hidden` swatch that carries a themed 1px border. Colour is never the only channel, and `#FFFFFFFF` on light theme is otherwise an invisible square. |
| Author URLs clickable? | **No, and the reason is restated correctly.** | The earlier draft cited the plugin page CSP from `decorators.md`. That governs `/decorate/*` and `/map`; the card and panel render inside the Mattermost webapp under Mattermost's CSP. What the refusal actually buys: no anchor is created, because a plugin-owned post body gets no markdown renderer. What it does **not** buy is that the URL is unreachable, since in the fence case the raw event is still in `post.Message`, which Mattermost autolinks on mobile, search, export and the plugin's own `Fallback`. Stated as a limit. `__video/@url` stays off the card entirely. |
| Presence-only blocks (`archive`) | **A stated row, or no row.** | An empty block serialising to an empty map contradicts `putIfSet`, and a row reading "Archive: true" is not a sentence a reader acts on. `archive` writes `archive="stated"` or nothing, and the panel renders one plain-language line. |
| Class gates parsing? | **No.** | A misclassification would silently lose data. Everything registered is parsed and stored; the class picks one summary line. |
| Class on multi-event cards? | **No.** | The card already lists rather than details a batch. Each event keeps its own class for the panel. |
| Mirrored `detail.ts` catalog plus two new sync tests? | **No.** | Flat registry-derived keys are covered by the existing `TestWebappCotShapeMatches` regex with zero new test code. The nested `props["detail"]` shape was the only reason a catalog and two scrapers were needed, and it also invited asserting row order and panel section, which would turn a readability reshuffle in TSX into a Go build failure. |
| A generic block, or thirty typed structs? | **Generic, four fields.** | Thirty structs is thirty parse arms, thirty props arms and thirty readers. `CLAUDE.md` argues against abstractions the code does not need; at fifteen entries and growing, this one it does. |
| Registry in Go or a CSV like `types.csv`? | **Go.** | `Unit` and `Zero` are behaviour, not data, and a CSV would need a parallel vocabulary and a loader that can fail at build time. |

## Files to Modify

| File | Change |
|---|---|
| `server/cot/parse.go` | Parent and namespace aware descent with the stack alive inside `readText`; `Block` collection; first-wins; attributes in the budget |
| `server/cot/extensions.go` | **New.** The registry, `Attr`/`Extension`, the flow-tag reader |
| `server/cot/classify.go` | **New.** Two-pass, case-sensitive, match-kind per row |
| `server/cot/cot.go` | Registry-derived keys, signed degrees renderer, `argb` decode, `flow`, `class`, `detail_unknown`, `maxInlineSrcRunes` raised |
| `server/cot/types.go` | Two `wholeTypes` rows for the TAK request and response types |
| `server/hooks_cot.go` | Graded degradation before refusal |
| `server/errcode/codes.go` | One code for the degraded stamp, plus its `AllCodes` row |
| `server/cot/parse_test.go` | Per-extension fixtures; parent, namespace, repeat and duplicate-attribute regressions |
| `server/cot/cot_test.go` | Per-extension props; `Zero` counts; the closed-key-space test; the 32-event budget test |
| `server/cot/classify_test.go` | **New.** Two-pass precedence, spoofing, case sensitivity, degrade-to-default |
| `server/cot_sync_test.go` | Registry-derived fixture; `flow` and `class` as known non-`text` keys |
| `server/command_cot_example.go` | One added example: a GeoChat event |
| `webapp/src/cot/types.ts` | New fields, `class`, `flow`, `detailUnknown`, `argb` re-validation |
| `webapp/src/cot/CotCard.tsx` | Class summary line, chat heading, `ErrorBoundary`, `aria-label` with callsign |
| `webapp/src/cot/CotPanel.tsx` | Grouped sections in pinned order, real headings, collapsed Processing, `ErrorBoundary` |
| `webapp/src/cot/types.spec.ts`, `CotPanel.pw.tsx`, `CotPostBody.pw.tsx`, harnesses | Coverage below |
| `docs/design/cot.md` | Every Decision, the provenance table, the two-pass rule, the frozen-class note |
| `docs/design/unverified.md` | Extensions whose shape is convention rather than a citable schema |
| `public/help/formats.html`, `panel.html`, `error-codes.html` | What the card and panel show; the new code |
| `CLAUDE.md` | One sync-table row for the registry |

## Tasks

1. [ ] Write the `docs/design/cot.md` section first, carrying every Decision and
       the provenance table. Rationale goes in before the code.
2. [ ] `server/cot/parse.go`: parent and namespace awareness, the stack through
       `readText`, first-wins, attributes in the budget. Regression tests for all
       four before anything is added.
3. [ ] `server/cot/extensions.go`: the registry and the flow-tag reader.
4. [ ] `server/cot/classify.go` and its tests.
5. [ ] `server/cot/cot.go`: keys, signed degrees, `argb`, `flow`, `class`,
       `detail_unknown`, the `src` cap. Every value through `sanitize`.
6. [ ] The closed-key-space test and the 32-event budget test.
7. [ ] `server/hooks_cot.go` graded degradation and its `TF-NNNN`.
8. [ ] `webapp/src/cot/types.ts` readers, then the sync test.
9. [ ] `CotPanel.tsx` groups and headings, then `CotCard.tsx` summaries.
10. [ ] Playwright per class and per group.
11. [ ] The GeoChat example row and the test holding it to what it claims.
12. [ ] `public/help/` pages.
13. [ ] `make check-style && make test`.

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| A post that stamps today stops stamping, and the reader gets raw XML | Graded degradation (drop extension keys, keep the card) before refusal; a budget test at **32 maximal events with `lead` and `trail` at `maxNoteRunes` each** and `src` at its new cap, which is the shape that actually decides it |
| An author re-shapes a card with one empty element | Two-pass classification, type-final; `TestAClassNeverOverridesAnAtomType` |
| An author-chosen string lands beside `format`/`value`/`affiliation` | Flow tags are a list, not keys; a test asserts the event's top-level key set against a hard-coded allowlist using a fixture whose flow-tag names are literally `format`, `value`, `affiliation`, `lat`, `uid` |
| Author CSS reaches a style property | `argb` decoded and validated in Go, re-validated in the webapp, and the general rule written into `cot.md` |
| A chat card is read as a message from a named person | Non-message chrome, labelled as what the event states, and a test asserting no avatar or username element |
| The parent stack desyncs through `readText` | The stack moves with `budget.enter`/`leave`; a fixture with markup inside `<remarks>` followed by a sibling extension |
| The sync fixture silently narrows as entries are added | Registry names are closed and repeats are first-wins, so no cap can drop a key from the fixture. There is no `maxCotBlocks` to pin |
| Two identical events posted either side of this change render differently | Recorded in `cot.md` beside the frozen-props argument, which is where a maintainer will look |
| A wrong extension shape ships as a confident decode | Provenance table; anything uncited renders as stated and is listed in `unverified.md` |

## UX Summary

| Scenario | Behaviour |
|---|---|
| `a-f-G-U-C-I` position report with `takv`, `status`, `precisionlocation` | Card unchanged. Panel gains Device, Position quality and Telemetry groups |
| `b-t-f` GeoChat | Card names the stated sender and room as a reading, message text once, position rows unmoved |
| `b-t-f` with no `__chat` element | Default layout. No empty chat chrome |
| `b-r-f-h-c` MEDEVAC | Summary line of precedence and patient counts, stated zeros included; full detail in the panel |
| `b-l-p-c` sensor with `__video` | FOV / azimuth / range summary; the panel says a stream is associated and shows the address as text |
| Hostile contact carrying an empty `<__chat/>` | Default layout. The type code decided |
| `t-x-takp-v` protocol event | Unchanged from today: named as a TAK client version, no map, `0,0` with the null-island note |
| An event carrying extensions this build does not know | Panel states how many were not recognised and points at "As posted", which now covers the whole source |
| A post stamped before this change | Renders exactly as today |

## Testing Plan

**Go unit**
- One parse fixture per registry entry.
- Parent: `<contact>` outside `<detail>` is not the callsign; `<link>` inside
  `<__video>` is not a relation.
- Namespace: `<x:contact/>` and `x:callsign` are not read.
- Repeats and duplicate attributes: first-wins, both.
- `readText` desync: markup inside `<remarks>`, then a sibling extension.
- Budget: attributes counted; 32 maximal events with maximal `lead`/`trail`/`src`
  either fit or degrade to a card, never to raw XML.
- Closed key space, fed hostile flow-tag names.
- `Zero` counts; signed degrees for negative pitch, roll, slope, elevation.
- `argb`: signed decimal in, `#RRGGBB` out, nothing out for garbage.
- Flow tags: ordering preserved, `xmlns` skipped, `version` excluded, oldest
  dropped, long names dropped not truncated, unparseable time kept.
- Classification: two-pass precedence, the spoof fixtures, case sensitivity,
  degrade-to-default.

**Cross-language**
- `TestWebappCotShapeMatches` on a registry-derived fixture.

**Webapp**
- `types.spec.ts`: v1, v2 and the new shape read; an unknown class falls to the
  default; `argb` re-validation rejects a URL.
- Playwright: one card per class; the chat card renders the message once and
  produces no avatar or username element; a panel with every group; a panel with
  none; a multi-event panel.

**Manual, recorded in `unverified.md` if not done**
- A real ATAK-generated event through a running server.

## Acceptance Criteria

- [ ] Every Phase 1 extension is parsed, sanitised, stored and shown in the panel.
- [ ] The event props top-level key set is closed and asserted.
- [ ] No author string reaches a URL, an `src`, a `style` value or a `className`.
- [ ] One empty `<detail>` child cannot change an atom event's layout.
- [ ] A post that stamps today still produces a card after this change.
- [ ] The source pane covers everything the parser read.
- [ ] No post stamped before this change renders differently.
- [ ] Adding a registry entry without its webapp read fails `make test`.
- [ ] `make check-style && make test` pass.

## Checklist

- [ ] **Diagnostics**: one new `TF-NNNN` for the degraded stamp. Four edits
      together: the constant, the `AllCodes` entry, the call site, and the row in
      `public/help/error-codes.html`.
- [ ] **Slash command**: `example-details` gains one row; no new subcommand.
- [ ] **Design note before code**, per `CLAUDE.md`.
- [ ] **No prose comments** in new or modified code.
- [ ] **No em dashes** anywhere.

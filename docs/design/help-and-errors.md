# Built-in documentation and error codes

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Built-in documentation

`public/help/` holds eleven static HTML pages and one stylesheet. Mattermost
serves the bundle's `public/` directory at `/plugins/<id>/public/**`, so
**there is no route for this in the server code** and nothing to add to
`ServeHTTP`. The build already copies it: `build/setup.mk` sets `HAS_PUBLIC`
from the directory's existence and the `bundle` target acts on it.

The bundle is organized **by feature, not by surface**. Each decorator owns one
page covering it end to end, its grammars, its examples, its panel and its
switches, because a reader arrives asking about coordinates rather than about
sidebars. `formats.html` and `panel.html` were that split the other way round and
are now the index and the shared-mechanics pages respectively.

| Page | Covers | Kept in sync with |
|---|---|---|
| `help.html` | Landing page, what a decorator is, the consequences of server-side decoration, nav cards | The overall surface |
| `dtg.html` | Every date and time grammar, the zone letters, the declined list, the panel | `server/decorators/dtg/` |
| `location.html` | The twelve coordinate grammars, the rows, the map, the declined list, the panel | `server/decorators/location/` |
| `airfields.html` | The label-only grammar, the database, the table expansion, the panel | `server/decorators/airport/` |
| `cot.html` | The whole schema: `event`, `point`, `detail`, the type tables, the limits and refusals, worked examples, the card, the panel, the map | `server/cot/`, `server/hooks_cot.go`, `webapp/src/cot/` |
| `formats.html` | The index, and the rules every decorator shares: boundaries, consumed labels, protected spans | `server/decorators/tagger.go`, `boundary.go` |
| `panel.html` | What a hover, a click and a standalone page are; preferences, restore defaults, zone ordering | `webapp/src/decorators/` |
| `admin.html` | One section per switch, and what a switch does not do | `plugin.json` `settings_schema.settings` |
| `commands.html` | `examples`, `check`, bare and unknown subcommands | `server/command*.go` |
| `troubleshooting.html` | Symptom, cause, fix, quoting the exact user-facing strings | Every message in the server |
| `error-codes.html` | The `TF-NNNN` registry, grouped by source file | `server/errcode/codes.go` |

`admin.html` and `troubleshooting.html` stay **whole registries** rather than
being split across the decorator pages. An admin auditing switches and a
responder matching a symptom both scan the complete list, and splitting them
would disperse the two inventories `TestEverySettingIsDocumented` and
`TestEveryCodeIsDocumented` guard. The decorator pages link into them instead.

### The examples come from the slash commands

The decorator pages carry **everything** `/tactical-fusion examples` posts, and
more. `exampleSets` in `server/command_examples.go` and the two events in
`server/command_cot_example.go` are the curated subset a channel meets the plugin
through; the pages are where the boundaries, the declined shapes and the
protected spans live. `TestEveryCommandExampleIsDocumented` and
`TestEveryCotExampleIsDocumented` hold the pages to the commands in one
direction only: a page may show more than a command does, and may not show
less. So the corpus is verified against the real matcher, and a page inherits that
guarantee instead of restating grammar by hand.

Before this, roughly half of it appeared nowhere in the documentation, and the
overlap that did exist often taught the same grammar with *different literals*.
`TestEveryCommandExampleIsDocumented` and `TestEveryCotExampleIsDocumented` close
that: **a page may show more than the command does, never less.** Adding a row to
the catalog now fails the build until a page carries it.

`undocumentedExamples` is the exemption list, for rows that exercise packing or
budget logic rather than teaching a grammar. It is empty today. If it grows past
a handful, the docs are drifting rather than the test being wrong.

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

### Examples are selectable, and copyable where the browser allows it

Every example a reader is meant to paste carries `class="copyable"`, which is
`user-select: all` and a small drawn icon. One click selects the whole example;
the reader presses Ctrl or Cmd C. **That is the floor, and it needs no script.**

On top of it, `copy.js` makes both shapes of copyable actually copy: a real
**Copy** button on each `pre.copyable` block, and a click handler on each
`code.copyable` run. It is an enhancement and never the mechanism: the script
returns before it touches the DOM when `navigator.clipboard` or its `writeText`
is missing, so what is left is exactly the page that shipped before it.

**Both shapes or neither.** The same drawn icon sits on an inline run and on a
block, so wiring one and leaving the other as select-only made one icon mean two
things, and it looked fine on either page taken alone: the blocks are all on
`cot.html` and the 110 inline runs are all on four other pages, so no single page
showed the inconsistency. `TestTheCopyScriptWiresBothCopyables` is what holds
them together.

The selectors are `pre.copyable` and `code.copyable`, and the second one has to
stay that narrow. Every block is `<pre class="copyable"><code>` with the class on
the `pre` alone, so a selector matching any descendant `code` would bind each
block's own child a second time.

Neither handler calls `preventDefault`, which is load-bearing rather than an
omission: the click that copies is also the click that selects, and suppressing
the default would take the fallback away on exactly the pages that have the
script.

That distinction is the whole design, because the fallback is not the rare case.
`navigator.clipboard` is `undefined` on a non-secure origin, and `CopyButton.tsx`
already recorded the measurement that settles it: plain HTTP on-prem is the
deployment norm for this audience rather than an edge case, which is why the
sidebar's own copy buttons hide themselves rather than fail silently. A button
that was the only way to take an example would be absent on exactly the
air-gapped installs these pages exist to serve.

`TestTheCopyScriptOnlyEnhances` is what holds that: it fails if the script stops
guarding on `clipboard.writeText`, and it fails if the stylesheet loses
`user-select: all` or the drawn icon, since either would strand those readers
with nothing.

An inline run cannot show a label the way the button does, so a copied one swaps
its two drawn squares for a drawn check for 1500ms: `just-copied` sets
`content: none` on `::before` and turns `::after` into a rotated border corner.
The confirmation is announced as well, through one shared `aria-live` region
appended to the body rather than an attribute per element, since 110 live
regions on `location.html` would be 110 things for a screen reader to track. The
region is cleared and re-set on a timer because assigning the same text twice
announces nothing.

**A block button does not also speak.** Its own label changes to `Copied`, and
changing the accessible name of the focused element is already announced, so
routing it through the live region as well said it twice. The region is for the
inline runs, which have no name to change. `copy(element, spoken)` is that
split, and it is the only reason the argument takes a second parameter.

Both feedback states are drawn, and a failed copy is now one of them: an inline
run swaps its squares for a drawn cross, the block button says so in words, and
the shared live region announces either. A failure used to be silent on the
inline path, which is indistinguishable from the click having done nothing.
Every timer is stored on its element and cleared before a new one is scheduled,
or a second click inside 1500ms had its confirmation cut short by the first
click's timer. The live region is built at init rather than on first use,
because a region inserted and mutated 50ms later is not reliably registered by a
screen reader, and the first copy is the one that matters.

**Inline runs are deliberately not focusable.** Making them keyboard-reachable
means `tabindex="0"` on all 110, which is 63 extra tab stops on `location.html`
alone. The argument that holds is not that `user-select: all` is a keyboard
path, because it is not: it is a selection *granularity* rule, and with caret
browsing off a keyboard-only reader cannot place a selection there at all. It is
that nothing exposes the affordance to assistive technology in the first place.
`code.copyable` carries no role, no `tabindex` and no ARIA, so the click is a
redundant pointer shortcut over text that is already in the DOM, and it makes no
promise it cannot keep. The text itself is reachable by every reader; the
shortcut is not, and does not claim to be.

**Every `<pre>` carries `tabindex="0"`.** They are `overflow-x: auto` and some
examples are far wider than the column, so without it the part off screen could
not be reached without a pointer. `TestTheCopyScriptWiresBothCopyables` sweeps
every page for a `pre` that lost it. The 28 blocks that get a button need it
least and still carry it, since the button removes a reason to scroll rather
than the ability to.

**The 28 buttons are named apart.** All of them read `Copy`, so a screen reader's
button list was 28 identical entries; each now carries an `aria-label` of
`Copy: <nearest preceding heading>`. `TestHelpPagesAreSelfContained` allows exactly one script element,
compared verbatim against `allowedScript`, and `TestEveryHelpPageLoadsTheCopyScript`
requires it on every page: the script no-ops where there are no examples, so a
page that forgets it costs nothing until somebody adds one.

**Loaded by `src`, never inline.** A strict CSP on the host serving the bundle
would need a nonce or a hash to run an inline body, and a static file has
nowhere to put one. `defer` so the examples exist when it runs.

The button replaces the drawn icon rather than sitting beside it: the script
adds `has-copy-button` to the block, and the stylesheet sets `content: none` on
both pseudo-elements there. Two copy affordances on one block would be asking
the reader which one is real.

**The button sits in normal flow, in a `.copy-bar` above the block.** It was
absolutely positioned, with space reserved for it in the block's own
`padding-top`, and those two scaled from different font sizes: `pre` is a fixed
13px while the button inherited from the body, so under text-only zoom or a user
minimum font size the button grew, the reservation did not, and the control came
down over the first line of the example. Reserving in `em` fixed content zoom
and not that. In flow there is nothing to reserve and nothing to get out of step,
and it also puts the control before the block it acts on in DOM order rather than
after it, which is the order a keyboard reader meets them in.

`.example` carries the code background and the rounding once the wrapper exists,
and `.example > pre` gives up its own, so the bar and the block read as one
surface. Without the script there is no wrapper and the `pre` keeps all three.

**The button goes in a wrapper, not in the `<pre>`, and the reason is
`overflow-x: auto`.** An absolutely positioned child of a scrolling box is
positioned against its padding box and travels with the content: on the one-line
event, which is about 2,100px wider than its box, scrolling to the end of the
line carried the button the same distance out of sight. The drawn icon had
always done this and it did not matter, because a decoration that drifts is not
a control a reader is trying to press. So `copy.js` wraps each block in
`div.example`, which is the `position: relative` the button anchors to and which
does not scroll.

That wrapper buys a second property worth keeping. The button is no longer
inside the element the text is read from, so no reading of the block can pick up
the word "Copy" and append it to an event the reader is about to paste into a
channel. The text still comes from the `<code>` child rather than the `<pre>`,
which costs nothing and holds if the wrapper ever goes away.

**The icon sits top right on a block and centered on an inline run.** One rule
cannot serve both: `top: 50%` is right for a one-line `<code>` and puts the icon
halfway down a twenty-line `<event>`, beside nothing, which is where a reader
reported not being able to find it. `pre.copyable` therefore overrides the
offsets. The two squares are staggered 4px apart and that stagger *is* the icon,
so both offsets have to move together or it stops reading as two sheets.

The icon is drawn from borders on `::before` and `::after` rather than set as a
font glyph in `content`. Two reasons: the bundle ships no web fonts and rides
the system stack, so a minimal host can render a glyph as tofu; and a drawn
shape with empty `content` is not announced by screen readers, where a glyph
would be.

`-webkit-user-select` is the only vendor prefix in the stylesheet. Safari still
requires it.

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


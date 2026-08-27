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

### Examples are selectable, not copyable

Every example a reader is meant to paste carries `class="copyable"`, which is
`user-select: all` and a small drawn icon. One click selects the whole example;
the reader presses Ctrl or Cmd C.

**It is not a clipboard button, and that is deliberate.** A real copy needs
`navigator.clipboard`, which is JavaScript this bundle may not have, and which
is `undefined` on a non-secure origin. `CopyButton.tsx` already recorded the
measurement that settles it: plain HTTP on-prem is the deployment norm for this
audience rather than an edge case, which is why the sidebar's own copy buttons
hide themselves rather than fail silently. A clipboard button here would be
absent on exactly the air-gapped installs these pages exist to serve, so the
affordance that works everywhere is the one that ships.

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


---
name: add-help-docs
description: Use when plugin code changes need to be reflected in the built-in user-facing documentation under public/help. Trigger after adding or modifying admin settings, slash commands, token grammars, error codes, or any user-visible message, or before cutting a release.
user-invocable: true
---

# Add Help Docs

Keep the built-in documentation in sync with the code.

The plugin ships its own help as static HTML in `public/help/`. Mattermost
serves the bundle's `public/` directory at `/plugins/<id>/public/**`, so there
is no route, no renderer, and no generator script in this repo. Every edit is
by hand.

## When to Use

- After adding, renaming, or removing an admin setting in `plugin.json`
- After changing a token grammar, or what the tagger protects
- After adding or changing a slash command
- After adding an error code, or changing any user-visible message text
- After changing hover, panel or card behavior under `webapp/src/decorators/`, `webapp/src/cot/` or `webapp/src/geojson/`
- After changing the plugin bridge (`server/bridge.go`, `bridgeclient/`, `webapp/src/bridge/`)
- Before cutting a release

## When NOT to Use

- Pure refactors with no user-visible change
- Test-only or build-only changes
- Changes already documented in a previous commit on the same branch

## Workflow

Run these steps in order. **Always produce a plan and get explicit user
confirmation before editing any documentation file.**

Before starting, create tasks using TaskCreate for each applicable step. Mark
each complete with TaskUpdate as you finish it.

### 1. Survey what changed

```bash
git diff main...HEAD -- server/ plugin.json webapp/src/ public/
git log main..HEAD --oneline
```

Focus on: new or renamed settings; new or changed token grammars; new slash
commands or arguments; new error codes; and any changed string a user can see.

### 2. Produce a plan

Write a short plan and present it. It must list which pages will be edited and
the specific user-visible change driving each edit, plus anything intentionally
left alone and why. Wait for explicit confirmation before continuing.

### 3. Update the pages

Edit only the pages whose scope actually changed. Match the existing tone and
heading structure.

#### Layout

The bundle is organized **by feature**: one page per decorator, covering it end
to end. `formats.html` and `panel.html` hold only what is shared.

| File | Covers | Kept in sync with |
|---|---|---|
| `help.html` | Landing page, what a decorator is, the consequences of server-side decoration, nav cards | The overall surface |
| `dtg.html` | Date and time grammars, zone letters, the declined list, the panel | `server/decorators/dtg/` |
| `location.html` | The twelve coordinate grammars, the rows, the map, the declined list, the panel | `server/decorators/location/` |
| `airfields.html` | The label-only grammar, the database, the table expansion, the panel | `server/decorators/airport/` |
| `cot.html` | The schema, the type tables, limits and refusals, worked examples, card, panel, map | `server/cot/`, `server/hooks_cot.go`, `webapp/src/cot/` |
| `geojson.html` | What is read and what is refused, the narrow recognition rule, styles, the card, the panel, the map | `server/geojson/`, `server/hooks_geojson.go`, `webapp/src/geojson/` |
| `formats.html` | The index, and the shared rules: boundaries, consumed labels, protected spans | `server/decorators/{tagger,boundary}.go` |
| `panel.html` | What a hover, a click and a standalone page are; preferences and row ordering | `webapp/src/decorators/` |
| `admin.html` | One section per switch, and what a switch does not do | `plugin.json` `settings_schema.settings` |
| `commands.html` | `/tactical-fusion examples` and `check`, bare and unknown subcommands | `server/command.go`, `server/command_examples.go`, `server/command_check.go` |
| `integration.html` | The plugin bridge for other plugins: the Go client, the routes, `window.TacticalFusion` | `server/bridge.go`, `bridgeclient/`, `webapp/src/bridge/` |
| `troubleshooting.html` | Symptom, cause and fix, quoting the exact user-facing strings | Every message the server can produce |
| `error-codes.html` | The `TF-NNNN` registry, grouped by source file | `server/errcode/codes.go` |
| `styles.css` | Shared stylesheet. Rarely changes | Adapted from `mattermost-plugin-chatsurfer` |
| `copy.js` | The one script. Makes copyable examples copy on click; every page loads it and works without it | |

The list of pages the tests walk is `helpPages` in `server/help_docs_test.go`. If this table and that list disagree, the list is right and this table needs fixing.

#### Change-to-file matrix

| What changed | Pages to update |
|---|---|
| New or renamed setting in `plugin.json` | `admin.html`: a section with `id` **and** `data-setting="<Key>"`, plus a row in the summary table |
| New or changed token grammar | That decorator's page (`dtg`, `location`, `airfields`), in the recognized or the declined table. A declined entry must say **why** |
| New row in `exampleSets` or a new entry in `cotExampleOrder` | The matching decorator page, or `cot.html`. `TestEveryCommandExampleIsDocumented` and `TestEveryCotExampleIsDocumented` **fail until you do** |
| New `<detail>` extension or CoT type | `cot.html`, in the extension table or the type tables |
| GeoJSON recognition, limits, styles, card or panel | `geojson.html` |
| Bridge route, wire type, decline reason or `window.TacticalFusion` member | `integration.html` |
| New protected span or boundary rule in the tagger | `formats.html#protected` or `formats.html#shared-rules` |
| New or renamed slash subcommand | `commands.html`, plus the "Other input" table if the unknown-subcommand text changed |
| New panel behavior for one decorator | That decorator's page |
| New panel behavior shared by all of them | `panel.html` |
| New error code | `error-codes.html` in that file's section, **and** `troubleshooting.html` if a reader can see it |
| Changed user-facing message text | `troubleshooting.html`, which quotes them verbatim |

**Examples come from the slash commands.** The decorator pages teach from the
same corpus `/tactical-fusion examples` posts: `exampleSets` in
`server/command_examples.go`, the CoT documents in
`server/command_cot_example.go`, and the GeoJSON document in
`server/command_geojson_example.go`. Copy the literal from there rather than
inventing one, so a reader meets the same token in both places. A page may show
more than the command does, never less.

There is intentionally no `api.html`. The JSON API is an implementation detail
of the sidebar, and its failures are documented as messages in
`troubleshooting.html` rather than as endpoints.

#### Rules

1. **No remote assets, and one script only.** No CDN, no web fonts, no external
   images, no inline event handlers. These must render on an air-gapped host.
   The only script a page may load is its own `copy.js`, which every page loads
   and which enhances and never enables: every page must still work with
   scripting off. Do not add a second script or make content depend on the first.
2. **No em dashes**, per `CLAUDE.md`. `check-style` does not lint HTML, so a
   test covers it.
3. **Preserve anchor ids.** Pages deep-link into each other and a rename fails
   silently: the browser lands at the top and the reader never learns they
   missed the section. If you must rename one, update every `href` to it.
4. **Every page's sidebar lists every page**, with `class="active"` on its own
   entry and nowhere else.
5. **Light only.** No `prefers-color-scheme` block. The decorator page is the
   themed one; these are not.
6. **Quote messages verbatim.** A troubleshooting row whose text does not match
   the Go string is worse than no row, because search will not find it.

#### Adding a page

1. Copy the shell from an existing page: `<head>`, the `<aside class="sidebar">`
   nav, and the breadcrumb. Move `class="active"` onto the new entry.
2. Add the new page to the nav block of **all** the other pages. The block is
   byte-identical everywhere except for which entry carries `class="active"`.
3. Add its filename to `helpPages` in `server/help_docs_test.go`.
4. Add a nav card in `help.html`.

#### Adding an error code

Four edits, and the tests enforce three of them:

1. A constant in `server/errcode/codes.go`, taking the next free number in that
   file's range. Never renumber, never reuse.
2. An entry in `AllCodes`, in the same order.
3. The call site. `errcode.WithCode` for a string, `errcode.Errorf` for an
   error; a log call takes `"error_code", errcode.X` as its first pair.
4. A row in `public/help/error-codes.html`, in that file's section.

### 4. Verify

```sh
make check-style
make test
make dist
tar tzf dist/com.mattermost.plugin-tactical-fusion-*.tar.gz | grep public/help
```

`server/help_docs_test.go` covers anchors, navigation, the air-gap rules, the
copy script, em dashes, the command examples, and both directions of the code
and setting inventories. It does not
cover whether the prose is any good: open `public/help/help.html` over `file://`
and read it.

### 5. Report

Summarize which pages changed and why, and anything intentionally left alone.

## Common Mistakes

- Skipping the plan step. Always write the plan and get confirmation first.
- Renaming an anchor id another page links to.
- Adding a section to `admin.html` without the `data-setting` attribute, so
  nothing pairs it with the manifest key.
- Adding a code to `AllCodes` but not to `error-codes.html`.
- Documenting a failure message in `admin.html` instead of
  `troubleshooting.html`. Admin is the happy path; troubleshooting is the home
  for every user-facing failure string.
- Adding a page but forgetting the other navigation blocks, or `helpPages`.
- Writing an example by hand when the slash command already has one. Use its
  literal; a test enforces that every catalog row appears somewhere.
- Using em dashes. The repo convention forbids them in docs and code.
- Hardcoding the plugin id. It belongs in `plugin.json` only; the webapp uses
  `docsUrl()` in `webapp/src/plugin_url.ts`. If a Go helper is ever needed it
  must be a **function**, not a package-level `var`, because var initializers
  run before the generated `init()` populates the manifest and would panic at
  activation.

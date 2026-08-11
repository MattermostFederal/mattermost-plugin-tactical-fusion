---
name: add-help-docs
description: Use when plugin code changes need to be reflected in the built-in user-facing documentation under public/help. Trigger after adding or modifying admin settings, slash commands, token grammars, error codes, or any user-visible message, or before cutting a release.
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
- After changing panel or editor behaviour in `webapp/src/decorators/dtg/`
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

| File | Covers | Kept in sync with |
|---|---|---|
| `help.html` | Landing page, what a decorator is, the consequences of server-side decoration, nav cards | The overall surface |
| `formats.html` | Recognised grammars, the declined list and its reasons, protected spans | `server/decorators/dtg/{dtg,parse}.go`, `server/decorators/tagger.go` |
| `panel.html` | The sidebar, the hover card, the standalone page, Customize your view, the zone picker, row ordering | `webapp/src/decorators/dtg/` |
| `admin.html` | One section per switch, and what a switch does not do | `plugin.json` `settings_schema.settings` |
| `commands.html` | `/tactical-fusion examples`, bare and unknown subcommands | `server/command.go`, `server/command_examples.go` |
| `troubleshooting.html` | Symptom, cause and fix, quoting the exact user-facing strings | Every message the server can produce |
| `error-codes.html` | The `TF-NNNN` registry, grouped by source file | `server/errcode/codes.go` |
| `styles.css` | Shared stylesheet. Rarely changes | Adapted from `mattermost-plugin-chatsurfer` |

#### Change-to-file matrix

| What changed | Pages to update |
|---|---|
| New or renamed setting in `plugin.json` | `admin.html`: a section with `id` **and** `data-setting="<Key>"`, plus a row in the summary table |
| New or changed token grammar | `formats.html`, in the recognised or the declined table. A declined entry must say **why** |
| New protected span in the tagger | `formats.html#protected` |
| New or renamed slash subcommand | `commands.html`, plus the "Other input" table if the unknown-subcommand text changed |
| New panel or editor behaviour | `panel.html` |
| New error code | `error-codes.html` in that file's section, **and** `troubleshooting.html` if a reader can see it |
| Changed user-facing message text | `troubleshooting.html`, which quotes them verbatim |

There is intentionally no `api.html`. The JSON API is an implementation detail
of the sidebar, and its failures are documented as messages in
`troubleshooting.html` rather than as endpoints.

#### Rules

1. **No JavaScript and no remote assets.** No CDN, no web fonts, no external
   images, no inline event handlers. These must render on an air-gapped host.
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
2. Add the new page to the nav block of **all** the other pages.
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

`server/help_docs_test.go` covers anchors, navigation, the air-gap rules, em
dashes, and both directions of the code and setting inventories. It does not
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
- Adding a page but forgetting the other six navigation blocks, or `helpPages`.
- Using em dashes. The repo convention forbids them in docs and code.
- Hardcoding the plugin id. It belongs in `plugin.json` only; the webapp uses
  `docsUrl()` in `webapp/src/plugin_url.ts`. If a Go helper is ever needed it
  must be a **function**, not a package-level `var`, because var initialisers
  run before the generated `init()` populates the manifest and would panic at
  activation.

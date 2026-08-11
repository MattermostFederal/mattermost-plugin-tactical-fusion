# Decorator Framework + DTG Decorator

> **Status: superseded. This is a historical record of what was planned, not a
> description of what shipped.** It is kept for the reasoning behind the design,
> particularly the revision history at the foot. Read `CLAUDE.md` for the
> implemented contract; where the two disagree, `CLAUDE.md` is right and this
> file is not to be used to "restore" anything.
>
> Known divergences, all deliberate:
>
> - **Admin settings.** The plan specifies a single `EnableDecoration` switch.
>   What shipped is `EnableDTG` plus one switch per format under it
>   (`EnableDTGMilitary`, `EnableDTGMoniker`, `EnableDTGTimestamp`), with the
>   parent enforced in `Plugin.dtgFormats`. There is deliberately no global
>   "decorate at all" switch.
> - **RFC 3339 timestamps** are implemented and share the DTG decorator. The
>   plan lists them as out of scope.
> - **Reader preferences** ("Customize your view", `/api/v1/preferences`) are
>   implemented. The plan lists them as out of scope.
> - **The browser-local row** described under "Timezone display list" was not
>   built. Readers choose their own rows instead.

## Overview

Introduce a generic **decorator** framework that scans posts for domain patterns and rewrites the matches into markdown links whose query string carries the **already-parsed** data. Clicking a link in the webapp opens a decorator-owned RHS panel; following the same link from a client that resolves relative URLs against the server (the mobile app, a browser tab) lands on a page the plugin renders from those same params.

Ship **DTG (Date-Time Group)** as the first decorator, rendering a military DTG as a multi-timezone conversion view.

Decoration happens **on the server**, once, in `MessageWillBePosted`. That is the only way the link reaches clients that do not run the webapp bundle. Edits are never re-decorated.

## Problem Statement

Tactical Fusion exists to enrich conversation with mission-relevant context (geospatial, CoT, time zones, IP intel, CVEs). Every one of those is the same shape: *find a token in a message, make it interactive, show detail somewhere*. Built ad hoc, that is N copies of pattern scanning, link generation, click interception, and RHS routing.

`mattermost-plugin-aocanywhere` solved this shape once (`webapp/src/enhanced_text/`), but two ways we do not want to copy:

1. **As a flat switch over a hardcoded union of ten types.** `patterns.ts` holds every regex, `display_text.ts` a `switch (type)`, `click_handler.ts` another `switch (type)`, `enhanced_text_styles.tsx` a CSS rule per type, and `RHSView.tsx`/`RHSTitle.tsx` a third and fourth per-type branch. Adding a type means editing six files.
2. **Client-side only.** `registerMessageWillFormatHook` runs in the webapp bundle, so the mobile app, email notifications, and every other non-webapp surface render the raw undecorated text. aoc's `server/enhanced_text.go` fallback page is therefore unreachable in practice: there is no link on those surfaces to click.

We want a registry so adding a decorator means adding one directory per side, and server-side decoration so the link exists everywhere.

Today the plugin is a bare scaffold: `/tactical-fusion hello` and a channel-header button that fires `window.alert`.

## Current State

| Surface | Today |
|---------|-------|
| `server/plugin.go` | `OnActivate` registers one slash command. No `ServeHTTP`, no message hooks. |
| `server/configuration.go` | `configuration` struct exists; `plugin.json` `settings_schema.settings` is empty. |
| `webapp/src/index.tsx` | `initialize(registry)` registers a channel header button that `window.alert`s. Does **not** take the Redux `store` argument. |
| Tests | `HeaderIcon.pw.tsx` (component), `manifest.spec.ts` (node). `.spec.ts` runs under plain `@playwright/test` (**no DOM**). `.pw.tsx` runs under `experimental-ct-react` (DOM). |
| Deps | react 19, redux 5, react-redux 9. **No** `mattermost-redux`, no `@mattermost/client`. `tsconfig` has `resolveJsonModule: true`. |
| Build | Go module root is the repo root. `Makefile` copies `public/` into the bundle when it exists (`HAS_PUBLIC`). |

### Current Gaps

- No RHS component registered, so no place for a decorator to render detail.
- `initialize()` never captures the Redux store, so nothing can dispatch `showRHSPlugin`.
- No `ServeHTTP`, so any URL under `/plugins/<id>/` 404s.
- No message hooks, so nothing ever sees post text.

## Phase Strategy

| Phase | Focus | Value |
|-------|-------|-------|
| **Phase 1** (this plan) | Go registry + tagger + message hooks + fallback page; webapp registry + click handler + styles + RHS; DTG decorator | **80% of value**: the reusable pattern plus one real decorator, working on every client |
| Phase 2 | Admin-configurable timezone list; spaced DTG form; bare non-`Z` short forms; a second decorator | Robustness, proves the seam |
| Phase 3 | Backfill command to decorate existing history; hover previews via `registerLinkTooltipComponent` | Optional |
| Phase 4 | Geospatial / CoT / IP / CVE decorators | Deferred |

## Design Principles

| Concern | Our Approach | Avoid | Reference |
|---------|-------------|-------|-----------|
| Where decoration happens | **Server-side**, once, in `MessageWillBePosted` | Client-side format hook (never reaches mobile) | contrast aoc `index.tsx:149` |
| Edits | **Passed through untouched** | Re-decorating, which means transforming text the user deliberately authored | see Decisions |
| What the URL carries | The **parsed, validated fields**, including a resolved epoch | The raw token, re-parsed by each consumer | see Decisions |
| URL form | **Root-relative**, scheme and host never stored | Absolute URLs frozen into permanent post text | see Decisions |
| Where parsing lives | **Go only**. TypeScript reads query params and renders | A second grammar in TS that must stay in sync | n/a |
| Link label | The original matched text, unchanged | Rewriting the label to an enriched string | see Decisions |
| Adding a decorator | One Go directory + one TS directory + one line in each registry | A new `case` in six files | contrast aoc's `switch (type)` |
| False positives | `Parse` returns `ok=false`, so no link is written at all | A link that does nothing when clicked | aoc links unparseable DTGs |
| Overlap resolution | **Longest match wins**, registration order breaks ties | Registration order alone (hidden global coupling) | aoc `tagger.ts:86` |
| Type dispatch | Type is a **path segment**, `/decorate/dtg?...` | `?type=dtg`, whose selectors also match `type=dtg2` | aoc `enhanced_text_styles.tsx:29` |
| RHS state | One `selection` observable (`{type, payload}`) | One store per decorator + N-way `if` chains | aoc has 7 stores, `RHSView.tsx:466-509` |
| Unhandled link types | Let the browser follow the link to the server page | `preventDefault` and silently do nothing | n/a |
| Never break posting | Any internal error returns the post unmodified | Returning a rejection string from the hook | n/a |

## Reference Patterns (aocanywhere)

- `webapp/src/enhanced_text/tagger.ts:41`: `findProtectedRanges` skips fenced blocks, inline code, and existing markdown links. **This is the non-obvious part. Port the algorithm to Go nearly verbatim.**
- `webapp/src/enhanced_text/tagger.ts:126`: right-to-left replacement so indices stay valid.
- `webapp/src/enhanced_text/click_handler.ts:25`: capture-phase delegated listener.
- `webapp/src/store/create_store.ts:13`: 30-line observable store, no Redux needed.
- `webapp/src/components/rhs/TimeZoneInfo.tsx`: the panel we are generalizing (live clock, relative offset, zone table with ±1 day badge). `formatRelativeOffset` at `:128`.
- `webapp/src/components/usmtf_post/common/formatters.tsx:614`: `formatDateInTimezone` and the Julian-day `dayOffset` trick.
- `server/enhanced_text.go`: the shape of the fallback page, which here finally becomes reachable.

## Requirements

- [ ] A decorator is one Go type plus one TS object, each registered in one place.
- [ ] Registered decorators automatically get pattern scanning, link generation, a server-rendered page, link styling, click routing, RHS title, and RHS panel.
- [ ] Adding a decorator requires no edits to framework files except one line in each registry.
- [ ] DTG is recognised in posted messages and linked, with the parsed fields in the query string.
- [ ] Clicking a DTG link in the webapp opens the timezone panel in the RHS.
- [ ] Opening the same URL in the mobile app or a browser renders an equivalent page from the plugin.
- [ ] Matches inside code blocks, inline code, and existing markdown links are left alone.
- [ ] A match that fails validation is left as plain text.
- [ ] Displayed link text is byte-identical to what the author typed.
- [ ] A decoration failure never blocks or alters posting beyond the intended rewrite.
- [ ] Decoration can be turned off by a system admin.
- [ ] `make check-style && make test` pass.

## Out of Scope (Phase 1)

- Decorating message history posted before install (see Phase 3).
- Any decorator other than DTG.
- Per-user on/off preferences (server-side decoration is global; the admin setting is the Phase 1 control).
- Spaced DTG form (`091630Z AUG 26`) and bare non-`Z` short forms.
- Admin-configurable timezone lists.
- Reverting decoration on uninstall. Stored text keeps the links.
- i18n (repo has no i18n infrastructure today).

## Technical Approach

### URL contract

```
<siteurl-path>/plugins/com.mattermost.plugin-tactical-fusion/decorate/<type>?<params>
```

**The stored URL is root-relative. It never contains the scheme or host.** In the common case where SiteURL has no path component, that is literally:

```
/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?...
```

This is a deliberate and fairly important choice, because the URL is written into `post.Message` permanently. An absolute `https://old-host/plugins/...` would be frozen into every historical post, and the day the server moves to a new hostname every one of those links breaks with no way to re-render them. Relative URLs simply follow the server.

`<siteurl-path>` is the **path** component of SiteURL, and only the path. It is empty for `https://host`, and `/mattermost` for `https://host/mattermost`, in which case the emitted URL is `/mattermost/plugins/...`. The server derives it once from SiteURL at decoration time, so subpath installs work without hardcoding.

If SiteURL is **empty or unparsable**, treat it as "no subpath" and carry on. Nothing about the host is stored, so `/plugins/...` still resolves against whichever server the reader has open. A path that is not rooted, such as the one `example.com/mm` parses into, is ignored rather than emitted, since that would produce a URL relative to whatever page the reader is on.

Residual risk: changing the *subpath* later still breaks old links, unlike changing the host. That is much rarer than a domain migration, and the Phase 3 backfill command can repair it.

One property worth knowing: the webapp click handler matches on the path substring `/plugins/<id>/decorate/` and reads its params from the href without ever navigating. So even a link stored with a stale absolute host would still open the correct RHS panel. Only the non-webapp fallback would degrade, which makes any future migration less painful than it looks.

**This choice is provisional until Task 0b.** Relative is right for the webapp and for migration safety, but the mobile app's handling of root-relative markdown links is unverified. If 0b shows mobile cannot resolve them, the decision becomes absolute URLs (accepting that a domain migration permanently breaks every historical link) or keeping relative and dropping the mobile claim. Do not treat the mobile promise elsewhere in this plan as settled until that gate passes.

The type is a **path segment**, not a query param. That makes server routing a trivial prefix match, and it removes the `type=dtg` / `type=dtg2` prefix-collision hazard in both the CSS selectors and the click handler's `closest()`.

For DTG:

| Param | Meaning | Example |
|-------|---------|---------|
| `t` | Resolved instant, epoch milliseconds UTC | `1786379400000` |
| `dtg` | Canonical DTG string, for display | `091630ZAUG26` |
| `z` | Zone letter | `Z` |
| `a` | Assumed fields: `""`, `"y"`, or `"my"` | `my` |

Carrying the **resolved instant** rather than the component fields is the point of this design. The server has already done the zone arithmetic and the validation, so neither the RHS panel nor the fallback page repeats it, and there is no way for Go and TypeScript to disagree about what instant a DTG means.

Example rewrite:

```
ARCT 091630ZAUG26 confirmed
->
ARCT [091630ZAUG26](/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=) confirmed
```

### Server: the `Decorator` contract

```go
// server/decorators/decorator.go
type Pattern struct {
    Regexp *regexp.Regexp
    // Extract pulls the canonical value from a submatch set.
    // Nil means "use submatch 1".
    Extract func(m []string) string
}

type Decorator interface {
    // Type is the URL path segment. Unique across decorators.
    Type() string

    // Patterns are tried in order; earlier ones win ties.
    Patterns() []Pattern

    // Parse validates a matched value against a reference time and returns the
    // query params encoding it. ok=false means "not really one of us", and the
    // tagger leaves the text untouched. Must not panic and must not be slow:
    // it runs inline on the post path.
    Parse(value string, ref time.Time) (params url.Values, ok bool)

    // RenderPage writes the standalone page for these params. Params come
    // from an untrusted URL and must be re-validated here.
    RenderPage(w http.ResponseWriter, params url.Values)
}
```

### Server modules

```
server/decorators/
  decorator.go   Decorator, Pattern
  registry.go    Register(), All(), Get(type)
  tagger.go      Decorate(message, ref) string
  page.go        shared HTML shell for RenderPage
  dtg/
    dtg.go       the Decorator
    parse.go     parseDTG, formatDTG, resolveInstant
    zones.go     zone letter -> offset; DISPLAY_ZONES
    page.go      the DTG page body
```

**tagger.go** is a Go port of aoc's `tagger.ts`:

1. `findProtectedRanges(message)`: fenced blocks, inline code, existing markdown links (aoc `tagger.ts:41`).
2. Flatten the registry into `[{decorator, pattern}]` and run each regex over the message.
3. Skip matches overlapping a protected range.
4. Call `Parse(value, ref)`. **If `ok=false`, skip and do not claim the range**, so a rejected long-form match cannot suppress a valid shorter one at the same span.
5. Resolve overlaps among survivors: **longest match wins**, registration order breaks ties.
6. Replace right-to-left with `[<escaped original>](<url>)`.

Escaping is the framework's job:
- Label: escape `[`, `]`, `*`, `_`, `` ` ``, `~`.
- URL: `url.Values.Encode()`, then replace `(` with `%28` and `)` with `%29`, since Go leaves parentheses unescaped and an unbalanced `)` terminates a markdown destination.

Returns the input unchanged when nothing matches.

### Server: message hook

```go
func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
    return p.decoratePost(post, time.Now().UTC()), ""
}
```

**Decoration happens once, at post time. There is no `MessageWillBeUpdated` hook.** Edits are passed through untouched: whatever the author types is what gets stored.

This is the single most important simplification in the design. It deletes the entire unwrap-and-re-decorate mechanism, which was the only part of the plan capable of destroying user-authored markdown, and with it the three-condition unwrap rule, the all-or-nothing coupling, and the ordering hazard of running our own transform over our own prior output.

What it means in practice:

- **An author who edits a decorator link owns the result.** Change the label and leave the URL alone, and the link text will disagree with the panel. That is now an explicit user action rather than a silent framework behaviour, and it is self-revealing: the panel renders the canonical DTG from the `dtg` param, so a mismatch is visible the moment it is opened rather than lurking.
- **Removing a link is how you undo decoration.** Delete the markdown syntax while editing and the text stays plain forever. That is a feature: it gives users an escape hatch that the admin-level setting cannot offer per-post.
- **A DTG typed during an edit is not decorated.** Decoration is a property of posting, not of the text. Consistent and easy to explain.
- **Breaking the markdown while editing yields broken markdown.** Their edit, their result. We do not police it.

Four details that matter:

- **`post.CreateAt` is not yet set in `MessageWillBePosted`.** Use `time.Now().UTC()` as the reference. Because decoration happens exactly once and the result is stored, the reference is frozen by construction, which is precisely the property the earlier client-side design had to work hard to fake.
- **Return `nil, ""` when nothing changed.** The documented hook contract is that `nil` means "allow without modification" and a non-nil post replaces the original. `decoratePost` returns `nil` unless it actually rewrote the message.
- **Never reject.** `decoratePost` recovers from any panic, logs via `p.API.LogWarn`, and returns `nil`. The second return value is always `""`. A regex bug must not stop people from posting.
- **Skip narrowly, not broadly.** Decoration is skipped when the admin setting is off and when `post.Type` is in an explicit deny list (`system_*` prefixed types). An earlier draft skipped every non-empty `post.Type`, which would also skip custom post types from integrations and other plugins that may carry real mission content.

**Bail if decoration would exceed `MaxPostSize`.** `091630ZAUG26` is 12 characters and becomes roughly 120 once linked, so a DTG-dense message that is visibly well under the limit can cross it after rewriting. The server would then reject the post and the author would see an opaque "post too long" error for text they can see fits. Before returning, compare the decorated message against `p.API.GetConfig().ServiceSettings.MaxPostSize` (counted in runes, and configurable, so read it rather than assuming 4000). If it would exceed, return `nil` and log at warn. Losing decoration is strictly better than losing the post.

**Idempotence and hook ordering.** Mattermost runs `MessageWillBePosted` across plugins in an undefined order, feeding each the previous output. We make no ordering assumptions. Existing markdown links are a protected range, so if another plugin links a DTG before we run, we skip it and the post is simply undecorated: degraded, never corrupt. The same protection makes the tagger idempotent on its own output, which `tagger_test.go` asserts.

### Server: the page

`ServeHTTP` routes `GET /decorate/<type>` to `Get(type).RenderPage(w, r.URL.Query())`, and returns 404 otherwise. A shared shell in `page.go` supplies a self-contained, mobile-friendly, theme-neutral HTML document; the decorator supplies the body.

**The `/decorate/*` route is deliberately public.** Mattermost sets the `Mattermost-User-Id` header only for requests carrying a valid session, and plugins must enforce their own auth. The client this page exists to serve is precisely the one without a session cookie: the mobile app opening a link in an in-app browser. Requiring auth here would break the feature it was added for.

That is safe only because the page is a pure function of its query string:

- It renders nothing but a timezone conversion of values supplied in the URL.
- It performs **no** workspace lookup: no post, channel, user, team, or config read.
- It never reads or trusts `Mattermost-User-Id`.
- It leaks nothing, because a caller who constructs the URL already knows everything the page will show.

Any future decorator whose page would need workspace data must not follow this pattern; it needs its own authenticated route. That constraint belongs in the `Decorator` doc comment so it is not discovered the hard way.

`RenderPage` re-validates every param, since they arrive from an untrusted URL: `t` must parse as an integer within a sane epoch range and `dtg` must match the canonical grammar before anything is echoed. All interpolation goes through `html.EscapeString`. `ServeHTTP` always sets `X-Content-Type-Options: nosniff`.

**Cache headers are decorator-owned, not a framework default.** DTG sets `Cache-Control: public, max-age=3600` because its rendering is deterministic in its params and contains nothing user-specific. A framework-wide default of `public` would be wrong the moment a decorator carries anything sensitive, so the shell leaves the header to the decorator and defaults to `no-store` if none is set.

The DTG page renders the same content as the RHS panel: canonical DTG, relative offset, the assumed-fields note, and the zone table, all computed in Go via `time.LoadLocation`. A small inline script ticks the live Zulu clock; with JavaScript disabled the page still shows the full table.

**`server/main.go` must `import _ "time/tzdata"`.** Go's `time.LoadLocation` reads the host IANA database, which minimal plugin runtime images frequently lack; without the embedded copy every non-UTC row fails at runtime while UTC keeps working, so it would pass a careless smoke test. `zones_test.go` asserts a non-UTC zone resolves.

### Webapp: the `Decorator` contract

Because the server ships parsed data, the webapp side has no patterns and no grammar:

```ts
// webapp/src/decorators/types.ts
export interface Decorator<T> {
    /** Must equal the Go decorator's Type(). */
    type: string;

    /** Params from the clicked URL. Return null if they are unusable. */
    fromParams: (params: URLSearchParams) => T | null;

    /** RHS header text. */
    summary: (payload: T) => string;

    /** Link colours; the framework generates the CSS rule. */
    style: {color: string; background: string};

    /** RHS body. */
    Panel: React.ComponentType<{payload: T}>;
}
```

```
webapp/src/decorators/
  types.ts          contract + "how to add a decorator" doc comment
  registry.ts       register(), all(), get(type), decoratePathPrefix()
  click_handler.ts  dispatchDecoratorClick() + installDecoratorClickHandler()
  styles.ts         installDecoratorStyles()
  selection.ts      selection observable + initRhs()/openRhs()/toggleRhs()/clearSelection()
  index.ts          registerBuiltinDecorators()
  dtg/
    index.ts        the Decorator<Dtg>
    zones.ts        DISPLAY_ZONES (IANA names)
    DtgPanel.tsx    live clock + relative offset + zone table
```

**`decoratePathPrefix()`** returns `` `${window.basename ?? ''}/plugins/${manifest.id}/decorate/` ``. A hardcoded root-relative path breaks subpath installs (`https://host/mattermost`).

**click_handler.ts** splits routing from DOM wiring so routing is testable without a DOM:

```ts
export function dispatchDecoratorClick(type: string, params: URLSearchParams): boolean {
    const d = get(type);
    if (!d) {
        return false;            // unknown type: let the browser follow the link
    }
    const payload = d.fromParams(params);
    if (payload === null) {
        return false;
    }
    setSelection({type, payload});
    openRhs();
    return true;
}
```

The DOM listener calls `preventDefault()` **only when `dispatchDecoratorClick` returns true**. Two consequences, both wanted:

- A link type the webapp does not know (an older bundle against a newer server) degrades gracefully to the server page instead of dying silently.
- Ctrl/cmd/middle clicks are left alone entirely, so "open in new tab" reaches the real page. This is the opposite of the previous revision's decision, and it is now the right one: the URL is a genuine destination.

Capture phase is still required, otherwise MM's internal-link handler routes the click through React Router first (aoc `click_handler.ts:26-28`). The installer is idempotent and returns a disposer.

**styles.ts** builds a static stylesheet from the registry once at startup and appends a `<style>` to `document.head`. Not a React component: nothing is dynamic, and this avoids `registerGlobalComponent`, which the local type definitions flag `INTERNAL: Subject to change without notice` (`index.d.ts:934`).

Selectors key on the **full plugin path**, `a[href*="/plugins/<id>/decorate/<type>?"]`, not the bare `/decorate/<type>?` suffix, so an unrelated link elsewhere on the page cannot match. The type is CSS-escaped before interpolation. `click_handler.ts` uses the same full-prefix string in its `closest()` call, from a single shared helper so the two can never drift.

**selection.ts** holds one observable `{type, payload} | null`, so `RhsView` is a lookup rather than an `if` chain:

```tsx
const RhsView = () => {
    const sel = useSelection();
    const d = sel && get(sel.type);
    if (!sel || !d) {
        return <EmptyState/>;
    }
    return <d.Panel payload={sel.payload}/>;
};
```

`RhsTitle` lives in the same file and renders `d.summary(sel.payload)` or `'Tactical Fusion'` when there is no selection.

### RHS lifecycle

| Event | Behavior |
|-------|----------|
| Header button clicked | `toggleRhs()` **and clear the selection**, so the button always lands on the empty state, which is also the only way back from a panel |
| Decorator link clicked | Set selection, `openRhs()`, `preventDefault()` |
| Second link clicked | Panel swaps |
| Ctrl/cmd/middle click | Not intercepted; opens the server page in a new tab |
| Unknown type, or unusable params | Not intercepted; the browser navigates to the server page |
| Channel switch | Selection is retained. A timezone conversion is not channel-scoped. Explicitly accepted |
| RHS closed by the user | MM does not notify plugins, so the selection persists. Reopening via the header button clears it, so there is no stale-panel trap |

### The DTG decorator

**Recognised forms (Phase 1).**

| Form | Example | Zone letters | Notes |
|------|---------|--------------|-------|
| `DDHHMM<Z>MMMYY` | `091630ZAUG26` | any `A`-`Z` except `I`, `J` | standard NATO form |
| `DDHHMM<Z>MMMYYYY` | `091630ZAUG2026` | any `A`-`Z` except `I`, `J` | 4-digit variant |
| `DDHHMMZ` | `091630Z` | **literal `Z` only** | month and year inferred from the reference time |

Every pattern is anchored with `\b` in front and `(?![A-Za-z0-9])` behind.

The zone letter widens past `Z` only on the forms where the 3-letter month abbreviation anchors the match. Bare `\b\d{6}[A-Z]\b` is too loose for a general-purpose plugin: it hits part numbers, serials and truncated hashes, and it mis-tags `091630JUL26` as `091630J` plus a dangling `UL26`. aoc restricts its only bare pattern to the 14-character `Z` form (`patterns.ts:55`) for exactly this reason, and gates everything shorter behind a `TIME:`/`FROM:` prefix.

**Zone letters.** `Z` = UTC, `A`-`H` = UTC+1..+8, `K`-`M` = UTC+10..+12, `N`-`Y` = UTC-1..-12. There is **no `I`**. `J` (observer's local time) is **rejected in Phase 1**: it would make the resolved instant reader-dependent, which is incompatible with baking a single `t` into the URL.

**Validation** (`Parse` returns `ok=false` otherwise): hour 00-23, minute 00-59, month one of the twelve abbreviations, zone letter valid, and **day valid for that month including leap years**. A plain 01-31 check is not enough: `time.Date(2026, 2, 31, ...)` normalises silently to 3 March, and the page would then confidently show a wrong date in every row. `parse.spec` must assert a parse then re-format round trip.

Two-digit year `NN` maps to `20NN`.

**Resolving the instant.** `resolveInstant` builds the UTC time and **subtracts the zone letter's offset**. aoc's `dateTimeToDate` (`formatters.tsx:601-611`) hardcodes UTC because it only ever handled Zulu.

**Assumed fields.** For the short form, month and year come from the reference time, and `a=my` records that. Both the panel and the page show "month and year taken from the post date". aoc assumes silently, which is the actual bug worth fixing.

**Timezone display list.** `DISPLAY_ZONES` is UTC, plus a small curated operational set, all **IANA names** so `Intl` and `time.LoadLocation` handle DST. The military letter table is used only to interpret the DTG's own zone, never to render rows. The RHS panel adds a browser-local row, which the server page cannot know. The list is hardcoded on both sides; a Go test reads `webapp/src/decorators/dtg/zones.ts` and fails if the IANA strings diverge. The `±1` day badge is computed against the DTG's own zone date, not UTC (aoc's `dayOffset`, `formatters.tsx:629-643`, is UTC-relative, which reads wrong for a non-Zulu DTG).

### Admin setting

`plugin.json` gains one boolean, `EnableDecoration`, default `true`. `configuration.go` exposes it and `decoratePost` returns early when it is off. Since decoration rewrites stored text, an admin needs a way to stop it without uninstalling.

### Webapp bootstrap

```tsx
public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
    registerBuiltinDecorators();

    const {showRHSPlugin, toggleRHSPlugin} =
        registry.registerRightHandSidebarComponent(RhsView, <RhsTitle/>);
    initRhs(store, showRHSPlugin, toggleRHSPlugin);

    this.disposers.push(installDecoratorClickHandler());
    installDecoratorStyles();

    registry.registerChannelHeaderButtonAction(
        <HeaderIcon/>, () => { clearSelection(); toggleRhs(); },
        'Tactical Fusion', 'Tactical Fusion',
    );
}

public uninitialize() {
    this.disposers.forEach((d) => d());
    this.disposers = [];
}
```

No `registerMessageWillFormatHook`: the link is already in the message. `uninitialize` matters, or a re-registration adds a second capture listener that dispatches every click twice.

`GlobalState` is not exported by `@mattermost/types` for the webapp, so declare a minimal local slice in `webapp/src/types/store.ts` rather than pulling in `mattermost-redux`.

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Decorate client-side or server-side? | **Server-side** | The only way the link exists on mobile, in email, and in any non-webapp client. Accepted cost: stored post text is rewritten |
| Rewrite stored `post.Message`? | **Yes**, no backup copy in props | Confirmed. Displayed text is unchanged and search still contains the token, but editing shows raw markdown to the author, exports contain links, and removing the plugin leaves 404 links in old posts |
| Decorate existing history? | No, Phase 3 | The hook only sees newly created posts. A backfill command is separable and riskier |
| What goes in the URL? | The **resolved instant** plus display fields | The server already parsed and validated; shipping `t` means the RHS and the page cannot disagree about the instant, and neither repeats the zone arithmetic |
| Absolute or relative URL in the stored message? | **Root-relative**, carrying only SiteURL's path component | The URL is permanent in `post.Message`. An absolute URL would freeze the hostname into every historical post and break them all on a domain migration, with no re-render available to fix them. Relative follows the server. Cost: email clients cannot resolve a relative href, so that surface degrades to visible label text |
| Type as query param or path segment? | **Path segment** | Trivial server routing, and it removes the `type=dtg` / `type=dtg2` prefix collision in CSS selectors and `closest()` |
| Rewrite the link label to enriched text? | **No**, keep the original text | Operators copy DTGs between systems; rewriting silently changes what they paste and what screen readers announce. Enrichment belongs in the panel and the page |
| Parse in Go, TS, or both? | **Go only** | A second grammar in TypeScript is a permanent sync hazard. TS reads params |
| Reference time for inference | `time.Now()` at post time | Decoration runs once and is stored, so the reference is frozen by construction. With no edit-time decoration there is no second reference to reconcile |
| Behavior on unknown link type | Do not intercept; let the browser navigate | An older bundle against a newer server degrades to the server page instead of a dead click |
| Ctrl/cmd/middle click | Not intercepted | The URL is now a real destination, so "open in new tab" should work |
| Overlap resolution | Longest match wins, registration order as tiebreak | Registration order alone is a hidden global coupling: adding a decorator could silently change an existing one |
| Does a rejected match claim its range? | **No** | Otherwise a rejected long-form match suppresses a valid short-form one at the same span |
| Bare short forms with non-`Z` letters? | Deferred to Phase 2 | Too loose without a month abbreviation to anchor them |
| `J` (local) zone letter | Rejected in Phase 1 | A reader-dependent instant cannot be baked into `t` |
| On decoration error | Return `nil`, log a warning | A regex bug must never block posting. `nil` is the documented "no modification" value |
| Is `/decorate/*` authenticated? | **Public** | `Mattermost-User-Id` is absent for exactly the cookie-less clients the page exists to serve. Safe only because the page is a pure function of its query string and touches no workspace data, which is now a documented constraint on the `Decorator` interface |
| Re-decorate on edit? | **No.** No `MessageWillBeUpdated` hook at all | The unwrap-and-re-decorate machinery this replaces was the only part of the design that could destroy user-authored markdown. Editing a decorator link is now a deliberate user action with a predictable result: change the label and it disagrees with the panel; delete the link syntax and the text stays plain forever. Users get a per-post escape hatch, and the framework stops second-guessing text a human deliberately wrote. A DTG typed during an edit is simply not decorated |
| Decoration would exceed `MaxPostSize` | **Return `nil`, log at warn** | A 12-character DTG becomes ~120, so a short-looking message can cross the limit and be rejected with an opaque error. Losing decoration beats losing the post |
| Hook ordering with other rewriting plugins | No assumptions; guarantee only self-idempotence | Order is undefined. If another plugin links a DTG first, we skip it: degraded, never corrupt |
| How to skip system posts | Deny-list `system_*` types | Skipping every non-empty `post.Type` would also skip integration and plugin post types that may carry mission content |
| Admin kill switch? | Yes, one boolean | Decoration rewrites stored content; an admin needs to stop it without uninstalling |
| Per-user toggle? | No | Server-side decoration is global by nature. Revisit in Phase 2 as a rendering-level preference |
| Registry typing across heterogeneous `T` (TS) | Registry stores `Decorator<any>` with one documented eslint-disable in `registry.ts` | `Decorator<T>` is invariant (`T` is in parameter position in `summary` and `Panel`), so `<d.Panel payload={sel.payload}/>` will not typecheck otherwise. Authoring stays typed; the erasure is confined to one file |
| Redux or plain observable for selection? | Plain observable | ~25 lines, no `registerReducer`, no `mattermost-redux` |
| Timezone list duplication | Hardcode both sides, cross-check in a Go test | A shared embedded JSON would have to live inside `server/` for `//go:embed`, forcing an ugly relative import from the webapp. A 15-line test is cheaper and just as safe |
| i18n now? | No | Repo has no i18n infrastructure; strings are centralised so it stays a mechanical follow-up |

## Files to Modify / Add

| File | Change |
|------|--------|
| `server/decorators/decorator.go` | **new**: `Decorator`, `Pattern` |
| `server/decorators/registry.go` | **new**: `Register`/`All`/`Get` |
| `server/decorators/tagger.go` | **new**: protected ranges, parse gate, longest-wins, escaping, replace |
| `server/decorators/page.go` | **new**: shared HTML shell |
| `server/decorators/dtg/{dtg,parse,zones,page}.go` | **new**: DTG decorator |
| `server/hooks.go` | **new**: `MessageWillBePosted` and `decoratePost`. Deliberately no `MessageWillBeUpdated` |
| `server/http.go` | **new**: `ServeHTTP` routing `/decorate/<type>` |
| `server/configuration.go` | add `EnableDecoration` |
| `server/main.go` | add `import _ "time/tzdata"` |
| `server/plugin.go` | register decorators in `OnActivate` |
| `plugin.json` | add the `EnableDecoration` setting |
| `webapp/src/decorators/types.ts` | **new**: contract + add-a-decorator doc comment |
| `webapp/src/decorators/registry.ts` | **new**: `register`/`all`/`get`, `decoratePathPrefix()` |
| `webapp/src/decorators/click_handler.ts` | **new**: `dispatchDecoratorClick` + idempotent installer returning a disposer |
| `webapp/src/decorators/styles.ts` | **new**: registry-driven stylesheet |
| `webapp/src/decorators/selection.ts` | **new**: selection observable + RHS actions |
| `webapp/src/decorators/index.ts` | **new**: `registerBuiltinDecorators()` |
| `webapp/src/decorators/dtg/{index,zones}.ts` | **new**: DTG decorator, display zones |
| `webapp/src/decorators/dtg/DtgPanel.tsx` | **new**: timezone panel |
| `webapp/src/components/rhs/RhsView.tsx` | **new**: `RhsView` + `RhsTitle` + `EmptyState` |
| `webapp/src/types/store.ts` | **new**: minimal `GlobalState` slice |
| `webapp/src/index.tsx` | rewrite `initialize`; add `uninitialize`; header button toggles RHS; drop the `alert` |
| `CLAUDE.md` | Architecture section covering both `decorators/` trees and how to add one |

## Tasks

**Step 0: prerequisite spike (do this before writing any framework code)**

0. [ ] **Confirm search still matches a decorated DTG.** By hand on a scratch server: post `ARCT 091630ZAUG26 confirmed`, edit the message in the database (or post the decorated form directly) so it reads `ARCT [091630ZAUG26](/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=) confirmed`, then search for `091630ZAUG26`.

    This is a **design stop**, not a checkbox. The whole plan rests on rewriting `post.Message`, and if Mattermost's indexer splits on `[` or `]` then decorated posts become unfindable by the exact token operators search for. Test against the same search backend the deployment will use (database search and Bleve tokenize differently). If it fails, stop and reconsider: the fallbacks are decorating only inside `post.Props` with a custom post type, or reverting to client-side decoration and giving up the mobile promise.

0b. [ ] **Confirm the mobile app resolves a root-relative markdown link against the server URL.** Using the same hand-decorated post, tap the link in the iOS or Android app and confirm it opens `<server>/plugins/.../decorate/dtg?...` rather than failing or resolving against nothing.

    This gates the mobile promise, which is the entire reason revision 2 moved decoration to the server. If mobile does not resolve relative hrefs, the choice becomes: store absolute URLs and accept that a domain migration permanently breaks every historical link, or keep relative and drop the mobile claim. Decide it here, not after the framework is built.

**Server framework**
1. [ ] `decorators/decorator.go` + `registry.go` + `registry_test.go`: registration order, duplicate-type guard, `Get` miss.
2. [ ] `decorators/tagger.go` + `tagger_test.go`: protected ranges; parse gate; rejection does **not** claim a range; longest-wins overlap; label escaping; `(`/`)` in the URL; unchanged output when nothing matches; URL built from both an empty and a `/mattermost` SiteURL path, never containing a scheme or host; an unset or malformed SiteURL still decorates, with no subpath.

   Plus idempotence: running the tagger twice over the same message produces identical output, and a message already containing our decorator links is left alone.

2b. [ ] `MaxPostSize` handling in `hooks.go` + tests: a message that only overflows after decoration returns `nil` and posts undecorated.
3. [ ] `decorators/page.go`: shared HTML shell.

**DTG decorator (server)**
4. [ ] `dtg/zones.go` + `zones_test.go`: letter to offset (no `I`, `J` rejected); `DISPLAY_ZONES`; the cross-check test against `webapp/src/decorators/dtg/zones.ts`.
5. [ ] `dtg/parse.go` + `parse_test.go`: three forms; every rejection branch; day-in-month including leap years; `091630JUL26` and other adjacency cases; 2-digit year; assumed flags; parse then re-format round trip; `resolveInstant` for a non-`Z` letter.
6. [ ] `dtg/dtg.go` + `dtg/page.go` + `page_test.go`: params round-trip; page re-validates untrusted params; XSS payloads in `dtg` are escaped; renders without JavaScript.

**Server wiring**
7. [ ] `server/hooks.go` + `hooks_test.go`: `MessageWillBePosted` only, reference time is now, deny-listed `post.Type` skipped, setting off skips, `nil` returned when nothing changed, panic recovery returns `nil` and never a rejection string. Assert **no `MessageWillBeUpdated` method exists on `Plugin`**, so the pass-through-on-edit decision cannot be undone by accident. Also **verify by hand which post sources actually reach the hook**: a `p.API.CreatePost` call, an incoming webhook, a bot post, and an `in_channel` slash command response. Record the real behavior in `CLAUDE.md` rather than assuming; ephemeral slash responses are not stored posts and are expected to be out of reach.
8. [ ] `server/http.go` + `http_test.go`: 200 for a known type, 404 for an unknown type and any other path, **200 with no `Mattermost-User-Id` header** (the public contract), and cache headers set.
9. [ ] `configuration.go`, `plugin.json`, `plugin.go`, `main.go`: the `EnableDecoration` setting, decorator registration, and the `_ "time/tzdata"` import.

**Webapp**
10. [ ] `decorators/types.ts` + `registry.ts` + `registry.spec.ts`: duplicate guard, `Get` miss, `decoratePathPrefix()` under a subpath basename.
11. [ ] `decorators/selection.ts` + `selection.spec.ts`: get/set/subscribe/unsubscribe/clear.
12. [ ] `decorators/click_handler.ts` + `click_handler.spec.ts` (pure dispatch: known type, unknown type, unusable params) + `click_handler.pw.tsx` (capture listener fires, `preventDefault` only on handled links, modified clicks pass through, disposer works, double-install is a no-op).
13. [ ] `decorators/styles.ts` + `styles.spec.ts`: one rule per registered decorator, selector keyed on the path segment.
14. [ ] `components/rhs/RhsView.tsx` + `RhsView.pw.tsx`: selection to panel; null selection to `EmptyState`; unknown type to `EmptyState`; `RhsTitle` null-safe.
15. [ ] `dtg/index.ts` + `dtg/zones.ts` + `DtgPanel.tsx` + `DtgPanel.pw.tsx`: `fromParams` rejects malformed params; clock interval cleared on unmount; relative offset; assumed note; zone table; `±1` badge relative to the DTG's own zone.
16. [ ] `types/store.ts`; rewrite `webapp/src/index.tsx`; add `uninitialize`.

**Docs and verification**
17. [ ] `CLAUDE.md` Architecture section.
18. [ ] `make check-style && make test`; `make sbom-audit`; manual smoke via `make deploy`, including the mobile app.

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| **A tagger bug corrupts user messages permanently.** This is the sharpest edge of server-side decoration | Panic recovery returning the post unmodified; never return a rejection string; admin kill switch; `tagger_test.go` asserts byte-exact output on a corpus including adversarial markdown. Verify on a scratch server before any real deployment |
| Mattermost search may tokenize `[091630ZAUG26](...)` differently than bare text | **Task 0, before any code.** Promoted to a prerequisite spike because a failure here invalidates the architecture, not just a task |
| `MessageWillBePosted` may not fire for posts created by plugins, bots, or integrations | **Verify during task 7** across `p.API.CreatePost`, an incoming webhook, a bot post, and an `in_channel` command response. Document the real behavior; do not assume. Ephemeral responses are known to be out of scope |
| Non-UTC timezone rows fail at runtime on minimal images | `import _ "time/tzdata"` in `server/main.go`; `zones_test.go` asserts a non-UTC zone resolves. Without it, UTC still works, so this hides from a casual smoke test |
| A hand-edited decorator link shows a label that disagrees with its panel | Accepted by design. Editing the link is a deliberate user action, and the mismatch is self-revealing because the panel renders the canonical DTG from the `dtg` param. Deleting the link syntax is the supported way to opt a post out |
| A short-looking post is rejected because decoration pushed it over `MaxPostSize` | Size check against the configured limit before returning; `nil` on overflow. Test a message that only overflows after decoration |
| A DTG added while editing an existing post is not decorated | Accepted. Decoration is a property of posting, not of the text. Simple to explain and easy to predict |
| A malformed SiteURL writes a relative prefix into permanent post text | Only a rooted path is used; anything else falls back to no subpath, which still resolves correctly. Decoration is never skipped for this |
| The mobile app may not resolve root-relative markdown links | **Task 0b, before any code.** If it does not, the choice is absolute URLs (permanent, breaks on domain migration) or dropping the mobile claim. Decide before building |
| Changing SiteURL's **subpath** later breaks stored links | Rarer than a domain change, which relative URLs already survive. The Phase 3 backfill command can rewrite the prefix. Note that webapp clicks keep working regardless, since the handler matches on the path substring and never navigates |
| The public `/decorate/*` route becomes an information leak if a future decorator needs workspace data | The public contract is documented on the `Decorator` interface: a page must be a pure function of its query string. Anything needing workspace data gets its own authenticated route |
| Editing a post shows raw markdown to the author | Accepted and confirmed. The label preserves the original text, so the author can still read the DTG inside the link syntax |
| Bare `DDHHMMZ` false positives | Literal `Z` only; `\b` plus a trailing `(?![A-Za-z0-9])`; full range and day-in-month validation; no link on failure |
| Removing the plugin leaves 404 links in old posts | Accepted and confirmed. Noted in `CLAUDE.md` so it is not a surprise later |
| Untrusted params reflected into the page | `RenderPage` re-validates every param against the canonical grammar before echoing; all interpolation through `html.EscapeString` |
| Go and TypeScript timezone lists drift | Cross-check test in `zones_test.go` reads the TS file |
| Listener leak or double-install on plugin reload | Idempotent installer returning a disposer; `uninitialize` calls it |
| Double-decoration | No edit hook, plus existing markdown links are a protected range, so the tagger is idempotent. Asserted in `tagger_test.go` |

## UX Summary

| Scenario | Behavior |
|----------|----------|
| User posts `ARCT 091630ZAUG26 confirmed` | Stored message becomes `ARCT [091630ZAUG26](/plugins/.../decorate/dtg?t=...&dtg=091630ZAUG26&z=Z&a=) confirmed` |
| Webapp reader sees it | Styled link reading `091630ZAUG26` |
| Webapp reader clicks it | RHS opens, titled `09 Aug 2026 16:30Z`, showing the zone table |
| **Mobile reader taps it** | Opens the plugin-rendered page with the same zone table |
| Email notification | Shows the DTG label as readable text. The link is **not** expected to work: a mail client has no base URL to resolve a root-relative href against. Accepted cost of migration-safe URLs |
| Ctrl-click or middle-click | Opens the server page in a new tab |
| User posts `091630Z` | Links; panel and page both show "month and year taken from the post date" |
| User posts `311200ZFEB26` (31 Feb) | Left as plain text, no link |
| User posts `091630JUL26` | Left as plain text, no link (short form requires literal `Z`) |
| DTG inside a code fence or an existing markdown link | Left untouched |
| Reader copies the post text | Gets `091630ZAUG26`, byte-identical to what the author typed |
| Author edits the post | Sees the raw markdown link in the edit box. Whatever they save is stored verbatim, with no re-decoration |
| Author edits the label inside a decorator link | Kept exactly as typed. The link text will disagree with the panel, which renders the canonical DTG from the URL. Their edit, their result |
| Author deletes the link syntax while editing | The text stays plain permanently. This is the supported way to opt one post out of decoration |
| Author types a new DTG while editing | Not decorated. Decoration only happens at post time |
| Posts from before install | Undecorated. Phase 3 adds a backfill |
| Admin turns `EnableDecoration` off | New posts are not decorated; existing links keep working |
| Header button clicked | Selection cleared, RHS toggles to the empty state |

## Testing Plan

**Go (`_test.go`)** covers the registry, the tagger (including a byte-exact corpus and adversarial markdown), DTG parsing and validation, instant resolution for non-`Z` letters, the page renderer against untrusted params, the message hooks, and `ServeHTTP` routing. All DTG tests pass an explicit fixed reference time, so nothing is flaky at month boundaries or on 29 February.

**Unit (`.spec.ts`, node, no DOM)**: `registry`, `selection`, `dispatchDecoratorClick`, `styles` (string output), `dtg.fromParams`.

**Component (`.pw.tsx`)**: `DtgPanel`, `RhsView`/`RhsTitle`, and the click handler's DOM wiring (including that modified clicks and unknown types are *not* intercepted).

**Cross-boundary test**: a Go test asserts that a URL produced by `Decorate` is consumed correctly by the TypeScript `fromParams` fixture, using a shared golden file of `(input message, expected URL)` pairs that both `tagger_test.go` and `dtg/index.spec.ts` read. This is the seam most likely to rot.

**Framework-genericity test**: `registry_test.go` and `tagger_test.go` register a fixture decorator defined in the test file. This is the only real proof of the "add a decorator without editing the framework" claim, and it doubles as the overlap fixture.

**Manual**: `make deploy`; post each UX-table sample; **verify search still finds a decorated DTG**; verify a webhook post is decorated; confirm the link works in the mobile app; confirm it works on a subpath install (`https://host/mattermost`); toggle `EnableDecoration`.

`tagger_test.go` covers URL construction against both an empty SiteURL path and a `/mattermost` subpath, and asserts no emitted URL ever contains a scheme or host.

## Acceptance Criteria

- [ ] The fixture decorator in `registry_test.go` works without editing any file in `server/decorators/` outside its own directory.
- [ ] `091630ZAUG26`, `091630ZAUG2026`, and `091630Z` are decorated with correct params; `311200ZFEB26` and `091630JUL26` are not.
- [ ] Link text is byte-identical to the source text.
- [ ] A DTG inside a code fence or an existing markdown link is untouched, and re-saving a post does not double-decorate.
- [ ] Clicking a DTG link in the webapp opens the panel; ctrl-clicking opens the server page.
- [ ] **Opening the URL in the mobile app renders the timezone table.**
- [ ] A non-`Z` DTG such as `091630RAUG26` resolves to the correct UTC instant, identically in the panel and the page.
- [ ] Short-form DTGs show the assumed-fields note in both places.
- [ ] **Task 0 passed**: searching for a decorated DTG still finds the post, on the deployment's search backend.
- [ ] **The page loads with no Mattermost session**, in a private browser window and in the mobile app.
- [ ] No stored URL contains a scheme or host, on either a root install or a `/mattermost` subpath install.
- [ ] A post whose decorated form would exceed `MaxPostSize` posts successfully, undecorated.
- [ ] Running the tagger twice over the same message produces identical output.
- [ ] Editing a post stores exactly what the author typed, with no re-decoration, including when they alter or delete a decorator link.
- [ ] A non-UTC zone row renders correctly on a minimal container image (proves `tzdata` is embedded).
- [ ] Hand-editing a decorator link and re-saving stores the edit verbatim, leaving the URL exactly as the author left it.
- [ ] A forced panic in the tagger leaves the post unmodified and does not block posting.
- [ ] `EnableDecoration=false` stops decoration of new posts.
- [ ] `make check-style` and `make test` pass.

## Checklist

- [ ] **Diagnostics**: N/A, this repo has no diagnostics channel.
- [ ] **Slash command**: `/tactical-fusion dtg <value>` to open the panel without a post, Phase 2.
- [ ] **Conventional commit**: `feat: add decorator framework and DTG decorator` (server + webapp), minor bump.

---

## Revision History

### Revision 2c (current): no re-decoration on edit

Decoration now happens exactly once, at post time. `MessageWillBeUpdated` is gone, and edits are stored verbatim.

This deletes the largest remaining source of risk in the plan. Revision 2b's unwrap-and-re-decorate machinery existed only to stop a hand-edited label drifting from its `t` param, and it was the one operation capable of destroying user-authored markdown. Removed with it: the three-condition unwrap rule and its worked-cases table, the all-or-nothing coupling with the size check, the `oldPost.CreateAt` second reference time, and the hazard of running our own transform over our own prior output.

What replaces it is a stance rather than a mechanism: if an author edits a decorator link, that is a deliberate act and we honour it. Changing the label produces a link whose text disagrees with the panel, which is self-revealing because the panel renders the canonical DTG from the URL. Deleting the link syntax leaves the text plain forever, which gives users a per-post escape hatch the admin setting cannot offer. A DTG typed during an edit is simply not decorated.

`hooks_test.go` now asserts that no `MessageWillBeUpdated` method exists on `Plugin`, so this cannot be silently reintroduced.

### Revision 2b: second technical review applied

Reviewed by Codex (`gpt-5.5`), Gemini (run in an isolated working directory), and a seq-server reasoning pass. Verdicts split: Codex NEEDS WORK, Gemini READY. Sided with Codex, because the unwrap case is a data-integrity bug in code that rewrites stored user content.

Two blockers fixed, both flagged by both models:

1. **Decoration could push a post over `MaxPostSize`.** A 12-character DTG becomes roughly 120, so a visibly short but DTG-dense message would be rejected after the hook with an opaque "post too long". Now bounded by a check against the configured limit, returning `nil` on overflow.
2. **Unwrap-on-edit could destroy user-authored markdown.** Revision 2a's fix for label/`t` desync was itself unsafe: `[091630ZAUG26 with note](...)` would have been restructured into `[091630ZAUG26](...) with note`. Now gated on three strict conditions that degrade to "leave untouched" whenever uncertain, with a worked-cases table and a dedicated test block.

A coupling neither model spotted, found in the seq-server pass: if the size check trips *after* unwrapping, returning the unwrapped intermediate would silently strip a user's existing links. Decoration is now explicitly all-or-nothing per message.

Quality fixes: ignore a SiteURL path that is not rooted rather than writing a relative prefix into permanent text; make `Cache-Control` decorator-owned instead of a framework-wide `public`; key CSS and `closest()` on the full `/plugins/<id>/decorate/<type>?` prefix from one shared helper; and mark the root-relative URL choice provisional until Task 0b validates it.

Rejected: Codex's third blocker (hook ordering) was downgraded, since its two concrete requirements fall out of the tightened unwrap rule, leaving a compatibility stance plus an idempotence test. Gemini's claim that the **desktop** app mishandles root-relative links was dropped: the desktop app is Electron wrapping the same webapp bundle, so the click handler intercepts and it never navigates. Gemini's request to identify a search fallback was already satisfied by Task 0.

### Revision 2a: first technical review applied

Reviewed by Codex (`gpt-5.5`) and Gemini in technical-feasibility mode. seq-server was not run; the review was cut short to deal with the Gemini CLI having written an unrequested implementation into the working tree (parked on the `scratch/gemini-autogen` branch).

Four blockers fixed:

1. **`/decorate/*` auth was undefined.** Both models flagged it independently. `Mattermost-User-Id` is set only for authenticated requests, so the cookie-less mobile and email clients this page exists for would have hit a login screen. Now explicitly public, with the "pure function of its query string" constraint documented on the `Decorator` interface so a future decorator cannot quietly break it.
2. **No `tzdata`.** Both models. `time.LoadLocation` needs the host IANA database, which minimal images lack. Now an explicit `import _ "time/tzdata"` with a test, and noted as a failure mode that hides from a casual smoke test because UTC keeps working.
3. **The search check was a task, not a gate.** Both models. Promoted to Task 0 with named fallbacks, because a failure there invalidates server-side rewriting rather than costing a fix.
4. **Edits could desync a label from its `t`.** Codex only. Revision 2's "our own links are a protected range" rule was the cause: a hand-edited label kept stale params, so the link text and the panel would disagree. The tagger now unwraps its own links before re-decorating.

Quality fixes: return `nil` rather than the original post when nothing changed (the documented hook contract), narrow the `post.Type` skip from "any non-empty" to a `system_*` deny list, and stop claiming slash-command responses are decorated without testing it.

Rejected: HMAC-signed page URLs (over-engineering for a timezone table, and it would break the email case it was meant to serve; Codex independently agreed), and an i18n shim ahead of any i18n framework.

### Revision 2: server-side decoration

Changed at the user's direction so the decoration reaches the mobile app. The link, with the parsed data in its query string, now lives in the stored message rather than being generated per-render in the webapp.

Consequences, all deliberate:

- **Parsing moved entirely to Go.** The webapp no longer has patterns, a grammar, or a `parse` function; it reads query params. This removes the Go/TypeScript sync hazard by construction.
- **The URL carries the resolved instant (`t`)**, not the component fields, so the RHS panel and the server page cannot disagree about what a DTG means.
- **The fallback page is back, and is now genuinely reachable.** Revision 1 cut it because client-side decoration made it unreachable; that reasoning no longer applies.
- **`registerMessageWillFormatHook` is gone.** So is the memoization hazard that dominated revision 1: decoration happens once, at post time, so the reference time is frozen by construction rather than by careful design.
- **Type moved from `?type=` to a path segment**, which incidentally kills the `type=dtg2` prefix-collision problem.
- **Modified clicks and unknown types are no longer intercepted**, reversing revision 1, because the URL is now a real destination.
- **New risks accepted**: stored post text is rewritten permanently, history before install is undecorated, editing shows raw markdown, and uninstalling leaves 404 links. Confirmed by the user.
- **New risks to verify early**: whether search still matches a decorated DTG, and whether `MessageWillBePosted` fires for plugin and webhook posts. Both are called out as blocking checks in task 7 rather than late surprises.
- **Added an admin kill switch**, which was unnecessary when decoration was a per-render client concern.

### Revision 1: client-side decoration

Reviewed by Gemini (spec), `design-flaw-finder`, and `simplicity-reviewer`. Codex was unavailable (`gpt-5.3-codex` and `gpt-5.2-codex` both rejected by this ChatGPT-account key).

Blockers those reviews resolved, most of which still apply to revision 2:

1. **Day validation**: `01-31` silently rolled `31 Feb` into `3 Mar`. Now day-in-month with leap years.
2. **Over-wide pattern surface**: bare `\b\d{6}[A-Z]\b` mis-tags `091630JUL26` and ordinary serials. Narrowed; `J` rejected.
3. **Hidden global coupling**: registration order as sole priority meant adding a decorator could silently change an existing one. Now longest-match-wins.
4. **Rejected matches claiming ranges**: would suppress a valid shorter match at the same span. Now they do not claim.
5. **Undefined states**: `RhsTitle` with a null selection, `EmptyState` content, no route back from a panel, channel switch, listener leak on reload. All specified.
6. **Label rewriting**: broke copy-paste fidelity for operators. Label is now the original text.
7. **Contradictions**: "zero framework edits" vs `index.ts`; "click ignored" vs "shows empty state". Reconciled.

Superseded by revision 2: the memoization analysis, the client-side `parse` purity requirement, the CSS-only per-user toggle (which was broken three ways and is now replaced by an admin setting), and the argument for cutting the server page.

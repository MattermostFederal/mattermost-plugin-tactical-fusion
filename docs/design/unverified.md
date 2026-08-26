# Unverified before deployment

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

Prerequisites that need a running server and have **not** been checked.

### Cursor on Target

**The `<detail>` extension shapes.** `docs/design/cot.md` carries the provenance
table. Three rows in it rest on nothing citable and render as the stated string
rather than as a decode: `precisionlocation/@pdop`, `@hdop` and `@vdop`;
`status/@readiness`; and `_medevac_/@security`, whose numeric 9-line vocabulary
is plausible and uncited. None has been seen against a real emitter.

**The geometry shapes are convention, and the failure is visible.** `shape`,
`polyline`, `vertex`, `ellipse` and a route's `link/@point` are all read as ATAK
is understood to write them and none has been checked against a real emitter. If
a shape is wrong the geometry does not draw and `detail_unknown` counts the
element, which is what an unrecognised element already does, so the failure
surfaces rather than producing a wrong outline. The reading most likely to be
wrong is the ellipse's: whether `major`/`minor` are semi-axes or full axes, and
whether `angle` is measured clockwise from north.

**`__network` was suggested and not added.** The Phase 1 domain review said
`__network` is ATAK's standard element for device telemetry and that `_radio` is
specific to certain radio integrations. `_radio` has a source this repository
already cites for `Attitude` and `_medevac_`; `__network` has none that names
its attributes. Adding it would have meant inventing attribute names, which
produces rows that never populate while claiming knowledge this build does not
have. If a real event turns up carrying one, the registry row is four lines.

**No real ATAK output has been through the registry.** Every fixture in the test
suite is hand-written or built from the registry itself, so the parser has never
met an event a device actually produced. The shapes most likely to be wrong are
the ones with the most attributes: `_medevac_` and `sensor`.

- **Does `plugin.API.GetFile` succeed inside `MessageWillBePosted`**, before the
  file is attached to the post? Expected yes: the upload has already written the
  `FileInfo` and the filestore object, and only `PostId` is unset. If it does
  not, the file path has to move to `MessageHasBeenPosted` plus `UpdatePost`,
  which is a visible edit and a different design. The fenced-block path does no
  file IO and is unaffected either way.
- **Does an ephemeral sent from inside `MessageWillBePosted` arrive after the
  post it is about?** The refusal notice is worded to stand alone either way,
  since it carries no `PostId` and cannot point at anything, but if it arrives
  first it will read oddly.
- **Does `ShowMore` clamp a Cursor on Target card at 600px?**
  `FULL_HEIGHT_POST_TYPES` is a one-entry allowlist for `custom_spillage_report`
  rather than a `custom_` prefix rule, so it probably does. The card is taller
  than a coordinate-only post, so decide whether it fits.

### From the original plan


- Whether Mattermost's search still matches a DTG once the message contains
  `[091630ZAUG26](/plugins/...)`. If the indexer splits on brackets, decorated
  posts become unfindable and the server-side approach needs rethinking.
- Whether the mobile app resolves root-relative markdown links against the
  server URL, **and whether its in-app browser carries the session**. This one
  got more expensive when `/decorate` and `/map` were gated: the whole argument
  for the route being public was that this client has no session, and nobody has
  ever checked. If it does not, every decorated link on mobile costs a sign-in
  before the page. Answer it with a phone and record both halves here.

  **The mobile source points the opposite way from what that assumed**, and this
  is worth chasing rather than leaving as a note. `openLink` runs
  `normalizeProtocol`, which returns a string with no `:` unchanged, then
  `matchDeepLink`, whose `parseDeepLink` recognizes only channels, DMs, `/pl/`
  permalinks, playbooks and magic links. `/plugins/…` matches none of those, so
  the bare relative string reaches `Linking.openURL` and the failure path shows
  "Unable to open the link." The code path is read from source; whether
  `Linking.openURL` actually rejects it at the OS level is not, and needs a
  phone. If it does, **every decorated link on mobile is dead today**, which is a
  larger problem than any one feature and should be its own piece of work.

Also unverified: which post sources actually reach `MessageWillBePosted`
(`p.API.CreatePost`, incoming webhooks, bot posts, `in_channel` command
responses). Record the real behavior here once tested.

And whether a plugin-rendered anchor still collects a
`registerLinkTooltipComponent` hover. See "The map under a post": it is expected
not to, which costs a coordinate-only post its hover card and nothing else.


**No checklist has been through this, and it may never be.** The `<checklist>`
counter was written against a schema no accessible source defines. The TAK
feature is ExCheck, whose templates travel as uploaded XML files with their own
REST API rather than as `<detail>` children, so it is an open question whether a
CoT event carries a `<checklist>` inline at all. Nothing here decodes an
attribute, so the failure mode if the guess is wrong is a count of elements that
are really something else, under names the event supplied. See "Checklists" in
[`cot.md`](cot.md).


**The sidebar scroll fix is only provable on a running server.** `RhsView`'s
container gained `height: 100%`, `flex: 1 1 auto`, `minHeight: 0` and
`overflowY: auto`, because Mattermost renders a plugin's RHS component inside
`.sidebar-right__body`, a flex column that clips rather than scrolls: a panel
taller than the sidebar had everything past the first screen unreachable, which
is why it surfaced first on a two-event Cursor on Target post. The component
tests mount the panel in a bare div and render none of that chrome, so they
cannot see the bug or the fix. Verify on a real install, in both themes and with
a short window, and record the result here.


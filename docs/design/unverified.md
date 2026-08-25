# Unverified before deployment

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

Prerequisites that need a running server and have **not** been checked.

### Cursor on Target

**The `<detail>` extension shapes.** `docs/design/cot.md` carries the provenance
table. Three rows in it rest on nothing citable and render as the stated string
rather than as a decode: `precisionlocation/@pdop`, `@hdop` and `@vdop`;
`status/@readiness`; and `_medevac_/@security`, whose numeric 9-line vocabulary
is plausible and uncited. None has been seen against a real emitter.

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


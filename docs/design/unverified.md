# Unverified before deployment

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

### Unverified before deployment

Two prerequisites from the implementation plan need a running server and have
**not** been checked:

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


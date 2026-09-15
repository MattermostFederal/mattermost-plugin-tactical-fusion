# The plugin bridge

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

Another plugin can ask Tactical Fusion for the exact markdown a decorator would
write, and embed it in whatever that plugin outputs: a bot post, an ephemeral
reply, a dialog, or a React surface in its own webapp. The reader-facing contract
is `public/help/integration.html`; the Go caller's is `bridgeclient/README.md`.

## Two operations, two transports, one implementation

`decorate` runs the tagger over a whole message. `link` builds one link for a
token whose type the caller already knows. Both live in `server/bridge.go` as
`decorateText` and `buildLink`, and both transports call those two functions and
nothing else:

| Caller | Route | Gate |
|---|---|---|
| Another plugin's server, through `PluginHTTP` | `/bridge/v1/{decorate,link,info}` | `Mattermost-Plugin-ID` present |
| Another plugin's webapp, through `window.TacticalFusion` | `/api/v1/{decorate,link}` | the existing session gate |

Neither transport has a decoration rule of its own. `decorate` is `Tagger.Decorate`,
the same call `/tactical-fusion check` makes, so protected spans, boundary guards,
labels and idempotence come with it rather than being reimplemented. `link` is the
decorator's own `Parse` plus `Tagger.LinkFor`, and `LinkFor` is what
`applyReplacements` calls for every link the post hook writes, so the label
escaping and URL building cannot differ between a posted message and a bridge
answer. `TestBridgeLinkIsTheLinkTheTaggerWrites` holds that for every type.

`info` is bridge-only. It is feature detection for a server caller deciding
whether to depend on a type, and a webapp already has `window.TacticalFusion.types`.

## Why `PluginHTTP`

Mattermost has no RPC hook between plugins. What it provides is
`plugin.API.PluginHTTP` (server 5.18+), which hands a request to another plugin's
`ServeHTTP` and streams the response back. The Agents plugin's LLM Bridge is built
the same way, with a `/bridge/v1` route group and an importable `bridgeclient`
package, and copying that shape means a plugin author who has used one already
knows the other.

## Why `Mattermost-Plugin-ID` can be trusted

`ServeInterPluginRequest` (`channels/app/plugin_requests.go`) sets
`Mattermost-Plugin-ID` to the calling plugin's id, and the handler for external
`/plugins/{id}/...` requests deletes both that header and `Mattermost-User-Id`
before a plugin sees the request. So a browser cannot forge it, and a request
carrying it came from a plugin on this server. That is the entire gate, and it is
the same gate the Agents plugin uses.

What the gate does not say is **who the reader is**: a plugin-to-plugin request
carries no user. That is acceptable only because every bridge answer is a pure
function of the request and the admin switches, with no per-user data, no KV read
and no post lookup. **Never add a route under `/bridge/v1` that reads per-user or
per-channel state**; that one needs a user id the caller cannot forge, which this
gate does not provide.

The bridge branch in `ServeHTTP` sits before the method check and the session
gate for the same reason `/api/v1` does: the page routes redirect a sessionless
request to a login page, which is meaningless to a plugin, and the API refuses a
request without a user, which a plugin never has.

## Why `link` takes no field label

In a posted message `ICAO:`, `MGRS:` and `GEOREF:` exist to stop false positives:
four letters alone are prose. A bridge caller has named the type explicitly, which
is a stronger signal than any label, so `link` hands the bare token straight to
`Parse`. A token still carrying its label is `not_recognized`, and the docs say to
strip it.

Every other rule `Parse` applies still holds: the J zone is declined, the location
decorator validates the params through the same `validateParams` its page uses, and
an airfield must be in the database. `TestBridgeLinkOpensAPageThatRenders` follows
each link to its page and requires a 200, which is the invariant that matters
most: a link the bridge hands out is embedded permanently in someone else's output.

## Why the admin switches still apply

A format switch governs decoration, and a bridge caller is decorating. An admin who
turned off UTM because of its band-letter ambiguity turned it off for every writer,
including another plugin. So `decorate` uses the plugin's own registry, whose
patterns read the switches, and `link` reads them too.

`link` needs more than `Parse` for that, because not every decorator's `Parse`
consults its switches. Location's does; DTG's and the airfield's do not, since the
tagger gates those in `Patterns()` and `Parse` only ever sees what a pattern
matched. `formatEnabled` fills the gap: DTG checks `Timestamp` when the params
carry `o` and `Military` otherwise, and the airfield checks `Airfield`.

`disabled` is distinguished from `not_recognized` by parsing again with a
zero-config decorator, which reads every format. A caller can then tell the author
"your admin has this off" rather than "that is not a coordinate", and no second
registry has to be kept in step with the first.

Links already handed out keep working after a switch is turned off, exactly as
links in messages do, because the page routes do not read format switches.

## Size

`decorate` reports `fits_post` against `safePostRunes`, the 4,000 rune floor, and
does not truncate or skip. A bridge caller may not be writing a post at all, so
refusing would be wrong, and the post size limit cannot be read (see
[`decorators.md`](decorators.md)), so the floor is the only honest number to report.
Bodies are capped at 64 KiB, which is well above the largest post any server
accepts.

## A caller's post and our hook

A plugin that creates a post from bridge output sends it through
`MessageWillBePosted` like any other post. Nothing is linked twice, because a
decorator link is a protected span and `Decorate` is idempotent.
`TestBridgeDecorateIsIdempotent` pins that.

## Panics

The bridge recovers a panic in either operation, answers `TF-19008`, and logs
through an API handle captured before the deferred call, the same discipline as
`decorateMessage`. Plugin RPC serves a request on a goroutine, so an unrecovered
panic there takes the whole plugin process down, and message decoration with it.

## The webapp half

Mattermost gives plugin webapps no registry for sharing functions with each other,
so the API is a global: `window.TacticalFusion`, frozen, installed in `initialize()`
and withdrawn by its disposer. Plugins initialize in no defined order, so a
`tactical-fusion:ready` event is dispatched on install. A host **attaches its
listener first and re-checks the global second**. The event is dispatched
synchronously inside `installBridgeGlobal`, so a host that checked first could see
no global during its render, have Tactical Fusion install before its effect
attached the listener, and miss the event for that installation, staying on its
plain-text fallback until Tactical Fusion installs again, which in a live tab means
a plugin upgrade or re-enable dispatching a fresh event. The help page's `useTacticalFusion`
example is written in that order for that reason.

The disposer deletes the global only if it is still the one it installed, so an
upgrade in a live tab, which runs the new `initialize()` before the old
`uninitialize()` has necessarily finished, cannot leave the page with none.

`Link` renders through `HoverLink`, so a host gets the hover card without
`registerLinkTooltipComponent` ever seeing the anchor (see "A hover card is sized by
its content" in [`decorators.md`](decorators.md)), and the capture-phase click
handler opens the sidebar for any anchor under `/decorate/` anywhere in the
document. Our bundle's `react` is an external, the same `window.React` the host
uses, so the component mounts inside another plugin's tree without a second React.

`link` caches per type, token, label and reference time for five minutes, bounded
at 500 entries, because a host rendering a list mounts one `Link` per row. A
decline is cached, since an unreadable token does not become readable in five
minutes; a transport failure is not. A caller passing an `AbortSignal` bypasses
the cache, because a shared promise that one caller aborts would reject for all of
them.

## Where the wire types live

In `bridgeclient`, and the server imports them. A Go caller and the server then
compile against the same structs and cannot drift. The TypeScript copies in
`webapp/src/bridge/types.ts` are held to them by `TestWebappBridgeShapeMatches`,
on names, types, optionality and order, and the decline reasons by
`TestWebappBridgeDeclineReasonsMatch`.

`bridgeclient` imports nothing but the standard library, so a caller inherits no
dependency from it. It does inherit this module's `go` directive, which the README
says.

## Versioning

The path carries the version. Within `/bridge/v1` changes are additive only: new
optional request fields, new response fields, new types, new reasons. Removing or
renaming anything is `/bridge/v2`, served beside v1 rather than replacing it,
because the callers are other teams' plugins upgraded on their own schedule.

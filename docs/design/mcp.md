# The Agents MCP server

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

The [Mattermost Agents plugin](https://github.com/mattermost/mattermost-plugin-agents)
can discover and call this plugin's operations as MCP tools, so an LLM agent
answering in a channel can decorate a message, build one link, convert a
coordinate or look up an airfield. The endpoint is `/mcp`, and
`server/mcp.go` owns the lifecycle while `server/mcp_tools.go` owns the tools.

## A third transport, not a route under `/bridge/v1`

`decorate` and `link` already have two transports, and
[`bridge.md`](bridge.md) argues that neither owns a decoration rule of its own.
MCP is the third, and the same rule holds: the transport decides who may ask,
never what the answer is built from.

| Caller | Route | Gate |
|---|---|---|
| Another plugin's server, through `PluginHTTP` | `/bridge/v1/{decorate,link,info}` | `Mattermost-Plugin-ID` present |
| Another plugin's webapp, through `window.TacticalFusion` | `/api/v1/{decorate,link}` | the existing session gate |
| The Agents plugin's LLM agents | `/mcp` | `Mattermost-Plugin-ID` equals `mattermost-ai` |

It is not a route under `/bridge/v1` for three reasons.

The wire protocol is not ours. MCP is JSON-RPC over a streamable HTTP handler
that the go-sdk owns, with `initialize`, `tools/list` and `tools/call` on one
path. `/bridge/v1` is a hand-written path-per-operation JSON API whose shape is
a contract with other teams' plugins, and additive-only within `v1`. Serving a
JSON-RPC envelope from inside that would make the version in the path mean two
different things.

The gate is narrower. `/bridge/v1` admits any plugin that can set
`Mattermost-Plugin-ID`. `pluginmcp.Server.ServeHTTP` admits only
`mattermost-ai`. Widening the bridge gate to let the MCP handler sit under it
would have been a loss, and narrowing one route inside a group whose whole
argument is a single gate would have been a second rule to remember.

The reader rule is the opposite one. See below.

## The user-scoping rule, which is the one real exception

`bridge.md` states the invariant plainly: **never add a route under
`/bridge/v1` that reads per-user or per-channel state**, because a
plugin-to-plugin request carries no reader and the gate cannot prove one.

MCP is the deliberate exception. Agents propagates the acting user in
`X-Mattermost-UserID`, and `pluginmcp.Server.ServeHTTP` reads it **after** the
plugin-id check and stashes it in the request context under an unexported key,
so no external caller and no other package can inject one.
`pluginmcp.GetUserID(ctx)` is the only trustworthy way to read it back.

A tool that wants a reader:

- **must** go through `pluginmcp.GetUserID(ctx)`, and
- **must** refuse when it returns `""`, rather than falling back to anything.

Never read `X-Mattermost-UserID` from the headers directly. The header is only
trustworthy inside a request that arrived through
`pluginmcp.Server.ServeHTTP`, and a handler that reads it itself would also
read a copy an external caller set on a request that reached some other route.

None of the four tools shipped today reads a reader. All four are pure
functions of their arguments and the admin switches, exactly like the bridge's,
and `TestEveryMCPToolAnswersWithoutAReader` reads the registry and calls every
one of them with no user id to hold that true. That test is what a
user-scoped tool has to change deliberately rather than quietly.

## The tools, and why they call the existing operations

`decorate_text` and `link_token` call `decorateText` and `buildLink` in
`server/bridge.go`, the same two functions both existing transports call and
the ones `TestBridgeLinkIsTheLinkTheTaggerWrites` holds to what the post hook
writes. `convert_coordinate` calls `location.Convert`, and `lookup_airfield`
calls `describeAirport`, which was lifted out of `serveAirport` so the HTTP
handler and the tool share one copy rather than growing a third that can drift.

The outputs are the existing structs, not new ones:
`bridgeclient.DecorateResponse`, `bridgeclient.LinkResponse`,
`location.Conversion` and `api.go`'s `airportResponse`. Reusing them means the
MCP schema cannot disagree with the JSON the other two transports serve, and it
means `TestWebappAirportShapeMatches` and the other sync tests keep governing
one shape instead of two. The cost is that a field renamed for the webapp also
renames in the MCP schema, which is the right direction for that coupling to
run: one truth, held by a test.

`decorate_text` carries `fits_post` through rather than dropping it. An agent
that pastes a decorated message into a post needs to know it will fit, and
`safePostRunes` is the only honest number available because the real limit
cannot be read from the API (see [`decorators.md`](decorators.md)).

**Format switches still govern decoration.** `link_token` refuses a token whose
format an admin turned off, because a bridge caller is decorating and so is an
agent. `lookup_airfield` does not consult `EnableAirport`, for the same reason
`/api/v1/airport` does not: a format switch governs decoration only, and a link
written while the decorator was on must keep resolving afterwards. Two tests
pin the pair in opposite directions.

### The tool budget

About ten tools, maximum. Every registered tool costs roughly 20 to 200 schema
tokens in **every** LLM request Agents makes, whether or not the model calls it,
so a narrow tool is not free and a plugin that registers thirty of them is
taxing every unrelated conversation on the server. Prefer union-typed arguments
over many near-duplicate tools.
`TestMCPExposesEveryToolWithinTheBudget` reads the registry and fails at
eleven, so growing past it is a decision somebody makes on purpose.

## Why registration failure is a warn, not a failed activation

`Register()` starts a goroutine and returns immediately, retrying on 1s, 2s, 4s
and 8s for up to 15 attempts until Agents acknowledges. A down, disabled or
not-yet-started Agents plugin must never keep Tactical Fusion from activating:
decoration, the pages, the slash command and both other transports have nothing
to do with MCP, and failing activation over an optional integration would take
all of them out.

So `registerMCPServerBestEffort` logs `TF-20002` and returns. Note that
`Register()` in `pluginmcp` today **always returns nil**, and reports its own
terminal failures through the standard library logger rather than to us
(`registration with Agents plugin failed permanently`, or `gave up after N
attempts`). The error check is kept because the exported signature returns an
error and a later version may use it; it is not currently reachable, and
`TestMCPRegistrationFailureIsAWarnRatherThanAFailedActivation` exercises it
through the `mcpRegister` seam rather than by pretending otherwise.

`ensureMCPServer` is separate from registration and its failure **is** fatal
(`TF-20000`), because the only way it fails is an incomplete manifest, which
means the build is broken rather than the environment.

`OnDeactivate` exists only to unregister, and is unconditional so a failure
elsewhere cannot leave Agents holding a registration for a plugin that is gone.
It warns on failure (`TF-20003`) rather than refusing to shut down.

## Routing, and the two orderings that are load-bearing

`serveMCPIfMatch` is called in `ServeHTTP` **after** the `/bridge/v1` prefix and
**before** the `r.Method != http.MethodGet` refusal and the session gate. Both
positions are defects waiting to happen if moved:

- MCP is a POST transport, so a match below the method check is answered 405.
- A plugin request carries no session, so a match below the session gate is
  answered with a redirect to a login page, which an MCP client reads as a
  malformed response rather than an auth failure.

`TestMCPRouteIsMatchedBeforeTheMethodCheck` and
`TestMCPRouteIsMatchedAboveTheSessionGate` pin them separately, because one
position could be fixed while the other regressed.

There is **no auth gate of our own** around `serveMCPIfMatch`.
`pluginmcp.Server.ServeHTTP` already refuses anything whose
`Mattermost-Plugin-ID` is not `mattermost-ai`, and Mattermost strips that header
from external requests, so the trust argument is exactly the one `callingPluginID`
rests on in `bridge.go`. A second check here would be redundant and would most
likely break the helper's.

The readiness refusal mirrors `serveBridgeOperation`: no server, no registry or
no configuration is `TF-20004` and a 503. A tool answering off a nil registry
is the same defect on a different transport.

## No admin switch

MCP ships on, with no `EnableMCP`, for the reason the bridge has no switch: it
answers only what the format switches already allow, and every tool is a pure
function of the request and those switches.

Worth knowing if that is ever revisited: a switch read at registration time
would not behave like a format switch. Turning it off would not retract a
registration Agents already holds until this plugin re-activates, because
registration is one-shot per `OnActivate`. Any switch added here has to say so
wherever it is documented.

## The dependency, and its license

`github.com/mattermost/mattermost-plugin-agents/v2/external/pluginmcp`, pinned
at `v2.7.0`. Two things about it that cost time to establish:

**The module path carries `/v2`.** The import is
`.../mattermost-plugin-agents/v2/external/pluginmcp`, not the unversioned path.
The unversioned path still resolves on the module proxy, but its latest tag
(`v1.14.2`) declares `module github.com/mattermost/mattermost-plugin-ai` and
ships no `external/` directory at all, so it cannot satisfy this import.

**The module is licensed at two levels.** Its root `LICENSE.txt` is Apache 2.0,
which `.licenses.json` allows. Its `enterprise/` subdirectory carries the
Mattermost Source Available License, which forbids distribution without an
Enterprise subscription and which the policy does not list. That is not a
problem here, and the reasoning is what matters if a later change makes it one:

- `external/pluginmcp` imports nothing but the standard library and
  `github.com/modelcontextprotocol/go-sdk/mcp`. It does not reach `enterprise/`,
  so no source-available code is compiled into the bundle.
- `licensecheck` filters the SBOM to the modules `go list -deps ./server/...`
  says are linked, then reads each one's license at module granularity, where
  this module reports Apache 2.0.
- `build/notices` reads license files with `os.ReadDir` at the module root only,
  not recursively, so `THIRD-PARTY-NOTICES.txt` carries the Apache 2.0 text and
  cannot pick up the source-available text by accident.

**An import that reached any other package in that module would have to be
checked again on all three counts**, and one that reached `enterprise/` would
put non-distributable code in a bundle that ships to customers.

The transitive additions were all permissive (MIT or BSD):
`google/jsonschema-go`, `segmentio/asm`, `segmentio/encoding`,
`yosida95/uritemplate/v3`, `golang.org/x/oauth2`, `golang.org/x/sync`,
`golang.org/x/time` and `gopkg.in/asn1-ber.v1`. Adding the module also bumped
`github.com/mattermost/ldap` to `v3.0.4+incompatible`.

`go-sdk` is pinned to `v1.7.0`, matching what `agents` v2.7.0 requires rather
than the latest tag, so the two halves of the MCP stack agree. It is mid
transition from MIT to Apache 2.0 and both arms are allowed.

## Tool names are namespaced for us

`pluginmcp.AddTool` prepends `{sanitizedPluginID}__`, replacing every character
outside `[A-Za-z0-9_-]` with `_` to satisfy the `^[a-zA-Z0-9_-]{1,128}$` that
Anthropic and Bifrost enforce on tool names. So `decorate_text` is written here
and the model sees
`com_mattermost_plugin-tactical-fusion__decorate_text`. An already-prefixed name
is not prefixed twice. `TestMCPToolNamesCarryTheSanitizedPluginNamespace` reads
the registry rather than listing the names.

This is also why **`plugin.json`'s `id` may not contain `__`**: that is the
namespace separator on the Agents side.

## Panics

`guardTool` wraps every handler and recovers, logging `TF-20005` and answering
an error. Plugin RPC serves a request on a goroutine, so an unrecovered panic
takes the whole plugin process down and message decoration with it, which is
the same reasoning `serveBridgeOperation` and `decorateMessage` already rest on.
The API handle is captured before the deferred call, as it is there.

`guardTool` is a free function rather than a method because Go disallows type
parameters on methods, and the handler signature
(`mcp.ToolHandlerFor[In, Out]`) is generic in both directions.

## Testing without an Agents plugin

`mcpNewServer`, `mcpRegister` and `mcpUnregister` are package-level `var`s so a
test can substitute them. Tests that want the real thing go through
`httptest.Server` and the go-sdk client with `Mattermost-Plugin-ID:
mattermost-ai` set by a `RoundTripper`, which is the only way to exercise
`tools/list` and `tools/call` as Agents will.

`fakeAPI.PluginHTTP` answers 200, which matters more than it looks: without it
the registration goroutine `OnActivate` starts would call a nil embedded
`plugin.API` and panic on a goroutine, taking the whole test binary down. It
answers 200 rather than 404 so the goroutine finishes on its first attempt
instead of living through 15 backoff rounds, and its call log is mutex-guarded
because `-race` is on by default and that goroutine writes to it while the test
reads.

## No webapp half

MCP is server-only. None of the cross-language sync tests in
[CLAUDE.md](../../CLAUDE.md) apply, and there must be no `webapp/src/mcp/`
without a guard test to match it.

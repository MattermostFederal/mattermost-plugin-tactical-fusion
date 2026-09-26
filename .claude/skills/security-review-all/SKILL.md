---
name: security-review-all
description: Whole-product offensive security audit of Tactical Fusion. Hunts for exploitable holes across the server, the webapp, the standalone pages, the bridge, the MCP server, the data pipeline and the build, proves each one against the code, and reports them ranked by impact with a fix. Use for periodic audits, before a release, or when asked to find security holes. Read-only.
user-invocable: true
---

# Security Review

You are a god tier security researcher. Find security holes in this product so we
can fix them.

Think like an attacker with a Mattermost account, a malicious plugin installed
beside this one, a poisoned dataset, or a crafted message pasted into chat. The
goal is real, exploitable findings with a concrete path from attacker input to
impact, not a checklist of best practices.

This skill is read-only. Do not edit, commit or push. Do not run anything that
reaches the network or mutates the working tree.

## Usage

```
/security-review-all            # The whole product
/security-review-all <path>           # Focus on one file or directory
/security-review-all --diff           # Only what changed on this branch since the trunk
```

## Step 1: Map the attack surface

Read `CLAUDE.md` first. Its Invariants section is the product's own statement of
what must hold; every invariant is a hypothesis to break. Then enumerate entry
points and who controls each one:

| Surface | Attacker | Where |
|---|---|---|
| Posted message text | Any user who can post | `server/hooks*.go`, `server/decorators/**`, `server/avreport/`, `server/cot/`, `server/geojson/` |
| File attachments read by stampers | Any user who can post | `hooks_stamp.go`, `hooks_avreport.go` |
| Post props (forged by a client) | Any user with API access | `hooks_stamp.go` strip table, webapp readers in `webapp/src/{cot,geojson,avreport}/` |
| Decorator link query strings | Anyone who can craft a URL | `server/http.go`, `/decorate/<type>`, `/map`, `mapairport.go` |
| Authenticated JSON API | Any logged in user | `server/api.go` |
| Plugin bridge | Any other installed plugin | `server/bridge.go`, `bridgeclient/` |
| MCP tools | An LLM agent steered by prompt injection | `server/mcp*.go` |
| Slash command | Any user in any channel | `server/command*.go` |
| Preferences KV | The reader, other readers, cluster peers | `server/preferences*.go` |
| Cyber datasets on disk | Whoever controls the install's data directory | `server/cyberdata.go`, `server/decorators/cyber/intel/` |
| Webapp rendering | Authors of any post the reader views | `webapp/src/**`, `window.TacticalFusion` in `webapp/src/bridge/` |
| Standalone pages | Authors, via echoed text | `server/decorators/` page shell, `Page.Capability`, `src/page/` |
| Build and release | Upstream data sources and dependencies | `build/`, `Makefile`, `.github/workflows/`, `go.mod`, `webapp/package.json` |

## Step 2: Hunt

Run the lenses below as parallel read-only `Explore` or `general-purpose`
agents, one per lens, each told the surface table above, told to cite
`file:line`, and told to return candidates with an attack path rather than
general advice. With a path argument, keep only the lenses that touch it.

1. **Authorization and identity.** Every `/api/v1` route checks the session
   user. `/bridge/v1` never returns per-user or per-channel data. `/mcp` reads a
   reader only through `pluginmcp.GetUserID`, never `X-Mattermost-UserID`. Can a
   user read another user's preferences, a post in a channel they cannot see
   (`/map?post=`), or a file they have no access to through a stamper or the
   attachment ownership check? Confused deputy through the bridge or MCP.
2. **Injection into rendered output.** XSS in hover cards, the RHS, the
   standalone pages and `NoteMarkdown`. Every `dangerouslySetInnerHTML`,
   `innerHTML`, `href` built from data (`javascript:` URLs), `isWebURL` and
   `isWebLink`, `html/template` versus `text/template`, and anything that
   echoes author text under `PageMapping`, where escaping is the only defense.
   Markdown injection through the rewritten message: can a token make the
   decorator emit a link, image or mention the author did not write?
3. **CSP and page capability.** `Page.Capability` decides the whole CSP. Find a
   page that asks for more than it needs, a relative-path assumption in
   `ScriptSrc` that breaks, or a route that sets no CSP at all.
4. **Message corruption and denial of posting.** `findProtectedRanges` gaps that
   let a decorator rewrite inside code, links or mentions. Inputs that panic
   outside a recover, blow the post size, or make `MessageWillBePosted` slow
   (regex backtracking is impossible in RE2, but quadratic loops, huge
   allocations and unbounded XML or JSON walks are not). A forged props key
   that survives the strip.
5. **Parser abuse.** The CoT XML parse (entity expansion, depth, element
   counts), the GeoJSON walk (depth, coordinate counts, numbers like `1e999`),
   the METAR, TAF and NOTAM decoders, the frequency and coordinate grammars,
   `.tsv.gz` unpacking in `intel/` (decompression bombs, path traversal on the
   file written beside itself), and every cap in `TestWebappBlobCapsMatch`
   checked on both sides.
6. **Webapp trust of props and API data.** The webapp's second gate (`styleOf`,
   readers in `cot/`, `geojson/`, `avreport/`) must reject a forged blob. Look
   for prototype pollution, unbounded arrays, and MapLibre expressions built
   from author data.
7. **SSRF, files and paths.** Any server-side fetch, filestore read, or path
   built from input. The basemap and package routes, `data-packages`, the
   `maplibre-<hash>/` directory, and anything under `/static/plugins/**`.
8. **LLM and MCP abuse.** Tool output that carries attacker text back into the
   agent unmarked, tools that write or post, argument sizes, and whether a
   prompt-injected agent can use a tool to exfiltrate or to act as the user.
9. **Cluster and cache.** `OnPluginClusterEvent` payloads trusted without
   checks, preference cache poisoning across users, and TTL mismatches that
   serve one user's data to another.
10. **Supply chain and release.** Unpinned Actions, workflow `pull_request_target`
    or script injection from PR titles, secrets exposed to forks, data fetched
    over HTTP or without a digest, copyleft or vulnerable dependencies, and
    anything the virus allowlist or `.grype.yaml` suppresses without a real
    reason.

Also check error paths: every failure carries a `TF-NNNN`, but does any leak a
path, a stack, a token or another user's data?

## Step 3: Prove it

A finding without proof is a guess. For each candidate:

- Trace the data flow from the attacker's input to the sink, reading the actual
  code at every hop. Drop it if a check anywhere on the path stops it.
- Name the attacker's required position (anonymous, any user, channel member,
  another plugin, admin, filesystem access). Admin-only and filesystem-only
  issues are real but rank lower.
- Where it is cheap and safe, write a failing test in the scratchpad or run the
  relevant existing test with a crafted input (`go test ./server/... -run`)
  to confirm. Never leave the working tree changed.
- Mark each finding CONFIRMED (proven or traced end to end) or PLAUSIBLE
  (traced, but depends on something you could not verify).

Be adversarial with yourself. Discard theoretical issues, defense-in-depth
nits with no path to impact, and anything the product's invariants already
explain as deliberate.

## Step 4: Report

Rank by severity, then by confidence. For each finding give:

- **Title** and severity (Critical, High, Medium, Low)
- **Location:** `file:line`
- **Attacker:** who can trigger it
- **Attack:** the exact input and the steps
- **Impact:** what they get
- **Fix:** the minimal change, and the regression test that should hold it

End with a short list of the surfaces you reviewed and found clean, so the next
audit knows what was covered, and anything you could not assess. Offer to fix
the confirmed findings; do not fix them unasked.

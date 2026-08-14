# Releasing

Releases are automated with [release-please](https://github.com/googleapis/release-please).
You don't hand-edit version numbers or the changelog — you write
[Conventional Commits](https://www.conventionalcommits.org/), and the tooling
does the rest.

## How it works

```
commit to main (feat:/fix:/...) ──► release-please.yml opens/updates a "Release PR"
                                          │
                              merge the Release PR
                                          │
                     tag v0.2.0 is created ──► release.yml builds + publishes
```

1. **You commit** to `main` (via PRs) using Conventional Commit messages.
2. **`release-please.yml`** reads the commits since the last tag, works out the
   next semver bump, and keeps a **Release PR** open. That PR bumps
   `plugin.json`'s `version` and updates `CHANGELOG.md`.
3. **You merge the Release PR** when you're ready to ship. That creates the
   `v*` git tag.
4. **`release.yml`** fires on the tag, runs the full security-gated `make
   release`, and publishes a GitHub Release with the bundle, checksum, SBOMs,
   CodeQL SARIF, and (if configured) a GPG signature.

Nothing releases until you merge the Release PR — merging normal PRs to `main`
only updates the pending release, it doesn't ship.

## Commit message format

The prefix on the commit **subject** drives the version bump:

| Prefix | Example | Bump | Changelog section |
|--------|---------|------|-------------------|
| `feat:` | `feat: add channel export command` | minor (`0.1.0 → 0.2.0`) | Features |
| `fix:` | `fix: handle empty webhook payload` | patch (`0.1.0 → 0.1.1`) | Bug Fixes |
| `perf:` | `perf: cache config lookups` | patch | Performance |
| `deps:` | `deps(go): bump gorilla/mux` | patch | Dependencies |
| `feat!:` / `BREAKING CHANGE:` | `feat!: drop MM 10 support` | minor while pre-1.0 (`0.1.0 → 0.2.0`) | Features + ⚠ Breaking |
| `docs:` `chore:` `test:` `refactor:` `style:` `build:` `ci:` | `chore: tidy imports` | none | hidden |

Rules of thumb:

- Subject lines with **no** conventional prefix are left out of the changelog —
  use `feat:`/`fix:` deliberately.
- Squash-merging PRs? The **PR title** becomes the commit subject, so title PRs
  with a conventional prefix.
### The major version is bumped by a person, never by a commit

Pre-1.0 (`0.x`), breaking changes bump the **minor**, per semver's
initial-development clause. That is configured rather than assumed:
`bump-minor-pre-major` in `.release-please-config.json` is what makes it true.
Without it, release-please promotes `0.x` to `1.0.0` on the first `feat:`, which
is exactly what it proposed before this was set.

So while the version is `0.x`, **no commit message can produce a major bump**.
Not `feat!:`, not a `BREAKING CHANGE:` footer. Reaching `1.0.0` means editing
this config or setting the version by hand, which is a decision somebody makes
on purpose and can be seen in a diff.

Know the limit of that guarantee: `bump-minor-pre-major` only governs `0.x`.
The day the version is `1.0.0`, a `feat!:` would take it to `2.0.0` on its own
again. Keeping major bumps manual past that point needs a further decision, and
the options are worse than this one: `"versioning": "always-bump-minor"` blocks
it but also turns every `fix:` into a minor bump, and the alternative is
process rather than configuration. Revisit it when 1.0.0 is actually in sight,
not before.

## Cutting a release (normal path)

1. Land your feature/fix PRs to `main` with conventional commit titles.
2. Find the open **"chore(main): release X.Y.Z"** PR that release-please keeps
   updated. Review its `CHANGELOG.md` and `plugin.json` diff.
3. Merge it. The tag and GitHub Release are created automatically.

## Cutting a release (manual fallback)

If you need to ship without release-please (hotfix, tooling outage):

```sh
# 1. Bump the version in plugin.json and add a CHANGELOG.md entry, then commit.
git commit -am "chore: release v0.2.0"
git push origin main

# 2. Tag and push — this fires release.yml.
git tag -a v0.2.0 -m "release v0.2.0"
git push origin v0.2.0
```

`make release-check` (run as the first step of `make release`) fails the build
if the working tree is dirty or `CHANGELOG.md` is missing, so commit everything
first.

## What ships in a release

`release.yml` runs `make release`, which produces and attaches:

- `<plugin-id>-<version>.tar.gz` — the plugin bundle (with `sbom/` and
  `security/` SARIF embedded inside it)
- `.sha256` checksum sidecar
- `.sig` detached GPG signature (only if a signing key is configured — see
  [SECURITY.md](SECURITY.md))
- `server-sbom.json` / `webapp-sbom.json` — CycloneDX SBOMs
- `codeql-go.sarif` / `codeql-js.sarif` — static-analysis results

## First release

The repo is seeded at `0.0.0` (`.release-please-manifest.json` and
`plugin.json`), meaning nothing has been released yet, so the first Release PR
is the one that publishes `0.1.0`. `.release-please-config.json` sets `bootstrap-sha` to the initial
scaffold commit so the first Release PR only considers commits made after the
scaffold. The first `feat:`/`fix:` commits drive the first real Release PR.

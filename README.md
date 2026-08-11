# Tactical Fusion

Tactical Fusion enriches conversations with mission-relevant context, including geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational information.

> **Status:** early. The decorator framework and the first decorator, date-time
> groups, are implemented. The remaining enrichment features described above are
> not.

Requires Mattermost **11.8.0** or later.

## What's included

- **Decorators**: military date-time groups (`091630ZAUG26`) and RFC 3339
  timestamps (`2026-08-09T16:30:00Z`) in posted messages become links. Hovering
  one shows a countdown, clicking one opens a timezone conversion panel in the
  right-hand sidebar, and clients that do not run the webapp get a
  plugin-rendered page instead. Run `/tactical-fusion examples` to see what is
  recognised and what is deliberately left alone.
- **Reader preferences**: "Customize your view", below the panel's timezone
  table, lets each reader choose their own timezone rows and how close a
  date-time group has to be before the countdown flashes. Stored per user and
  served from an authenticated `/api/v1/preferences` route.
- **Server**: Go plugin that decorates new messages, serves the decorator pages
  and the preferences API, and registers the `/tactical-fusion` slash command.
- **Webapp**: React plugin registering the right-hand sidebar, the link hover
  card, and the channel header button.
- **Admin settings**: four switches in the System Console, all defaulting to on:
  `EnableDTG` for the decorator, and `EnableDTGMilitary`, `EnableDTGMoniker` and
  `EnableDTGTimestamp` for the formats under it.
- **Built-in documentation**: static help pages bundled in `public/help/` and served
  by Mattermost at `/plugins/com.mattermost.plugin-tactical-fusion/public/help/help.html`.
  Linked from the System Console settings page and from the sidebar panel. Covers the
  supported formats, the panel, the admin switches, troubleshooting, and the `TF-NNNN`
  error code registry.
- **Build tooling**: Makefile, mattermost-plugin-starter-template build scripts, CI workflows.
- **CI/CD automation**: PR validation, security scanning (SBOM + Grype + CodeQL), automated releases via [release-please](https://github.com/googleapis/release-please), and Dependabot updates. See [Automation](#automation) below.
- **Editor integration**: `.claude/` (Claude Code agents, commands, skills) and `.vscode/` settings.

## How decoration works, and what it costs

Decoration happens on the server, in `MessageWillBePosted`, which is the only way
the link reaches clients that never run the webapp bundle. That has consequences
worth knowing before installing:

- **Stored post text is permanently rewritten.** Editing a post shows the author
  raw markdown, exports contain the links, and uninstalling the plugin leaves
  links that 404.
- **Only new posts are decorated.** History from before install is untouched.
- **Edits are stored verbatim.** Deleting the link syntax while editing is the
  supported way to opt one post out.
- Tokens inside code spans, code blocks, links and URLs are left exactly as
  written. Switching a format off stops new decoration and leaves every link
  already in the history working.

## Getting started

The repository ships a Docker Compose stack, which is what `make deploy` targets.

```sh
make docker-setup   # first run: start Mattermost and PostgreSQL, create admin and team
make deploy         # build the bundle, install it, and enable it
```

That serves Mattermost on `http://localhost:8065` with an `admin` / `password`
system admin account. Override the port with `MM_PORT`.

Useful alongside it: `make docker-logs`, `make docker-reset` (disable and
re-enable the plugin), `make docker-stop` (preserves data), `make docker-clean`
(removes data too).

### Deploying to your own server instead

```sh
make deploy-local
```

`deploy-local` targets `MM_LOCAL_SITEURL` (default `http://localhost:8065`) through
the bundled `pluginctl`, which authenticates via local mode (auto-detected socket,
or `MM_LOCALSOCKETPATH`), `MM_ADMIN_TOKEN`, or `MM_ADMIN_USERNAME` plus
`MM_ADMIN_PASSWORD`.

## Common commands

- `make dist` - build the plugin bundle
- `make check-style` - lint Go and webapp code
- `make test` - run tests
- `make coverage` - coverage summary for backend and frontend (`coverage-backend` and `coverage-frontend` individually)
- `make deploy` - build and deploy to the Docker Compose stack
- `make deploy-local` - build and deploy to the server at `MM_LOCAL_SITEURL`
- `make help` - every target with its description

## Automation

CI/CD is wired up for releases, security scanning, and dependency updates.

### CI workflows (`.github/workflows/`)

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `pr.yml` | PRs to `main` | Style checks, tests, and a bundle build |
| `security.yml` | PRs, push to `main`, weekly | SBOM + Grype CVE scan and CodeQL (Go + JS/TS), uploaded to Code Scanning |
| `release-please.yml` | push to `main` | Maintains the Release PR (version bump + changelog) |
| `release.yml` | `v*` tag | Full security-gated `make release` + publishes the GitHub Release |

### Releasing

Releases are automated with **release-please** driven by
[Conventional Commits](https://www.conventionalcommits.org/), so versions and the
changelog are not hand-edited. Commit `feat:`/`fix:` messages, merge the Release
PR release-please keeps open, and the tag + GitHub Release are created for you.

See **[docs/RELEASING.md](docs/RELEASING.md)** for the commit format, the bump
rules, and the manual fallback.

### Security tooling

An SBOM + CVE + static-analysis pipeline (CycloneDX, Grype, CodeQL, ClamAV,
optional GPG signing) is reproducible locally via `make`:

- `make sbom-audit` - generate SBOMs and fail on HIGH/CRITICAL CVEs
- `make codeql-analyze && make security-gate` - static analysis + finding gate
- `make release` - the full security-gated pipeline CI runs on a tag

Suppress false positives in `.grype.yaml` with a documented reason. Enabling
GitHub Code Scanning is required for the SARIF upload steps (free on public
repos; needs Advanced Security on private).

See **[docs/SECURITY.md](docs/SECURITY.md)** for the full checklist, the Code
Scanning requirement, suppression guidance, and signing setup.

### Dependency updates

Dependabot (`.github/dependabot.yml`) opens weekly PRs for Go modules, npm
packages, and GitHub Actions, with security updates firing immediately.

See the [Mattermost plugin developer docs](https://developers.mattermost.com/extend/plugins/) for more information.

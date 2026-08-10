# Mattermost Mission Context

Mattermost Mission Context enriches conversations with mission-relevant context, including geospatial data, CoT, time zones, IP intelligence, CVEs, and other operational information.

> **Status:** early scaffold. The server and webapp currently ship the starter
> `/mission-context hello` command and channel header button; the context
> enrichment features described above are not implemented yet.

## What's included

- **Server**: Go plugin registering a `/mission-context hello` slash command.
- **Webapp**: React plugin registering a channel header button.
- **Build tooling**: Makefile, mattermost-plugin-starter-template build scripts, CI workflows.
- **CI/CD automation**: PR validation, security scanning (SBOM + Grype + CodeQL), automated releases via [release-please](https://github.com/googleapis/release-please), and Dependabot updates. See [Automation](#automation) below.
- **Editor integration**: `.claude/` (Claude Code agents, commands, skills) and `.vscode/` settings.

## Getting started

Build and deploy against a running Mattermost server:

```sh
make deploy
```

`make deploy` uses `MM_SERVICESETTINGS_SITEURL` and `MM_ADMIN_TOKEN` to target the
server.

## Common commands

- `make dist` - build the plugin bundle
- `make check-style` - lint Go and webapp code
- `make test` - run tests
- `make deploy` - deploy to the Mattermost server specified by `MM_SERVICESETTINGS_SITEURL` and `MM_ADMIN_TOKEN`

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

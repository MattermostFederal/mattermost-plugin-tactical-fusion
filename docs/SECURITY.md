# Security

This plugin ships with a supply-chain and static-analysis pipeline wired into
both CI and the release build. Everything runs through `make`, so what CI checks
is exactly what you can reproduce locally — there are no CI-only gates.

GitHub also treats this file as the repository's security policy (it looks in
`docs/SECURITY.md`), so the reporting section below is what shows up under the
**Security** tab.

## Reporting a vulnerability

Do **not** open a public issue for a security vulnerability. Report it privately:

- Use **GitHub → Security → Report a vulnerability** (private vulnerability
  reporting) on this repo, or
- Email the maintainers listed in `plugin.json` (`support_url`).

Include repro steps, affected version, and impact. Expect an acknowledgement
within a few business days.

## What runs, and when

| Check | Make target | CI workflow | Gate |
|-------|-------------|-------------|------|
| SBOM generation (CycloneDX) | `make sbom` | `security.yml`, `release.yml` | — |
| Dependency CVE scan (Grype) | `make sbom-scan` | `security.yml`, `release.yml` | Fails on **HIGH/CRITICAL** |
| SBOM + scan in one step | `make sbom-audit` | — | Fails on **HIGH/CRITICAL** |
| Static analysis (CodeQL, Go + JS/TS) | `make codeql-analyze` | `security.yml`, `release.yml` | — |
| CodeQL finding gate | `make security-gate` | `security.yml`, `release.yml` | Fails on **error-level** findings |
| Malware scan of artifacts (ClamAV) | `make virus-scan` | `release.yml` | Fails on infected files |
| Bundled map data and font licence | `make bundle` | `pr.yml`, `release.yml` | Fails on a missing or malformed archive, or fonts without their notice |
| Detached GPG signature | `make release-sign` | `release.yml` | Skipped if no key configured |
| SHA256 checksum | `make release-checksum` | `release.yml` | — |

`security.yml` runs on every PR, every push to `main`, and weekly (to catch
newly-disclosed CVEs against unchanged code). `release.yml` runs the full chain
on every `v*` tag via `make release`.

SARIF from Grype and CodeQL is uploaded to **GitHub Code Scanning**, so findings
appear inline on PRs and in the Security tab.

## Requirement: enable Code Scanning

The SARIF upload steps are **visibility-aware** via
`continue-on-error: ${{ github.event.repository.private }}`:

- On **public** repos the upload is required — if Code Scanning is off it fails
  and the workflow goes red, so findings reach the Security tab.
- On **private** repos (where Code Scanning needs paid GitHub Advanced Security)
  the upload is tolerated — a missing GHAS license doesn't fail CI.

- **Public repos:** free. Enable under **Settings → Code security and analysis**.
- **Private repos:** requires GitHub Advanced Security (paid). If you can't
  enable it, **keep** the `upload-sarif` steps as-is — the `continue-on-error`
  above already tolerates the failed upload. The `make security-gate` / Grype
  `--fail-on high` gates remain the real enforcement either way.

## Running the checks locally

```sh
# Dependency CVEs (generates SBOMs, scans them, fails on HIGH/CRITICAL)
make sbom-audit

# Static analysis (downloads the CodeQL CLI bundle on first run, ~500MB)
make codeql-analyze
make security-gate

# Full release pipeline, exactly as CI runs it on a tag
make release
```

Tool installers are wired into the targets that need them (`install-sbom-tools`,
`install-grype`, `install-codeql`, `install-clamav`), so you don't install
anything by hand.

## Suppressing false positives (Grype)

Not every CVE applies to what actually ships. Common legitimate cases:

- A dev-only transitive dependency (eslint, babel, webpack) that never lands in
  `webapp/dist/main.js` or the server binary.
- A dependency Mattermost externalizes at runtime (react, redux), so the
  vulnerable copy in `node_modules` never ships in the bundle.

Add an entry to [`.grype.yaml`](../.grype.yaml) with a **reason** so the
suppression is auditable:

```yaml
ignore:
  - vulnerability: GHSA-xxxx-yyyy-zzzz
    package:
      name: some-package
      type: npm
    reason: "Dev-only transitive dependency not shipped in plugin bundle"
```

Keep suppressions specific (pin the CVE and package) and time-box them where you
can — re-check on the next dependency bump rather than suppressing forever.

**Neither case covers the map's dependencies.** `maplibre-gl`, `pmtiles` and its
transitive `fflate` all ship and all run in the reader's browser, so an advisory
against any of them has no legitimate suppression and the answer is to upgrade or
pin. If `pmtiles` ever becomes unfixable, the recorded way out is to replace it:
the archive is generated in a deliberately boring shape (spec v3, clustered, tile
type MVT) that a from-scratch reader can decode.

It no longer carries a single root directory. Extending the basemap to z8 spilled
it into leaf directories, which no flag prevents and which avoiding would mean
capping zoom, so a fallback reader now needs a second directory level and a cold
tile lookup can take two range requests. `TestArchiveIsClustered` and
`TestArchiveIsTheShapeTheReaderAssumes` assert what still holds.

## The bundled map data

`public/map/` ships two things the security pipeline treats differently from
code.

**`world.pmtiles`** is the basemap, built by `make map-tiles` from Natural Earth,
which is public domain. It is committed, so a change to it is a change in the
history rather than something a build produces unseen. It is **binary**, though,
so review means checking that the diff was expected and that it came from
`make map-tiles` over the pinned sources, not reading it. `make bundle` refuses
to assemble a bundle where it is missing or is not a PMTiles archive. That second check is not fussiness: a
truncated copy, an LFS pointer or an HTML error page saved over it all leave a
file of plausible size that fails only at runtime, in the reader's browser.

**`public/map/fonts/`** holds SDF glyph ranges generated from Noto Sans. These
carry a licence obligation rather than a vulnerability: SIL OFL 1.1 requires its
notice to travel with the font software, and ranges generated from a TTF are a
Modified Version of it. `make bundle` refuses to ship them without
`LICENSE.txt`. Noto declares no Reserved Font Name, which is what permits the
generated ranges to keep the `NotoSans-Regular` name.

Two limits worth stating plainly:

- **`public/` is served without authentication.** Mattermost serves the whole
  directory at `/plugins/<id>/public/**`, so the basemap and the fonts are
  retrievable by anyone who can reach the server. That is acceptable for what is
  there now, all of it public-domain or openly licensed reference data, and it is
  the constraint to weigh before putting anything else in that directory.
- **A hand-modified bundle is not the signed artifact.** `make release` runs
  `clamscan` over `dist/` and then signs and checksums it. An archive swapped
  into an already-built bundle is scanned by neither and matches neither the
  published `.sig` nor the `.sha256`. Rebuild from source with
  `make map-tiles && make dist` and re-sign locally instead.

## Signing releases (optional)

Release signing is off until you configure a key. To enable it, add two repo
secrets (**Settings → Secrets and variables → Actions**):

- `PLUGIN_SIGNING_KEY` — ASCII-armored private GPG key
- `PLUGIN_SIGNING_KEY_PASSPHRASE` — its passphrase

`release.yml` imports the key and `make release-sign` produces a detached
`.sig` attached to the GitHub Release. Consumers verify with:

```sh
gpg --verify <bundle>.tar.gz.sig <bundle>.tar.gz
```

## Dependency updates

Dependabot ([`.github/dependabot.yml`](../.github/dependabot.yml)) opens weekly
PRs for Go modules, npm packages, and GitHub Actions, and repo-level security
updates fire immediately when an advisory lands. GitHub Actions are pinned to
full commit SHAs — a moved tag can't swap the code — and Dependabot is what
keeps those pins current.

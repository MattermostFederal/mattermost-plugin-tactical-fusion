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

Include repro steps, affected version, and impact. Expect an acknowledgment
within a few business days.

## What runs, and when

| Check | Make target | CI workflow | Gate |
|-------|-------------|-------------|------|
| SBOM generation (CycloneDX) | `make sbom` | `security.yml`, `release.yml` | — |
| Dependency CVE scan (Grype) | `make sbom-scan` | `security.yml`, `release.yml` | Fails on **HIGH/CRITICAL** |
| Dependency license policy | `make license-check` | `security.yml`, `release.yml` | Fails on **copyleft, unstated or unrecognized** licenses in the shipped set |
| SBOM + scan + licenses in one step | `make sbom-audit` | — | Fails on any of the above |
| Static analysis (CodeQL, Go + JS/TS) | `make codeql-analyze` | `security.yml`, `release.yml` | — |
| CodeQL finding gate | `make security-gate` | `security.yml`, `release.yml` | Fails on **error-level** findings |
| Malware scan of artifacts (ClamAV) | `make virus-scan` | `release.yml` | Fails on infected files |
| Bundled map data and font license | `make bundle` | `pr.yml`, `release.yml` | Fails on a missing or malformed archive, or fonts without their notice |
| Third-party notices | `make bundle` | `pr.yml`, `release.yml` | Fails on a shipped component with no license text and no recorded fallback |
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
# Dependency CVEs and licenses (generates SBOMs, scans them, enforces the policy)
make sbom-audit

# Just the license policy, over SBOMs `make sbom` has already written
make sbom && make license-check

# Static analysis (downloads the CodeQL CLI bundle on first run, ~500MB)
make codeql-analyze
make security-gate

# Full release pipeline, exactly as CI runs it on a tag
make release
```

Tool installers are wired into the targets that need them (`install-sbom-tools`,
`install-grype`, `install-codeql`, `install-clamav`), so you don't install
anything by hand.

## Dependency licenses

The bundle ships to customers under a proprietary license, so copyleft in it is
a legal problem rather than untidiness. The policy is stated in `CLAUDE.md`
under "Dependency Licensing"; `.licenses.json` is what enforces it, and
`make license-check` is what runs the enforcement.

**Only what ships is judged.** Copyleft in the build toolchain is fine and there
is plenty of it: `lightningcss` is MPL-2.0 under `sass`, `caniuse-lite` is
CC-BY-4.0 under browserslist. None of it reaches a customer. The two halves are
narrowed differently:

- **Go**: `cyclonedx-gomod mod` lists the whole module graph, so the shipped set
  comes from `go list -deps` over `./server/...`, written to
  `dist/sbom/server-shipped.txt`, and the checker judges the intersection. That
  is the same question the compiler answers. `cyclonedx-gomod app` would give
  the same answer directly and is **not usable here**: it resolves the main
  module's version through git and hard fails with "reference not found" in a
  worktree or a shallow clone, which is both this checkout and every PR build.
- **npm**: a second SBOM, `webapp-runtime-sbom.json`, generated with
  `--omit dev`. The all-dependencies `webapp-sbom.json` stays for Grype, which
  should still see the toolchain.

**A detected license is not a declared one.** `cyclonedx-gomod` reports what it
detects under `components[].evidence.licenses` unless `-assert-licenses` moves
it to `components[].licenses`; the npm generator writes the declared field. The
Makefile passes `-assert-licenses` and the checker reads both shapes anyway,
because reading only one judged the entire Go tree as unstated.

**An unreadable license is forbidden**, per the policy. `github.com/tinylib/msgp`
is the one module whose LICENSE sits below the detector's 0.85 confidence, so it
carries an `UNDETECTED` exception recording that it was read by hand and is MIT.

### The MPL-2.0 exceptions

Five shipped Go modules are MPL-2.0, and all five are on the allowlist:
`hashicorp/go-plugin`, `yamux`, `errwrap`, `go-multierror` and `golang-lru/v2`.

MPL-2.0 is file-level copyleft. Section 3.3 explicitly permits distributing
covered files inside a "Larger Work" under other terms, including proprietary
ones; the obligation is that the license travel with those files and their
source stay available, not that our own code be disclosed. So shipping them is
lawful, and the exceptions record that decision rather than hide it.

Four of the five are not a choice: `go-plugin` is the RPC transport every
Mattermost plugin uses, reached through `mattermost/server/public`, and the
other three come with it. `golang-lru/v2` **is** a choice: it is a direct
require used in exactly one place, `expirable.LRU` in
`server/preferences_cache.go`, and roughly forty lines of stdlib would retire
the entry. Its exception says so.

### Adding or changing an exception

An exception names one component **and** one license. A module that relicenses
to something worse fails rather than staying quietly excused, and an exception
that stops matching anything is itself a failure, so the list cannot decay into
modules that left years ago. Every entry carries a `reason`, the same convention
`.grype.yaml` uses for CVE suppressions.

The deny list matches by substring so one entry covers a family: `GPL` catches
`GPL-2.0`, `GPL-3.0-only` and `GPL-3.0-or-later`. `LGPL` and `AGPL` are listed
separately because they do not share that stem. A disjunction like
`(MIT OR Apache-2.0)` passes on any allowed arm, since it is a choice the
consumer makes; a conjunction like `MIT AND GPL-3.0` binds you to both and is
judged on the worst arm.

### Third-party notices

The bundle carries `THIRD-PARTY-NOTICES.txt`, which reproduces the license and
copyright text of every dependency built into it: the Go modules linked into the
server binary and the npm packages bundled into the webapp. MIT and BSD require
their copyright notices be retained, Apache-2.0 requires NOTICE propagation, and
MPL-2.0 section 3.2 requires its text travel with the files it covers. The
bundle is distributed under a proprietary license, so all three attach to it.

`build/notices` generates it, and **`make bundle` runs it**, not
`release-bundle`. That placement is deliberate: every bundle carries the notices,
the same way the font and map notices already do, rather than only the tagged
release ones.

It reads the license files the dependencies themselves publish, from the module
cache and from `node_modules`. Three rules keep it honest:

- **Nothing is omitted silently.** A component with no `LICENSE`, `COPYING` or
  `NOTICE` file fails the build. Two npm packages are in that position today,
  `murmurhash-js` and `pmtiles`, and both are recorded under `noticeFallbacks`
  in `.licenses.json` with what their `package.json` actually declares plus a
  statement that upstream ships no file. No copyright line is invented.
- **A fallback that is no longer needed fails too**, so a hand-written stand-in
  cannot outlive the gap it filled or sit in front of a license file the package
  has since started shipping.
- **A source file named like a license is not one.** `github.com/lib/pq` ships
  `notice.go` and `notice_test.go` beside its real `LICENSE`, and reading them
  put Go source into the notices file.

The map data and the fonts are **not** repeated there. They carry their own
notices beside the data they belong to, at `public/map/LICENSE-OSM.txt` and
`public/map/fonts/LICENSE.txt`, and the preamble says so.

The shipped set is derived the same way the license gate derives it,
`go list -deps` over `./server/...` and `npm ls --omit=dev`, so the two agree on
what ships. Today both count 84 components.

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
carry a license obligation rather than a vulnerability: SIL OFL 1.1 requires its
notice to travel with the font software, and ranges generated from a TTF are a
Modified Version of it. `make bundle` refuses to ship them without
`LICENSE.txt`. Noto declares no Reserved Font Name, which is what permits the
generated ranges to keep the `NotoSans-Regular` name.

**`public/map/packages/`** holds the optional OpenStreetMap detail tier, which
is ODbL 1.0, and its tile schema is OpenMapTiles, which is CC-BY 4.0. Both are
attribution conditions rather than copyleft on our code, and both must be
displayed wherever the data is drawn: `public/map/LICENSE-OSM.txt` carries the
text, every map surface renders both credits, and `make bundle` refuses to ship
a detail package without the notice. The detail tier is optional, so its absence
is not an error; shipping it without the notice is.

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

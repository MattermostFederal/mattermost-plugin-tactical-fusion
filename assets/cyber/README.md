# Bundled cyber datasets

Everything in this directory is copied into the plugin bundle by the `bundle`
target in the `Makefile` and read at render time by the cyber decorator.

**This directory is under `assets/` rather than `public/` on purpose.**
Mattermost serves a plugin bundle's `public/` directory to anybody who can
reach the server, with no session; it does not serve `assets/`. A dataset here
is read by the plugin and never handed out, which is what makes this the right
place for one and the wrong place for anything an operator would not publish.
The watchlist in particular must never be moved under `public/`.

## What belongs here

| File | What it is | Ships |
|---|---|---|
| `kev.tsv` | CISA Known Exploited Vulnerabilities | yes |
| `cwedetail.tsv` | MITRE CWE research view 1000: each weakness's whole description, background, consequences, mitigations, detection methods and observed examples | yes |

Everything else the decorator reads is too large to bundle and is attached to a
release instead, for operators to drop into the directory named by the
`CyberDatasetsDir` setting: `cve.tsv`, `cvedetail.tsv`, `epss.tsv`, `ip.tsv`, plus any vendor
`.mmdb` database and the operator's own `watchlist.tsv`.

## Refreshing `kev.tsv`

CISA adds to the catalog most weekdays, so the committed file is a snapshot. Its
stamp names the catalog version it was built from. To refresh it, on a host with
network access:

```
curl -fL -o build/cyberdata/source/known_exploited_vulnerabilities.json \
  https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
go run ./build/cyberdata -only kev -label "CISA KEV catalog <catalogVersion>"
```

`make cyber-sources` followed by `make cyber-data` rebuilds it along with
everything else. An operator who wants a newer catalog than the bundle carries
drops a `kev.tsv` or `kev.tsv.gz` into `CyberDatasetsDir`; a file there replaces
the bundled one.

## Provenance

### `kev.tsv`

| | |
|---|---|
| Upstream | `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json` |
| Origin | Cybersecurity and Infrastructure Security Agency, United States |
| License | A work of the United States government, so not subject to domestic copyright |
| Format | The stamped, tab separated, bytewise-sorted shape `server/decorators/cyber/intel` reads |

The first line of every file here is a schema stamp. A file carrying no stamp,
or one built for a different reader, is skipped with `TF-21002` rather than
misread.

### `cwedetail.tsv`

| | |
|---|---|
| Upstream | `https://cwe.mitre.org/data/csv/1000.csv.zip` |
| Origin | MITRE CWE, research view 1000, fetched 2026-09-23 |
| License | MITRE CWE terms of use. Redistribution is permitted with attribution. |
| Format | Seven fields: the id, the whole description, the extended description, then consequences, mitigations, detection methods and observed examples as compact JSON arrays |

> This product uses information from MITRE CWE, the work of The MITRE
> Corporation. MITRE is not affiliated with and does not endorse this plugin.

The same export produces the `cwe.tsv` catalog compiled into the plugin, so the
two always name the same weaknesses. Rebuild both together:

```
go run ./build/cyberdata -only cwe,cwedetail -label "MITRE CWE research view 1000, fetched <date>"
```

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
| `kev.tsv` | CISA Known Exploited Vulnerabilities | yes, once generated |

Everything else the decorator reads is too large to bundle and is attached to a
release instead, for operators to drop into the directory named by the
`CyberDatasetsDir` setting: `cve.tsv`, `cvedetail.tsv`, `epss.tsv`, `ip.tsv`, plus any vendor
`.mmdb` database and the operator's own `watchlist.tsv`.

## `kev.tsv` is not committed yet

The host this feature was built on could not reach `www.cisa.gov`: it is
refused by the network policy in that environment. Shipping an invented KEV
catalog would tell a responder that a vulnerability is or is not being
exploited in the wild on the strength of nothing, so none was written.

Until it is generated, the decorator behaves exactly as it does on an install
with no datasets: a CVE still decorates and its panel says **"No known
exploited vulnerabilities dataset is installed."** rather than implying the
vulnerability is not listed. That distinction is the whole reason the status
sentences name the dataset.

To generate and commit it, on a host with network access:

```
make cyber-sources
make cyber-data
```

## Provenance

| | |
|---|---|
| Upstream | `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json` |
| Origin | Cybersecurity and Infrastructure Security Agency, United States |
| License | A work of the United States government, so not subject to domestic copyright |
| Format | The stamped, tab separated, bytewise-sorted shape `server/decorators/cyber/intel` reads |

The first line of every file here is a schema stamp. A file carrying no stamp,
or one built for a different reader, is skipped with `TF-20002` rather than
misread.

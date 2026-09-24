# Cyber catalogs

The two files here are compiled into the plugin binary with `go:embed` and are
what the decorator validates an ATT&CK or CWE identifier against. They are the
only cyber data that ships inside the Go binary: everything else is a file an
operator syncs into the directory named by the `CyberDatasetsDir` setting.

They are embedded rather than synced because **decoration has to be a function
of the build**. `Parse` runs in `MessageWillBePosted` and permanently rewrites
the stored message, so the same message must decorate the same way on every
node and on every day. A catalog an operator could change would make that
untrue.

## Provenance

| | |
|---|---|
| `attack.tsv` upstream | `https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack.json` |
| `attack.tsv` origin | MITRE ATT&CK, enterprise domain, STIX 2.1 |
| `attack.tsv` license | MITRE ATT&CK terms of use. Redistribution is permitted with attribution. |
| `cwe.tsv` upstream | `https://cwe.mitre.org/data/csv/1000.csv.zip` |
| `cwe.tsv` origin | MITRE CWE, research view 1000 |
| `cwe.tsv` license | MITRE CWE terms of use. Redistribution is permitted with attribution. |

> This product uses information from MITRE ATT&CK and MITRE CWE, both of which
> are the work of The MITRE Corporation. Neither MITRE nor ATT&CK is affiliated
> with or endorses this plugin.

## `cwe.tsv` is generated; `attack.tsv` is still a seed

`cwe.tsv` is the generator's output from MITRE's research view 1000, fetched
2026-09-23: 944 weaknesses, each with its name, abstraction, status, the first
sentence of its description, and its parents in that view.

**`attack.tsv` was written by hand, not produced by the generator.** The host
this feature was built on could not reach `raw.githubusercontent.com`. Rather
than ship invented technique names, which would be worse than shipping none, it
carries only entries that could be stated accurately without the upstream
source: every enterprise tactic and the widely used techniques and
sub-techniques. An identifier it does not hold is **left as written** rather
than linked to something wrong, the same rule an airfield code the database
does not hold follows. `go run ./build/cyberdata -only attack` after
`make cyber-sources` replaces it with the full upstream set.

Either file changes what decorates: the catalog is what `Parse` validates an
identifier against, so a CWE the new catalog holds and the seed did not now
becomes a link in messages posted after the change. Messages already posted
keep what they were given.

The reader refuses autolink triggers (`www.`, `://`) and any character outside
letters, digits and `_-,.'"()[]/&+:;*=<>`. MITRE's text uses `*`, `=`, `<` and
`>` in four entries, as in `'filedir*'` and `such as <, >`; every surface that
prints catalog text escapes it, and a test holds the standalone page to that.

## How they are produced

```
make cyber-sources    # fetches into build/cyberdata/source, which is gitignored
make cyber-data       # writes these two files, assets/cyber/kev.tsv, and build/cyberdata/out
```

`build/cyberdata/main.go` is standard library only and:

- keeps every tactic, technique and sub-technique that is neither revoked nor
  deprecated, and every CWE row with an id;
- derives each sub-technique's parent from its own id, and its tactics from the
  kill chain phases the upstream object names;
- cuts every description at its first sentence and at 300 runes, because the
  panel and the page both print it against a label;
- collapses tabs, newlines and runs of spaces, and **refuses the whole run** if
  a field still carries a separator, since the reader splits on one;
- refuses a duplicate id.

The ATT&CK reference URL is **not** a column. It is derived from the id in
`format.go`, so a value that reaches a reader can never be a link somebody else
chose.

## Regenerating

Not a prerequisite of `make test`, deliberately, and for the reason
`airport-data` is not either: this transform is filter, cut and sort, and its
drift shows up as an identifier that declines, which is visible and harmless.
`map-data-check` earns that slot because its encoding is opaque and its drift
fails invisibly on a plain-HTTP origin.

What keeps these files honest is a test over the embedded data rather than over
the generator: `TestEveryCatalogIdIsWellFormedAndReachable`,
`TestEveryCatalogReferenceResolves` and `TestEveryCatalogFieldPassesTheWhitelist`
in `catalog_test.go` fail the build on a file that is malformed, that names a
parent or a tactic it does not hold, or that carries a character the whitelist
refuses.

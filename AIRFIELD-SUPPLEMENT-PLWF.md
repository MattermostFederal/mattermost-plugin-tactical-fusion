# Supplement change: add PLWF, correct PTWF

**Status:** not started. Notes for a later planning pass.
**Date raised:** 2026-10-03 (rewritten the same day; see "What changed in this document").
**Origin:** the `indigo-mm-standard-v3` map handoff, confirmed by SME review of the Iron Fortress ATO in `mattermost-plugin-aocanywhere`.

## Why

Commit `f69822f` ("feat(bridge): add airfield lookup route and ten demo airfields",
PR #72) added the ten fictional Iron Fortress airfields to the supplement. The
set is one short, and one of the ten sits at the wrong position. Neither is
caused by #72's logic; both are data.

The v3 map handoff (`indigo-mm-standard-v3`) ships six airfield detail packs and
is the authority for the demo world. Diffing it against the shipped airfield
data turns up exactly two disagreements:

| field | handoff | airfield DB (main) | offset |
|---|---|---|---|
| RWBF | 26.351667, 127.769444 | 26.3517, 127.7694 | 6 m |
| RVGF | 15.1859, 120.5603 | 15.1860, 120.5600 | 34 m |
| PORF | 7.3673, 134.5443 | 7.3673, 134.5442 | 11 m |
| PNTF | 13.584, 144.929998 | 13.5840, 144.9300 | 0 m |
| **PTWF** | **21.3187, -157.9224** | **21.3353, -157.9483** | **3,256 m** |
| **PLWF** | **19.2820, 166.6360** | **absent** | **n/a** |

The first four are rounding between a four-decimal handoff table and the
embedded data. Leave them. The last two are real.

**PTWF is a copy-paste slip.** In the supplement, `PTWF` carries coordinates
byte-identical to `PGPC` (Granite Point); `PNTF` and `PFRC` are likewise
identical to each other. Two pairs share one coordinate each, and `PTWF` took
the wrong half of its pair. Both points fall inside the `demo-ptwf` pack's
bounds, so the map draws either one. It just does not put the pin on the
illustrated runway, 3.2 km away.

**PLWF is simply missing.** It is the handoff's headline v3 addition, the Wake
Island stand-in, and `demo-plwf.pmtiles` ships with artwork centred on it. The
handoff states that `PLWF` "already exists in AMPLIFI's scenario fields and uses
the Wake stand-in for planning" - it predates this repo's supplement rather than
being new. Today the airfield decorator cannot resolve the ident, so `PLWF` in a
message decorates as nothing and `/map?airport=PLWF` has nothing to open.

### Coordinates are scenario-frame values, not real-world positions

This is the part most likely to be "corrected" by a well-meaning later pass, so
it is stated plainly. The handoff:

> Message coordinates remain in the existing AMPLIFI **scenario frame**. [...]
> Each regional image is shifted so its visible runway midpoint matches the
> existing coordinate. PTWF's equivalent unsigned longitude is 202.0776, but use
> -157.9224 in Mattermost.

The coordinates came first and the artwork was built to them. They sit near the
real fields they stand in for, but they are not those fields' positions, and the
drawings are "distinct AI-generated fictional illustrations", "not copies of
real airport plans or authoritative runway layouts", produced with "No Earth
imagery or OSM geographic data".

So: **do not reconcile any demo coordinate against the real-world ICAO it masks,
and do not round or re-derive one.** In particular, `21.3187, -157.9224` is not
`PHIK`'s documented position in OurAirports and is not supposed to be. After the
correction `PTWF` lands about 250 m from `PCMN`, which is what the scenario frame
intends and is unremarkable here - `PNTF` and `PFRC` already share one coordinate
exactly.

### SME confirmation

The SMEs were shown the table above and approved acting on it:

> the fix your agent suggested should be the correct one. the lat longs it
> showed in your table are the ones i have on my end too.
>
> PTWF to 21.3187, -157.9224 and add a PLWF row at 19.2820, 166.6360
>
> we added PLWF earlier this week to make sure we could route FORGE 41 down to
> RVGF from PTWF.

(The note also writes `PLTF` once where it means `PLWF`; it writes `PLWF`
correctly twice, and the handoff and the map pack both name `PLWF`.)

### Ordering: Ops Center is upstream

`server/decorators/airport/data/README.md` records that the supplement values
"were copied from the `(DEMO-DATA)` rows of that plugin's
`assets/airport-codes.csv`". **Do not land this side first.** Wait for the
aocanywhere change (planned in that repo at
`docs/plans/2026-10-03-001-fix-iron-fortress-plwf-airfield-plan.md`), then copy
the two rows across with the coordinates at four decimals, exactly as the README
describes.

## Which branch this belongs on

`main` carries the ten demo airfields and the old `indopacom-*` map packs.
`update-maps-1` (and the identical `iron-fortress-demo`, commit `0d5d280`)
carries the six `demo-*` packs from the handoff but still `main`'s airfield
data - which is why the two disagree there.

This change belongs on **`main`**, as ordinary data. It is not map art and does
not depend on the basemap swap:

- On `main` it fixes the decorator and the bridge lookup for `PLWF` regardless of
  which packs ship.
- On the demo branch it is what makes the pins line up with the artwork.

`0d5d280` is explicitly marked "not for merging" and carries deliberately red
tests, so do **not** fold this change into it or branch from it. Land on `main`
and let the demo branch pick it up on its next rebase.

## Open question

One, and it does not block planning.

**The `Field` suffix on Lonewatch.** The handoff names the field "PLWF /
Lonewatch" throughout but never writes a full name. The aocanywhere plan uses
`Lonewatch Field (DEMO-DATA)` with municipality `Lonewatch`, matching how
`Tradewind Field (DEMO-DATA)` pairs with municipality `Tradewind`, and reading
Lone-Watch-Field exactly as `PNTF` reads North-Torr-Field. Only the suffix is
inferred; the name itself is sourced. Whatever Ops Center lands is what gets
copied here - this repo does not get its own answer.

## What changes here

### Data

| File | Change |
|---|---|
| `build/airportdata/supplement/airports.csv` | add the `PLWF` row; change `PTWF` lat/lon from `21.3353,-157.9483` to `21.3187,-157.9224` |
| `server/decorators/airport/data/airports.csv` | the same two edits. `PLWF` sorts between `PLPA` and `PLWN`, so it inserts before line 10286 |

The supplement's columns are
`ident,type,name,municipality,iso_country,iso_region,iata_code,elevation_ft,lat,lon,military`,
so the new row reads:

```
PLWF,medium_airport,Lonewatch Field (DEMO-DATA),Lonewatch,UM,UM-79,,14,19.2820,166.6360,
```

`UM` / `UM-79` / elevation `14` mirror `PWAK`'s non-positional attributes, the
way `PORF` mirrors Palau's. Those four fields only - the coordinates come from
the handoff, not from `PWAK`. No IATA code, no `military` designator.

Both files carry the same eleven columns, and `supplement_test.go` compares them
row for row, so the two edits have to agree exactly.

**Regeneration is not available offline.** `make airport-data` reads
`build/airportdata/source/`, which is gitignored and not present in a clean
checkout. Either re-fetch the three OurAirports files at the pinned commit
`3b27dacfa7700507e03401f2df024a1b1670d312` and rerun the generator, or
hand-splice the two rows into the generated file. The hand-splice is safe here:
the generator sorts by ident, so the result is byte identical either way, and
`TestTheDemoAirfieldsResolve` fails if the two files drift.

`PLWF` carries no runway, no frequency and no IATA code, like the other ten.
`MapBlob` is computed from the embedded row at request time, so no map asset
build is involved.

### Tests

| File | Change |
|---|---|
| `server/decorators/airport/supplement_test.go:36-38` | `if len(rows) != 10` becomes `11`, and the message "want the ten demo airfields" becomes eleven |

Nothing else in the test tree hardcodes the count.
`build/airportdata/main_test.go` builds its own synthetic supplements with
`writeSource`, so it is unaffected (it does use the literal `PTWF` as a fixture
ident in the duplicate-ident and short-row cases, but never its coordinates).
`server/bridge_test.go:307` exercises `PHIK`, `PNTF` and `QQQQ` and does not
assert positions.

Worth adding while here: nothing currently pins the supplement against the
handoff, which is how a 3.2 km error survived a green suite. A small table test
asserting each demo field's coordinates against the handoff's values would have
caught it. The handoff table is in the "Why" section above.

### Counts and docs

Merging eleven supplement rows instead of ten takes the embedded airfield count
from **19,290** to **19,291**. Every stated figure has to move with it.

| File | Line | Change |
|---|---|---|
| `server/decorators/airport/data/README.md` | 11 | "the ten demo airfields in `airports.csv`" becomes eleven |
| | 59 | "merge the ten rows of the supplement, for **19,290**" becomes eleven rows, **19,291** |
| | 90 | "603 of 19,290 names carry one" becomes 19,291. The 603 itself holds: it counts names carrying a military designator, and "Field" is not on that list |
| | ~112-122 | add `PLWF` to the supplement ident/name table, sorted between `PGPC` and `PNTF` |
| `public/help/airfields.html` | 50 | tagline "database of 19,290 fields" |
| | 129 | "Of the **19,290** codes this build carries" |
| | 379 | "It carries 19,290 fields with an ident..." |
| | 388-392 | "Ten of the fields are fictional" becomes eleven, and `PLWF` joins the `<code>` list between `PGPC` and `PNTF` |
| `CLAUDE.md` | 71 | "today ten fictional `(DEMO-DATA)` fields" becomes eleven |

Line 129 also states "**349 are ordinary English words**". `PLWF` is not an
English word, so that figure does not move.

`bridgeclient/README.md:162` and `public/help/integration.html:277` describe the
`(DEMO-DATA)` suffix without naming or counting the fields, so they need no
change.

The README's provenance section says the supplement fields sit "at or beside a
real field in the same area so the demo draws a plausible map". That is close
enough to stay, but if it is being touched anyway, "at the scenario-frame
position the demo basemap draws" is the accurate statement.

## Verification

- `go test ./server/decorators/airport/...` covers the supplement invariants:
  `TestTheDemoAirfieldsResolve` (supplement matches embedded, each ident parses
  to its own link and decorates), `TestEveryDemoAirfieldSaysItIsDemoData`,
  `TestOnlyTheSupplementCarriesTheDemoMarker`,
  `TestADemoAirfieldCarriesNoRunwaysFrequenciesOrIATACode` (which also asserts a
  map blob exists), and `TestEveryAirfieldHasAMapBlob`.
- `go test ./...` and `make check-style`.
- **Coordinate fidelity.** Every demo field's lat/lon matches the handoff table
  in the "Why" section. `PTWF` reads `21.3187,-157.9224` and no longer equals
  `PGPC`.
- **Pack containment.** `PLWF` at `19.2820, 166.6360` falls inside
  `demo-plwf.manifest.json`'s bounds (`166.5408..166.7312`, `19.2371..19.3269`),
  and `PTWF` at `21.3187, -157.9224` inside `demo-ptwf`'s
  (`-158.1538..-157.6910`, `21.2109..21.4265`). Note the pre-change `PTWF` value
  also falls inside its pack - containment alone would not have caught this
  defect, which is why the fidelity check above is the real gate.
- `GET /bridge/v1/airport?ident=PLWF` should answer `found=true` with the name
  and the Wake Island stand-in position, which is the route Ops Center calls now
  that it has dropped its own airfield dataset.
- Grep for `19,290` and `19290` across the tree afterwards; the three
  `airfields.html` hits and the two README hits should be the complete set.

## Paired change in mattermost-plugin-aocanywhere

Recorded here so whoever picks this up knows it does not stand alone. That side
has to land first, and is planned in full at
`docs/plans/2026-10-03-001-fix-iron-fortress-plwf-airfield-plan.md`:

- `assets/airport-codes.csv` gains the `PLWF` row and corrects `PTWF`.
- `server/demo/iron_fortress_ato_content.go:1011` and `:1021` revert `PTWF` back
  to `PLWF` (three occurrences across the two lines), undoing fix 1.1 of commit
  `428c8237`.
- `server/demo/iron_fortress_ato_content_test.go` grows a shared eleven-ident
  roster plus a bare-token scan, closing the gap that let a bare `PLWF` in chat
  prose go unchecked. `PWAK` joins the real-ICAO must-not-appear list.
- `public/ato/examples/iron-fortress-ato.html:281` and `public/help.html:676`
  add `PLWF`, and both drop the claim that the demo fields sit at the
  coordinates of the real fields they stand in for.
- `CHANGELOG.md` records the eleventh airfield and the `PTWF` correction.
- `docs/plans/2026-10-01-001-iron-fortress-ato-sme-review.md` section 1.1 is
  rewritten to record the reversal.

## What changed in this document

The first draft of these notes was written before the `indigo-mm-standard-v3`
handoff was in hand, and got three things wrong. They are corrected above, and
recorded here so an older copy is recognisable:

- It treated the coordinates as masks for real-world ICAOs and raised the
  `PTWF`/`PHIK` mismatch and the `PTWF`/`PCMN` proximity as open questions. Both
  were non-problems measured against the wrong frame. The handoff's values are
  authoritative.
- It said `PLWF` had no name and proposed inventing one. The handoff names it
  Lonewatch.
- It framed the `PTWF` change as a reposition on SME preference. It is a repair
  of a copy-paste slip, which the SME note approves rather than originates.

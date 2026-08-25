# Cursor on Target type data

`types.csv` is the type table this plugin embeds: 1,005 rows of `path,label`,
where `path` is a Cursor on Target atom code below the affiliation and `label`
is what to call it.

It is a plain, hand-maintainable data file. There is no generator and nothing to
regenerate it from: edit it directly.

## The shape

```
path,label
G-E-V-A-T,TANK
G-I-r,Road
G-I-R,RAW MATERIAL PRODUCTION/STORAGE
```

**The path starts below the affiliation.** A CoT atom is written
`a-<affiliation>-<code path>`, so `a-f-G-E-V-A-T` and `a-h-G-E-V-A-T` are the
same kind of thing with different affiliations. Keying the table below the
affiliation means one row answers for all eleven of them, and the affiliation
word is put in front of the label at render time: "Friendly TANK", "Hostile
TANK".

**Case is part of the code.** `G-I-r` is a road and `G-I-R` is raw material
production or storage. They are different types, not two spellings of one, and
`TestTypeCodesAreCaseSensitive` pins that a lookup never folds them together.

**Paths are matched whole, not letter by letter.** The letters do not compose
into English on their own: `G-U-C` is Ground, Unit, Combat, and no per-letter
gloss assembles into a name for it. Each row is therefore a complete path with
a finished label.

## Adding to it

Add a row. Nothing else has to change, and the label is shown to a reader
exactly as written here.

A path does not need its parent present. `longestAtomPath` keeps the deepest
match rather than stopping at the first miss, so adding `G-U-C-I-Q` works even
though `G-U-C-I` may be absent, and `TestADeeperPathResolvesThroughAMissingParent`
pins it.

Labels are Title Case, with acronyms and system names left in capitals: CSAR,
ECM, SAM, RPV/UAV, SOF, VSTOL, HAWK, VULCAN and the rest. Match that when adding
a row.

Some labels are composed rather than being the bare name of their own code
letter, because a leaf name alone can be meaningless: `G-U-C` is written
"Ground Combat Unit" rather than "Combat", since the card renders
`<affiliation> <label>` and "Friendly Combat" says almost nothing. Prefer a
label that reads as a finished phrase after an affiliation word.

**No two labels in this table are the same**, and `TestNoTwoTypesReadAlike`
keeps it that way: a label is the whole of what the card says an event IS, so
two paths sharing one is two different things a reader cannot tell apart.

Naming each leaf by its own code letter collided in two ways, and both are now
composed out. In the air branch the civil and military halves both read "Fixed
Wing", and eleven fixed against rotary pairs read identically to each other, so
a tanker was a tanker whether it was a KC-135 or a helicopter; every air label
now carries its wing type. Elsewhere a further thirty-six labels repeated, so a
row now says which kind of thing it is:

| Distinction | Written as |
|---|---|
| Incident state of one facility | `Fire (Staging Area)`, `Fire (Temporary)` |
| Facility against unit against equipment | `Law Enforcement Facility`, `Law Enforcement Resources`; `Mortar`, `Mortar Unit` |
| The same word in different domains | `Radio Station`, `Television Station`, `Surface Station`, `Submarine Station` |
| A qualifier shared by two parents | `Howitzer/Gun (Airborne)`, `Artillery Survey (Airborne)` |

One pair was identical upstream rather than merely terse: `G-U-U-A-C-R-S` and
`-W` both read "Chemical Wheeled Armored Vehicle". No consistent `-S` against
`-W` convention exists anywhere in this table, so the `-S` row is qualified by
its parent as "Chemical Recon Armored Vehicle" rather than asserted to be
tracked, which would have been a guess.

## What this table is not

It is a naming table, not a validator. A path absent from it is not an invalid
CoT type; it is one this build cannot name, and the decoder answers with the
deepest ancestor it does hold. `a-f-G-U-C-I-XYZ` keeps the infantry label and
the trailing code is not guessed at.

That is only honest because the raw type is rendered beside the label on every
surface, in parentheses, so a reader can always see what the label came from and
what it did not cover.

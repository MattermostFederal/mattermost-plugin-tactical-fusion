# Report vocabulary

Two tables the NOTAM decoder embeds. Both are subsets, chosen for the
contractions and codes that occur in ordinary aerodrome NOTAMs; a token the
table lacks is kept verbatim rather than guessed at.

## contractions.csv

Common contractions used in NOTAM text, with their expansions. The source is
FAA Order JO 7340.2 (Contractions), a United States government publication in
the public domain, read through the FAA's published NOTAM guidance. The subset
here is 150 entries; the full order runs to thousands, most of them never seen
in aerodrome NOTAMs.

Expansion is by whole word, with trailing punctuation kept, and only in the
text field of a NOTAM. A word not in the table is left exactly as written.

## qcodes.csv

The NOTAM Q-code table: the second and third letters (the subject) and the
fourth and fifth (the condition), each as a row carrying its role, `Q` plus the
two letters, and the meaning. The two roles share letter pairs (`LA` is the
approach lighting system as a subject and "operating on auxiliary power supply"
as a condition), so they are two tables, and a row is refused if its pair is
listed twice in the same role.

The source is the Q-code table as reproduced in the FAA's NOTAM Manual (FAA
Order JO 7930.2), a public domain publication of ICAO Doc 8126's table. The
subset here covers the aerodrome, lighting, movement area, navigation aid,
airspace and warning subjects and every condition code in common use. A code
the table lacks is listed under "Not decoded" with the raw five letters.

## Regenerating

Both are hand-maintained CSVs rather than generated: the upstream documents are
PDFs, and the subset is a judgment. Add a row when a real NOTAM shows a token
the table lacks, with the expansion taken from the order rather than from
memory. Every row is loaded at init and a malformed one fails the build.

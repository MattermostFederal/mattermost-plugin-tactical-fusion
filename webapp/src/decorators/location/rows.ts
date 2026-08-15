/**
 * The rows a location is rendered as, in the order they are shown.
 *
 * Mirrors `Rows` in `server/decorators/location/rows.go`, and
 * `webapp_sync_test.go` holds the two to the same ids in the same order. Three
 * things have to agree about what the rows are: the standalone page renders
 * them in Go, this panel renders them again, and a reader's stored view
 * settings name them.
 */

/**
 * A row id, which survives its label being reworded.
 *
 * These reach the KV store in a reader's hidden-row list, so they are a
 * contract rather than an implementation detail: renaming one silently unhides
 * a row for everybody who had hidden it.
 */
export type RowID =
    'decimal' | 'dms' | 'ddm' | 'mgrs' | 'usmtf' | 'utm' |
    'resolution' | 'confidence' | 'datum' | 'raw' | 'canonical';

export interface RowSpec {
    id: RowID;
    label: string;

    /**
     * Whether the row's value is worth copying.
     *
     * False for the rows that are prose rather than a value: a reader pasting
     * "about 11 m" or "WGS 84" into another system has copied a sentence, not a
     * position.
     */
    copyable: boolean;

    /**
     * What the editor says the row is, for a reader deciding whether to keep
     * it. Absent where the label already says it.
     */
    hint?: string;
}

/** Every row, in render order. */
export const ROWS: readonly RowSpec[] = [
    {id: 'raw', label: 'As written', copyable: true, hint: 'Exactly what the author typed'},
    {id: 'decimal', label: 'Lat / lon', copyable: true, hint: 'Decimal degrees'},
    {id: 'dms', label: 'DMS', copyable: true, hint: 'Degrees, minutes, seconds'},
    {id: 'ddm', label: 'DDM', copyable: true, hint: 'Degrees and decimal minutes'},
    {id: 'mgrs', label: 'MGRS', copyable: true, hint: 'Military grid reference'},
    {id: 'usmtf', label: 'USMTF', copyable: true, hint: 'The fixed-width compact form'},
    {id: 'utm', label: 'UTM', copyable: true, hint: 'Zone, band, easting, northing'},
    {id: 'resolution', label: 'Resolution', copyable: false, hint: 'How finely the original text was written'},
    {id: 'confidence', label: 'Confidence', copyable: false, hint: 'Only for a verified USMTF coordinate'},
    {id: 'datum', label: 'Datum', copyable: false, hint: 'Always WGS 84'},
    {id: 'canonical', label: 'Normalized', copyable: true, hint: 'Shown only when it differs from what the author wrote'},
];

const KNOWN = new Set<string>(ROWS.map((row) => row.id));

/** Whether an id names a row this build renders. */
export function isRowID(value: string): value is RowID {
    return KNOWN.has(value);
}

/**
 * The map, which a reader hides the same way they hide a row but which is not
 * one: a RowSpec carries a string value and the panel omits a row whose value
 * is empty, and a map has no string value.
 *
 * Written in its own shape rather than as a RowSpec literal, because the Go
 * sync test counts row-shaped literals in this file and requires the count to
 * equal the catalog.
 */
export const MAP_ID = 'map';

/** What the reader's hidden list may name: every row, plus the map. */
export type HideableID = RowID | typeof MAP_ID;

const HIDEABLE = new Set<string>([...ROWS.map((row) => row.id), MAP_ID]);

/**
 * Whether an id names something this build renders and a reader may hide.
 *
 * Deliberately wider than isRowID. RowID itself is NOT widened to admit the
 * map: LocationPanel declares its values as Record<RowID, string> and an
 * exhaustive object literal, so admitting a key with no string value there
 * fails the type check.
 */
export function isHideableID(value: string): value is HideableID {
    return HIDEABLE.has(value);
}

/**
 * Whether a row should be shown, given what the reader hid.
 *
 * Hidden rather than shown is the direction the whole setting is stored in: an
 * empty list means every row, so a reader who never chose keeps tracking
 * whatever the rows become, and a row added later appears for everybody.
 */
export function isRowVisible(hidden: readonly string[], id: HideableID): boolean {
    return !hidden.includes(id);
}

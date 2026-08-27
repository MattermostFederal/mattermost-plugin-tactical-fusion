/**
 * The groups the Cursor on Target sidebar panel is drawn as, in render order.
 *
 * Mirrors `Sections` in `server/cot/sections.go`, and `cot_sync_test.go` holds
 * the two to the same ids in the same order. Two things have to agree about
 * what the sections are: the server validates a hidden-section id against them,
 * and this panel renders them.
 */

/**
 * A section id, which survives its label being reworded.
 *
 * These reach the KV store in a reader's hidden-section list, so they are a
 * contract rather than an implementation detail: renaming one silently unhides
 * a section for everybody who had hidden it.
 */
export type SectionID =
    'map' | 'stale' | 'event' | 'remarks' | 'device' |
    'precision' | 'orientation' | 'payload' | 'shape' | 'flow' | 'source';

export interface SectionSpec {
    id: SectionID;
    label: string;

    /**
     * What the editor says the section is, for a reader deciding whether to
     * keep it. Webapp-only: the server stores ids and never a description.
     */
    hint: string;
}

/** Every hideable section, in the order the panel draws them. */
export const SECTIONS: readonly SectionSpec[] = [
    {id: 'map', label: 'Map', hint: 'Where the event is, with its accuracy ring'},
    {id: 'stale', label: 'Goes stale', hint: 'The live countdown to the end of the valid window'},
    {id: 'event', label: 'Event readings', hint: 'Position, accuracy, times, team, sender and UID'},
    {id: 'remarks', label: 'Remarks', hint: 'The free text the event carried'},
    {id: 'device', label: 'Device', hint: 'Platform, battery, network endpoint and stated display color'},
    {id: 'precision', label: 'Position quality', hint: 'Position and altitude source, and the dilution figures'},
    {id: 'orientation', label: 'Orientation', hint: 'Yaw, pitch, roll and slope'},
    {id: 'payload', label: 'Payload', hint: 'Sensor, video, GeoChat, MEDEVAC, geofence, attachments and checklist'},
    {id: 'shape', label: 'Shape', hint: 'The drawn outline, circle or route the event described'},
    {id: 'flow', label: 'Processing path', hint: 'The servers the event passed through, collapsed'},
    {id: 'source', label: 'As posted', hint: 'The event exactly as it was posted, collapsed'},
];

const KNOWN = new Set<string>(SECTIONS.map((section) => section.id));

/** Whether an id names a section this build renders. */
export function isSectionID(value: string): value is SectionID {
    return KNOWN.has(value);
}

/**
 * Whether a section should be shown, given what the reader hid.
 *
 * Hidden rather than shown is the direction the whole setting is stored in: an
 * empty list means every section, so a reader who never chose keeps tracking
 * whatever the panel becomes, and a section added later appears for everybody.
 */
export function isSectionVisible(hidden: readonly string[], id: SectionID): boolean {
    return !hidden.includes(id);
}

/**
 * The heading a section is drawn under.
 *
 * The catalog is what the editor's tickbox says, so a heading spelled
 * separately is a heading that can disagree with the box governing it. One
 * already had: the box read "Event readings" and the heading over it read
 * "Event", and the Go guard could not see it because it compares Go to
 * TypeScript rather than TypeScript to itself.
 */
export function sectionLabel(id: SectionID): string {
    return SECTIONS.find((section) => section.id === id)?.label ?? '';
}

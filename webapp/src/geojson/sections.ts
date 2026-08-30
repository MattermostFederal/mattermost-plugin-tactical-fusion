/**
 * A section id, which survives its label being reworded.
 *
 * These reach the KV store in a reader's hidden-section list, so they are a
 * contract rather than an implementation detail: renaming one silently unhides
 * a section for everybody who had hidden it.
 */
export type SectionID = 'map' | 'summary' | 'features' | 'properties' | 'source';

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
    {id: 'map', label: 'Map', hint: 'Every feature drawn together, fitted to all of them'},
    {id: 'summary', label: 'What the document holds', hint: 'How many features, of what geometry, and anything not drawn'},
    {id: 'features', label: 'Features', hint: 'Each feature by name, with its geometry and size'},
    {id: 'properties', label: 'Feature properties', hint: 'The keys and values each feature carries'},
    {id: 'source', label: 'As posted', hint: 'The document exactly as it was posted, collapsed'},
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
 * separately is a heading that can disagree with the box governing it.
 */
export function sectionLabel(id: SectionID): string {
    return SECTIONS.find((section) => section.id === id)?.label ?? '';
}

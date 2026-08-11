/**
 * The sidebar header for a DTG.
 *
 * A category rather than the value itself: the canonical DTG is already the
 * first and largest line of the panel, so repeating it in the header would
 * spend the space saying the same thing twice. Matches the heading on the
 * standalone page, in server/decorators/dtg/dtg.go.
 */
export const PANEL_TITLE = 'Date/Time';

/**
 * The sidebar header while the editor has the panel.
 *
 * Matches the link that opens it and the heading inside it, so the reader is
 * never told two different names for where they are.
 */
export const EDITOR_TITLE = 'Customize your view';

/*
 * These live apart from the decorator so the header component and the decorator
 * that registers it can both read them. Importing them from index.ts would put
 * a cycle between the two.
 */

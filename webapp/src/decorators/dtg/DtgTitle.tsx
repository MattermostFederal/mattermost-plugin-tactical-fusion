import React from 'react';

import {useEditing} from './editing';

/**
 * The sidebar header for a date-time group.
 *
 * A category rather than the value itself: the value is already the first line
 * of the panel, so repeating it in the header would spend the space saying the
 * same thing twice. Matches the heading on the standalone page.
 *
 * Exported from here rather than from a file of its own. The decorator needs it
 * for `summary`, and importing it from index.ts would be a cycle; importing it
 * from this component is not, because this component imports nothing from
 * index.ts.
 */
export const PANEL_TITLE = 'Date/Time';

/**
 * The sidebar header while the editor has the panel.
 *
 * Matches the link that opens it, so the reader is never told two different
 * names for where they are.
 */
export const EDITOR_TITLE = 'Customize your view';

/**
 * The sidebar header for a DTG, which follows whichever view the panel is on.
 *
 * The editor takes the panel over, so a header still reading "Date/Time" would
 * be describing something no longer on screen.
 */
const DtgTitle: React.FC = () => <>{useEditing() ? EDITOR_TITLE : PANEL_TITLE}</>;

export default DtgTitle;

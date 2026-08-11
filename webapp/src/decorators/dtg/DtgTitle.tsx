import React from 'react';

import {useEditing} from './editing';
import {EDITOR_TITLE, PANEL_TITLE} from './titles';

/**
 * The sidebar header for a DTG, which follows whichever view the panel is on.
 *
 * The editor takes the panel over, so a header still reading "Date/Time" would
 * be describing something no longer on screen.
 */
const DtgTitle: React.FC = () => <>{useEditing() ? EDITOR_TITLE : PANEL_TITLE}</>;

export default DtgTitle;

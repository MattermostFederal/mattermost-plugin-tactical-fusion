import React from 'react';

import {useEditing} from './editing';

export const PANEL_TITLE = 'Date/Time';

export const EDITOR_TITLE = 'Customize your view';

export function titleOf(payload: {canonical: string}): string {
    return `${PANEL_TITLE}: ${payload.canonical}`;
}

const DtgTitle: React.FC<{payload: {canonical: string}}> = ({payload}) => <>{useEditing() ? EDITOR_TITLE : titleOf(payload)}</>;

export default DtgTitle;

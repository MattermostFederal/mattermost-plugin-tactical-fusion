import React, {useState} from 'react';

import NoteHover from './NoteHover';

import {initRhs, _resetForTesting as resetSelection} from '../selection';

const NoteHarness: React.FC<{markdown: string; imageProxy: boolean}> = ({markdown, imageProxy}) => {
    useState(() => {
        resetSelection();
        initRhs({
            getState: () => ({entities: {general: {config: {HasImageProxy: String(imageProxy)}}}}),
            dispatch: (action: unknown) => action,
        } as never, null);
        return null;
    });

    return <NoteHover payload={{markdown}}/>;
};

export default NoteHarness;

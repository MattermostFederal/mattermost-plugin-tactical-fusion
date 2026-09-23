import React from 'react';

import NoteMarkdown from './NoteMarkdown';

import type {NotePayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    body: {
        maxHeight: 320,
        overflow: 'auto',
        fontSize: '13px',
        color: 'var(--center-channel-color)',
    },
};

const NoteHover: React.FC<{payload: NotePayload}> = ({payload}) => (
    <div
        style={styles.body}
        data-testid='note-hover'
    >
        <NoteMarkdown markdown={payload.markdown}/>
    </div>
);

export default NoteHover;

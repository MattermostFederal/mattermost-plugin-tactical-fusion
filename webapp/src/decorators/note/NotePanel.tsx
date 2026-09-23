import React from 'react';

import NoteMarkdown from './NoteMarkdown';

import ErrorBoundary from '../../components/ErrorBoundary';
import LinkButton from '../../components/LinkButton';
import Disclosure from '../../cot/Disclosure';
import {docsUrl} from '../../plugin_url';
import CopyButton from '../location/CopyButton';

import type {NotePayload} from './index';

export const SOURCE_LABEL = 'As posted';

export const RENDER_FAILED = 'This note could not be rendered.';

const styles: Record<string, React.CSSProperties> = {
    body: {overflowX: 'auto', margin: '0 0 12px', color: 'var(--center-channel-color)'},
    status: {fontSize: '13px', opacity: 0.8},
    posted: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: 0,
        maxHeight: 280,
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
    footer: {marginTop: '14px', fontSize: '12px'},
};

const NotePanel: React.FC<{payload: NotePayload}> = ({payload}) => (
    <div data-testid='note-panel'>
        <ErrorBoundary fallback={<p style={styles.status}>{RENDER_FAILED}</p>}>
            <div style={styles.body}>
                <NoteMarkdown markdown={payload.markdown}/>
            </div>
        </ErrorBoundary>

        <Disclosure
            label={SOURCE_LABEL}
            trailing={
                <CopyButton
                    label='Copy the note as posted'
                    value={payload.markdown}
                />
            }
        >
            <pre
                style={styles.posted}
                tabIndex={0}
                role='region'
                aria-label='The note as it was posted'
                data-testid='note-source'
            >{payload.markdown}</pre>
        </Disclosure>

        <div style={styles.footer}>
            <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
        </div>
    </div>
);

export default NotePanel;

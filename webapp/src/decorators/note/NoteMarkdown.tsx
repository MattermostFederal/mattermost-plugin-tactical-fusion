import React from 'react';

import ErrorBoundary from '../../components/ErrorBoundary';
import {hasImageProxy} from '../selection';

const RENDER_OPTIONS = {mentionHighlight: false};

const styles: Record<string, React.CSSProperties> = {
    source: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '12px',
        margin: 0,
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
};

function rendered(markdown: string): React.ReactNode | null {
    const utils = window.PostUtils;
    if (!utils) {
        return null;
    }
    try {
        const html = utils.formatText(markdown, {atMentions: false, mentionHighlight: false, markdown: true, proxyImages: hasImageProxy()});
        const node = utils.messageHtmlToComponent(html, RENDER_OPTIONS);
        return node === undefined || node === null || node === '' ? null : node;
    } catch {
        return null;
    }
}

const Source: React.FC<{markdown: string}> = ({markdown}) => (
    <pre
        style={styles.source}
        data-testid='note-source-fallback'
    >{markdown}</pre>
);

const NoteMarkdown: React.FC<{markdown: string}> = ({markdown}) => {
    const node = rendered(markdown);

    if (node === null) {
        return <Source markdown={markdown}/>;
    }

    return (
        <ErrorBoundary fallback={<Source markdown={markdown}/>}>
            <div
                className='post-message__text'
                data-testid='note-markdown'
            >{node}</div>
        </ErrorBoundary>
    );
};

export default NoteMarkdown;

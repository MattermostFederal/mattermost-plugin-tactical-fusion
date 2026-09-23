import React from 'react';

export interface PostUtils {
    formatText: (text: string, options?: Record<string, unknown>) => string;
    messageHtmlToComponent: (html: string, options?: Record<string, unknown>) => React.ReactNode;
}

declare global {
    interface Window {
        PostUtils?: PostUtils;
    }
}

const FORMAT_OPTIONS = {atMentions: false, mentionHighlight: false, markdown: true};

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
        return utils.messageHtmlToComponent(utils.formatText(markdown, FORMAT_OPTIONS), RENDER_OPTIONS);
    } catch {
        return null;
    }
}

const NoteMarkdown: React.FC<{markdown: string}> = ({markdown}) => {
    const node = React.useMemo(() => rendered(markdown), [markdown]);

    if (node === null) {
        return (
            <pre
                style={styles.source}
                data-testid='note-source-fallback'
            >{markdown}</pre>
        );
    }

    return (
        <div
            className='post-message__text'
            data-testid='note-markdown'
        >{node}</div>
    );
};

export default NoteMarkdown;

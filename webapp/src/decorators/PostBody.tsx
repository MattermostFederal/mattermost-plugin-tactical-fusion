import React from 'react';

import {soleDecoratorLink} from './inline';
import {standalonePayload} from './post_props';
import {get} from './registry';

import ErrorBoundary from '../components/ErrorBoundary';

interface PostLike {
    message?: string;
    props?: unknown;
}

interface Props {
    post?: PostLike;
    compactDisplay?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    lead: {marginRight: '0.35em'},
    message: {whiteSpace: 'pre-wrap'},
};

function agrees(a: URLSearchParams, b: URLSearchParams): boolean {
    return a.get('f') === b.get('f') && a.get('v') === b.get('v');
}

export const DecoratorPostBody: React.FC<Props> = ({post, compactDisplay}) => {
    const message = post?.message ?? '';
    const plain = <span style={styles.message}>{message}</span>;

    const payload = standalonePayload(post?.props);
    const link = soleDecoratorLink(message);
    if (payload === null || link === null || payload.type !== link.type ||
        !agrees(payload.params, link.params)) {
        return plain;
    }

    const decorator = get(payload.type);
    if (!decorator?.Inline) {
        return plain;
    }

    const parsed = decorator.fromParams(payload.params);
    if (parsed === null || parsed === undefined) {
        return plain;
    }

    const {Inline} = decorator;
    return (
        <div>
            <span>
                {link.lead !== '' && <span style={styles.lead}>{link.lead}</span>}
                <a href={link.href}>{link.label}</a>
            </span>
            {!compactDisplay && (
                <ErrorBoundary>
                    <Inline payload={parsed}/>
                </ErrorBoundary>
            )}
        </div>
    );
};

export default DecoratorPostBody;

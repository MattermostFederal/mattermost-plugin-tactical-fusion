import React from 'react';

import AirfieldsMap from './AirfieldsMap';
import type {RoutePayload} from './route';
import {airfieldsFromProps, payloadFor} from './route';

import ErrorBoundary from '../../components/ErrorBoundary';
import LinkButton from '../../components/LinkButton';
import type {DecoratorLink} from '../inline';
import {decoratorLinks} from '../inline';
import {openRhs, setSelection} from '../selection';

interface PostLike {
    id?: string;
    message?: string;
    props?: unknown;
    edit_at?: number;
}

interface Props {
    post?: PostLike;
    compactDisplay?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    message: {whiteSpace: 'pre-wrap'},
    legend: {margin: '6px 0 0', padding: 0, listStyle: 'none', fontSize: '13px'},
    number: {
        display: 'inline-block',
        minWidth: '1.4em',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        opacity: 0.7,
    },
};

function codeOf(link: DecoratorLink): string | null {
    return link.params.get('v') ?? link.params.get('i');
}

function agrees(links: DecoratorLink[], payload: RoutePayload): boolean {
    if (links.length !== payload.airfields.length) {
        return false;
    }
    return links.every((link, index) => link.type === 'airport' && codeOf(link) === payload.airfields[index].code);
}

function renderMessage(message: string, links: DecoratorLink[]): React.ReactNode[] {
    const parts: React.ReactNode[] = [];
    let cursor = 0;
    for (const [index, link] of links.entries()) {
        if (link.start > cursor) {
            parts.push(<span key={`text-${index}`}>{message.slice(cursor, link.start)}</span>);
        }
        parts.push(<a
            key={`link-${index}`}
            href={link.href}
                   >{link.label}</a>);
        cursor = link.end;
    }
    if (cursor < message.length) {
        parts.push(<span key='tail'>{message.slice(cursor)}</span>);
    }
    return parts;
}

export const AirfieldsPostBody: React.FC<Props> = ({post, compactDisplay}) => {
    const message = post?.message ?? '';
    const plain = <span style={styles.message}>{message}</span>;

    const payload = airfieldsFromProps(post?.props);
    if (payload === null || (post?.edit_at ?? 0) !== 0) {
        return plain;
    }

    const links = decoratorLinks(message);
    if (!agrees(links, payload)) {
        return plain;
    }

    const route: RoutePayload = {...payload, postId: post?.id ?? ''};

    return (
        <div data-testid='airfields-post'>
            <span style={styles.message}>{renderMessage(message, links)}</span>
            {!compactDisplay && (
                <ErrorBoundary>
                    <ol style={styles.legend}>
                        {route.airfields.map((airfield, index) => (
                            <li key={`${airfield.code}-${index}`}>
                                <span style={styles.number}>{`${index + 1}.`}</span>
                                <LinkButton
                                    onClick={() => {
                                        setSelection({type: 'airport', payload: payloadFor(airfield)});
                                        openRhs();
                                    }}
                                >
                                    {airfield.name || airfield.ident}
                                </LinkButton>
                            </li>
                        ))}
                    </ol>
                    <AirfieldsMap payload={route}/>
                </ErrorBoundary>
            )}
        </div>
    );
};

export default AirfieldsPostBody;

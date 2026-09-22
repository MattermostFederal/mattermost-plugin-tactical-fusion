import React from 'react';

import {KIND_LABELS, useCyber} from './cyber';
import type {CyberState} from './cyber';
import {useMentions} from './mentions';
import type {CyberLink, CyberMention} from './types';

import LinkButton from '../../components/LinkButton';
import CopyButton from '../location/CopyButton';
import {currentTeamId, setSelection} from '../selection';

import type {CyberPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    title: {
        fontSize: '18px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: '0 0 2px',
    },
    value: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '12px',
        letterSpacing: '0.04em',
        opacity: 0.6,
        color: 'var(--center-channel-color)',
        margin: '0 0 16px',
        wordBreak: 'break-all',
    },
    summary: {
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        margin: '0 0 16px',
    },
    table: {width: '100%', borderCollapse: 'collapse', fontSize: '13px'},
    th: {
        textAlign: 'left',
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        padding: '8px 10px 8px 0',
        verticalAlign: 'top',
        whiteSpace: 'nowrap',
        width: '38%',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    td: {
        padding: '8px 0',
        color: 'var(--center-channel-color)',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        wordBreak: 'break-word',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    copyCell: {
        width: '24px',
        padding: '6px 0 6px 8px',
        verticalAlign: 'top',
        textAlign: 'right',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    heading: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        margin: '20px 0 8px',
    },
    note: {
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.6,
        margin: '14px 0 0',
    },
    list: {listStyle: 'none', padding: 0, margin: 0, fontSize: '13px'},
    listItem: {margin: '0 0 8px'},
    verdict: {fontWeight: 600},
    mentionMeta: {
        fontSize: '12px',
        opacity: 0.6,
        color: 'var(--center-channel-color)',
    },
    mentionSnippet: {
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        margin: '2px 0 0',
        wordBreak: 'break-word',
    },
};

const Row: React.FC<{label: string; value: string}> = ({label, value}) => (
    <tr>
        <th
            scope='row'
            style={styles.th}
        >{label}</th>
        <td style={styles.td}>{value}</td>
        <td style={styles.copyCell}>
            <CopyButton
                label={`Copy ${label}`}
                value={value}
            />
        </td>
    </tr>
);

const Related: React.FC<{links: CyberLink[]}> = ({links}) => {
    if (links.length === 0) {
        return null;
    }

    return (
        <>
            <p style={styles.heading}>{'Related'}</p>
            <ul style={styles.list}>
                {links.map((link) => (
                    <li
                        key={`${link.kind}:${link.value}`}
                        style={styles.listItem}
                    >
                        <LinkButton
                            onClick={() => setSelection({
                                type: 'cyber',
                                payload: {kind: link.kind, value: link.value},
                            })}
                        >{link.label}</LinkButton>
                    </li>
                ))}
            </ul>
        </>
    );
};

function mentionTime(createAt: number): string {
    if (!createAt) {
        return '';
    }

    return new Date(createAt).toLocaleString();
}

const Mention: React.FC<{mention: CyberMention}> = ({mention}) => {
    const where = [mention.channel, mentionTime(mention.createAt)].filter(Boolean).join(' - ');

    return (
        <li style={styles.listItem}>
            {mention.permalink ? (
                <a href={mention.permalink}>{where || 'Open the post'}</a>
            ) : (
                <span style={styles.mentionMeta}>{where}</span>
            )}
            <p style={styles.mentionSnippet}>{mention.snippet}</p>
        </li>
    );
};

const Mentions: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const team = currentTeamId();
    const state = useMentions(team, payload.kind, payload.value);

    if (state.status === 'idle') {
        return null;
    }

    return (
        <>
            <p style={styles.heading}>{'Earlier mentions'}</p>
            {state.status === 'loading' && <p style={styles.note}>{'Looking for earlier mentions...'}</p>}
            {state.status === 'failed' && (
                <p style={styles.note}>{'Earlier mentions could not be searched for.'}</p>
            )}
            {state.status === 'ready' && state.data && (
                state.data.mentions.length === 0 ? (
                    <p style={styles.note}>{'No earlier mention of this indicator was found in this team.'}</p>
                ) : (
                    <>
                        <ul style={styles.list}>
                            {state.data.mentions.map((mention) => (
                                <Mention
                                    key={mention.postId}
                                    mention={mention}
                                />
                            ))}
                        </ul>
                        {state.data.truncated && (
                            <p style={styles.note}>{'Only the most recent mentions are shown.'}</p>
                        )}
                    </>
                )
            )}
        </>
    );
};

function renderBody(state: CyberState, payload: CyberPayload): React.ReactNode {
    if (state.status === 'loading') {
        return <p style={styles.note}>{'Looking this indicator up...'}</p>;
    }
    if (state.status === 'rejected') {
        return <p style={styles.note}>{'That is not an indicator this plugin issued.'}</p>;
    }
    if (state.status === 'failed' || !state.data) {
        return <p style={styles.note}>{'This indicator could not be looked up. The server did not answer.'}</p>;
    }

    const details = state.data;

    return (
        <>
            <p style={styles.title}>{details.title}</p>
            {details.title !== details.value && <p style={styles.value}>{details.value}</p>}
            {details.summary !== '' && <p style={styles.summary}>{details.summary}</p>}

            {details.rows.length > 0 && (
                <table style={styles.table}>
                    <tbody>
                        {details.rows.map((row) => (
                            <Row
                                key={row.label}
                                label={row.label}
                                value={row.value}
                            />
                        ))}
                    </tbody>
                </table>
            )}

            {details.status !== '' && <p style={styles.note}>{details.status}</p>}

            {details.watchlist.length > 0 && (
                <>
                    <p style={styles.heading}>{'Watchlist'}</p>
                    <ul style={styles.list}>
                        {details.watchlist.map((entry, index) => (
                            <li
                                key={`${entry.source}:${entry.updated}:${index}`}
                                style={styles.listItem}
                            >
                                <span style={styles.verdict}>{entry.verdict}</span>
                                {entry.note !== '' && ` ${entry.note}`}
                                <span style={styles.mentionMeta}>
                                    {[entry.source, entry.updated].filter(Boolean).join(' - ')}
                                </span>
                            </li>
                        ))}
                    </ul>
                </>
            )}

            <Related links={details.related}/>
            <Mentions payload={payload}/>

            <p style={styles.heading}>{'Datasets'}</p>
            <ul style={styles.list}>
                {details.datasets.map((dataset) => (
                    <li
                        key={dataset.name}
                        style={styles.mentionMeta}
                    >
                        {dataset.label}
                        {dataset.present ? `: installed${dataset.generated ? `, generated ${dataset.generated}` : ''}` : ': not installed'}
                    </li>
                ))}
            </ul>
        </>
    );
}

const CyberPanel: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const state = useCyber(payload.kind, payload.value);

    return (
        <div aria-label={KIND_LABELS[payload.kind] ?? 'Cyber context'}>
            {renderBody(state, payload)}
        </div>
    );
};

export default CyberPanel;

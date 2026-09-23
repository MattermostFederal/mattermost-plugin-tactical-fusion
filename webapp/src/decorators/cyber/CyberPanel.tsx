import React from 'react';

import {KIND_LABELS, isKind, useCyber} from './cyber';
import type {CyberState} from './cyber';
import type {CyberLink, CyberReference} from './types';

import LinkButton from '../../components/LinkButton';
import CopyButton from '../location/CopyButton';
import {setSelection} from '../selection';

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
    meta: {
        fontSize: '12px',
        opacity: 0.6,
        color: 'var(--center-channel-color)',
    },
    details: {margin: '20px 0 0'},
    toggle: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        cursor: 'pointer',
        margin: '0 0 8px',
    },
    line: {
        margin: '0 0 6px',
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        overflowWrap: 'anywhere',
    },
    reference: {color: 'var(--link-color)', overflowWrap: 'anywhere'},
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

const Collapsible: React.FC<{title: string; count: number; children: React.ReactNode}> = ({title, count, children}) => (
    <details style={styles.details}>
        <summary style={styles.toggle}>{`${title} (${count})`}</summary>
        {children}
    </details>
);

const Lines: React.FC<{title: string; lines: string[]}> = ({title, lines}) => {
    if (lines.length === 0) {
        return null;
    }

    return (
        <Collapsible
            title={title}
            count={lines.length}
        >
            <ul style={styles.list}>
                {lines.map((line) => (
                    <li
                        key={line}
                        style={styles.line}
                    >{line}</li>
                ))}
            </ul>
        </Collapsible>
    );
};

const References: React.FC<{references: CyberReference[]}> = ({references}) => {
    if (references.length === 0) {
        return null;
    }

    return (
        <Collapsible
            title='References'
            count={references.length}
        >
            <ul style={styles.list}>
                {references.map((reference) => (
                    <li
                        key={reference.url}
                        style={styles.line}
                    >
                        <a
                            href={reference.url}
                            target='_blank'
                            rel='noopener noreferrer'
                            style={styles.reference}
                        >{reference.url}</a>
                        {reference.tags !== '' && <span style={styles.meta}>{` ${reference.tags}`}</span>}
                    </li>
                ))}
            </ul>
        </Collapsible>
    );
};

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

function renderBody(state: CyberState): React.ReactNode {
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
                                <span style={styles.meta}>
                                    {[entry.source, entry.updated].filter(Boolean).join(' - ')}
                                </span>
                            </li>
                        ))}
                    </ul>
                </>
            )}

            <Related links={details.related}/>
            <Lines
                title='Affected, as reported'
                lines={details.affected}
            />
            <Lines
                title='Affected, per NVD'
                lines={details.configurations}
            />
            <References references={details.references}/>
        </>
    );
}

const CyberPanel: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const state = useCyber(payload.kind, payload.value);

    return (
        <div aria-label={isKind(payload.kind) ? KIND_LABELS[payload.kind] : 'Cyber context'}>
            {renderBody(state)}
        </div>
    );
};

export default CyberPanel;

import React, {useState} from 'react';

import Badges from './Badges';
import {KIND_LABELS, isKind, useCyber} from './cyber';
import type {CyberState} from './cyber';
import type {CyberItem, CyberLink, CyberReference, CyberSection, CyberVectorMetric} from './types';

import LinkButton from '../../components/LinkButton';
import {pluginBaseUrl} from '../../plugin_url';
import HoverLink from '../HoverLink';
import CopyButton from '../location/CopyButton';
import {setSelection} from '../selection';

import type {CyberPayload} from './index';

const MONOSPACE = 'ui-monospace, SFMono-Regular, Menlo, monospace';
const RULE = '1px solid rgba(var(--center-channel-color-rgb), 0.08)';
const SUMMARY_CLAMP_LINES = 6;
const SUMMARY_CLAMP_CHARS = 420;

const styles: Record<string, React.CSSProperties> = {
    title: {
        fontSize: '20px',
        lineHeight: '26px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: 0,
        overflowWrap: 'anywhere',
    },
    value: {
        fontFamily: MONOSPACE,
        fontSize: '12px',
        letterSpacing: '0.04em',
        opacity: 0.6,
        color: 'var(--center-channel-color)',
        margin: '2px 0 0',
        wordBreak: 'break-all',
    },
    badgesInPanel: {margin: '10px 0 0'},
    summaryWrap: {margin: '16px 0 0'},
    summary: {
        fontSize: '14px',
        lineHeight: '20px',
        color: 'var(--center-channel-color)',
        margin: 0,
        overflowWrap: 'anywhere',
    },
    clamped: {
        display: '-webkit-box',
        WebkitLineClamp: SUMMARY_CLAMP_LINES,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
    },
    more: {fontSize: '13px', margin: '4px 0 0'},
    table: {width: '100%', borderCollapse: 'collapse', fontSize: '13px', margin: '16px 0 0'},
    th: {
        textAlign: 'left',
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.64,
        color: 'var(--center-channel-color)',
        padding: '9px 12px 9px 0',
        verticalAlign: 'top',
        width: '34%',
        borderTop: RULE,
    },
    td: {
        padding: '8px 0',
        color: 'var(--center-channel-color)',
        fontVariantNumeric: 'tabular-nums',
        overflowWrap: 'anywhere',
        verticalAlign: 'top',
        borderTop: RULE,
    },
    code: {fontFamily: MONOSPACE, fontSize: '12px'},
    copyCell: {
        width: '24px',
        padding: '6px 0 6px 8px',
        verticalAlign: 'top',
        textAlign: 'right',
        borderTop: RULE,
    },
    heading: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontWeight: 600,
        opacity: 0.64,
        color: 'var(--center-channel-color)',
        margin: '24px 0 8px',
    },
    note: {
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.64,
        margin: '14px 0 0',
    },
    list: {listStyle: 'none', padding: 0, margin: 0, fontSize: '13px'},
    listItem: {margin: '0 0 8px'},
    verdict: {fontWeight: 600},
    meta: {
        fontSize: '12px',
        opacity: 0.64,
        color: 'var(--center-channel-color)',
    },
    related: {display: 'flex', gap: '8px', alignItems: 'baseline', textAlign: 'left', width: '100%'},
    relatedCode: {
        fontFamily: MONOSPACE,
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.64,
        flex: '0 0 auto',
    },
    section: {margin: '16px 0 0', borderTop: RULE, paddingTop: '12px'},
    toggle: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        cursor: 'pointer',
    },
    toggleTitle: {opacity: 0.64},
    count: {
        marginLeft: '6px',
        padding: '0 6px',
        borderRadius: '8px',
        background: 'rgba(var(--center-channel-color-rgb), 0.08)',
        fontVariantNumeric: 'tabular-nums',
        letterSpacing: 0,
    },
    sectionBody: {margin: '10px 0 0'},
    line: {
        margin: 0,
        padding: '6px 0',
        fontSize: '13px',
        lineHeight: '18px',
        color: 'var(--center-channel-color)',
        overflowWrap: 'anywhere',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.04)',
    },
    product: {fontWeight: 600},
    itemLink: {fontWeight: 600, textAlign: 'left'},
    itemAnchor: {fontWeight: 600, color: 'var(--link-color)', textDecoration: 'none', overflowWrap: 'anywhere'},
    itemText: {margin: '2px 0 0', whiteSpace: 'pre-line'},
    itemTextAlone: {margin: 0, whiteSpace: 'pre-line'},
    reference: {
        display: 'block',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        textDecoration: 'none',
        color: 'var(--link-color)',
    },
    host: {fontWeight: 600},
    path: {color: 'var(--center-channel-color)', opacity: 0.64},
    vector: {
        display: 'grid',
        gridTemplateColumns: 'minmax(0, 1fr) auto',
        gap: '4px 12px',
        margin: 0,
        fontSize: '13px',
        color: 'var(--center-channel-color)',
    },
    vectorMetric: {margin: 0, opacity: 0.72},
    vectorValue: {margin: 0, textAlign: 'right'},
    vectorSevere: {color: 'var(--error-text, #d24b4e)', fontWeight: 600},
    tags: {display: 'flex', flexWrap: 'wrap', gap: '4px', margin: '4px 0 0'},
    tag: {
        fontSize: '11px',
        lineHeight: '16px',
        padding: '0 6px',
        borderRadius: '4px',
        background: 'rgba(var(--center-channel-color-rgb), 0.08)',
        color: 'var(--center-channel-color)',
    },
    tagExploit: {background: 'rgba(210, 75, 78, 0.14)', color: 'var(--error-text, #d24b4e)'},
    tagFix: {background: 'rgba(6, 214, 160, 0.16)'},
};

const FIX_TAGS = new Set(['Patch', 'Vendor Advisory', 'Mitigation', 'Release Notes']);

function isCodeLike(value: string): boolean {
    return value !== '' && !(/\s/).test(value);
}

function breakAfterSlashes(value: string): React.ReactNode[] {
    const nodes: React.ReactNode[] = [];
    let offset = value.indexOf('/');
    let from = 0;

    while (offset !== -1) {
        nodes.push(value.slice(from, offset + 1), <wbr key={offset}/>);
        from = offset + 1;
        offset = value.indexOf('/', from);
    }
    nodes.push(value.slice(from));

    return nodes;
}

function rowValue(value: string, query: string): React.ReactNode {
    if (query !== '') {
        return <HoverLink href={`${pluginBaseUrl()}/decorate/dtg?${query}`}>{value}</HoverLink>;
    }
    return isCodeLike(value) ? breakAfterSlashes(value) : value;
}

const Row: React.FC<{label: string; value: string; query: string; copyable: boolean}> = ({label, value, query, copyable}) => {
    const code = query === '' && isCodeLike(value);

    return (
        <tr>
            <th
                scope='row'
                style={styles.th}
            >{label}</th>
            <td style={code ? {...styles.td, ...styles.code} : styles.td}>
                {rowValue(value, query)}
            </td>
            {copyable && (
                <td style={styles.copyCell}>
                    <CopyButton
                        label={`Copy ${label}`}
                        value={value}
                    />
                </td>
            )}
        </tr>
    );
};

const Vector: React.FC<{metrics: CyberVectorMetric[]}> = ({metrics}) => {
    if (metrics.length === 0) {
        return null;
    }

    return (
        <>
            <p style={styles.heading}>{'Vector, decoded'}</p>
            <dl
                style={styles.vector}
                data-testid='cyber-vector'
            >
                {metrics.map((metric) => (
                    <React.Fragment key={metric.metric}>
                        <dt style={styles.vectorMetric}>{metric.metric}</dt>
                        <dd style={metric.severe ? {...styles.vectorValue, ...styles.vectorSevere} : styles.vectorValue}>
                            {metric.value}
                        </dd>
                    </React.Fragment>
                ))}
            </dl>
        </>
    );
};

const Summary: React.FC<{text: string}> = ({text}) => {
    const [expanded, setExpanded] = useState(false);
    if (text === '') {
        return null;
    }

    const long = text.length > SUMMARY_CLAMP_CHARS;

    return (
        <div style={styles.summaryWrap}>
            <p style={long && !expanded ? {...styles.summary, ...styles.clamped} : styles.summary}>{text}</p>
            {long && (
                <LinkButton
                    style={styles.more}
                    onClick={() => setExpanded(!expanded)}
                >{expanded ? 'Show less' : 'Show more'}</LinkButton>
            )}
        </div>
    );
};

const Collapsible: React.FC<{title: string; count: number; children: React.ReactNode}> = ({title, count, children}) => (
    <details style={styles.section}>
        <summary style={styles.toggle}>
            <span style={styles.toggleTitle}>{title}</span>
            <span style={styles.count}>{count}</span>
        </summary>
        <div style={styles.sectionBody}>{children}</div>
    </details>
);

function splitProduct(line: string): [string, string] {
    const at = line.indexOf(': ');
    if (at <= 0) {
        return [line, ''];
    }
    return [line.slice(0, at), line.slice(at + 2)];
}

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
                {lines.map((line) => {
                    const [product, versions] = splitProduct(line);
                    return (
                        <li
                            key={line}
                            style={styles.line}
                        >
                            <span style={styles.product}>{product}</span>
                            {versions !== '' && `: ${versions}`}
                        </li>
                    );
                })}
            </ul>
        </Collapsible>
    );
};

function referenceParts(url: string): {host: string; path: string} {
    const parsed = new URL(url);
    const path = `${parsed.pathname}${parsed.search}${parsed.hash}`;
    return {host: parsed.host, path: path === '/' ? '' : path};
}

function tagStyle(tag: string): React.CSSProperties {
    if (tag === 'Exploit') {
        return {...styles.tag, ...styles.tagExploit};
    }
    if (FIX_TAGS.has(tag)) {
        return {...styles.tag, ...styles.tagFix};
    }
    return styles.tag;
}

function referenceRank(reference: CyberReference): number {
    const tags = reference.tags.split(', ');
    if (tags.some((tag) => FIX_TAGS.has(tag))) {
        return 0;
    }
    if (tags.includes('Exploit')) {
        return 1;
    }
    return 2;
}

export function rankedReferences(references: CyberReference[]): CyberReference[] {
    return references.
        map((reference, position) => ({reference, position, rank: referenceRank(reference)})).
        sort((a, b) => a.rank - b.rank || a.position - b.position).
        map(({reference}) => reference);
}

const SectionItem: React.FC<{item: CyberItem}> = ({item}) => {
    let head: React.ReactNode = null;
    if (item.kind !== '' && item.value !== '') {
        head = (
            <LinkButton
                style={styles.itemLink}
                onClick={() => setSelection({type: 'cyber', payload: {kind: item.kind, value: item.value}})}
            >{item.head}</LinkButton>
        );
    } else if (item.url !== '' && item.head !== '') {
        head = (
            <a
                href={item.url}
                target='_blank'
                rel='noopener noreferrer'
                style={styles.itemAnchor}
            >{item.head}</a>
        );
    } else if (item.head !== '') {
        head = <span style={styles.product}>{item.head}</span>;
    }

    return (
        <li style={styles.line}>
            {head}
            {item.text !== '' && <p style={head ? styles.itemText : styles.itemTextAlone}>{item.text}</p>}
        </li>
    );
};

const Sections: React.FC<{sections: CyberSection[]}> = ({sections}) => (
    <>
        {sections.map((section) => (
            <Collapsible
                key={section.title}
                title={section.title}
                count={section.items.length}
            >
                <ul style={styles.list}>
                    {section.items.map((item, index) => (
                        <SectionItem
                            key={`${item.head}:${item.value}:${item.text.slice(0, 40)}:${String(index)}`}
                            item={item}
                        />
                    ))}
                </ul>
            </Collapsible>
        ))}
    </>
);

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
                {rankedReferences(references).map((reference) => {
                    const {host, path} = referenceParts(reference.url);
                    const tags = reference.tags.split(', ').filter(Boolean);
                    return (
                        <li
                            key={reference.url}
                            style={styles.line}
                        >
                            <a
                                href={reference.url}
                                target='_blank'
                                rel='noopener noreferrer'
                                title={reference.url}
                                style={styles.reference}
                            >
                                <span style={styles.host}>{host}</span>
                                <span style={styles.path}>{path}</span>
                            </a>
                            {tags.length > 0 && (
                                <div style={styles.tags}>
                                    {tags.map((tag) => (
                                        <span
                                            key={tag}
                                            style={tagStyle(tag)}
                                        >{tag}</span>
                                    ))}
                                </div>
                            )}
                        </li>
                    );
                })}
            </ul>
        </Collapsible>
    );
};

function relatedName(link: CyberLink): string {
    const prefix = `${link.value} `;
    return link.label.startsWith(prefix) ? link.label.slice(prefix.length) : '';
}

const Related: React.FC<{links: CyberLink[]}> = ({links}) => {
    if (links.length === 0) {
        return null;
    }

    return (
        <>
            <p style={styles.heading}>{'Related'}</p>
            <ul style={styles.list}>
                {links.map((link) => {
                    const name = relatedName(link);
                    return (
                        <li
                            key={`${link.kind}:${link.value}`}
                            style={styles.listItem}
                        >
                            <LinkButton
                                style={styles.related}
                                onClick={() => setSelection({
                                    type: 'cyber',
                                    payload: {kind: link.kind, value: link.value},
                                })}
                            >
                                {name === '' ? link.label : (
                                    <>
                                        <span style={styles.relatedCode}>{link.value}</span>
                                        <span>{name}</span>
                                    </>
                                )}
                            </LinkButton>
                        </li>
                    );
                })}
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
            <Badges
                details={details}
                style={styles.badgesInPanel}
            />
            <Summary text={details.summary}/>

            {details.rows.length > 0 && (
                <table style={styles.table}>
                    <tbody>
                        {details.rows.map((row) => (
                            <Row
                                key={row.label}
                                label={row.label}
                                value={row.value}
                                query={row.query}
                                copyable={details.kind !== 'cve'}
                            />
                        ))}
                    </tbody>
                </table>
            )}

            <Vector metrics={details.vector}/>

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
            <Sections sections={details.sections}/>
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

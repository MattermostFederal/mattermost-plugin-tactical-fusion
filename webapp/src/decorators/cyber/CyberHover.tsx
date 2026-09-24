import React from 'react';

import Badges, {hasBadges} from './Badges';
import {useCyber} from './cyber';
import type {CyberResponse, CyberWatchEntry} from './types';

import type {CyberPayload} from './index';

const CARD_WIDTH_PX = 320;

const badge: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    padding: '1px 7px',
    borderRadius: '4px',
    fontSize: '11px',
    lineHeight: '16px',
    fontWeight: 600,
    whiteSpace: 'nowrap',
};

const styles: Record<string, React.CSSProperties> = {
    line: {
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        whiteSpace: 'nowrap',
    },
    badges: {flexWrap: 'nowrap'},
    credit: {display: 'block', fontSize: '11px', opacity: 0.72, marginTop: '2px', whiteSpace: 'nowrap'},
    card: {
        width: `${CARD_WIDTH_PX}px`,
        maxWidth: '100%',
        whiteSpace: 'normal',
        color: 'var(--center-channel-color)',
        display: 'flex',
        flexDirection: 'column',
        gap: '4px',
    },
    title: {fontSize: '14px', lineHeight: '19px', fontWeight: 600, overflowWrap: 'anywhere'},
    subtitle: {fontSize: '12px', lineHeight: '16px', opacity: 0.72, overflowWrap: 'anywhere'},
    row: {display: 'flex', flexWrap: 'wrap', gap: '4px'},
    status: {...badge, background: 'rgba(245, 171, 0, 0.18)', color: 'var(--center-channel-color)'},
    summary: {
        fontSize: '12px',
        lineHeight: '17px',
        display: '-webkit-box',
        WebkitLineClamp: 3,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
    },
    tag: {...badge, fontWeight: 400, background: 'rgba(var(--center-channel-color-rgb), 0.08)'},
    facts: {fontSize: '11px', lineHeight: '15px', opacity: 0.72},
    cardCredit: {fontSize: '11px', opacity: 0.72},
};

const VERDICT_STYLES: Record<string, React.CSSProperties> = {
    malicious: {background: 'rgba(210, 75, 78, 0.14)', color: 'var(--error-text, #d24b4e)'},
    suspicious: {background: 'rgba(245, 171, 0, 0.18)', color: 'var(--center-channel-color)'},
    benign: {background: 'rgba(6, 214, 160, 0.16)', color: 'var(--center-channel-color)'},
};

const OTHER_VERDICT: React.CSSProperties = {background: 'rgba(var(--center-channel-color-rgb), 0.12)'};

function distinctVerdicts(entries: CyberWatchEntry[]): string[] {
    return [...new Set(entries.map((entry) => entry.verdict.trim().toLowerCase()).filter(Boolean))];
}

const Verdicts: React.FC<{entries: CyberWatchEntry[]}> = ({entries}) => (
    <>
        {distinctVerdicts(entries).map((verdict) => (
            <span
                key={verdict}
                data-testid='cyber-verdict'
                style={{...badge, ...(VERDICT_STYLES[verdict] ?? OTHER_VERDICT)}}
            >{`Watchlist: ${verdict}`}</span>
        ))}
    </>
);

function hasGlance(details: CyberResponse): boolean {
    const {glance} = details;
    return glance.subtitle !== '' || glance.summary !== '' || glance.tags.length > 0 ||
        glance.facts.length > 0 || glance.status !== '' || details.watchlist.length > 0;
}

const GlanceCard: React.FC<{details: CyberResponse}> = ({details}) => {
    const {glance} = details;
    const showBadges = glance.status !== '' || details.watchlist.length > 0;

    return (
        <div
            style={styles.card}
            data-testid='cyber-glance'
        >
            <span style={styles.title}>{details.title}</span>
            {glance.subtitle !== '' && <span style={styles.subtitle}>{glance.subtitle}</span>}
            {showBadges && (
                <div style={styles.row}>
                    {glance.status !== '' && <span style={styles.status}>{glance.status}</span>}
                    <Verdicts entries={details.watchlist}/>
                </div>
            )}
            {glance.summary !== '' && <span style={styles.summary}>{glance.summary}</span>}
            {glance.tags.length > 0 && (
                <div style={styles.row}>
                    {glance.tags.map((tag) => (
                        <span
                            key={tag}
                            style={styles.tag}
                        >{tag}</span>
                    ))}
                </div>
            )}
            {glance.facts.length > 0 && <span style={styles.facts}>{glance.facts.join(' \u00b7 ')}</span>}
            {details.credits.map((credit) => (
                <span
                    key={credit.text}
                    style={styles.cardCredit}
                    data-testid='cyber-credit'
                >{credit.text}</span>
            ))}
        </div>
    );
};

const CyberHover: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const state = useCyber(payload.kind, payload.value);

    if (state.status !== 'ready' || !state.data) {
        return null;
    }

    const details = state.data;

    if (hasBadges(details)) {
        return (
            <div style={{...styles.row, flexWrap: 'nowrap'}}>
                <Badges
                    details={details}
                    style={styles.badges}
                />
                <Verdicts entries={details.watchlist}/>
            </div>
        );
    }

    if (hasGlance(details)) {
        return <GlanceCard details={details}/>;
    }

    return <span style={styles.line}>{details.headline}</span>;
};

export default CyberHover;

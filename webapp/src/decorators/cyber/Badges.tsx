import React from 'react';

import {isSeverity} from './cyber';
import type {CyberSeverity} from './cyber';
import type {CyberResponse} from './types';

const styles: Record<string, React.CSSProperties> = {
    badges: {display: 'flex', flexWrap: 'wrap', gap: '6px'},
    badge: {
        display: 'inline-flex',
        alignItems: 'center',
        gap: '6px',
        padding: '2px 8px',
        borderRadius: '4px',
        fontSize: '12px',
        lineHeight: '18px',
        fontWeight: 600,
        whiteSpace: 'nowrap',
    },
    score: {fontVariantNumeric: 'tabular-nums', fontWeight: 700},
    exploited: {
        background: 'rgba(210, 75, 78, 0.12)',
        color: 'var(--error-text, #d24b4e)',
        boxShadow: 'inset 0 0 0 1px var(--error-text, #d24b4e)',
    },
};

const SEVERITY_STYLES: Record<CyberSeverity, React.CSSProperties> = {
    critical: {background: '#b3261e', color: '#ffffff'},
    high: {background: '#d9531e', color: '#ffffff'},
    medium: {background: '#f2b21b', color: '#1f1f1f'},
    low: {background: '#3b7fc4', color: '#ffffff'},
    none: {background: 'rgba(var(--center-channel-color-rgb), 0.12)', color: 'var(--center-channel-color)'},
};

function capitalized(word: string): string {
    return word.charAt(0).toUpperCase() + word.slice(1);
}

export function hasBadges(details: CyberResponse): boolean {
    return isSeverity(details.severity) || details.exploited;
}

const Badges: React.FC<{details: CyberResponse; style?: React.CSSProperties}> = ({details, style}) => {
    if (!hasBadges(details)) {
        return null;
    }

    const severity = isSeverity(details.severity) ? details.severity : null;

    return (
        <div style={{...styles.badges, ...style}}>
            {severity && (
                <span
                    data-testid='cyber-severity'
                    style={{...styles.badge, ...SEVERITY_STYLES[severity]}}
                >
                    {details.score !== '' && <span style={styles.score}>{details.score}</span>}
                    {capitalized(severity)}
                </span>
            )}
            {details.exploited && (
                <span
                    data-testid='cyber-exploited'
                    style={{...styles.badge, ...styles.exploited}}
                >{'Known exploited'}</span>
            )}
        </div>
    );
};

export default Badges;

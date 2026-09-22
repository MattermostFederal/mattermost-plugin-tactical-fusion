import React from 'react';

import LinkButton from '../../components/LinkButton';
import {docsUrl} from '../../plugin_url';
import CopyButton from '../location/CopyButton';

import {describe} from './index';
import type {FrequencyPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    token: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '18px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: '0 0 2px',
    },
    band: {fontSize: '14px', color: 'var(--center-channel-color)', margin: '0 0 16px'},
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
    plain: {fontFamily: 'inherit'},
    copyCell: {
        width: '24px',
        padding: '6px 0 6px 8px',
        verticalAlign: 'top',
        textAlign: 'right',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    footer: {marginTop: '14px', fontSize: '12px'},
};

const Row: React.FC<{label: string; value: string; plain?: boolean; copyable?: boolean}> = ({label, value, plain, copyable}) => (
    <tr>
        <th
            scope='row'
            style={styles.th}
        >{label}</th>
        <td style={plain ? {...styles.td, ...styles.plain} : styles.td}>{value}</td>
        <td style={styles.copyCell}>
            {copyable && (
                <CopyButton
                    label={`Copy ${label}`}
                    value={value}
                />
            )}
        </td>
    </tr>
);

const FrequencyPanel: React.FC<{payload: FrequencyPayload}> = ({payload}) => {
    const details = describe(payload);

    return (
        <div data-testid='frequency-panel'>
            <p
                style={styles.token}
                data-testid='frequency-token'
            >{details.token}</p>
            <p
                style={styles.band}
                data-testid='frequency-band'
            >{details.band}</p>

            <table style={styles.table}>
                <tbody>
                    <Row
                        label='MHz'
                        value={details.mhz}
                        copyable={true}
                    />
                    <Row
                        label='kHz'
                        value={details.khz}
                        copyable={true}
                    />
                    {details.channel !== '' && (
                        <Row
                            label='Channel'
                            value={details.channel}
                            plain={true}
                        />
                    )}
                    {details.use !== '' && (
                        <Row
                            label='Use'
                            value={details.use}
                            plain={true}
                        />
                    )}
                </tbody>
            </table>

            <div style={styles.footer}>
                <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
            </div>
        </div>
    );
};

export default FrequencyPanel;

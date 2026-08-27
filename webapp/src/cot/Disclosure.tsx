import React, {useState} from 'react';

const styles: Record<string, React.CSSProperties> = {
    details: {margin: '16px 0 4px'},
    summary: {
        background: 'rgba(var(--center-channel-color-rgb), 0.04)',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: 4,
        cursor: 'pointer',
        padding: '6px 10px',
    },
    row: {
        alignItems: 'center',
        display: 'inline-flex',
        gap: '8px',
        verticalAlign: 'middle',
    },
    label: {fontSize: '13px', fontWeight: 600},
    state: {color: 'var(--link-color)', fontSize: '12px'},
    body: {marginTop: '8px'},
};

interface Props {
    label: string;

    /** A control beside the label, such as a copy button. */
    trailing?: React.ReactNode;
    children: React.ReactNode;
}

export const Disclosure: React.FC<Props> = ({label, trailing, children}) => {
    const [open, setOpen] = useState(false);

    return (
        <details
            style={styles.details}
            open={open}
            onToggle={(toggled) => setOpen(toggled.currentTarget.open)}
        >
            <summary style={styles.summary}>
                <span style={styles.row}>
                    <span style={styles.label}>{label}</span>
                    <span style={styles.state}>{open ? 'Hide' : 'Show'}</span>
                    {trailing !== undefined && (
                        <span onClick={(clicked) => clicked.preventDefault()}>{trailing}</span>
                    )}
                </span>
            </summary>
            <div style={styles.body}>{children}</div>
        </details>
    );
};

export default Disclosure;

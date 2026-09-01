import React, {useState} from 'react';

const styles: Record<string, React.CSSProperties> = {
    details: {margin: '16px 0 4px'},
    summary: {
        alignItems: 'center',
        background: 'var(--center-channel-bg)',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: 4,
        cursor: 'pointer',
        display: 'flex',
        gap: '8px',
        justifyContent: 'space-between',
        listStyle: 'none',
        padding: '6px 10px',
    },
    row: {
        alignItems: 'center',
        display: 'inline-flex',
        gap: '8px',
        minWidth: 0,
    },
    label: {fontSize: '13px', fontWeight: 600},
    chevron: {color: 'var(--link-color)', flex: 'none'},
    body: {marginTop: '8px'},
};

const ChevronGlyph: React.FC<{open: boolean}> = ({open}) => (
    <svg
        width='16'
        height='16'
        viewBox='0 0 24 24'
        fill='none'
        stroke='currentColor'
        strokeWidth='2'
        strokeLinecap='round'
        strokeLinejoin='round'
        style={styles.chevron}
        data-testid='disclosure-chevron'
        data-state={open ? 'open' : 'closed'}
        aria-hidden='true'
        focusable='false'
    >
        <path d={open ? 'M6 15l6-6 6 6' : 'M6 9l6 6 6-6'}/>
    </svg>
);

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
                    {trailing !== undefined && (
                        <span onClick={(clicked) => clicked.preventDefault()}>{trailing}</span>
                    )}
                </span>
                <ChevronGlyph open={open}/>
            </summary>
            <div style={styles.body}>{children}</div>
        </details>
    );
};

export default Disclosure;

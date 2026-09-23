import manifest from 'manifest';
import React, {useState} from 'react';

import {EXAMPLES_COMMAND, postExamples} from './examples';
import {useHistory} from './history';

import CotCustomize from '../../cot/Customize';
import DtgCustomize from '../../decorators/dtg/Customize';
import LocationCustomize from '../../decorators/location/Customize';
import {setSelection} from '../../decorators/selection';
import GeoJsonCustomize from '../../geojson/Customize';
import {docsUrl} from '../../plugin_url';
import LinkButton from '../LinkButton';

export const HISTORY_HEADING = 'Recently opened';

export const HISTORY_EMPTY = 'Highlighted values in a message, such as date-time groups and coordinates, open their details here. The ones you open are listed here until you reload.';

export const SETTINGS_HEADING = 'Your settings';

export const EXAMPLES_WARNING = 'This posts about a dozen messages to this channel that everyone can see.';

type SettingsSection = 'dtg' | 'location' | 'cot' | 'geojson';

export const SETTINGS_SECTIONS: ReadonlyArray<{id: SettingsSection; label: string}> = [
    {id: 'dtg', label: 'Date-time groups'},
    {id: 'location', label: 'Coordinates'},
    {id: 'cot', label: 'Cursor on Target'},
    {id: 'geojson', label: 'GeoJSON'},
];

const styles: Record<string, React.CSSProperties> = {
    root: {color: 'var(--center-channel-color)'},
    heading: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        margin: '0 0 8px',
    },
    section: {margin: '0 0 24px'},
    list: {listStyle: 'none', padding: 0, margin: 0},
    item: {padding: '4px 0', fontSize: '14px', overflowWrap: 'anywhere'},
    hint: {fontSize: '13px', opacity: 0.65, margin: 0},
    settingsTitle: {fontSize: '16px', fontWeight: 600, margin: '0 0 12px'},
    footer: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
        paddingTop: '12px',
        fontSize: '13px',
    },
    footerRow: {margin: '0 0 8px'},
    confirm: {margin: '0 0 8px', fontSize: '13px'},
    status: {margin: '0 0 8px', fontSize: '13px', opacity: 0.8},
    gap: {marginLeft: '12px'},
    version: {fontSize: '12px', opacity: 0.5, margin: '8px 0 0'},
};

const History: React.FC = () => {
    const entries = useHistory();

    return (
        <section
            style={styles.section}
            aria-labelledby='tf-home-history'
        >
            <h3
                id='tf-home-history'
                style={styles.heading}
            >{HISTORY_HEADING}</h3>
            {entries.length === 0 ? (
                <p style={styles.hint}>{HISTORY_EMPTY}</p>
            ) : (
                <ul
                    style={styles.list}
                    data-testid='tf-home-history'
                >
                    {entries.map((entry) => (
                        <li
                            key={`${entry.selection.type}:${entry.label}`}
                            style={styles.item}
                        >
                            <LinkButton onClick={() => setSelection(entry.selection)}>{entry.label}</LinkButton>
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
};

const SettingsEditor: React.FC<{section: SettingsSection; onClose: () => void}> = ({section, onClose}) => {
    const label = SETTINGS_SECTIONS.find((s) => s.id === section)?.label ?? '';

    return (
        <div style={styles.root}>
            <h3 style={styles.settingsTitle}>{`${SETTINGS_HEADING}: ${label}`}</h3>
            {section === 'dtg' && (
                <DtgCustomize
                    instant={new Date()}
                    onClose={onClose}
                />
            )}
            {section === 'location' && <LocationCustomize onClose={onClose}/>}
            {section === 'cot' && <CotCustomize onClose={onClose}/>}
            {section === 'geojson' && <GeoJsonCustomize onClose={onClose}/>}
        </div>
    );
};

const Settings: React.FC<{onOpen: (section: SettingsSection) => void}> = ({onOpen}) => (
    <section
        style={styles.section}
        aria-labelledby='tf-home-settings'
    >
        <h3
            id='tf-home-settings'
            style={styles.heading}
        >{SETTINGS_HEADING}</h3>
        <ul style={styles.list}>
            {SETTINGS_SECTIONS.map((section) => (
                <li
                    key={section.id}
                    style={styles.item}
                >
                    <LinkButton onClick={() => onOpen(section.id)}>{section.label}</LinkButton>
                </li>
            ))}
        </ul>
    </section>
);

type ExamplesState = 'idle' | 'confirming' | 'posting' | 'posted' | {error: string};

const ExamplesButton: React.FC = () => {
    const [state, setState] = useState<ExamplesState>('idle');

    const post = async () => {
        setState('posting');
        try {
            await postExamples();
            setState('posted');
        } catch (err) {
            setState({error: err instanceof Error ? err.message : String(err)});
        }
    };

    if (state === 'confirming') {
        return (
            <div
                style={styles.confirm}
                role='group'
                aria-label='Post the examples'
            >
                <p style={styles.confirm}>{EXAMPLES_WARNING}</p>
                <LinkButton onClick={post}>{'Post them'}</LinkButton>
                <LinkButton
                    style={styles.gap}
                    onClick={() => setState('idle')}
                >{'Cancel'}</LinkButton>
            </div>
        );
    }

    return (
        <div style={styles.footerRow}>
            <LinkButton
                disabled={state === 'posting'}
                onClick={() => setState('confirming')}
            >{`Post the examples (${EXAMPLES_COMMAND})`}</LinkButton>
            {state === 'posting' && <p style={styles.status}>{'Posting…'}</p>}
            {state === 'posted' && <p style={styles.status}>{'Posted to this channel.'}</p>}
            {typeof state === 'object' && (
                <p
                    style={styles.status}
                    role='alert'
                >{state.error}</p>
            )}
        </div>
    );
};

const Footer: React.FC = () => (
    <footer style={styles.footer}>
        <ExamplesButton/>
        <div style={styles.footerRow}>
            <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
        </div>
        <p style={styles.version}>{`Version ${manifest.version}`}</p>
    </footer>
);

export const Home: React.FC = () => {
    const [editing, setEditing] = useState<SettingsSection | null>(null);

    if (editing !== null) {
        return (
            <SettingsEditor
                section={editing}
                onClose={() => setEditing(null)}
            />
        );
    }

    return (
        <div
            style={styles.root}
            data-testid='tf-home'
        >
            <History/>
            <Settings onOpen={setEditing}/>
            <Footer/>
        </div>
    );
};

export default Home;

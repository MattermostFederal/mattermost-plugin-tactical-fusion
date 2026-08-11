import React, {useState} from 'react';

import {DEFAULT_GROUPS} from './zone_fixtures';
import ZonePicker from './ZonePicker';
import type {ZoneChoice, ZoneGroup} from './zones';

/**
 * Harness for the zone picker.
 *
 * Picks are appended to a list the test can read, and the picked zone is removed
 * from the groups, which is what the real editor does and what the "add two
 * bases from one search" flow depends on.
 *
 * Groups arrive by name rather than as a prop, because Playwright CT cannot pass
 * a plain array through a component boundary intact.
 */
const ZonePickerHarness: React.FC<{
    groups?: 'default' | 'none' | 'empty-first-group';
    disabled?: boolean;
    twice?: boolean;
}> = ({groups = 'default', disabled, twice}) => {
    const [picked, setPicked] = useState<string[]>([]);
    const [removed, setRemoved] = useState<string[]>([]);

    let chosen: ZoneGroup[] = DEFAULT_GROUPS;
    if (groups === 'none') {
        chosen = [];
    } else if (groups === 'empty-first-group') {
        chosen = [{label: DEFAULT_GROUPS[0].label, zones: []}, DEFAULT_GROUPS[1]];
    }

    // Groups are not pruned when they empty out, so the picker is the one
    // deciding what to do with an empty one.
    const remaining = chosen.map((group) => ({
        ...group,
        zones: group.zones.filter((zone) => !removed.includes(zone.key)),
    }));

    const onPick = (zone: ZoneChoice) => {
        setPicked((current) => [...current, zone.key]);
        setRemoved((current) => [...current, zone.key]);
    };

    return (
        <div>
            <ZonePicker
                groups={remaining}
                disabled={disabled}
                onPick={onPick}
            />

            {twice && (
                <ZonePicker
                    groups={remaining}
                    onPick={onPick}
                />
            )}

            <p data-testid='picked'>{picked.join(',')}</p>
            <p data-testid='pick-count'>{String(picked.length)}</p>

            {/* Somewhere to move focus to, so blur can be exercised. */}
            <button data-testid='elsewhere'>{'elsewhere'}</button>
        </div>
    );
};

export default ZonePickerHarness;

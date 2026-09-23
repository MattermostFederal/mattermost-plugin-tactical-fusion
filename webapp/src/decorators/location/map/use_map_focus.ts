import type React from 'react';
import {useState} from 'react';

import type {MapFocus} from './focus';

import {useFeatures} from '../../../features/store';
import {usePreferences} from '../../../preferences/store';
import {INLINE_ID, isRowVisible} from '../rows';

export const SHOW_ON_MAP = 'Show on the map: ';

export const SR_ONLY: React.CSSProperties = {
    border: 0,
    clip: 'rect(0 0 0 0)',
    height: 1,
    margin: -1,
    overflow: 'hidden',
    padding: 0,
    position: 'absolute',
    whiteSpace: 'nowrap',
    width: 1,
};

export function useMapFocus<T>(
    focusFor: (item: T, seq: number) => MapFocus | null,
): [MapFocus | undefined, (item: T) => void] {
    const [focus, setFocus] = useState<MapFocus | undefined>(undefined);

    const show = (item: T) => {
        setFocus((previous) => focusFor(item, (previous?.seq ?? 0) + 1) ?? previous);
    };

    return [focus, show];
}

export function useInlineMapShown(): boolean {
    const {preferences} = usePreferences();
    const {features} = useFeatures();

    return features.mapInline && isRowVisible(preferences.location.hiddenRows, INLINE_ID);
}

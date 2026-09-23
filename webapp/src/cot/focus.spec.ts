import {expect, test} from '@playwright/test';

import {focusFor} from './CotMap';
import {COT_PROPS_KEY, fromProps} from './types';

function eventOf(fields: Record<string, unknown>) {
    return fromProps({
        [COT_PROPS_KEY]: {
            version: 1,
            source: 'fence',
            lead: '',
            trail: '',
            src: '<event/>',
            event: {uid: 'ANDROID-1', cot_type: 'a-f-G-U-C', ...fields},
        },
    })!.events[0];
}

const PLACED = {lat: '21.3353', lon: '-157.9483', format: 'dd', value: '21.3353,-157.9483'};

test('a placed event focuses on its position with no box', () => {
    expect(focusFor(eventOf(PLACED), 4)).toEqual({seq: 4, box: null, center: {lat: 21.3353, lon: -157.9483}});
});

test('an event with no position, or one the map cannot place, cannot be focused', () => {
    expect(focusFor(eventOf({}), 1)).toBeNull();
    expect(focusFor(eventOf({...PLACED, lon: ''}), 1)).toBeNull();
    expect(focusFor(eventOf({...PLACED, lat: '89.9', value: '89.9,-157.9483'}), 1)).toBeNull();
});

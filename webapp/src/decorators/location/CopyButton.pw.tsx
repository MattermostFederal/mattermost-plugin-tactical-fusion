import React from 'react';

import CopyButtonHarness from './CopyButtonHarness';

import {expect, test} from '../../../playwright/ct-coverage';

test('acknowledges a copy once the write lands', async ({mount}) => {
    const component = await mount(<CopyButtonHarness/>);

    await component.getByRole('button', {name: 'Copy lat/lon'}).click();
    await expect(component.getByRole('status')).toHaveText('');

    await component.getByRole('button', {name: 'resolve write'}).click();

    // The accessible name stays put across the copied state, deliberately: a
    // button that renamed itself to "Copied" would move out from under a screen
    // reader mid-read. The acknowledgement is the status region and the glyph.
    await expect(component.getByRole('button', {name: 'Copy lat/lon'})).toBeVisible();
    await expect(component.getByRole('status')).toHaveText('Copy lat/lon: copied');
});

// The sidebar keeps the panel mounted across a change of selection, so a write
// still in flight for one coordinate must never acknowledge on the next: the
// reader would be told a value was copied that never was.
test('a write in flight does not acknowledge on a different coordinate', async ({mount}) => {
    const component = await mount(<CopyButtonHarness/>);

    await component.getByRole('button', {name: 'Copy lat/lon'}).click();
    await component.getByRole('button', {name: 'switch coordinate'}).click();
    await component.getByRole('button', {name: 'resolve write'}).click();

    await expect(component.getByRole('button', {name: 'Copy lat/lon'})).toBeVisible();
    await expect(component.getByRole('status')).toHaveText('');
});

// The sibling of the test above, and the half that was missing: there, the
// reader moves on before the write lands, so the generation guard is what stops
// the acknowledgement. Here the write has already landed and "Copied" is on
// screen, so what has to clear it is the effect, including the pending timer it
// leaves behind. A timer surviving a change of coordinate would put the
// acknowledgement back a second later, under the new value.
test('a landed acknowledgement does not follow the reader to the next coordinate', async ({mount}) => {
    const component = await mount(<CopyButtonHarness/>);

    await component.getByRole('button', {name: 'Copy lat/lon'}).click();
    await component.getByRole('button', {name: 'resolve write'}).click();
    await expect(component.getByRole('status')).toHaveText('Copy lat/lon: copied');

    await component.getByRole('button', {name: 'switch coordinate'}).click();

    await expect(component.getByRole('status')).toHaveText('');
});

// A refused write is the browser saying no, which is not something the reader
// can act on: no permission prompt of ours would help and no error message
// would tell them anything. The control stays as it was and the value stays
// selectable.
test('says nothing when the browser refuses the write', async ({mount}) => {
    const component = await mount(<CopyButtonHarness/>);

    await component.getByRole('button', {name: 'Copy lat/lon'}).click();
    await component.getByRole('button', {name: 'refuse write'}).click();

    await expect(component.getByRole('status')).toHaveText('');
    await expect(component.getByRole('button', {name: 'Copy lat/lon'})).toBeVisible();

    // And it is still armed: a refusal is not a broken button.
    await component.getByRole('button', {name: 'Copy lat/lon'}).click();
    await component.getByRole('button', {name: 'resolve write'}).click();
    await expect(component.getByRole('status')).toHaveText('Copy lat/lon: copied');
});

// Undefined on any non-secure origin, which for an on-prem Mattermost over
// plain HTTP is the deployment norm. The value stays selectable either way.
test('hides itself when the browser has no clipboard', async ({mount}) => {
    const component = await mount(<CopyButtonHarness clipboard='no'/>);

    await expect(component.getByRole('button', {name: 'Copy lat/lon'})).toHaveCount(0);
    await expect(component.getByRole('button', {name: 'switch coordinate'})).toBeVisible();
});

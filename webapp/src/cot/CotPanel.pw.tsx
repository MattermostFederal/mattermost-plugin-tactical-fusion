import React from 'react';

import CotPanelHarness from './CotPanelHarness';

import {expect, test} from '../../playwright/ct-coverage';

test('the sidebar is empty until the card asks for it', async ({mount}) => {
    const component = await mount(<CotPanelHarness/>);

    await expect(component.getByTestId('rhs')).toContainText('Tactical Fusion');
    await expect(component.getByTestId('rhs')).not.toContainText('Readings for the');
});

test('the card opens the sidebar on its own event', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness event={{callsign: 'DELTA1', uid: 'ANDROID-1'}}/>,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(
        component.getByTestId('rhs').getByRole('group', {name: /^Readings for the Cursor on Target event /}),
    ).toBeVisible();
    await expect(component.getByTestId('rhs')).toContainText('ANDROID-1');
});

test('the sidebar header names the event rather than the feature', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{callsign: 'DELTA1'}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('Cursor on Target: DELTA1');
});

test('an event with no callsign is named by its uid', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{callsign: ''}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('Cursor on Target: ANDROID-1');
});

/*
 * The countdown is the one reading in the feature that depends on the reader's
 * clock, which is why it is here and not on the card: a ticking clock in a
 * channel reads as a live feed and puts a timer behind every post in the window.
 */
test.describe('the countdown', () => {
    test('runs in the panel, and says what it is counted against', async ({mount}) => {
        const staleAt = String(Date.now() + 3_600_000);
        const component = await mount(
            <CotPanelHarness event={{stale: '231245ZAUG26', staleAt}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).toContainText('Goes stale');
        await expect(component.getByTestId('rhs')).toContainText('clock');
    });

    test('is absent when the event states no stale time', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{staleAt: ''}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('Goes stale');
    });

    test('is absent when the stale time is not a number', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{staleAt: 'soon'}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('Goes stale');
    });
});

test('an accuracy the event never stated is not invented in the panel either', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{ce: ''}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('Not stated');
});

test('the panel links a position the server could spell', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            event={{lat: '34.0561', lon: '-118.2500', format: 'dd', value: '34.0561,-118.2500'}}
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(
        component.getByTestId('rhs').getByRole('link', {name: '34.0561, -118.2500'}),
    ).toHaveAttribute('href', /\/decorate\/location\?f=dd&v=/);
});

test('the panel shows the source as posted, reachable without a pointer', async ({mount}) => {
    const component = await mount(<CotPanelHarness src='<event uid="X"/>'/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    const pane = component.getByTestId('rhs').getByRole('region', {name: 'The event as it was posted'});
    await expect(pane).toContainText('<event uid="X"/>');
    await expect(pane).toHaveAttribute('tabindex', '0');
});

test('the panel names the source file for a file post', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            source='file'
            fileName='event.cot'
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('event.cot');
});

test.describe('the extension groups', () => {
    test('a device position report gains its groups', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                detail={{
                    takvPlatform: 'ATAK-CIV',
                    takvVersion: '5.1.0',
                    statusBattery: '87%',
                    precisionGeopointsrc: 'GPS',
                    attitudeYaw: '183.5°',
                    attitudePitch: '-7.1°',
                }}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Device'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Position quality'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Orientation'})).toBeVisible();
        await expect(rhs).toContainText('ATAK-CIV');
        await expect(rhs).toContainText('87%');
    });

    // Yaw is orientation about the vertical axis; track/@course is the event's
    // own word for direction of travel. Both labelled Heading would disagree.
    test('yaw is called yaw and never heading', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{attitudeYaw: '183.5°'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs).toContainText('Yaw');
        await expect(rhs).not.toContainText('Heading');
    });

    test('an event carrying nothing extra gains no group headings', async ({mount}) => {
        const component = await mount(<CotPanelHarness/>);

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Device'})).toHaveCount(0);
        await expect(rhs.getByRole('heading', {name: 'Payload'})).toHaveCount(0);
        await expect(rhs).not.toContainText('Processing path');
    });

    test('the payload groups its blocks rather than adding a heading each', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                detail={{
                    sensorFov: '18°',
                    videoConnAddress: '10.0.0.5',
                    chatRoom: 'Operations',
                    medevacUrgent: '2',
                }}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Payload'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Sensor'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'MEDEVAC'})).toBeVisible();
        await expect(rhs).toContainText('10.0.0.5');
    });

    // Provenance rather than situational awareness, so it must not push the
    // countdown and the remarks down the panel.
    test('the processing path is collapsed', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{flow: [{system: 'systemA', time: '231910Z AUG 26'}]}}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByText('Processing path (1)')).toBeVisible();
        await expect(rhs.getByRole('rowheader', {name: 'systemA'})).toBeHidden();

        await rhs.getByText('Processing path (1)').click();
        await expect(rhs.getByRole('rowheader', {name: 'systemA'})).toBeVisible();
        await expect(rhs).toContainText('231910Z AUG 26');
    });

    // Once the panel enumerates blocks, an event with none reads as "carried
    // nothing" rather than "we did not recognise what it carried".
    test('what this build did not read is stated rather than silent', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness event={{detailUnknown: '3'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs')).toContainText('3 other <detail> elements');
    });

    test('a stated colour is shown as text beside its swatch', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{colorArgb: '#ff0000'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs')).toContainText('#ff0000');
    });

    // A props blob is not a trusted input: the post type is forgeable and props
    // under a plugin's key are not protected. This is the only author-derived
    // value in the bundle that reaches a style property.
    test('a colour that is not a hex triple never reaches a style property', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{colorArgb: 'url(https://attacker.example/px)'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs).toContainText('url(https://attacker.example/px)');
        await expect(rhs.locator('[style*="attacker.example"]')).toHaveCount(0);
    });

    // A three-event panel announcing one string three times gives a screen
    // reader nothing to tell them apart.
    test('each event names itself in its own readings label', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness event={{callsign: 'DELTA1'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(
            component.getByTestId('rhs').getByRole('group', {name: 'Readings for the Cursor on Target event DELTA1'}),
        ).toBeVisible();
    });
});

test('a degraded post says so rather than reading as an empty event', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness event={{detailDropped: 'stated'}}/>,
    );

    await component.getByRole('button', {name: 'Open details'}).click();
    await expect(component.getByTestId('rhs')).toContainText('more detail than it had room to store');
});

test.describe('the panel draws only what the event carried', () => {
    // The one payload fixture fills all four sub-blocks, so nothing pinned that
    // an empty one omits its heading rather than printing a bare <h4>.
    test('a payload block with nothing in it draws no heading', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{sensorFov: '18°', sensorModel: 'EO/IR'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Payload'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Sensor'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Video'})).toHaveCount(0);
        await expect(rhs.getByRole('heading', {name: 'GeoChat'})).toHaveCount(0);
        await expect(rhs.getByRole('heading', {name: 'MEDEVAC'})).toHaveCount(0);
    });

    test('one unrecognised element is singular', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{detailUnknown: '1'}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs).toContainText('1 other <detail> element ');
        await expect(rhs).not.toContainText('elements');
    });

    // Go pins that <archive/> stores a presence value; this is the plain
    // language it becomes, which the design note prefers to "Archive: true".
    test('a stated archive flag reads as a sentence', async ({mount}) => {
        const component = await mount(<CotPanelHarness detail={{archive: 'stated'}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Device'})).toBeVisible();
        await expect(rhs).toContainText('The event asks to be kept');
        await expect(rhs).not.toContainText('true');
    });
});

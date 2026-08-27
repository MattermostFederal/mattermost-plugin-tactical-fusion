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

test.describe('the card is its own click target', () => {
    test('clicking the card body opens the sidebar', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness event={{callsign: 'DELTA1', uid: 'ANDROID-1'}}/>,
        );

        await component.getByTestId('cot-card').getByText('DELTA1').click();

        await expect(component.getByTestId('rhs')).toContainText('ANDROID-1');
    });

    test('clicking a link inside the card does not', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{lat: '34.0561', lon: '-118.2500', format: 'dd', value: '34.0561,-118.2500'}}
            />,
        );

        // The anchor is a real one, so the navigation is stopped rather than
        // the click, which would take the harness with it.
        await component.evaluate(() => {
            document.addEventListener('click', (clicked) => {
                if ((clicked.target as Element).closest('a')) {
                    clicked.preventDefault();
                }
            });
        });

        await component.getByTestId('cot-card').
            getByRole('link', {name: '34.0561, -118.2500'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('Readings for the');
    });
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
    test('runs in the panel while the event is still good for something', async ({mount}) => {
        const staleAt = String(Date.now() + 3_600_000);
        const component = await mount(
            <CotPanelHarness event={{stale: '231245ZAUG26', staleAt}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await expect(rhs).toContainText('Goes stale');
        await expect(rhs).toContainText(/in \d+m \d+s/);
        await expect(rhs).not.toContainText('clock');
    });

    test('is a standing word rather than a clock once the event is stale', async ({mount}) => {
        const staleAt = String(Date.now() - 3_600_000);
        const component = await mount(
            <CotPanelHarness event={{stale: '231245ZAUG26', staleAt}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await expect(rhs).toContainText('Stale');
        await expect(rhs).not.toContainText('ago');

        // The heading is what a counting number needs to mean anything. Over the
        // standing word it was a label with its own value repeated underneath.
        await expect(rhs).not.toContainText('Goes stale');
    });

    test('is drawn above the readings rather than after them', async ({mount}) => {
        const staleAt = String(Date.now() + 3_600_000);
        const component = await mount(<CotPanelHarness event={{staleAt}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        const headings = component.getByTestId('rhs').getByRole('heading', {level: 3});
        await expect(headings.first()).toHaveText('Goes stale');
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

test.describe('a post carrying several events', () => {
    const three = [
        {callsign: 'ALPHA', remarks: 'Holding at the bridge'},
        {callsign: 'BRAVO'},
        {callsign: 'CHARLIE'},
    ];

    test('says how many, where one event says nothing', async ({mount}) => {
        const component = await mount(<CotPanelHarness events={three}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).toContainText('3 events in this post');
    });

    test('a single event is not counted at the reader', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{callsign: 'ALPHA'}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('events in this post');
    });

    test('the sidebar title counts the events rather than naming the first', async ({mount}) => {
        const component = await mount(<CotPanelHarness events={three}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs-title')).toContainText('3 events');
        await expect(component.getByTestId('rhs-title')).not.toContainText('ALPHA');
    });

    test('every event is drawn, not just the first', async ({mount}) => {
        const component = await mount(<CotPanelHarness events={three}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await expect(rhs.getByRole('group', {name: /^Readings for the/})).toHaveCount(3);
        await expect(rhs).toContainText('BRAVO');
        await expect(rhs).toContainText('Holding at the bridge');
    });

    test('each event after the first is ruled off from the one above', async ({mount}) => {
        const component = await mount(<CotPanelHarness events={three}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        const separated = await component.getByTestId('rhs').locator('div').evaluateAll(
            (nodes) => nodes.filter((node) => getComputedStyle(node).marginTop === '20px').length,
        );

        expect(separated).toBe(2);
    });
});

test('every reading the event carried is labeled with its own value', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            event={{
                hae: '1250 m',
                ce: '12 m',
                le: '30 m',
                speed: '4.5 m/s',
                course: '270°',
                group: 'Cyan',
                role: 'Team Lead',
                parent: 'ANDROID-9',
                related: 'ANDROID-7',
                timeAt: '1000000',
                staleAt: '1003600',
                stale: '231245ZAUG26',
            }}
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    const pairs = await component.getByTestId('rhs').
        getByRole('group', {name: /^Readings for the/}).
        evaluate((list) => {
            const out: Record<string, string> = {};
            const terms = list.querySelectorAll('dt');
            terms.forEach((term) => {
                out[term.textContent ?? ''] = term.nextElementSibling?.textContent ?? '';
            });
            return out;
        });

    expect(pairs).toMatchObject({
        'Altitude (HAE)': '1250 m',
        Accuracy: '12 m circular, 30 m vertical',
        Speed: '4.5 m/s',
        Course: '270°',
        Team: 'Cyan',
        Role: 'Team Lead',
        'Sent by': 'ANDROID-9',
        'Relates to': 'ANDROID-7',
        UID: 'ANDROID-1',
    });

    expect(pairs.Stale).toContain('valid for');
});

test('an event whose type this build does not recognize says so', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness event={{typeLabel: '', cotType: 'z-z-z'}}/>,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('Unrecognized event type');
    await expect(component.getByTestId('rhs')).toContainText('(z-z-z)');
});

test('a position the build will not stand behind carries its note, unlinked', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            event={{lat: '0.0000', lon: '0.0000', positionNote: 'This event states no position.'}}
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    const rhs = component.getByTestId('rhs');
    await expect(rhs).toContainText('This event states no position.');
    await expect(rhs.getByRole('link', {name: '0.0000, 0.0000'})).toHaveCount(0);
    await expect(rhs).toContainText('0.0000, 0.0000');
});

test.describe('a time the panel was given', () => {
    test('is a link to the date-time group when the server could spell it', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{
                    time: '231943Z AUG 26',
                    timeQuery: 'v=231943ZAUG26',
                    start: '231943Z AUG 26',
                    startQuery: 'v=231943ZAUG26',
                }}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await expect(rhs).toContainText('Valid from');
        await expect(rhs.getByRole('link', {name: '231943Z AUG 26'}).first()).toHaveAttribute(
            'href',
            /\/decorate\/dtg\?v=231943ZAUG26$/,
        );
    });

    test('is plain text when the server could not', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness event={{time: '2026-08-23T11:43:38Z', timeQuery: ''}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await expect(rhs).toContainText('2026-08-23T11:43:38Z');
        await expect(rhs.getByRole('link', {name: '2026-08-23T11:43:38Z'})).toHaveCount(0);
    });
});

test('the panel shows the source as posted, reachable without a pointer', async ({mount}) => {
    const component = await mount(<CotPanelHarness src='<event uid="X"/>'/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    const rhs = component.getByTestId('rhs');
    const pane = rhs.getByRole('region', {name: 'The event as it was posted'});
    await expect(pane).toBeHidden();

    await rhs.getByText('As posted').click();

    await expect(pane).toContainText('<event uid="X"/>');
    await expect(pane).toHaveAttribute('tabindex', '0');
});

// A disclosure styled like the group headings around it reads as a heading with
// a small triangle, which is how the first one shipped. The state word is the
// part that cannot be mistaken for a label.
test('a collapsed disclosure says it can be opened, and says so again when it is', async ({mount}) => {
    const component = await mount(<CotPanelHarness src='<event uid="X"/>'/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    const rhs = component.getByTestId('rhs');
    const summary = rhs.locator('summary').filter({hasText: 'As posted'});

    await expect(summary).toContainText('Show');
    await expect(summary).not.toContainText('Hide');

    await rhs.getByText('As posted').click();

    await expect(summary).toContainText('Hide');
    await expect(summary).not.toContainText('Show');
});

test('the source can be copied without collapsing the disclosure', async ({mount}) => {
    const component = await mount(<CotPanelHarness src='<event uid="X"/>'/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    const rhs = component.getByTestId('rhs');
    await rhs.getByText('As posted').click();

    const pane = rhs.getByRole('region', {name: 'The event as it was posted'});
    await expect(pane).toBeVisible();

    await rhs.getByRole('button', {name: 'Copy the event as posted'}).click();

    await expect(pane).toBeVisible();
    await expect(rhs).toContainText('Copy the event as posted: copied');
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
    // own word for direction of travel. Both labeled Heading would disagree.
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
    // nothing" rather than "we did not recognize what it carried".
    test('what this build did not read is stated rather than silent', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness event={{detailUnknown: '3'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs')).toContainText('3 other <detail> elements');
    });

    test('a stated color is shown as text beside its swatch', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{colorArgb: '#ff0000'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs')).toContainText('#ff0000');
    });

    // A props blob is not a trusted input: the post type is forgeable and props
    // under a plugin's key are not protected. This is the only author-derived
    // value in the bundle that reaches a style property.
    test('a color that is not a hex triple never reaches a style property', async ({mount}) => {
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

    test('one unrecognized element is singular', async ({mount}) => {
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

test.describe('the shape an event describes', () => {
    test('a drawn outline names its kind and counts its points', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{geometry: {kind: 'polyline', closed: true, count: '3', points: [{lat: 1, lon: 2}, {lat: 3, lon: 4}], major: '', minor: '', angle: '', note: ''}}}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Shape'})).toBeVisible();
        await expect(rhs).toContainText('Drawn outline');
        await expect(rhs).toContainText('3');
    });

    test('an ellipse names its axes rather than a point count', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{geometry: {kind: 'ellipse', closed: false, count: '', points: [], major: '100 m', minor: '50 m', angle: '-45°', note: ''}}}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs).toContainText('Circle or ellipse');
        await expect(rhs).toContainText('100 m');
        await expect(rhs).toContainText('-45°');
    });

    // Not drawn is said, not left blank. A shape section with a kind and no
    // reason would read as a shape this build simply failed to draw.
    test('a shape that is not drawn says why', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                event={{geometry: {kind: 'polyline', closed: false, count: '900', points: [], major: '', minor: '', angle: '', note: 'This build does not draw a shape with this many points.'}}}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs')).toContainText('does not draw a shape with this many points');
    });

    test('an event that describes no shape draws no Shape heading', async ({mount}) => {
        const component = await mount(<CotPanelHarness/>);

        await component.getByRole('button', {name: 'Open details'}).click();
        await expect(component.getByTestId('rhs').getByRole('heading', {name: 'Shape'})).toHaveCount(0);
    });
});

test.describe('the long tail', () => {
    test('a radio reading carries its unit, under Device', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{radioRssi: '-71 dBm', radioGps: '3'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Device'})).toBeVisible();
        await expect(rhs).toContainText('-71 dBm');
    });

    test('a geofence and an attachment count sit under Payload', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness detail={{geofenceMonitor: 'All', geofenceSphere: '500 m', attachmentsCount: '2'}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Geofence'})).toBeVisible();
        await expect(rhs.getByRole('heading', {name: 'Attachments'})).toBeVisible();
        await expect(rhs).toContainText('500 m');
        await expect(rhs).toContainText('2');
    });

    // Routing and protocol say how the message traveled, which is what the
    // processing path already is, so they share its collapsed section.
    test('routing and protocol are filed with the processing path', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                detail={{destinationServers: 'takserver-hi:8089:tcp', takcontrol: 'stated', takcontrolSupportVersion: '1'}}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByText('Routing')).toBeVisible();
        await rhs.getByText('Routing').click();
        await expect(rhs).toContainText('takserver-hi:8089:tcp');
        await expect(rhs).toContainText('protocol exchange');
    });

    // The rows are the event's own element names, so the panel is repeating
    // what was written rather than claiming to know what a column or a task is.
    test('a checklist is counted under Payload, by the names the event used', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness
                checklist={{
                    count: '6',
                    kinds: [
                        {name: 'checklistColumn', count: '4'},
                        {name: 'checklistTask', count: '2'},
                    ],
                }}
            />,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Checklist'})).toBeVisible();

        const pairs = await rhs.locator('dt').evaluateAll(
            (terms) => terms.map((term) => [
                term.textContent ?? '',
                term.nextElementSibling?.textContent ?? '',
            ]),
        );
        expect(pairs).toContainEqual(['checklistColumn', '4']);
        expect(pairs).toContainEqual(['checklistTask', '2']);
    });

    test('a checklist that counted nothing still says it was there', async ({mount}) => {
        const component = await mount(
            <CotPanelHarness checklist={{count: '', kinds: []}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByRole('heading', {name: 'Checklist'})).toBeVisible();
        await expect(rhs).toContainText('None this build could count');
    });

    test('an event with no checklist draws no Checklist heading', async ({mount}) => {
        const component = await mount(<CotPanelHarness/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(
            component.getByTestId('rhs').getByRole('heading', {name: 'Checklist'}),
        ).toHaveCount(0);
    });

    test('an event with no routing draws no section for it', async ({mount}) => {
        const component = await mount(<CotPanelHarness/>);

        await component.getByRole('button', {name: 'Open details'}).click();
        const rhs = component.getByTestId('rhs');

        await expect(rhs.getByText('Routing')).toHaveCount(0);
        await expect(rhs.getByRole('heading', {name: 'Geofence'})).toHaveCount(0);
    });
});

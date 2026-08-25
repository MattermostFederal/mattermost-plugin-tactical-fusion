import React from 'react';

import CotPostBodyHarness from './CotPostBodyHarness';

import {expect, test} from '../../playwright/ct-coverage';

test('renders the card for a well formed post', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{callsign: 'DELTA1', time: '231143ZAUG26'}}/>,
    );

    await expect(component.getByTestId('cot-card')).toBeVisible();
    await expect(component).toContainText('DELTA1');
    await expect(component).toContainText('Friendly Ground');
    await expect(component).toContainText('231143ZAUG26');
});

test('keeps the text the author wrote around the event, in reading order', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            lead='latest PLI'
            trail='from ALPHA'
        />,
    );

    await expect(component).toContainText('latest PLI');
    await expect(component).toContainText('from ALPHA');
    await expect(component.getByTestId('cot-card')).toBeVisible();
});

/*
 * Post.Type survives an edit and Props may not, so a stamped post can arrive
 * describing something that is no longer there. Every one of these stands the
 * card down rather than asserting a position it cannot vouch for.
 */
test.describe('a post the card cannot vouch for', () => {
    test('an edited post falls back to its own text', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                message='I fixed the coordinate'
                editAt={1700000000000}
            />,
        );

        await expect(component.getByTestId('cot-card')).toHaveCount(0);
        await expect(component).toContainText('I fixed the coordinate');
    });

    test('a post whose props were lost falls back to its own text', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                uid={null}
                message='```cot\n<event/>\n```'
            />,
        );

        await expect(component.getByTestId('cot-card')).toHaveCount(0);
        await expect(component).toContainText('<event/>');
    });

    test('a version this bundle does not know falls back', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                version={99}
                message='some text'
            />,
        );

        await expect(component.getByTestId('cot-card')).toHaveCount(0);
        await expect(component).toContainText('some text');
    });

    test('a file post whose attachment was swapped out falls back', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                source='file'
                fileId='abcdefghijklmnopqrstuvwxyz'
                fileIds={['zyxwvutsrqponmlkjihgfedcba']}
                message='see attached'
            />,
        );

        await expect(component.getByTestId('cot-card')).toHaveCount(0);
        await expect(component).toContainText('see attached');
    });
});

/*
 * The file case has an empty message by construction, and a post whose type a
 * plugin owns has already lost Mattermost's attachment list. A fallback that
 * rendered post.message alone would therefore be a permanently blank post.
 */
test.describe('the fallback is never blank', () => {
    test('an empty message with attachments still offers them', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                uid={null}
                message=''
                fileIds={['abcdefghijklmnopqrstuvwxyz']}
            />,
        );

        await expect(component.getByTestId('cot-card')).toHaveCount(0);
        await expect(component.getByRole('link', {name: 'file'})).toHaveAttribute(
            'href', '/api/v4/files/abcdefghijklmnopqrstuvwxyz',
        );
    });

    test('an empty message with nothing at all still says something', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                uid={null}
                message=''
            />,
        );

        await expect(component).toContainText('no readable content');
    });
});

test('a file post always offers its source', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            source='file'
            fileId='abcdefghijklmnopqrstuvwxyz'
            fileName='event.cot'
            fileIds={['abcdefghijklmnopqrstuvwxyz']}
        />,
    );

    await expect(component.getByRole('link', {name: 'Download event.cot'})).toHaveAttribute(
        'href', '/api/v4/files/abcdefghijklmnopqrstuvwxyz',
    );
});

// The source is the panel's, not the card's: the card names the event and the
// panel is where a reader goes to check it against the XML.
test('the card does not carry the source', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness src='<event uid="ANDROID-1"/>'/>,
    );

    await expect(component.getByTestId('cot-card')).toBeVisible();
    await expect(component.getByText('Show XML')).toHaveCount(0);
    await expect(component).not.toContainText('<event uid="ANDROID-1"/>');
});

test('an accuracy the event never stated is not invented', async ({mount}) => {
    const component = await mount(<CotPostBodyHarness event={{ce: ''}}/>);

    await expect(component).toContainText('Not stated');
});

test('a stated accuracy is shown', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{ce: '45.3 m', le: '99.5 m'}}/>,
    );

    await expect(component).toContainText('45.3 m circular');
    await expect(component).toContainText('99.5 m vertical');
});

test('a position note is shown beside the reading it explains', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            event={{
                lat: '0.000000',
                lon: '0.000000',
                position_note: 'The event reports 0,0, which is the Cursor on Target value for a position that was never set.',
            }}
        />,
    );

    await expect(component).toContainText('0.000000, 0.000000');
    await expect(component).toContainText('never set');
    await expect(component.getByRole('link')).toHaveCount(0);
});

test('a linkable position opens the coordinate tools', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            event={{lat: '34.0561', lon: '-118.2500', format: 'dd', value: '34.0561,-118.2500'}}
        />,
    );

    const link = component.getByRole('link', {name: '34.0561, -118.2500'});
    await expect(link).toHaveAttribute('href', /\/decorate\/location\?f=dd&v=34\.0561%2C-118\.2500$/);
});

test('an unrecognized type says so rather than guessing', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{type_label: '', cot_type: 'z-q-q'}}/>,
    );

    await expect(component).toContainText('Unrecognized event type');
    await expect(component).toContainText('(z-q-q)');
});

// The raw code sits beside the label so a reader can always see what the label
// was derived from, and what it did not cover.
test('the raw type is shown in parentheses beside the label', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{type_label: 'Friendly Ground Combat Unit', cot_type: 'a-f-G-U-C'}}/>,
    );

    const card = component.getByTestId('cot-card');
    await expect(card).toContainText('Friendly Ground Combat Unit');
    await expect(card).toContainText('(a-f-G-U-C)');
});

test('the rows are a labelled description list', async ({mount}) => {
    const component = await mount(<CotPostBodyHarness/>);

    await expect(
        component.getByRole('group', {name: /^Details of the Cursor on Target event /}),
    ).toBeVisible();
});

// A file id reaches an href, so the function has to be safe on its own terms
// rather than on an invariant of the host that populates file_ids.
test('a file id that is not one is not turned into a link', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            source='file'
            fileId='../../../admin/console'
            fileName='event.cot'
            fileIds={['../../../admin/console']}
        />,
    );

    await expect(component.getByRole('link', {name: /Download/})).toHaveCount(0);
});

// A prototype key is not an affiliation. The dot is decorative and redundant
// with the type label, but a stringified function reaching a style declaration
// is still a rendering nobody chose.
test('an affiliation naming a prototype key draws no dot', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{affiliation: 'constructor'}}/>,
    );

    await expect(component.getByTestId('cot-card')).toBeVisible();
    const dots = component.locator('span[aria-hidden="true"]');
    await expect(dots).toHaveCount(0);
});

test('a time the server could spell opens the date-time group tools', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness
            event={{
                time: '231143ZAUG26',
                time_q: 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z',
            }}
        />,
    );

    const link = component.getByRole('link', {name: '231143ZAUG26'});
    await expect(link).toHaveAttribute('href', /\/decorate\/dtg\?a=&dtg=231143ZAUG26&t=\d+&z=Z$/);
});

// An instant outside the grammar's century carries no query, and the reading is
// still shown rather than dropped.
test('a time the server could not spell is shown without a link', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{time: '231143ZAUG99', time_q: ''}}/>,
    );

    await expect(component).toContainText('231143ZAUG99');
    await expect(component.getByRole('link', {name: '231143ZAUG99'})).toHaveCount(0);
});

test('the card says what kind of thing it is before saying which one', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{callsign: 'DELTA1'}}/>,
    );

    const card = component.getByTestId('cot-card');
    await expect(card).toContainText('Cursor on Target (CoT):');

    // The kind reads before the event, not after it.
    const text = (await card.textContent()) ?? '';
    expect(text.indexOf('Cursor on Target (CoT):')).toBeLessThan(text.indexOf('DELTA1'));
});

test('the kind is stated even for an event with nothing else to show', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{callsign: '', typeLabel: '', type_label: ''}}/>,
    );

    await expect(component.getByTestId('cot-card')).toContainText('Cursor on Target (CoT):');
});

/*
 * registerLinkTooltipComponent only ever offers a link Mattermost's own markdown
 * renderer drew, and this card draws its own anchors, so the framework's hover
 * never fires here. These pin the card's own.
 */
test.describe('the hover the card carries itself', () => {
    test('a time shows its countdown on pointer', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{
                    time: '231143ZAUG26',
                    time_q: 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z',
                }}
            />,
        );

        const link = component.getByRole('link', {name: '231143ZAUG26'});
        await expect(component.locator('.tactical-fusion-hover-card')).toHaveCount(0);

        await link.hover();

        // The countdown itself, not just the chrome around it: the card is
        // hidden when it renders nothing, and that rule lives in a stylesheet
        // this harness does not install.
        const card = component.locator('.tactical-fusion-hover-card');
        await expect(card).toBeVisible();
        await expect(card).toContainText(/\d/);
        expect((await card.textContent())?.trim().length).toBeGreaterThan(0);
    });

    test('and takes it away again', async ({mount, page}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{
                    time: '231143ZAUG26',
                    time_q: 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z',
                }}
            />,
        );

        await component.getByRole('link', {name: '231143ZAUG26'}).hover();
        await expect(component.locator('.tactical-fusion-hover-card')).toBeVisible();

        await page.mouse.move(0, 0);
        await expect(component.locator('.tactical-fusion-hover-card')).toHaveCount(0);
    });

    test('is reachable by keyboard, not only by pointer', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{
                    time: '231143ZAUG26',
                    time_q: 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z',
                }}
            />,
        );

        await component.getByRole('link', {name: '231143ZAUG26'}).focus();
        await expect(component.locator('.tactical-fusion-hover-card')).toBeVisible();
    });

    test('a time the server could not spell has no link and no card', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness event={{time: '231143ZAUG99', time_q: ''}}/>,
        );

        await component.getByText('231143ZAUG99').hover();
        await expect(component.locator('.tactical-fusion-hover-card')).toHaveCount(0);
    });
});

/*
 * A block of several events. One post stays one post: the card names each track
 * and links its position, and the panel behind "Open details" carries the rest.
 */
test.describe('a post carrying several events', () => {
    test('names every one of them, and says how many', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                events={[
                    {callsign: 'DELTA1'},
                    {callsign: 'BRAVO2', type_label: 'Friendly Infantry Unit'},
                    {callsign: 'TRACK9', type_label: 'Hostile Armor Unit'},
                ]}
            />,
        );

        const card = component.getByTestId('cot-card');
        await expect(card).toContainText('3 events');

        await Promise.all(
            ['DELTA1', 'BRAVO2', 'TRACK9'].map((callsign) => expect(card).toContainText(callsign)),
        );
        await expect(card).toContainText('Hostile Armor Unit');
    });

    test('lists them rather than repeating a full card each', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness events={[{callsign: 'DELTA1'}, {callsign: 'BRAVO2'}]}/>,
        );

        await expect(
            component.getByRole('list', {name: 'The events in this post'}),
        ).toBeVisible();

        // The single-event detail table is what a block deliberately does not show.
        await expect(
            component.getByRole('group', {name: /^Details of the Cursor on Target event /}),
        ).toHaveCount(0);
    });

    test('a single event still gets the full detail', async ({mount}) => {
        const component = await mount(<CotPostBodyHarness event={{callsign: 'DELTA1'}}/>);

        await expect(
            component.getByRole('group', {name: /^Details of the Cursor on Target event /}),
        ).toBeVisible();
        await expect(component.getByTestId('cot-card')).not.toContainText('1 events');
    });

    test('every position in the block is linked', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                events={[
                    {lat: '34.0561', lon: '-118.2500', format: 'dd', value: '34.0561,-118.2500'},
                    {lat: '35.0000', lon: '-119.0000', format: 'dd', value: '35.0000,-119.0000'},
                ]}
            />,
        );

        await expect(component.getByRole('link', {name: '34.0561, -118.2500'})).toBeVisible();
        await expect(component.getByRole('link', {name: '35.0000, -119.0000'})).toBeVisible();
    });
});

test('an event that names who sent it says so', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{parent: 'ALPHA', related: 'ANDROID-9'}}/>,
    );

    await expect(component.getByTestId('cot-card')).toContainText('ALPHA');
    await expect(component.getByTestId('cot-card')).toContainText('ANDROID-9');
});

test.describe('the class picks one line, and nothing else moves', () => {
    test('a chat event names the stated sender as a reading, not as a message', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{
                    class: 'chat',
                    chat_sender: 'ALPHA-1',
                    chat_room: 'Operations',
                    remarks: 'Moving to checkpoint Bravo.',
                }}
            />,
        );

        await expect(component.getByTestId('cot-card')).toContainText('Event states sender:');
        await expect(component.getByTestId('cot-card')).toContainText('ALPHA-1 to Operations');
        await expect(component.getByTestId('cot-card')).toContainText('Moving to checkpoint Bravo.');
    });

    // senderCallsign is author-chosen and has no relationship to the Mattermost
    // identity that posted. Anything shaped like a quoted message from a named
    // person would borrow Mattermost's own attribution.
    test('a chat card draws no avatar and no blockquote', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{class: 'chat', chat_sender: 'ADMIN', remarks: 'trust me'}}
            />,
        );

        await expect(component.locator('img')).toHaveCount(0);
        await expect(component.locator('blockquote')).toHaveCount(0);
    });

    // The message is above the rows, so drawing Remarks as well would put the
    // same string on the card twice.
    test('a chat card renders the message exactly once', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{class: 'chat', chat_sender: 'ALPHA-1', remarks: 'checkpoint bravo'}}
            />,
        );

        await expect(component.getByText('checkpoint bravo')).toHaveCount(1);
        await expect(component.getByTestId('cot-card')).not.toContainText('Remarks');
    });

    // A b-t-f with no __chat element has no sender and no room, so the chat
    // layout would be empty chrome.
    test('a chat class with no chat block falls back to the ordinary card', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness event={{class: 'chat', remarks: 'still a remark'}}/>,
        );

        await expect(component.getByTestId('cot-card')).not.toContainText('Event states sender');
        await expect(component.getByTestId('cot-card')).toContainText('Remarks');
    });

    // A stated zero and an unstated field are different facts.
    test('a medevac card keeps the stated zeros', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{
                    class: 'medevac',
                    medevac_urgent: '0',
                    medevac_priority: '1',
                    medevac_routine: '0',
                }}
            />,
        );

        await expect(component.getByTestId('cot-card')).toContainText('Patients stated:');
        await expect(component.getByTestId('cot-card')).toContainText('0 urgent · 1 priority · 0 routine');
    });

    test('a sensor card carries its field of view', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{class: 'sensor', sensor_fov: '18°', sensor_azimuth: '185°'}}
            />,
        );

        await expect(component.getByTestId('cot-card')).toContainText('field of view 18°');
    });

    // The address is author-controlled and stays off the card entirely.
    test('a video card says a stream exists and never where it is', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                event={{class: 'video', video_url: 'rtsp://attacker.example/steal'}}
            />,
        );

        await expect(component.getByTestId('cot-card')).toContainText('a stream is associated');
        await expect(component.getByTestId('cot-card')).not.toContainText('attacker.example');
    });

    // A class this build does not know falls to the default rather than to a
    // blank card, which is what lets the server add one before the bundle ships.
    test('a class this build does not know renders the ordinary card', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness event={{class: 'teleport', remarks: 'ordinary'}}/>,
        );

        await expect(component.getByTestId('cot-card')).toContainText('Remarks');
        await expect(component.getByTestId('cot-card')).toContainText('ordinary');
    });

    // The card lists rather than details a batch, so there is no one class for
    // a post carrying a chat, a sensor and three position reports.
    test('a multi-event post carries no class summary', async ({mount}) => {
        const component = await mount(
            <CotPostBodyHarness
                events={[
                    {class: 'chat', chat_sender: 'ALPHA-1', chat_room: 'Ops'},
                    {cot_type: 'a-f-G-U-C'},
                ]}
            />,
        );

        await expect(component.getByTestId('cot-card')).not.toContainText('Event states sender');
    });
});

// The degraded rung also drops `class`, so the summary line vanishes. A reader
// who never opens the panel would otherwise see only the absence.
test('a degraded card says so where the reader is', async ({mount}) => {
    const component = await mount(
        <CotPostBodyHarness event={{detail_dropped: 'stated'}}/>,
    );

    await expect(component.getByTestId('cot-card')).toContainText('too large to store');
});

// The class picks the layout from the type code, so it can name a block the
// event never carried. `ClassSummary` says it degrades to nothing in that case
// and `docs/design/cot.md` states the rule, but only chat was ever tested for
// it: a summary label with nothing after it would have gone unnoticed on the
// other three.
test.describe('a class whose block is absent degrades to the ordinary card', () => {
    const cases = [
        {name: 'medevac', label: 'Patients stated'},
        {name: 'sensor', label: 'Sensor:'},
        {name: 'video', label: 'Video:'},
    ];

    for (const {name, label} of cases) {
        test(`${name} with no ${name} block`, async ({mount}) => {
            const component = await mount(
                <CotPostBodyHarness event={{class: name, remarks: 'still a remark'}}/>,
            );

            const card = component.getByTestId('cot-card');
            await expect(card).not.toContainText(label);
            await expect(card).toContainText('Remarks');
            await expect(card).toContainText('still a remark');
        });
    }
});
